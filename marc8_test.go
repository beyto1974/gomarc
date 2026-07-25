package marc

import (
	"bufio"
	"bytes"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

// Ported from test/test_marc8.py. Cases requiring RawField / to_unicode=false
// (test_marc8_reader, test_marc8_read_write, test_reading_utf8_with_flag's/
// test_reading_utf8_without_flag's raw-bytes halves) are deferred: RawField
// is not yet implemented (see DecodeMARC's to_unicode=false TODO).

func TestMarc8ToUnicodeBulk(t *testing.T) {
	marc8File, err := os.Open("testdata/test_marc8.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = marc8File.Close() }()
	utf8File, err := os.Open("testdata/test_utf8.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = utf8File.Close() }()

	marc8Scanner := bufio.NewScanner(marc8File)
	marc8Scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	utf8Scanner := bufio.NewScanner(utf8File)
	utf8Scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	count := 0
	for marc8Scanner.Scan() {
		if !utf8Scanner.Scan() {
			t.Fatalf("test_utf8.txt shorter than test_marc8.txt at line %d", count+1)
		}
		marc8Line := marc8Scanner.Bytes()
		wantUTF8 := utf8Scanner.Text()
		if len(marc8Line) == 0 || wantUTF8 == "" {
			break
		}
		count++
		got, err := marc8ToUnicode(marc8Line, true)
		if err != nil {
			t.Fatalf("line %d: %v", count, err)
		}
		if got != wantUTF8 {
			t.Errorf("line %d: marc8ToUnicode(%q) = %q, want %q", count, marc8Line, got, wantUTF8)
		}
	}
	if count != 1515 {
		t.Errorf("want 1515 comparisons, got %d", count)
	}
}

func TestMarc8ReaderToUnicode(t *testing.T) {
	rdr := NewReaderFromBytes(mustReadFile(t, "marc8.dat"), WithToUnicode(true))
	rec, err := rdr.Next()
	if err != nil {
		t.Fatal(err)
	}
	v, ok := rec.Get("240").Subfield("a")
	if !ok {
		t.Fatal("want 240$a present")
	}
	if want := "De la solitude à la communauté."; v != want {
		t.Errorf("got %q want %q", v, want)
	}
}

func TestMarc8ReaderTo1251(t *testing.T) {
	rdr := NewReaderFromBytes(mustReadFile(t, "1251.dat"), WithFileEncoding("cp1251"))
	rec, err := rdr.Next()
	if err != nil {
		t.Fatal(err)
	}
	v, _ := rec.Get("245").Subfield("a")
	want := "Основы гидравлического расчета инженерных сетей"
	if v != want {
		t.Errorf("got %q want %q", v, want)
	}
}

func TestMarc8ReaderToUnicodeBadEaccSequence(t *testing.T) {
	rdr := NewReaderFromBytes(mustReadFile(t, "bad_eacc_encoding.dat"), WithHideUTF8Warnings(true))
	rec, err := rdr.Next()
	if err != nil {
		t.Fatal(err)
	}
	v, _ := rec.Get("880").Subfield("a")
	if n := utf8.RuneCountInString(v); n != 12 {
		t.Errorf("want length 12, got %d (%q)", n, v)
	}
	if !strings.HasSuffix(v, " ") {
		t.Errorf("want trailing space, got %q", v)
	}
}

func TestMarc8ReaderToUnicodeBadEscape(t *testing.T) {
	rdr := NewReaderFromBytes(mustReadFile(t, "bad_marc8_escape.dat"))
	rec, err := rdr.Next()
	if err != nil {
		t.Fatal(err)
	}
	v, _ := rec.Get("260").Subfield("b")
	want := "La Sociét\x1b,"
	if v != want {
		t.Errorf("got %q want %q", v, want)
	}
}

func TestMarc8Subscript2(t *testing.T) {
	got, err := marc8ToUnicode([]byte("CO\x1bb2\x1bs is a gas"), false)
	if err != nil {
		t.Fatal(err)
	}
	if want := "CO₂ is a gas"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	got, err = marc8ToUnicode([]byte("CO\x1bb2\x1bs"), false)
	if err != nil {
		t.Fatal(err)
	}
	if want := "CO₂"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestMarc8EszettEuro(t *testing.T) {
	got, err := marc8ToUnicode([]byte("ESZETT SYMBOL: \xc7 is U+00DF"), false)
	if err != nil {
		t.Fatal(err)
	}
	if want := "ESZETT SYMBOL: ß is U+00DF"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	got, err = marc8ToUnicode([]byte("EURO SIGN: \xc8 is U+20AC"), false)
	if err != nil {
		t.Fatal(err)
	}
	if want := "EURO SIGN: € is U+20AC"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestMarc8Alif(t *testing.T) {
	got, err := marc8ToUnicode([]byte("ALIF: \xae is U+02BC"), false)
	if err != nil {
		t.Fatal(err)
	}
	if want := "ALIF: ʼ is U+02BC"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestReadingUTF8WithFlag(t *testing.T) {
	rdr := NewReaderFromBytes(mustReadFile(t, "utf8_with_leader_flag.dat"), WithToUnicode(true))
	rec, err := rdr.Next()
	if err != nil {
		t.Fatal(err)
	}
	v, _ := rec.Get("240").Subfield("a")
	want := "De la solitude a" + string(rune(0x0300)) + " la communaute" + string(rune(0x0301)) + "."
	if v != want {
		t.Errorf("got %q want %q", v, want)
	}
}

func TestReadingUTF8WithoutFlag(t *testing.T) {
	data := mustReadFile(t, "utf8_without_leader_flag.dat")

	rdr := NewReaderFromBytes(data, WithToUnicode(true), WithHideUTF8Warnings(true))
	rec, err := rdr.Next()
	if err != nil {
		t.Fatal(err)
	}
	v, _ := rec.Get("240").Subfield("a")
	// Without force_utf8, the multi-byte UTF-8 combining marks are misread
	// byte-by-byte as MARC-8, replaced with spaces per pymarc's behavior.
	if want := "De la solitude a   la communaute ."; v != want {
		t.Errorf("got %q want %q", v, want)
	}

	rdr2 := NewReaderFromBytes(data, WithToUnicode(true), WithForceUTF8(true), WithHideUTF8Warnings(true))
	rec2, err := rdr2.Next()
	if err != nil {
		t.Fatal(err)
	}
	v2, _ := rec2.Get("240").Subfield("a")
	want2 := "De la solitude a" + string(rune(0x0300)) + " la communaute" + string(rune(0x0301)) + "."
	if v2 != want2 {
		t.Errorf("got %q want %q", v2, want2)
	}
}

func TestRecordCreateForceUTF8(t *testing.T) {
	r := mustRecord(t, WithForceUTF8(true))
	if r.Leader.Byte(9) != 'a' {
		t.Errorf("got %q", string(r.Leader.Byte(9)))
	}
}

func TestWritingUnicode(t *testing.T) {
	r := mustRecord(t)
	r.AddField(NewDataField("245", "1", "0", Subfield{Code: "a", Value: string(rune(0x1234))}))
	leader, err := NewLeader("         a              ")
	if err != nil {
		t.Fatal(err)
	}
	r.Leader = leader

	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.Write(r); err != nil {
		t.Fatal(err)
	}

	rdr := NewReaderFromBytes(buf.Bytes(), WithToUnicode(true))
	rec, err := rdr.Next()
	if err != nil {
		t.Fatal(err)
	}
	v, _ := rec.Get("245").Subfield("a")
	if v != string(rune(0x1234)) {
		t.Errorf("got %q", v)
	}
}
