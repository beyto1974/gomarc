package marc

import (
	"errors"
	"testing"
)

// Ported from test/test_record.py. Tests requiring fixture files (test/*.dat) or a
// MARCReader (test_multiple_isbn, test_copy, test_remove_fields,
// test_as_marc_to_unicode_conversion, test_map_marc8_record_against_unicode_as_marc)
// are deferred to the reader/writer and marc8 phases.

func mustRecord(t *testing.T, opts ...RecordOption) *Record {
	t.Helper()
	r, err := NewRecord(opts...)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRecordAddField(t *testing.T) {
	r := mustRecord(t)
	f := NewDataField("245", "1", "0", Subfield{Code: "a", Value: "Python"}, Subfield{Code: "c", Value: "Guido"})
	r.AddField(f)
	found := false
	for _, existing := range r.Fields {
		if existing == f {
			found = true
		}
	}
	if !found {
		t.Error("field not found after AddField")
	}
}

func TestRecordFields(t *testing.T) {
	r := mustRecord(t, WithFields(
		NewDataField("245", "1", "0", Subfield{Code: "a", Value: "Python"}, Subfield{Code: "c", Value: "Guido"}),
		NewDataField("260", " ", " ", Subfield{Code: "a", Value: "Amsterdam"}),
	))
	if v, _ := r.Get("245").Subfield("a"); v != "Python" {
		t.Errorf("got %q", v)
	}
	if v, _ := r.Get("260").Subfield("a"); v != "Amsterdam" {
		t.Errorf("got %q", v)
	}
}

func TestRecordRemoveField(t *testing.T) {
	r := mustRecord(t)
	f := NewDataField("245", "1", "0", Subfield{Code: "a", Value: "Python"}, Subfield{Code: "c", Value: "Guido"})
	r.AddField(f)
	if v, _ := r.Get("245").Subfield("a"); v != "Python" {
		t.Fatalf("got %q", v)
	}
	if err := r.RemoveField(f); err != nil {
		t.Fatal(err)
	}
	if r.Get("245") != nil {
		t.Error("want 245 removed")
	}
	other := NewControlField("001", "abcd1234")
	if err := r.RemoveField(other); !errors.Is(err, ErrFieldNotFound) {
		t.Errorf("want ErrFieldNotFound, got %v", err)
	}
}

func TestRecordQuickAccess(t *testing.T) {
	r := mustRecord(t)
	title := NewDataField("245", "1", "0", Subfield{Code: "a", Value: "Python"}, Subfield{Code: "c", Value: "Guido"})
	r.AddField(title)
	if r.Get("245") != title {
		t.Error("want same field")
	}
	if r.Get("999") != nil {
		t.Error("want nil for missing tag")
	}
}

func TestRecordMembership(t *testing.T) {
	r := mustRecord(t)
	r.AddField(NewDataField("245", "1", "0"))
	if !r.Contains("245") {
		t.Error("want contains 245")
	}
	if r.Contains("999") {
		t.Error("want not contains 999")
	}
}

func TestRecordFieldNotFoundEmpty(t *testing.T) {
	r := mustRecord(t)
	if len(r.Fields) != 0 {
		t.Errorf("want 0 fields, got %d", len(r.Fields))
	}
}

func TestRecordFind(t *testing.T) {
	r := mustRecord(t)
	s1 := NewDataField("650", "", "0", Subfield{Code: "a", Value: "Programming Language"})
	r.AddField(s1)
	s2 := NewDataField("650", "", "0", Subfield{Code: "a", Value: "Object Oriented"})
	r.AddField(s2)
	found := r.GetFields("650")
	if len(found) != 2 || found[0] != s1 || found[1] != s2 {
		t.Errorf("got %v", found)
	}
	if all := r.GetFields(); len(all) != 2 {
		t.Errorf("want 2 fields with no tag, got %d", len(all))
	}
}

func TestRecordMultiFind(t *testing.T) {
	r := mustRecord(t)
	r.AddField(NewDataField("650", "", "0", Subfield{Code: "a", Value: "Programming Language"}))
	r.AddField(NewDataField("651", "", "0", Subfield{Code: "a", Value: "Object Oriented"}))
	found := r.GetFields("650", "651")
	if len(found) != 2 {
		t.Errorf("got %d", len(found))
	}
}

func TestRecordGetLinkedFields(t *testing.T) {
	r := mustRecord(t)
	t1 := NewDataField("245", "1", "0", Subfield{Code: "6", Value: "880-01"}, Subfield{Code: "a", Value: "title"})
	r.AddField(t1)
	t2 := NewDataField("880", "1", "0", Subfield{Code: "6", Value: "245-01"}, Subfield{Code: "a", Value: "linked title"})
	r.AddField(t2)
	pd1 := NewDataField("260", "0", "2", Subfield{Code: "6", Value: "880-02"}, Subfield{Code: "a", Value: "Tokyo"})
	r.AddField(pd1)
	pd2 := NewDataField("880", "0", "2", Subfield{Code: "6", Value: "260-02"}, Subfield{Code: "a", Value: "linked Tokyo"})
	r.AddField(pd2)

	got, err := r.GetLinkedFields(t1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != t2 {
		t.Errorf("got %v", got)
	}

	got, err = r.GetLinkedFields(pd1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != pd2 {
		t.Errorf("got %v", got)
	}
}

func TestRecordMissingLinkedFieldsError(t *testing.T) {
	r := mustRecord(t)
	t1 := NewDataField("245", "1", "0", Subfield{Code: "6", Value: "880-01"}, Subfield{Code: "a", Value: "title"})
	r.AddField(t1)
	if _, err := r.GetLinkedFields(t1); !errors.Is(err, ErrMissingLinkedFields) {
		t.Errorf("want ErrMissingLinkedFields, got %v", err)
	}
}

func TestRecordBadLeader(t *testing.T) {
	r := mustRecord(t)
	err := r.DecodeMARC([]byte("foo"), decodeOptions{toUnicode: true, utf8Handling: "strict", encoding: "iso8859-1"})
	if !errors.Is(err, ErrRecordLeaderInvalid) {
		t.Errorf("want ErrRecordLeaderInvalid, got %v", err)
	}
}

func TestRecordBadBaseAddress(t *testing.T) {
	r := mustRecord(t)
	err := r.DecodeMARC([]byte("00695cam  2200241Ia 45x00"), decodeOptions{toUnicode: true, utf8Handling: "strict", encoding: "iso8859-1"})
	if !errors.Is(err, ErrBaseAddressInvalid) {
		t.Errorf("want ErrBaseAddressInvalid, got %v", err)
	}
}

func TestRecordTitle(t *testing.T) {
	r := mustRecord(t)
	if _, ok := r.Title(); ok {
		t.Error("want no title")
	}
	r.AddField(NewDataField("245", "0", "1", Subfield{Code: "a", Value: "Foo :"}, Subfield{Code: "b", Value: "bar"}))
	if v, _ := r.Title(); v != "Foo : bar" {
		t.Errorf("got %q", v)
	}

	r2 := mustRecord(t)
	r2.AddField(NewDataField("245", "0", "1", Subfield{Code: "a", Value: "Farghin"}))
	if v, _ := r2.Title(); v != "Farghin" {
		t.Errorf("got %q", v)
	}
}

func TestRecordIssnTitle(t *testing.T) {
	r := mustRecord(t)
	if _, ok := r.IssnTitle(); ok {
		t.Error("want no issn_title")
	}
	r.AddField(NewDataField("222", "", "", Subfield{Code: "a", Value: "Foo :"}, Subfield{Code: "b", Value: "bar"}))
	if v, _ := r.IssnTitle(); v != "Foo : bar" {
		t.Errorf("got %q", v)
	}

	r2 := mustRecord(t)
	r2.AddField(NewDataField("222", "", "", Subfield{Code: "a", Value: "Farghin"}))
	if v, _ := r2.IssnTitle(); v != "Farghin" {
		t.Errorf("got %q", v)
	}

	r3 := mustRecord(t)
	r3.AddField(NewDataField("222", "", "", Subfield{Code: "b", Value: "bar"}))
	if _, ok := r3.IssnTitle(); ok {
		t.Error("want no issn_title when only $b present")
	}
}

func TestRecordISBN(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"9781416566113", "9781416566113"},
		{"978-1416566113", "9781416566113"},
		{"ISBN-978-1416566113", "9781416566113"},
		{"0456789012 (reel 1)", "0456789012"},
		{"006073132X", "006073132X"},
	}
	for _, c := range cases {
		r := mustRecord(t)
		r.AddField(NewDataField("020", "0", "1", Subfield{Code: "a", Value: c.value}))
		got, ok := r.ISBN()
		if !ok || got != c.want {
			t.Errorf("ISBN(%q): got %q ok=%v want %q", c.value, got, ok, c.want)
		}
	}
	r := mustRecord(t)
	if _, ok := r.ISBN(); ok {
		t.Error("want no isbn")
	}
}

func TestRecordISSN(t *testing.T) {
	r := mustRecord(t)
	if _, ok := r.ISSN(); ok {
		t.Error("want no issn")
	}
	r.AddField(NewDataField("022", "0", "", Subfield{Code: "a", Value: "0395-2037"}))
	if v, _ := r.ISSN(); v != "0395-2037" {
		t.Errorf("got %q", v)
	}
}

func TestRecordISSNL(t *testing.T) {
	r := mustRecord(t)
	if _, ok := r.ISSNL(); ok {
		t.Error("want no issnl")
	}
	r.AddField(NewDataField("022", "0", "", Subfield{Code: "l", Value: "0395-2037"}))
	if v, _ := r.ISSNL(); v != "0395-2037" {
		t.Errorf("got %q", v)
	}
}

func TestRecordAuthor(t *testing.T) {
	r := mustRecord(t)
	if _, ok := r.Author(); ok {
		t.Error("want no author")
	}
	r.AddField(NewDataField("100", "1", "0", Subfield{Code: "a", Value: "Bletch, Foobie,"}, Subfield{Code: "d", Value: "1979-1981."}))
	if v, _ := r.Author(); v != "Bletch, Foobie, 1979-1981." {
		t.Errorf("got %q", v)
	}

	r2 := mustRecord(t)
	r2.AddField(NewDataField("130", "0", " ", Subfield{Code: "a", Value: "Bible."}, Subfield{Code: "l", Value: "Python."}))
	if _, ok := r2.Author(); ok {
		t.Error("want no author from 130")
	}
}

func TestRecordUniformTitle(t *testing.T) {
	r := mustRecord(t)
	if _, ok := r.UniformTitle(); ok {
		t.Error("want no uniformtitle")
	}
	r.AddField(NewDataField("130", "0", " ", Subfield{Code: "a", Value: "Tosefta."}, Subfield{Code: "l", Value: "English."}, Subfield{Code: "f", Value: "1977."}))
	if v, _ := r.UniformTitle(); v != "Tosefta. English. 1977." {
		t.Errorf("got %q", v)
	}

	r2 := mustRecord(t)
	r2.AddField(NewDataField("240", "1", "4", Subfield{Code: "a", Value: "The Pickwick papers."}, Subfield{Code: "l", Value: "French."}))
	if v, _ := r2.UniformTitle(); v != "The Pickwick papers. French." {
		t.Errorf("got %q", v)
	}
}

func TestRecordSubjects(t *testing.T) {
	r := mustRecord(t)
	if got := r.Subjects(); len(got) != 0 {
		t.Errorf("want 0 subjects, got %d", len(got))
	}
	r.AddField(NewDataField("630", "0", " ", Subfield{Code: "a", Value: "Tosefta."}, Subfield{Code: "l", Value: "English."}, Subfield{Code: "f", Value: "1977."}))
	r.AddField(NewDataField("730", "0", " ", Subfield{Code: "a", Value: "Tosefta."}, Subfield{Code: "l", Value: "English."}, Subfield{Code: "f", Value: "1977."}))
	r.AddField(NewDataField("600", "1", "0", Subfield{Code: "a", Value: "Le Peu, Pepe."}))
	got := r.Subjects()
	if len(got) != 2 {
		t.Fatalf("want 2 subjects, got %d", len(got))
	}
	if want := "=630  0\\$aTosefta.$lEnglish.$f1977."; got[0].String() != want {
		t.Errorf("got %q want %q", got[0].String(), want)
	}
	if want := "=600  10$aLe Peu, Pepe."; got[1].String() != want {
		t.Errorf("got %q want %q", got[1].String(), want)
	}
}

func TestRecordAddedEntries(t *testing.T) {
	r := mustRecord(t)
	if got := r.AddedEntries(); len(got) != 0 {
		t.Errorf("want 0, got %d", len(got))
	}
	r.AddField(NewDataField("730", "0", " ", Subfield{Code: "a", Value: "Tosefta."}, Subfield{Code: "l", Value: "English."}, Subfield{Code: "f", Value: "1977."}))
	r.AddField(NewDataField("700", "1", "0", Subfield{Code: "a", Value: "Le Peu, Pepe."}))
	r.AddField(NewDataField("245", "0", "0", Subfield{Code: "a", Value: "Le Peu's Tosefa."}))
	got := r.AddedEntries()
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if want := "=730  0\\$aTosefta.$lEnglish.$f1977."; got[0].String() != want {
		t.Errorf("got %q", got[0].String())
	}
	if want := "=700  10$aLe Peu, Pepe."; got[1].String() != want {
		t.Errorf("got %q", got[1].String())
	}
}

func TestRecordPhysicalDescription(t *testing.T) {
	r := mustRecord(t)
	if got := r.PhysicalDescription(); len(got) != 0 {
		t.Errorf("want 0, got %d", len(got))
	}
	r.AddField(NewDataField("300", "\\", "", Subfield{Code: "a", Value: "1 photographic print :"}, Subfield{Code: "b", Value: "gelatin silver ;"}, Subfield{Code: "c", Value: "10 x 56 in."}))
	r.AddField(NewDataField("300", "\\", "", Subfield{Code: "a", Value: "FOO"}, Subfield{Code: "b", Value: "BAR"}, Subfield{Code: "c", Value: "BAZ"}))
	got := r.PhysicalDescription()
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if want := "=300  \\$a1 photographic print :$bgelatin silver ;$c10 x 56 in."; got[0].String() != want {
		t.Errorf("got %q", got[0].String())
	}
	if want := "=300  \\$aFOO$bBAR$cBAZ"; got[1].String() != want {
		t.Errorf("got %q", got[1].String())
	}
}

func TestRecordNotes(t *testing.T) {
	r := mustRecord(t)
	if got := r.Notes(); len(got) != 0 {
		t.Errorf("want 0, got %d", len(got))
	}
	r.AddField(NewDataField("500", " ", " ", Subfield{Code: "a", Value: "Recast in bronze from artist's plaster original of 1903."}))
	got := r.Notes()
	if len(got) != 1 || got[0].FormatField() != "Recast in bronze from artist's plaster original of 1903." {
		t.Errorf("got %v", got)
	}
}

func TestRecordPublisher(t *testing.T) {
	r := mustRecord(t)
	if _, ok := r.Publisher(); ok {
		t.Error("want no publisher")
	}
	r.AddField(NewDataField("260", " ", " ",
		Subfield{Code: "a", Value: "Paris :"}, Subfield{Code: "b", Value: "Gauthier-Villars ;"},
		Subfield{Code: "a", Value: "Chicago :"}, Subfield{Code: "b", Value: "University of Chicago Press,"},
		Subfield{Code: "c", Value: "1955."}))
	if v, _ := r.Publisher(); v != "Gauthier-Villars ;" {
		t.Errorf("got %q", v)
	}

	r2 := mustRecord(t)
	r2.AddField(NewDataField("264", " ", "1", Subfield{Code: "a", Value: "London :"}, Subfield{Code: "b", Value: "Penguin,"}, Subfield{Code: "c", Value: "1961."}))
	if v, _ := r2.Publisher(); v != "Penguin," {
		t.Errorf("got %q", v)
	}
}

func TestRecordPubYear(t *testing.T) {
	r := mustRecord(t)
	if _, ok := r.PubYear(); ok {
		t.Error("want no pubyear")
	}
	r.AddField(NewDataField("260", " ", " ",
		Subfield{Code: "a", Value: "Paris :"}, Subfield{Code: "b", Value: "Gauthier-Villars ;"},
		Subfield{Code: "a", Value: "Chicago :"}, Subfield{Code: "b", Value: "University of Chicago Press,"},
		Subfield{Code: "c", Value: "1955."}))
	if v, _ := r.PubYear(); v != "1955." {
		t.Errorf("got %q", v)
	}

	r2 := mustRecord(t)
	r2.AddField(NewDataField("264", " ", "1", Subfield{Code: "a", Value: "London :"}, Subfield{Code: "b", Value: "Penguin,"}, Subfield{Code: "c", Value: "1961."}))
	if v, _ := r2.PubYear(); v != "1961." {
		t.Errorf("got %q", v)
	}
}

func TestRecordAlphaTag(t *testing.T) {
	r := mustRecord(t)
	r.AddField(NewDataField("CAT", " ", " ", Subfield{Code: "a", Value: "foo"}))
	r.AddField(NewDataField("CAT", " ", " ", Subfield{Code: "b", Value: "bar"}))
	fields := r.GetFields("CAT")
	if len(fields) != 2 {
		t.Fatalf("want 2, got %d", len(fields))
	}
	if v, _ := fields[0].Subfield("a"); v != "foo" {
		t.Errorf("got %q", v)
	}
	if v, _ := fields[1].Subfield("b"); v != "bar" {
		t.Errorf("got %q", v)
	}
	if v, _ := r.Get("CAT").Subfield("a"); v != "foo" {
		t.Errorf("got %q", v)
	}
}

func TestRecordAsMarcWithExplicitLeader(t *testing.T) {
	r := mustRecord(t)
	r.AddField(NewDataField("245", "0", "1", Subfield{Code: "a", Value: "The pragmatic programmer"}))
	leader, err := NewLeader("00067    a2200037   4500")
	if err != nil {
		t.Fatal(err)
	}
	r.Leader = leader
	before := r.Leader.String()
	if _, err := r.AsMARC(); err != nil {
		t.Fatal(err)
	}
	if r.Leader.String() != before {
		t.Errorf("leader mutated: got %q want %q", r.Leader.String(), before)
	}
}

func TestRecordInitWithNoLeader(t *testing.T) {
	r := mustRecord(t)
	r.AddField(NewDataField("245", "0", "1", Subfield{Code: "a", Value: "The pragmatic programmer"}))
	data, err := r.AsMARC()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data[0:24]); got != "00067    a2200037   4500" {
		t.Errorf("got %q", got)
	}
}

func TestRecordInitWithNoLeaderButForceUTF8(t *testing.T) {
	r := mustRecord(t, WithForceUTF8(true))
	r.AddField(NewDataField("245", "0", "1", Subfield{Code: "a", Value: "The pragmatic programmer"}))
	data, err := r.AsMARC()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data[0:24]); got != "00067    a2200037   4500" {
		t.Errorf("got %q", got)
	}
}

func TestRecordInitWithLeader(t *testing.T) {
	r := mustRecord(t, WithLeaderString("abcdefghijklmnopqrstuvwx"))
	r.AddField(NewDataField("245", "0", "1", Subfield{Code: "a", Value: "The pragmatic programmer"}))
	data, err := r.AsMARC()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data[0:24]); got != "00067fghia2200037rst4500" {
		t.Errorf("got %q", got)
	}
}

func TestRecordInitWithLeaderAndForceUTF8(t *testing.T) {
	r := mustRecord(t, WithLeaderString("abcdefghijklmnopqrstuvwx"), WithForceUTF8(true))
	r.AddField(NewDataField("245", "0", "1", Subfield{Code: "a", Value: "The pragmatic programmer"}))
	data, err := r.AsMARC()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data[0:24]); got != "00067fghia2200037rst4500" {
		t.Errorf("got %q", got)
	}
}

func TestRecordAsMarcConsistency(t *testing.T) {
	r := mustRecord(t, WithForceUTF8(true))
	if _, err := r.AsMARC(); err != nil {
		t.Fatal(err)
	}
	if r.Leader == nil {
		t.Error("want non-nil leader after AsMARC")
	}
}

func TestRecordFieldsParameter(t *testing.T) {
	r := mustRecord(t, WithFields(
		NewDataField("245", "", "", Subfield{Code: "a", Value: "A title"}),
		NewDataField("500", "", "", Subfield{Code: "a", Value: "A comment"}),
	))
	if v, _ := r.Get("245").Subfield("a"); v != "A title" {
		t.Errorf("got %q", v)
	}
	if v, _ := r.Get("500").Subfield("a"); v != "A comment" {
		t.Errorf("got %q", v)
	}
}
