package marc

import (
	"encoding/xml"
	"io"
	"strings"
)

// MARCXML (de)serialization. Ported from pymarc/marcxml.py.
const marcXMLNamespace = "http://www.loc.gov/MARC21/slim"

// decodeRecord walks the <record> subtree token by token, building a Record directly
// without reflection or intermediate decoding structs for high performance.
func decodeRecord(d *xml.Decoder, start xml.StartElement) (*Record, error) {
	rec := &Record{ToUnicode: true}
	var leaderStr string
	hasLeader := false

	for {
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "leader":
				val, err := readCharData(d, "leader")
				if err != nil {
					return nil, err
				}
				leaderStr = val
				hasLeader = true
			case "controlfield":
				var tag string
				for _, attr := range t.Attr {
					if attr.Name.Local == "tag" {
						tag = attr.Value
						break
					}
				}
				val, err := readCharData(d, "controlfield")
				if err != nil {
					return nil, err
				}
				rec.AddField(NewControlField(tag, val))
			case "datafield":
				var tag, ind1, ind2 string
				ind1, ind2 = " ", " "
				for _, attr := range t.Attr {
					switch attr.Name.Local {
					case "tag":
						tag = attr.Value
					case "ind1":
						if attr.Value != "" {
							ind1 = attr.Value
						}
					case "ind2":
						if attr.Value != "" {
							ind2 = attr.Value
						}
					}
				}
				f := NewDataField(tag, ind1, ind2)
				if err := decodeSubfields(d, f); err != nil {
					return nil, err
				}
				rec.AddField(f)
			default:
				if err := d.Skip(); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				if !hasLeader {
					leaderStr = defaultLeaderInput()
				}
				ldr, err := NewLeader(leaderStr)
				if err != nil {
					return nil, err
				}
				rec.Leader = ldr
				return rec, nil
			}
		}
	}
}

func readCharData(d *xml.Decoder, elemName string) (string, error) {
	var buf strings.Builder
	depth := 0
	for {
		tok, err := d.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.CharData:
			if depth == 0 {
				buf.Write(t)
			}
		case xml.StartElement:
			depth++
			if err := d.Skip(); err != nil {
				return "", err
			}
			depth--
		case xml.EndElement:
			if t.Name.Local == elemName && depth == 0 {
				return buf.String(), nil
			}
		}
	}
}

func decodeSubfields(d *xml.Decoder, f *Field) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "subfield" {
				var code string
				for _, attr := range t.Attr {
					if attr.Name.Local == "code" {
						code = attr.Value
						break
					}
				}
				val, err := readCharData(d, "subfield")
				if err != nil {
					return err
				}
				f.AddSubfield(code, val)
			} else {
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name.Local == "datafield" {
				return nil
			}
		}
	}
}

// XMLReader iterates over <record> elements in a MARCXML collection (or a
// single bare <record>), decoding one record at a time rather than loading
// the whole document. Ported from pymarc.marcxml.XmlHandler/parse_xml.
type XMLReader struct {
	dec *xml.Decoder
}

// NewXMLReader builds an XMLReader over r.
func NewXMLReader(r io.Reader) *XMLReader {
	return &XMLReader{dec: xml.NewDecoder(r)}
}

// Next decodes and returns the next <record>, or (nil, io.EOF) at end of document.
func (xr *XMLReader) Next() (*Record, error) {
	for {
		tok, err := xr.dec.Token()
		if err == io.EOF {
			return nil, io.EOF
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "record" {
			continue
		}
		return decodeRecord(xr.dec, start)
	}
}

// ParseXML parses every <record> in r into Records.
func ParseXML(r io.Reader) ([]*Record, error) {
	xr := NewXMLReader(r)
	var records []*Record
	for {
		rec, err := xr.Next()
		if err == io.EOF {
			return records, nil
		}
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
}

// XMLWriter writes records as a MARCXML <collection>. Close must be called to
// emit the closing tag. Ported from pymarc.writer.XMLWriter.
type XMLWriter struct {
	dst io.Writer
}

// NewXMLWriter builds an XMLWriter, writing the XML declaration and opening
// <collection> tag.
func NewXMLWriter(w io.Writer) (*XMLWriter, error) {
	if _, err := io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`+
		`<collection xmlns="`+marcXMLNamespace+`">`); err != nil {
		return nil, err
	}
	return &XMLWriter{dst: w}, nil
}

// Write serializes and writes a single record as a <record> element.
func (xw *XMLWriter) Write(r *Record) error {
	enc := xml.NewEncoder(xw.dst)
	recordStart := xml.StartElement{Name: xml.Name{Local: "record"}}
	if err := enc.EncodeToken(recordStart); err != nil {
		return err
	}

	leaderStart := xml.StartElement{Name: xml.Name{Local: "leader"}}
	if err := enc.EncodeToken(leaderStart); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.CharData(r.Leader.String())); err != nil {
		return err
	}
	if err := enc.EncodeToken(leaderStart.End()); err != nil {
		return err
	}

	for _, f := range r.Fields {
		if f.ControlField {
			start := xml.StartElement{Name: xml.Name{Local: "controlfield"}, Attr: []xml.Attr{
				{Name: xml.Name{Local: "tag"}, Value: f.Tag},
			}}
			if err := enc.EncodeToken(start); err != nil {
				return err
			}
			if err := enc.EncodeToken(xml.CharData(f.Data)); err != nil {
				return err
			}
			if err := enc.EncodeToken(start.End()); err != nil {
				return err
			}
			continue
		}

		start := xml.StartElement{Name: xml.Name{Local: "datafield"}, Attr: []xml.Attr{
			{Name: xml.Name{Local: "tag"}, Value: f.Tag},
			{Name: xml.Name{Local: "ind1"}, Value: f.Indicator1()},
			{Name: xml.Name{Local: "ind2"}, Value: f.Indicator2()},
		}}
		if err := enc.EncodeToken(start); err != nil {
			return err
		}
		for _, s := range f.Subfields {
			subStart := xml.StartElement{Name: xml.Name{Local: "subfield"}, Attr: []xml.Attr{
				{Name: xml.Name{Local: "code"}, Value: s.Code},
			}}
			if err := enc.EncodeToken(subStart); err != nil {
				return err
			}
			if err := enc.EncodeToken(xml.CharData(s.Value)); err != nil {
				return err
			}
			if err := enc.EncodeToken(subStart.End()); err != nil {
				return err
			}
		}
		if err := enc.EncodeToken(start.End()); err != nil {
			return err
		}
	}

	if err := enc.EncodeToken(recordStart.End()); err != nil {
		return err
	}
	return enc.Flush()
}

// Close writes the closing </collection> tag. The writer must not be used afterward.
func (xw *XMLWriter) Close() error {
	_, err := io.WriteString(xw.dst, "</collection>")
	return err
}
