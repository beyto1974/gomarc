package marc

import (
	"encoding/xml"
	"io"
)

// MARCXML (de)serialization. Ported from pymarc/marcxml.py.
const marcXMLNamespace = "http://www.loc.gov/MARC21/slim"

// xmlFieldEntry preserves control/data field order within a <record>, which a
// naive struct-tag decode (separate slices per element name) would lose.
type xmlFieldEntry struct {
	control      bool
	controlfield xmlControlfield
	datafield    xmlDatafield
}

type xmlControlfield struct {
	Tag   string `xml:"tag,attr"`
	Value string `xml:",chardata"`
}

type xmlSubfield struct {
	Code  string `xml:"code,attr"`
	Value string `xml:",chardata"`
}

type xmlDatafield struct {
	Tag       string        `xml:"tag,attr"`
	Ind1      string        `xml:"ind1,attr"`
	Ind2      string        `xml:"ind2,attr"`
	Subfields []xmlSubfield `xml:"subfield"`
}

type xmlRecordElem struct {
	Leader string
	Fields []xmlFieldEntry
}

// UnmarshalXML walks the <record> subtree token by token so that
// controlfield/datafield order is preserved, matching MARCXML's field order.
func (rec *xmlRecordElem) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "leader":
				var s string
				if err := d.DecodeElement(&s, &t); err != nil {
					return err
				}
				rec.Leader = s
			case "controlfield":
				var cf xmlControlfield
				if err := d.DecodeElement(&cf, &t); err != nil {
					return err
				}
				rec.Fields = append(rec.Fields, xmlFieldEntry{control: true, controlfield: cf})
			case "datafield":
				var df xmlDatafield
				if err := d.DecodeElement(&df, &t); err != nil {
					return err
				}
				rec.Fields = append(rec.Fields, xmlFieldEntry{control: false, datafield: df})
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

func (rec *xmlRecordElem) toRecord() (*Record, error) {
	leader, err := NewLeader(rec.Leader)
	if err != nil {
		return nil, err
	}
	r := &Record{Leader: leader, ToUnicode: true}
	for _, fe := range rec.Fields {
		if fe.control {
			r.AddField(NewControlField(fe.controlfield.Tag, fe.controlfield.Value))
			continue
		}
		ind1, ind2 := fe.datafield.Ind1, fe.datafield.Ind2
		if ind1 == "" {
			ind1 = " "
		}
		if ind2 == "" {
			ind2 = " "
		}
		subs := make([]Subfield, len(fe.datafield.Subfields))
		for i, s := range fe.datafield.Subfields {
			subs[i] = Subfield{Code: s.Code, Value: s.Value}
		}
		r.AddField(NewDataField(fe.datafield.Tag, ind1, ind2, subs...))
	}
	return r, nil
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
		var elem xmlRecordElem
		if err := xr.dec.DecodeElement(&elem, &start); err != nil {
			return nil, err
		}
		return elem.toRecord()
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
