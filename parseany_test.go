package marc

import (
	"os"
	"testing"
)

func TestParseAnyDetectsFormats(t *testing.T) {
	jsonData, err := os.ReadFile("testdata/one.json")
	if err != nil {
		t.Fatalf("read testdata/one.json: %v", err)
	}
	xmlData, err := os.ReadFile("testdata/utf8.xml")
	if err != nil {
		t.Fatalf("read testdata/utf8.xml: %v", err)
	}

	rec, err := ParseAny(jsonData)
	if err != nil {
		t.Fatalf("ParseAny(json): %v", err)
	}
	if rec.Get("245") == nil {
		t.Error("expected a 245 field from testdata/one.json")
	}

	rec, err = ParseAny(xmlData)
	if err != nil {
		t.Fatalf("ParseAny(xml): %v", err)
	}
	if rec.Get("245") == nil {
		t.Error("expected a 245 field from testdata/utf8.xml")
	}

	marcBytes, err := rec.AsMARC()
	if err != nil {
		t.Fatalf("AsMARC: %v", err)
	}
	rec, err = ParseAny(marcBytes)
	if err != nil {
		t.Fatalf("ParseAny(iso2709): %v", err)
	}
	if rec.Get("245") == nil {
		t.Error("expected a 245 field from round-tripped ISO 2709 bytes")
	}
}
