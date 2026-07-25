package marc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// MARC-in-JSON (de)serialization. Ported from pymarc/marcjson.py.
// See: https://web.archive.org/web/20151112001548/http://dilettantes.code4lib.org/blog/2010/09/a-proposal-to-serialize-marc-in-json

type jsonSubfield map[string]string

type jsonField map[string]json.RawMessage

type jsonRecord struct {
	Leader string      `json:"leader"`
	Fields []jsonField `json:"fields"`
}

type jsonDataField struct {
	Ind1      string         `json:"ind1"`
	Ind2      string         `json:"ind2"`
	Subfields []jsonSubfield `json:"subfields"`
}

// lenientJSONFilter streams data from src, escaping raw control bytes (e.g.
// literal newlines) found inside JSON string literals. Real-world
// MARC-in-JSON is sometimes produced with unescaped control characters in
// fixed-field data (pymarc's JSONReader reads it via json.load(...,
// strict=False), which permits this); Go's encoding/json has no equivalent
// lenient mode, so this pre-escapes on the fly rather than requiring the
// whole document to be buffered first.
type lenientJSONFilter struct {
	src               io.Reader
	inString, escaped bool
	pending           []byte
	rbuf              []byte
	err               error
}

func (l *lenientJSONFilter) Read(p []byte) (int, error) {
	for len(l.pending) == 0 {
		if l.err != nil {
			return 0, l.err
		}
		if l.rbuf == nil {
			l.rbuf = make([]byte, 4096)
		}
		n, err := l.src.Read(l.rbuf)
		if n > 0 {
			l.pending = append(l.pending, l.transform(l.rbuf[:n])...)
		}
		if err != nil {
			l.err = err
		}
	}
	n := copy(p, l.pending)
	l.pending = l.pending[n:]
	return n, nil
}

func (l *lenientJSONFilter) transform(data []byte) []byte {
	out := make([]byte, 0, len(data))
	for _, b := range data {
		if !l.inString {
			out = append(out, b)
			if b == '"' {
				l.inString = true
			}
			continue
		}
		if l.escaped {
			out = append(out, b)
			l.escaped = false
			continue
		}
		switch b {
		case '\\':
			out = append(out, b)
			l.escaped = true
		case '"':
			out = append(out, b)
			l.inString = false
		case '\n':
			out = append(out, '\\', 'n')
		case '\r':
			out = append(out, '\\', 'r')
		case '\t':
			out = append(out, '\\', 't')
		default:
			if b < 0x20 {
				out = append(out, []byte(fmt.Sprintf(`\u%04x`, b))...)
			} else {
				out = append(out, b)
			}
		}
	}
	return out
}

// sanitizeLenientJSON is the non-streaming form of lenientJSONFilter, used by
// ParseJSON which already requires the whole document in memory.
func sanitizeLenientJSON(data []byte) []byte {
	out, err := io.ReadAll(&lenientJSONFilter{src: bytes.NewReader(data)})
	if err != nil {
		// bytes.Reader never returns an error other than io.EOF, which
		// io.ReadAll does not surface as an error.
		panic(err)
	}
	return out
}

// ParseJSON parses MARC-in-JSON data, which may be a single record object or
// an array of record objects, into Records. Matches pymarc's parse_json_to_array.
func ParseJSON(data []byte) ([]*Record, error) {
	trimmed := bytes.TrimSpace(sanitizeLenientJSON(data))
	var raws []json.RawMessage
	if len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &raws); err != nil {
			return nil, fmt.Errorf("marc: parsing MARC-in-JSON array: %w", err)
		}
	} else {
		raws = []json.RawMessage{trimmed}
	}

	records := make([]*Record, 0, len(raws))
	for _, raw := range raws {
		var jr jsonRecord
		if err := json.Unmarshal(raw, &jr); err != nil {
			return nil, fmt.Errorf("marc: parsing MARC-in-JSON record: %w", err)
		}
		rec, err := jsonRecordToRecord(jr)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}

func jsonRecordToRecord(jr jsonRecord) (*Record, error) {
	leader, err := NewLeader(jr.Leader)
	if err != nil {
		return nil, err
	}
	r := &Record{Leader: leader, ToUnicode: true}

	for _, jf := range jr.Fields {
		for tag, raw := range jf {
			var controlData string
			if err := json.Unmarshal(raw, &controlData); err == nil {
				r.AddField(NewControlField(tag, controlData))
				continue
			}
			var df jsonDataField
			if err := json.Unmarshal(raw, &df); err != nil {
				return nil, fmt.Errorf("marc: field %q: %w", tag, err)
			}
			subs := make([]Subfield, 0, len(df.Subfields))
			for _, sf := range df.Subfields {
				for code, value := range sf {
					subs = append(subs, Subfield{Code: code, Value: value})
				}
			}
			r.AddField(NewDataField(tag, df.Ind1, df.Ind2, subs...))
		}
	}
	return r, nil
}

// JSONReader iterates over records in a MARC-in-JSON document, decoding one
// record at a time rather than loading the whole document, whether it is a
// top-level array or a single bare record object. Unlike ParseJSON, it does
// not require the input to fit in memory at once.
type JSONReader struct {
	br      *bufio.Reader
	dec     *json.Decoder
	isArray bool
	done    bool
}

// NewJSONReader builds a JSONReader over r.
func NewJSONReader(r io.Reader) *JSONReader {
	return &JSONReader{br: bufio.NewReader(&lenientJSONFilter{src: r})}
}

// init peeks the first non-whitespace byte to determine whether the document
// is a top-level array (in which case it consumes the opening '[' so the
// decoder tracks array context for Decoder.More) or a single bare record.
func (jr *JSONReader) init() error {
	for {
		b, err := jr.br.ReadByte()
		if err != nil {
			return err
		}
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		}
		if err := jr.br.UnreadByte(); err != nil {
			return err
		}
		jr.dec = json.NewDecoder(jr.br)
		if b == '[' {
			jr.isArray = true
			if _, err := jr.dec.Token(); err != nil {
				return err
			}
		}
		return nil
	}
}

// Next decodes and returns the next record, or (nil, io.EOF) at end of document.
func (jr *JSONReader) Next() (*Record, error) {
	if jr.done {
		return nil, io.EOF
	}
	if jr.dec == nil {
		if err := jr.init(); err != nil {
			jr.done = true
			return nil, err
		}
	}

	if jr.isArray {
		if !jr.dec.More() {
			jr.done = true
			if _, err := jr.dec.Token(); err != nil { // consume closing ']'
				return nil, err
			}
			return nil, io.EOF
		}
	} else {
		jr.done = true
	}

	var jr2 jsonRecord
	if err := jr.dec.Decode(&jr2); err != nil {
		jr.done = true
		return nil, fmt.Errorf("marc: parsing MARC-in-JSON record: %w", err)
	}
	return jsonRecordToRecord(jr2)
}

// JSONWriter writes records as a MARC-in-JSON array. Close must be called to
// emit the closing bracket. Ported from pymarc.writer.JSONWriter.
type JSONWriter struct {
	dst        io.Writer
	writeCount int
}

// NewJSONWriter builds a JSONWriter, writing the opening "[".
func NewJSONWriter(w io.Writer) (*JSONWriter, error) {
	if _, err := io.WriteString(w, "["); err != nil {
		return nil, err
	}
	return &JSONWriter{dst: w}, nil
}

// Write serializes and writes a single record.
func (jw *JSONWriter) Write(r *Record) error {
	if jw.writeCount > 0 {
		if _, err := io.WriteString(jw.dst, ","); err != nil {
			return err
		}
	}
	b, err := json.Marshal(r.AsDict())
	if err != nil {
		return err
	}
	if _, err := jw.dst.Write(b); err != nil {
		return err
	}
	jw.writeCount++
	return nil
}

// Close writes the closing "]". The writer must not be used afterward.
func (jw *JSONWriter) Close() error {
	_, err := io.WriteString(jw.dst, "]")
	return err
}
