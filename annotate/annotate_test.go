package annotate

import (
	"strings"
	"testing"

	marc "github.com/beyto1974/gomarc"
	"github.com/beyto1974/gomarc/schema"
)

func TestBuildKnownField(t *testing.T) {
	rec, err := marc.NewRecord(marc.WithFields(
		marc.NewDataField("245", "0", "0",
			marc.Subfield{Code: "a", Value: "Gomarc"},
			marc.Subfield{Code: "b", Value: "annotated MARC21 output"},
		),
	))
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}

	ar := Build(rec, schema.Common)

	if len(ar.Legend) != 1 {
		t.Fatalf("want 1 legend entry, got %d", len(ar.Legend))
	}
	fl := ar.Legend[0]
	if fl.Label != "Title Statement" {
		t.Errorf("Legend Label = %q, want %q", fl.Label, "Title Statement")
	}
	if fl.Definition == "" {
		t.Error("Legend Definition should not be empty for a known tag")
	}
	if len(fl.Indicators) != 2 {
		t.Fatalf("want 2 legend indicators, got %d", len(fl.Indicators))
	}
	if fl.Indicators[0].Values["0"] != "No added entry" {
		t.Errorf("Ind1 Values[0] = %q, want %q", fl.Indicators[0].Values["0"], "No added entry")
	}
	if fl.Indicators[1].Values["0"] != "No nonfiling characters" {
		t.Errorf("Ind2 Values[0] = %q, want %q", fl.Indicators[1].Values["0"], "No nonfiling characters")
	}
	if len(fl.Subfields) != 2 {
		t.Fatalf("want 2 legend subfields (only $a/$b are used), got %d: %+v", len(fl.Subfields), fl.Subfields)
	}
	var subA, subC *SubfieldLegend
	for i := range fl.Subfields {
		switch fl.Subfields[i].Code {
		case "a":
			subA = &fl.Subfields[i]
		case "c":
			subC = &fl.Subfields[i]
		}
	}
	if subA == nil || subA.Label != "Title proper" {
		t.Errorf("legend subfield $a Label = %+v, want Title proper", subA)
	}
	if subC != nil {
		t.Errorf("legend should not list $c (defined in schema but not used in this record), got %+v", subC)
	}
}

func TestBuildSkipsEmptySubfieldValue(t *testing.T) {
	rec, err := marc.NewRecord(marc.WithFields(
		marc.NewDataField("245", "0", "0",
			marc.Subfield{Code: "a", Value: "Gomarc"},
			marc.Subfield{Code: "b", Value: ""},
		),
	))
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}

	ar := Build(rec, schema.Common)
	df := ar.Fields[0]
	if len(df.Subfields) != 1 || df.Subfields[0].Code != "a" {
		t.Errorf("data subfields = %+v, want only $a (empty $b dropped)", df.Subfields)
	}

	for _, sf := range ar.Legend[0].Subfields {
		if sf.Code == "b" {
			t.Errorf("legend should not list $b, its only occurrence had an empty value: %+v", sf)
		}
	}
}

func TestBuildRepeatedTagDedupesLegend(t *testing.T) {
	rec, err := marc.NewRecord(marc.WithFields(
		marc.NewDataField("650", " ", "0", marc.Subfield{Code: "a", Value: "Cataloging"}),
		marc.NewDataField("650", " ", "0", marc.Subfield{Code: "a", Value: "Metadata"}, marc.Subfield{Code: "x", Value: "Standards"}),
	))
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}

	ar := Build(rec, schema.Common)
	if len(ar.Legend) != 1 {
		t.Fatalf("want 1 legend entry for repeated tag, got %d", len(ar.Legend))
	}
	if len(ar.Fields) != 2 {
		t.Fatalf("want 2 data field occurrences, got %d", len(ar.Fields))
	}
	if ar.Fields[0].Subfields[0].Value != "Cataloging" || ar.Fields[1].Subfields[0].Value != "Metadata" {
		t.Errorf("data occurrences out of order or wrong values: %+v", ar.Fields)
	}

	// Legend must union subfield codes across all occurrences of the tag:
	// $a from the first 650, $x from the second — but not e.g. $z, unused here.
	codes := map[string]bool{}
	for _, sf := range ar.Legend[0].Subfields {
		codes[sf.Code] = true
	}
	if !codes["a"] || !codes["x"] {
		t.Errorf("legend subfields = %+v, want union including $a and $x", ar.Legend[0].Subfields)
	}
	if codes["z"] {
		t.Errorf("legend should not list $z (defined in schema but not used in either occurrence)")
	}
}

func TestBuildUnknownTagFallback(t *testing.T) {
	rec, err := marc.NewRecord(marc.WithFields(
		marc.NewDataField("069", "#", "#", marc.Subfield{Code: "z", Value: "local data"}),
	))
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}

	ar := Build(rec, schema.All)
	if len(ar.Legend) != 1 || ar.Legend[0].Label != "(undefined tag)" {
		t.Errorf("Legend = %+v, want fallback label", ar.Legend)
	}
	if ar.Legend[0].Definition != "" {
		t.Errorf("Definition = %q, want empty for undefined tag", ar.Legend[0].Definition)
	}
	if len(ar.Fields[0].Subfields) != 1 || ar.Fields[0].Subfields[0].Value != "local data" {
		t.Errorf("data subfields = %+v", ar.Fields[0].Subfields)
	}
}

func TestBuild008MaterialType(t *testing.T) {
	tests := []struct {
		name     string
		leader   string
		data008  string
		wantType string
	}{
		{
			name:     "Books",
			leader:   "00755cam  22002414a 4500",
			data008:  "000107s2000    nyua          001 0 eng  ",
			wantType: "Books",
		},
		{
			name:     "Music",
			leader:   "00640njm a2200205uu 4500",
			data008:  "100813s9999    xx ||nn s|||||||||||||| d",
			wantType: "Music",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := marc.NewRecord(
				marc.WithLeaderString(tt.leader),
				marc.WithFields(marc.NewControlField("008", tt.data008)),
			)
			if err != nil {
				t.Fatalf("NewRecord: %v", err)
			}
			ar := Build(rec, schema.Common)
			if ar.Field008 == nil {
				t.Fatal("Field008 should not be nil when record has an 008 field")
			}
			if ar.Field008.MaterialType != tt.wantType {
				t.Errorf("MaterialType = %q, want %q", ar.Field008.MaterialType, tt.wantType)
			}
			if len(ar.Field008.Positions) == 0 {
				t.Error("Positions should not be empty")
			}
		})
	}
}

func TestBuildLeaderPositions(t *testing.T) {
	rec, err := marc.NewRecord(marc.WithLeaderString("00755cam  22002414a 4500"))
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	ar := Build(rec, schema.Common)
	found := false
	for _, p := range ar.Leader {
		if p.Position == "06" {
			found = true
			if p.Value != "a" {
				t.Errorf("Leader/06 Value = %q, want %q", p.Value, "a")
			}
			if p.Meaning != "Language material" {
				t.Errorf("Leader/06 Meaning = %q, want %q", p.Meaning, "Language material")
			}
		}
	}
	if !found {
		t.Fatal("leader position 06 not found in annotated output")
	}
}

func TestRecordMarkdown(t *testing.T) {
	rec, err := marc.NewRecord(marc.WithFields(
		marc.NewDataField("650", " ", "0", marc.Subfield{Code: "a", Value: "Cataloging"}),
		marc.NewDataField("650", " ", "0", marc.Subfield{Code: "a", Value: "Metadata"}),
	))
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	ar := Build(rec, schema.Common)
	md := ar.Markdown()

	if !strings.Contains(md, "### 650 —") {
		t.Errorf("Markdown missing legend header, got:\n%s", md)
	}
	if strings.Count(md, "### 650 —") != 1 {
		t.Errorf("Legend header for repeated tag should appear once, got:\n%s", md)
	}
	if !strings.Contains(md, "650 #0 $a Cataloging") || !strings.Contains(md, "650 #0 $a Metadata") {
		t.Errorf("Markdown missing terse data lines, got:\n%s", md)
	}

	// Leader/08 (Type of control) is a fixed position present on every
	// record; blank is a real, always-present value (not an absent field)
	// and should still render with its decoded meaning.
	if !strings.Contains(md, "**08**") {
		t.Errorf("Markdown should still render leader position 08, got:\n%s", md)
	}
}
