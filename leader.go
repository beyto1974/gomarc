package marc

import "fmt"

// Leader is the mutable 24-byte MARC record leader. Ported from pymarc/leader.py.
//
// See https://www.loc.gov/marc/bibliographic/bdleader.html for field meanings.
//
// Values are accessed either through named accessors (RecordStatus, BibliographicLevel,
// ...) or through raw position access (Byte, Slice) mirroring Python's leader[5] / leader[0:4].
type Leader struct {
	data string
}

// NewLeader builds a Leader from a 24-byte string.
func NewLeader(s string) (*Leader, error) {
	if len(s) != LeaderLen {
		return nil, ErrRecordLeaderInvalid
	}
	return &Leader{data: s}, nil
}

// String returns the raw 24-byte leader.
func (l *Leader) String() string {
	return l.data
}

// Byte returns the byte at position i (equivalent to Python's leader[i]).
func (l *Leader) Byte(i int) byte {
	return l.data[i]
}

// Slice returns the substring [start:end) (equivalent to Python's leader[start:end]).
func (l *Leader) Slice(start, end int) string {
	return l.data[start:end]
}

// replaceValues replaces the leader contents at position with value, matching
// pymarc's Leader._replace_values.
func (l *Leader) replaceValues(position int, value string) error {
	if position < 0 {
		return fmt.Errorf("position must be positive")
	}
	after := position + len(value)
	if after > LeaderLen {
		return fmt.Errorf("%w: %q is too long to be inserted at %d", ErrBadLeaderValue, value, position)
	}
	l.data = l.data[:position] + value + l.data[after:]
	return nil
}

// SetSlice sets the substring starting at position, matching Python's leader[start:] = value.
func (l *Leader) SetSlice(position int, value string) error {
	return l.replaceValues(position, value)
}

func fixedLenSetter(l *Leader, position, wantLen int, fieldName, value string) error {
	if len(value) != wantLen {
		return fmt.Errorf("%w: %s is %d char(s) field, got %q", ErrBadLeaderValue, fieldName, wantLen, value)
	}
	return l.replaceValues(position, value)
}

// RecordLength returns the record length (00-04).
func (l *Leader) RecordLength() string { return l.Slice(0, 5) }

// SetRecordLength sets the record length (00-04).
func (l *Leader) SetRecordLength(value string) error {
	return fixedLenSetter(l, 0, 5, "record length", value)
}

// RecordStatus returns the record status (05).
func (l *Leader) RecordStatus() byte { return l.Byte(5) }

// SetRecordStatus sets the record status (05).
func (l *Leader) SetRecordStatus(value string) error {
	return fixedLenSetter(l, 5, 1, "record status", value)
}

// TypeOfRecord returns the type of record (06).
func (l *Leader) TypeOfRecord() byte { return l.Byte(6) }

// SetTypeOfRecord sets the type of record (06).
func (l *Leader) SetTypeOfRecord(value string) error {
	return fixedLenSetter(l, 6, 1, "type of record", value)
}

// BibliographicLevel returns the bibliographic level (07).
func (l *Leader) BibliographicLevel() byte { return l.Byte(7) }

// SetBibliographicLevel sets the bibliographic level (07).
func (l *Leader) SetBibliographicLevel(value string) error {
	return fixedLenSetter(l, 7, 1, "bibliographic level", value)
}

// TypeOfControl returns the type of control (08).
func (l *Leader) TypeOfControl() byte { return l.Byte(8) }

// SetTypeOfControl sets the type of control (08).
func (l *Leader) SetTypeOfControl(value string) error {
	return fixedLenSetter(l, 8, 1, "type of control", value)
}

// CodingScheme returns the character coding scheme (09).
func (l *Leader) CodingScheme() byte { return l.Byte(9) }

// SetCodingScheme sets the character coding scheme (09).
func (l *Leader) SetCodingScheme(value string) error {
	return fixedLenSetter(l, 9, 1, "character coding scheme", value)
}

// IndicatorCount returns the indicator count (10).
func (l *Leader) IndicatorCount() byte { return l.Byte(10) }

// SetIndicatorCount sets the indicator count (10).
func (l *Leader) SetIndicatorCount(value string) error {
	return fixedLenSetter(l, 10, 1, "indicator count", value)
}

// SubfieldCodeCount returns the subfield code count (11).
func (l *Leader) SubfieldCodeCount() byte { return l.Byte(11) }

// SetSubfieldCodeCount sets the subfield code count (11).
func (l *Leader) SetSubfieldCodeCount(value string) error {
	return fixedLenSetter(l, 11, 1, "subfield code count", value)
}

// BaseAddress returns the base address of data (12-16).
func (l *Leader) BaseAddress() string { return l.Slice(12, 17) }

// SetBaseAddress sets the base address of data (12-16).
func (l *Leader) SetBaseAddress(value string) error {
	return fixedLenSetter(l, 12, 5, "base address of data", value)
}

// EncodingLevel returns the encoding level (17).
func (l *Leader) EncodingLevel() byte { return l.Byte(17) }

// SetEncodingLevel sets the encoding level (17).
func (l *Leader) SetEncodingLevel(value string) error {
	return fixedLenSetter(l, 17, 1, "encoding level", value)
}

// CatalogingForm returns the descriptive cataloging form (18).
func (l *Leader) CatalogingForm() byte { return l.Byte(18) }

// SetCatalogingForm sets the descriptive cataloging form (18).
func (l *Leader) SetCatalogingForm(value string) error {
	return fixedLenSetter(l, 18, 1, "descriptive cataloging form", value)
}

// MultipartResource returns the multipart resource record level (19).
func (l *Leader) MultipartResource() byte { return l.Byte(19) }

// SetMultipartResource sets the multipart resource record level (19).
func (l *Leader) SetMultipartResource(value string) error {
	return fixedLenSetter(l, 19, 1, "multipart resource record level", value)
}

// LengthOfFieldLength returns the length of the length-of-field portion (20).
func (l *Leader) LengthOfFieldLength() byte { return l.Byte(20) }

// SetLengthOfFieldLength sets the length of the length-of-field portion (20).
func (l *Leader) SetLengthOfFieldLength(value string) error {
	return fixedLenSetter(l, 20, 1, "length of the length-of-field portion", value)
}

// StartingCharacterPositionLength returns the length of the starting-character-position portion (21).
func (l *Leader) StartingCharacterPositionLength() byte { return l.Byte(21) }

// SetStartingCharacterPositionLength sets the length of the starting-character-position portion (21).
func (l *Leader) SetStartingCharacterPositionLength(value string) error {
	return fixedLenSetter(l, 21, 1, "length of the starting-character-position portion", value)
}

// ImplementationDefinedLength returns the length of the implementation-defined portion (22).
func (l *Leader) ImplementationDefinedLength() byte { return l.Byte(22) }

// SetImplementationDefinedLength sets the length of the implementation-defined portion (22).
func (l *Leader) SetImplementationDefinedLength(value string) error {
	return fixedLenSetter(l, 22, 1, "length of the implementation-defined portion", value)
}
