package marc

import (
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

// sanitizeLenientJSON escapes raw control bytes (e.g. literal newlines) found
// inside JSON string literals. Real-world MARC-in-JSON is sometimes produced
// with unescaped control characters in fixed-field data (pymarc's JSONReader
// reads it via json.load(..., strict=False), which permits this); Go's
// encoding/json has no equivalent lenient mode, so we pre-escape instead.
func sanitizeLenientJSON(data []byte) []byte {
	var out bytes.Buffer
	inString := false
	escaped := false
	for _, b := range data {
		if !inString {
			out.WriteByte(b)
			if b == '"' {
				inString = true
			}
			continue
		}
		if escaped {
			out.WriteByte(b)
			escaped = false
			continue
		}
		switch b {
		case '\\':
			out.WriteByte(b)
			escaped = true
		case '"':
			out.WriteByte(b)
			inString = false
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if b < 0x20 {
				fmt.Fprintf(&out, `\u%04x`, b)
			} else {
				out.WriteByte(b)
			}
		}
	}
	return out.Bytes()
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
