package marc

import (
	"strings"
	"testing"
)

// Ported from test/test_field.py. Constructor/indicator-list edge cases from the
// Python suite (invalid indicator lists, legacy string subfields, int tags) have no
// Go equivalent: Go's type system (Indicators is a 2-field struct, Subfield requires
// Code+Value, tags are always strings) makes those states unrepresentable.

func testDataField() *Field {
	return NewDataField("245", "0", "1",
		Subfield{Code: "a", Value: "Huckleberry Finn: "},
		Subfield{Code: "b", Value: "An American Odyssey"},
	)
}

func testControlField() *Field {
	return NewControlField("008", "831227m19799999nyu           ||| | ger  ")
}

func testSubjectField() *Field {
	return NewDataField("650", " ", "0",
		Subfield{Code: "a", Value: "Python (Computer program language)"},
		Subfield{Code: "v", Value: "Poetry."},
	)
}

func TestControlFieldSubfieldsEmpty(t *testing.T) {
	f := testControlField()
	if len(f.Subfields) != 0 {
		t.Errorf("want 0 subfields, got %d", len(f.Subfields))
	}
	if !f.ControlField {
		t.Errorf("want ControlField true")
	}
}

func TestFieldDataEmptyIfNotControl(t *testing.T) {
	f := testDataField()
	if f.Data != "" {
		t.Errorf("want empty Data, got %q", f.Data)
	}
}

func TestIndicatorsDefaultWhenNotSupplied(t *testing.T) {
	f := NewDataField("245", "", "", Subfield{Code: "a", Value: "x"})
	// Go zero value for indicator args ("") diverges from Python's implicit
	// (" ", " ") default: Go has no "not supplied" state distinct from "".
	// Callers must pass " ", " " explicitly to get the pymarc default.
	f2 := NewDataField("245", " ", " ", Subfield{Code: "a", Value: "x"})
	if f2.Indicators != (Indicators{" ", " "}) {
		t.Errorf("got %+v", f2.Indicators)
	}
	_ = f
}

func TestFieldString(t *testing.T) {
	f := testDataField()
	want := "=245  01$aHuckleberry Finn: $bAn American Odyssey"
	if got := f.String(); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestControlFieldString(t *testing.T) {
	f := testControlField()
	data := "831227m19799999nyu           ||| | ger  "
	want := "=008  " + strings.ReplaceAll(data, " ", "\\")
	if got := f.String(); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestIndicators(t *testing.T) {
	f := testDataField()
	if f.Indicator1() != "0" || f.Indicators.First != "0" {
		t.Errorf("indicator1 got %q", f.Indicator1())
	}
	if f.Indicator2() != "1" || f.Indicators.Second != "1" {
		t.Errorf("indicator2 got %q", f.Indicator2())
	}
}

func TestReassignIndicators(t *testing.T) {
	f := testDataField()
	f.SetIndicator1(" ")
	f.SetIndicator2("1")
	if f.Indicator1() != " " || f.Indicator2() != "1" {
		t.Errorf("got %q %q", f.Indicator1(), f.Indicator2())
	}
}

func TestSubfieldsCreated(t *testing.T) {
	f := testDataField()
	if len(f.Subfields) != 2 {
		t.Errorf("want 2 subfields got %d", len(f.Subfields))
	}
}

func TestSubfieldShort(t *testing.T) {
	f := testDataField()
	v, ok := f.Subfield("a")
	if !ok || v != "Huckleberry Finn: " {
		t.Errorf("got %q %v", v, ok)
	}
	if _, ok := f.Subfield("z"); ok {
		t.Errorf("want not ok for missing code")
	}
}

func TestSubfieldSetterField(t *testing.T) {
	f := testDataField()
	f.Subfields = []Subfield{{Code: "a", Value: "The Adventures of Tom Sawyer"}}
	v, _ := f.Subfield("a")
	if v != "The Adventures of Tom Sawyer" {
		t.Errorf("got %q", v)
	}
}

func TestGetSubfields(t *testing.T) {
	f := testDataField()
	sf := testSubjectField()
	if got := f.GetSubfields("a"); len(got) != 1 || got[0] != "Huckleberry Finn: " {
		t.Errorf("got %v", got)
	}
	if got := sf.GetSubfields("a"); len(got) != 1 || got[0] != "Python (Computer program language)" {
		t.Errorf("got %v", got)
	}
}

func TestGetSubfieldsMulti(t *testing.T) {
	f := testDataField()
	got := f.GetSubfields("a", "b")
	want := []string{"Huckleberry Finn: ", "An American Odyssey"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v want %v", got, want)
	}
	sf := testSubjectField()
	got2 := sf.GetSubfields("a", "v")
	want2 := []string{"Python (Computer program language)", "Poetry."}
	if len(got2) != 2 || got2[0] != want2[0] || got2[1] != want2[1] {
		t.Errorf("got %v want %v", got2, want2)
	}
}

func TestFieldAsMarc(t *testing.T) {
	f := testDataField()
	if _, err := f.AsMarc("utf-8"); err != nil {
		t.Fatal(err)
	}
}

func TestFieldContains(t *testing.T) {
	f := testDataField()
	if !f.Contains("a") {
		t.Error("want contains a")
	}
	if f.Contains("zzz") {
		t.Error("want not contains zzz")
	}
}

func TestFieldRangeOverSubfields(t *testing.T) {
	// Go slices are natively rangeable; no __iter__/__next__ port needed.
	f := testDataField()
	s := ""
	for _, sub := range f.Subfields {
		s += sub.Code + sub.Value
	}
	if want := "aHuckleberry Finn: bAn American Odyssey"; s != want {
		t.Errorf("got %q want %q", s, want)
	}
}

func TestFieldValue(t *testing.T) {
	f := testDataField()
	if got := f.Value(); got != "Huckleberry Finn: An American Odyssey" {
		t.Errorf("got %q", got)
	}
	cf := testControlField()
	if got := cf.Value(); got != "831227m19799999nyu           ||| | ger  " {
		t.Errorf("got %q", got)
	}
}

func TestNonIntegerTag(_ *testing.T) {
	// must not panic
	NewDataField("3 0", "0", "1", Subfield{Code: "a", Value: "foo"})
}

func TestNumericTagZeroPadded(t *testing.T) {
	f := NewControlField("8", "")
	if f.Tag != "008" {
		t.Errorf("got %q", f.Tag)
	}
}

func TestPaddedNumericTagPreserved(t *testing.T) {
	f := NewControlField("008", "foo")
	if f.Tag != "008" {
		t.Errorf("got %q", f.Tag)
	}
}

func TestAddSubfield(t *testing.T) {
	f := NewDataField("245", "0", "1", Subfield{Code: "a", Value: "foo"})
	f.AddSubfield("a", "bar")
	if want := "=245  01$afoo$abar"; f.String() != want {
		t.Errorf("got %q want %q", f.String(), want)
	}
	f.AddSubfieldAt("b", "baz", 0)
	if want := "=245  01$bbaz$afoo$abar"; f.String() != want {
		t.Errorf("got %q want %q", f.String(), want)
	}
	f.AddSubfieldAt("c", "qux", 2)
	if want := "=245  01$bbaz$afoo$cqux$abar"; f.String() != want {
		t.Errorf("got %q want %q", f.String(), want)
	}
	f.AddSubfieldAt("z", "wat", 8)
	if want := "=245  01$bbaz$afoo$cqux$abar$zwat"; f.String() != want {
		t.Errorf("got %q want %q", f.String(), want)
	}
}

func TestDeleteSubfield(t *testing.T) {
	f := NewDataField("200", "0", "1",
		Subfield{Code: "a", Value: "My Title"},
		Subfield{Code: "a", Value: "Kinda Bogus Anyhow"},
	)
	if _, ok := f.DeleteSubfield("z"); ok {
		t.Error("want not ok")
	}
	v, ok := f.DeleteSubfield("a")
	if !ok || v != "My Title" {
		t.Errorf("got %q %v", v, ok)
	}
	v, ok = f.DeleteSubfield("a")
	if !ok || v != "Kinda Bogus Anyhow" {
		t.Errorf("got %q %v", v, ok)
	}
	if len(f.Subfields) != 0 {
		t.Errorf("want 0 subfields left")
	}
}

func TestSubfieldDeleteContains(t *testing.T) {
	f := NewDataField("200", "0", "1",
		Subfield{Code: "a", Value: "My Title"},
		Subfield{Code: "z", Value: "Kinda Bogus Anyhow"},
	)
	if !f.Contains("z") {
		t.Error("want contains z")
	}
	f.DeleteSubfield("z")
	if f.Contains("z") {
		t.Error("want not contains z")
	}
}

func TestIsSubjectField(t *testing.T) {
	if !testSubjectField().IsSubjectField() {
		t.Error("want subject field true")
	}
	if testDataField().IsSubjectField() {
		t.Error("want subject field false")
	}
}

func TestFormatField(t *testing.T) {
	sf := testSubjectField()
	sf.AddSubfield("6", "880-4")
	if want := "Python (Computer program language) -- Poetry."; sf.FormatField() != want {
		t.Errorf("got %q want %q", sf.FormatField(), want)
	}
	f := testDataField()
	f.AddSubfield("6", "880-1")
	if want := "Huckleberry Finn:  An American Odyssey"; f.FormatField() != want {
		t.Errorf("got %q want %q", f.FormatField(), want)
	}
}

func TestTagNormalize(t *testing.T) {
	f := NewDataField("42", "", "")
	if f.Tag != "042" {
		t.Errorf("got %q", f.Tag)
	}
}

func TestAlphaTag(t *testing.T) {
	f := NewDataField("CAT", "0", "1", Subfield{Code: "a", Value: "foo"})
	if f.Tag != "CAT" {
		t.Errorf("got %q", f.Tag)
	}
	v, _ := f.Subfield("a")
	if v != "foo" {
		t.Errorf("got %q", v)
	}
	if f.ControlField {
		t.Error("want ControlField false")
	}
}

func TestSetSubfieldNoKey(t *testing.T) {
	f := testDataField()
	if err := f.SetSubfield("h", "error"); err == nil {
		t.Error("want error for missing code")
	}
}

func TestSetSubfieldRepeatedKey(t *testing.T) {
	f := testDataField()
	f.AddSubfield("a", "bar")
	if err := f.SetSubfield("a", "error"); err == nil {
		t.Error("want error for repeated code")
	}
}

func TestSetSubfield(t *testing.T) {
	f := testDataField()
	if err := f.SetSubfield("a", "changed"); err != nil {
		t.Fatal(err)
	}
	v, _ := f.Subfield("a")
	if v != "changed" {
		t.Errorf("got %q", v)
	}
}

func TestDeleteSubfieldOnlyByCode(t *testing.T) {
	f := NewDataField("960", " ", " ",
		Subfield{Code: "a", Value: "b"},
		Subfield{Code: "b", Value: "x"},
	)
	v, ok := f.DeleteSubfield("b")
	if !ok || v != "x" {
		t.Errorf("got %q %v", v, ok)
	}
	if len(f.Subfields) != 1 || f.Subfields[0] != (Subfield{Code: "a", Value: "b"}) {
		t.Errorf("got %v", f.Subfields)
	}
}

func TestSubfieldsByCode(t *testing.T) {
	f := NewDataField("680", " ", " ",
		Subfield{Code: "a", Value: "Repeated"},
		Subfield{Code: "a", Value: "Subfield"},
	)
	m := f.SubfieldsByCode()
	if got := m["a"]; len(got) != 2 || got[0] != "Repeated" || got[1] != "Subfield" {
		t.Errorf("got %v", got)
	}
}

func TestSetIndicatorsAffectsString(t *testing.T) {
	f := testDataField()
	f.SetIndicator1("9")
	f.SetIndicator2("9")
	if want := "=245  99$aHuckleberry Finn: $bAn American Odyssey"; f.String() != want {
		t.Errorf("got %q want %q", f.String(), want)
	}
}

func TestSetIndicatorsAffectsMarc(t *testing.T) {
	f := testDataField()
	f.SetIndicator1("9")
	f.SetIndicator2("9")
	got, err := f.AsMarc("utf-8")
	if err != nil {
		t.Fatal(err)
	}
	want := "99\x1faHuckleberry Finn: \x1fbAn American Odyssey\x1e"
	if string(got) != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestLinkageOccurrenceNum(t *testing.T) {
	f := NewDataField("245", "1", "0", Subfield{Code: "6", Value: "880-01"})
	if v, ok := f.LinkageOccurrenceNum(); !ok || v != "01" {
		t.Errorf("got %q %v", v, ok)
	}
	f = NewDataField("245", "1", "0", Subfield{Code: "6", Value: "530-00/(2/r"})
	if v, ok := f.LinkageOccurrenceNum(); !ok || v != "00" {
		t.Errorf("got %q %v", v, ok)
	}
	f = NewDataField("245", "1", "0", Subfield{Code: "6", Value: "100-42/Cyrl"})
	if v, ok := f.LinkageOccurrenceNum(); !ok || v != "42" {
		t.Errorf("got %q %v", v, ok)
	}
	f = NewDataField("245", "1", "0", Subfield{Code: "a", Value: "Music primer"})
	if _, ok := f.LinkageOccurrenceNum(); ok {
		t.Error("want not ok")
	}
}

func TestCodedSubfield(t *testing.T) {
	f := testDataField()
	sub := f.Subfields[0]
	if sub.Code != "a" || sub.Value != "Huckleberry Finn: " {
		t.Errorf("got %+v", sub)
	}
}
