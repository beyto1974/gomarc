package marc

import (
	"fmt"
	"strconv"
	"strings"
)

// Subfield is a code/value pair within a data Field. Ported from pymarc.field.Subfield.
type Subfield struct {
	Code  string
	Value string
}

// Indicators are the two indicator characters of a non-control Field.
// Ported from pymarc.field.Indicators.
type Indicators struct {
	First  string
	Second string
}

// Field represents a single MARC field: either a control field (tag < "010",
// carrying raw Data) or a data field (carrying Indicators and Subfields).
// Ported from pymarc/field.py.
type Field struct {
	Tag          string
	ControlField bool
	Data         string
	Indicators   Indicators
	Subfields    []Subfield
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// normalizeTag zero-pads all-digit tags to 3 characters, matching pymarc's
// Field.__init__ tag normalization; non-digit tags pass through unchanged.
func normalizeTag(tag string) string {
	if !isAllDigits(tag) {
		return tag
	}
	n, err := strconv.Atoi(tag)
	if err != nil {
		return tag
	}
	return fmt.Sprintf("%03d", n)
}

func isControlTag(tag string) bool {
	return isAllDigits(tag) && tag < "010"
}

// NewField builds a Field, replicating pymarc's Field.__init__ branching: tags
// normalized to 3-digit zero-padded form when numeric; tags below "010" become
// control fields carrying data (indicators/subfields are ignored for those, as
// in pymarc); all other tags become data fields carrying indicators/subfields.
func NewField(tag string, indicators Indicators, subfields []Subfield, data string) *Field {
	t := normalizeTag(tag)
	f := &Field{Tag: t}
	if isControlTag(t) {
		f.ControlField = true
		f.Data = data
		return f
	}
	f.Subfields = subfields
	f.Indicators = indicators
	return f
}

// NewControlField builds a control field (e.g. tag "001", "008") with raw data.
func NewControlField(tag, data string) *Field {
	return NewField(tag, Indicators{}, nil, data)
}

// NewDataField builds a data field with the given indicators and subfields.
// Pass " " for a blank indicator, matching pymarc's default (" ", " ").
func NewDataField(tag, ind1, ind2 string, subfields ...Subfield) *Field {
	return NewField(tag, Indicators{First: ind1, Second: ind2}, subfields, "")
}

// String returns the MARCMaker-style representation of the field.
func (f *Field) String() string {
	if f.ControlField {
		data := strings.ReplaceAll(f.Data, " ", "\\")
		return fmt.Sprintf("=%s  %s", f.Tag, data)
	}
	var ind strings.Builder
	for _, v := range []string{f.Indicators.First, f.Indicators.Second} {
		if v == " " || v == "\\" || v == "" {
			ind.WriteString("\\")
		} else {
			ind.WriteString(v)
		}
	}
	var subf strings.Builder
	for _, s := range f.Subfields {
		subf.WriteString("$" + s.Code + s.Value)
	}
	return fmt.Sprintf("=%s  %s%s", f.Tag, ind.String(), subf.String())
}

// Subfield returns the value of the first subfield with the given code.
// ok is false if the field is a control field or the code is absent.
func (f *Field) Subfield(code string) (value string, ok bool) {
	if f.ControlField {
		return "", false
	}
	for _, s := range f.Subfields {
		if s.Code == code {
			return s.Value, true
		}
	}
	return "", false
}

// Contains reports whether the field has a subfield with the given code.
func (f *Field) Contains(code string) bool {
	_, ok := f.Subfield(code)
	return ok
}

// SetSubfield sets the value of the single subfield with the given code.
// Returns an error if the field is a control field, no subfield has that
// code, or more than one subfield has that code.
func (f *Field) SetSubfield(code, value string) error {
	if f.ControlField {
		return fmt.Errorf("field is a control field")
	}
	matchIdx := -1
	for i, s := range f.Subfields {
		if s.Code == code {
			if matchIdx != -1 {
				return fmt.Errorf("more than one code %q", code)
			}
			matchIdx = i
		}
	}
	if matchIdx == -1 {
		return fmt.Errorf("no code %q", code)
	}
	f.Subfields[matchIdx].Value = value
	return nil
}

// Value returns the field's subfields (or Data for control fields) joined as a string.
func (f *Field) Value() string {
	if f.ControlField {
		return f.Data
	}
	parts := make([]string, len(f.Subfields))
	for i, s := range f.Subfields {
		parts[i] = strings.TrimSpace(s.Value)
	}
	return strings.Join(parts, " ")
}

// GetSubfields returns the values of all subfields matching any of the given codes,
// in field order.
func (f *Field) GetSubfields(codes ...string) []string {
	if f.ControlField || len(codes) == 0 {
		return nil
	}
	var out []string
	for _, s := range f.Subfields {
		for _, c := range codes {
			if s.Code == c {
				out = append(out, s.Value)
				break
			}
		}
	}
	return out
}

// AddSubfield appends a subfield to the end of the field. No-op on control fields.
func (f *Field) AddSubfield(code, value string) {
	if f.ControlField {
		return
	}
	f.Subfields = append(f.Subfields, Subfield{Code: code, Value: value})
}

// AddSubfieldAt inserts a subfield at pos, or appends if pos is out of range.
// No-op on control fields.
func (f *Field) AddSubfieldAt(code, value string, pos int) {
	if f.ControlField {
		return
	}
	sf := Subfield{Code: code, Value: value}
	if pos < 0 || pos > len(f.Subfields) {
		f.Subfields = append(f.Subfields, sf)
		return
	}
	f.Subfields = append(f.Subfields[:pos:pos], append([]Subfield{sf}, f.Subfields[pos:]...)...)
}

// DeleteSubfield removes and returns the value of the first subfield with the
// given code. ok is false if none was found (or the field is a control field).
func (f *Field) DeleteSubfield(code string) (value string, ok bool) {
	if f.ControlField {
		return "", false
	}
	for i, s := range f.Subfields {
		if s.Code == code {
			f.Subfields = append(f.Subfields[:i], f.Subfields[i+1:]...)
			return s.Value, true
		}
	}
	return "", false
}

// SubfieldsByCode groups subfield values by code, preserving field order within each code.
func (f *Field) SubfieldsByCode() map[string][]string {
	out := map[string][]string{}
	if f.ControlField {
		return out
	}
	for _, s := range f.Subfields {
		out[s.Code] = append(out[s.Code], s.Value)
	}
	return out
}

// IsControlField reports whether the field is a control field. Prefer the
// ControlField field directly; kept for parity with pymarc's is_control_field().
func (f *Field) IsControlField() bool {
	return f.ControlField
}

// LinkageOccurrenceNum returns the occurrence number portion of subfield 6
// (e.g. "01" from "880-01"), or ok=false if subfield 6 is absent.
func (f *Field) LinkageOccurrenceNum() (string, bool) {
	ocn, ok := f.Subfield("6")
	if !ok || ocn == "" {
		return "", false
	}
	rest, _, _ := strings.Cut(ocn, "-")
	_ = rest
	after, found := strings.CutPrefix(ocn, rest+"-")
	if !found {
		return "", false
	}
	num, _, _ := strings.Cut(after, "/")
	return num, true
}

// AsMarc encodes the field into MARC transmission-format bytes. Only "utf-8"
// encoding is currently supported.
func (f *Field) AsMarc(encoding string) ([]byte, error) {
	if encoding != "utf-8" {
		return nil, fmt.Errorf("marc: unsupported encoding %q", encoding)
	}
	if f.ControlField {
		return []byte(f.Data + string(rune(EndOfField))), nil
	}
	var b strings.Builder
	b.WriteString(f.Indicators.First)
	b.WriteString(f.Indicators.Second)
	for _, s := range f.Subfields {
		b.WriteByte(SubfieldIndicator)
		b.WriteString(s.Code)
		b.WriteString(s.Value)
	}
	b.WriteByte(EndOfField)
	return []byte(b.String()), nil
}

// IsSubjectField reports whether the field's tag starts with "6".
func (f *Field) IsSubjectField() bool {
	return strings.HasPrefix(f.Tag, "6")
}

// FormatField returns the field's subfields as a pretty string: subject
// fields join v/x/y/z subfields with " -- ", and subfield 6 is skipped.
func (f *Field) FormatField() string {
	if f.ControlField {
		return f.Data
	}
	isSubject := f.IsSubjectField()
	var b strings.Builder
	for _, s := range f.Subfields {
		if s.Code == "6" {
			continue
		}
		if isSubject && (s.Code == "v" || s.Code == "x" || s.Code == "y" || s.Code == "z") {
			b.WriteString(" -- " + s.Value)
		} else {
			b.WriteString(" " + s.Value)
		}
	}
	return strings.TrimSpace(b.String())
}

// Indicator1 returns the first indicator, or "" for control fields.
func (f *Field) Indicator1() string {
	if f.ControlField {
		return ""
	}
	return f.Indicators.First
}

// SetIndicator1 sets the first indicator. No-op on control fields.
func (f *Field) SetIndicator1(value string) {
	if f.ControlField {
		return
	}
	f.Indicators.First = value
}

// Indicator2 returns the second indicator, or "" for control fields.
func (f *Field) Indicator2() string {
	if f.ControlField {
		return ""
	}
	return f.Indicators.Second
}

// SetIndicator2 sets the second indicator. No-op on control fields.
func (f *Field) SetIndicator2(value string) {
	if f.ControlField {
		return
	}
	f.Indicators.Second = value
}
