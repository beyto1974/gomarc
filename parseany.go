package marc

import (
	"bytes"
	"fmt"
)

// ParseAny parses a single MARC21 record from data, auto-detecting the
// format: ISO 2709 (transmission format), MARCXML, or MARC-in-JSON. If the
// input contains more than one record, only the first is returned.
func ParseAny(data []byte) (*Record, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("marc: empty input")
	}

	switch trimmed[0] {
	case '<':
		recs, err := ParseXML(bytes.NewReader(trimmed))
		if err != nil {
			return nil, err
		}
		if len(recs) == 0 {
			return nil, fmt.Errorf("marc: no records found in XML input")
		}
		return recs[0], nil
	case '{', '[':
		recs, err := ParseJSON(trimmed)
		if err != nil {
			return nil, err
		}
		if len(recs) == 0 {
			return nil, fmt.Errorf("marc: no records found in JSON input")
		}
		return recs[0], nil
	default:
		rdr := NewReaderFromBytes(trimmed)
		rec, err := rdr.Next()
		if err != nil {
			return nil, err
		}
		return rec, nil
	}
}
