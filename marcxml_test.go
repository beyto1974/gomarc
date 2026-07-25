package marc

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
)

// Ported from test/test_xml.py.

func mustOpen(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestXMLMapXML(t *testing.T) {
	seen := 0
	xr := NewXMLReader(mustOpen(t, "batch.xml"))
	for {
		_, err := xr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		seen++
	}
	if seen != 2 {
		t.Errorf("want 2, got %d", seen)
	}
}

func TestXMLParseToArray(t *testing.T) {
	records, err := ParseXML(mustOpen(t, "batch.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("want 2 records, got %d", len(records))
	}
	record := records[0]
	if got := len(record.GetFields()); got != 18 {
		t.Errorf("want 18 fields, got %d", got)
	}
	f008 := record.Get("008")
	if f008 == nil || f008.Data != "910926s1957    nyuuun              eng  " {
		t.Errorf("008 got %+v", f008)
	}
	f245 := record.Get("245")
	if f245.Indicator1() != "0" || f245.Indicator2() != "4" {
		t.Errorf("245 indicators got %q %q", f245.Indicator1(), f245.Indicator2())
	}
	if v, _ := f245.Subfield("a"); v != "The Great Ray Charles" {
		t.Errorf("245$a got %q", v)
	}
	if v, _ := f245.Subfield("h"); v != "[sound recording]." {
		t.Errorf("245$h got %q", v)
	}
}

func TestXMLRoundTrip(t *testing.T) {
	records, err := ParseXML(mustOpen(t, "batch.xml"))
	if err != nil {
		t.Fatal(err)
	}
	record1 := records[0]

	var buf bytes.Buffer
	xw, err := NewXMLWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := xw.Write(record1); err != nil {
		t.Fatal(err)
	}
	if err := xw.Close(); err != nil {
		t.Fatal(err)
	}

	records2, err := ParseXML(&buf)
	if err != nil {
		t.Fatal(err)
	}
	record2 := records2[0]

	if record1.Leader.String() != record2.Leader.String() {
		t.Errorf("leader got %q want %q", record2.Leader.String(), record1.Leader.String())
	}
	f1, f2 := record1.GetFields(), record2.GetFields()
	if len(f1) != len(f2) {
		t.Fatalf("field count got %d want %d", len(f2), len(f1))
	}
	for i := range f1 {
		if f1[i].Tag != f2[i].Tag {
			t.Errorf("field %d: tag got %q want %q", i, f2[i].Tag, f1[i].Tag)
		}
		if f1[i].ControlField {
			if f1[i].Data != f2[i].Data {
				t.Errorf("field %d: data got %q want %q", i, f2[i].Data, f1[i].Data)
			}
		} else {
			if f1[i].Indicators != f2[i].Indicators {
				t.Errorf("field %d: indicators got %+v want %+v", i, f2[i].Indicators, f1[i].Indicators)
			}
			g1, g2 := f1[i].GetSubfields(), f2[i].GetSubfields()
			if len(g1) != len(g2) {
				t.Errorf("field %d: subfield count got %d want %d", i, len(g2), len(g1))
			}
		}
	}
}

func TestXMLNamespaces(t *testing.T) {
	// pymarc's record_to_xml() takes a namespace=bool toggle for whether an
	// individual <record> carries the MARCXML xmlns. Our XMLWriter instead
	// always puts the namespace on the enclosing <collection>, which is the
	// more idiomatic MARCXML shape and is present unconditionally.
	rdr := NewReaderFromBytes(mustReadFile(t, "test.dat"))
	rec, err := rdr.Next()
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	xw, err := NewXMLWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := xw.Write(rec); err != nil {
		t.Fatal(err)
	}
	if err := xw.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`xmlns="http://www.loc.gov/MARC21/slim"`)) {
		t.Error("want xmlns present on collection wrapper")
	}
}

func TestXMLBadTag(t *testing.T) {
	_, err := ParseXML(mustOpen(t, "bad_tag.xml"))
	if !errors.Is(err, ErrRecordLeaderInvalid) {
		t.Errorf("want ErrRecordLeaderInvalid, got %v", err)
	}
}

func BenchmarkParseXML(b *testing.B) {
	data, err := os.ReadFile("testdata/batch.xml")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := bytes.NewReader(data)
		_, err := ParseXML(r)
		if err != nil {
			b.Fatal(err)
		}
	}
}
