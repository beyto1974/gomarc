package schema

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestBuildCommon(t *testing.T) {
	s := Build(Common)
	if len(s.Fields) < 80 {
		t.Fatalf("Build(Common) returned %d fields, want >= 80", len(s.Fields))
	}
	for _, f := range s.Fields {
		if len(f.Tag) != 3 {
			t.Errorf("field %q: tag is not 3 digits", f.Tag)
		}
	}
}

func TestBuildAll(t *testing.T) {
	common := Build(Common)
	all := Build(All)
	if len(all.Fields) <= len(common.Fields) {
		t.Fatalf("Build(All) returned %d fields, want more than Common's %d", len(all.Fields), len(common.Fields))
	}
}

func TestLeaderCoverage(t *testing.T) {
	s := Build(Common)
	if len(s.LeaderFields) != 16 {
		t.Fatalf("got %d leader fields, want 16", len(s.LeaderFields))
	}

	covered := 0
	for _, l := range s.LeaderFields {
		lo, hi, err := parsePositionRange(l.Position)
		if err != nil {
			t.Fatalf("bad position %q: %v", l.Position, err)
		}
		covered += hi - lo + 1
	}
	if covered != 24 {
		t.Fatalf("leader positions cover %d bytes, want 24", covered)
	}
}

var errBadPosition = errors.New("invalid position string")

// parsePositionRange parses "NN" or "NN-NN" into an inclusive [lo, hi] range.
func parsePositionRange(s string) (lo, hi int, err error) {
	parts := strings.SplitN(s, "-", 2)
	lo, err = atoiStrict(parts[0])
	if err != nil {
		return 0, 0, err
	}
	if len(parts) == 1 {
		return lo, lo, nil
	}
	hi, err = atoiStrict(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return lo, hi, nil
}

func atoiStrict(s string) (int, error) {
	if s == "" {
		return 0, errBadPosition
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errBadPosition
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func TestField008Coverage(t *testing.T) {
	s := Build(Common)
	wantTypes := []string{"Books", "Serials", "Maps", "Music", "VisualMaterials", "ComputerFiles", "MixedMaterials"}
	if len(s.Field008.ByType) != len(wantTypes) {
		t.Fatalf("got %d material types, want %d", len(s.Field008.ByType), len(wantTypes))
	}
	for _, wt := range wantTypes {
		if _, ok := s.Field008.ByType[wt]; !ok {
			t.Errorf("missing material type %q", wt)
		}
	}

	positions := map[string]bool{}
	for _, p := range s.Field008.Common {
		positions[p.Position] = true
	}
	if !hasPositionCovering(positions, "00") || !hasPositionCovering(positions, "05") {
		t.Errorf("Field008.Common missing coverage for positions 00-05: %v", positions)
	}
	if !hasPositionCovering(positions, "35") || !hasPositionCovering(positions, "39") {
		t.Errorf("Field008.Common missing coverage for positions 35-39: %v", positions)
	}
}

func hasPositionCovering(positions map[string]bool, target string) bool {
	if positions[target] {
		return true
	}
	want, err := atoiStrict(target)
	if err != nil {
		return false
	}
	for p := range positions {
		lo, hi, err := parsePositionRange(p)
		if err != nil {
			continue
		}
		if want >= lo && want <= hi {
			return true
		}
	}
	return false
}

func TestLookup(t *testing.T) {
	s := Build(Common)
	f, ok := s.Lookup("245")
	if !ok {
		t.Fatal("tag 245 not found")
	}
	if f.Label == "" {
		t.Error("245 label is empty")
	}
	if len(f.Subfields) < 3 {
		t.Errorf("245 has %d subfields, want >= 3", len(f.Subfields))
	}
}

func TestAsJSON(t *testing.T) {
	s := Build(Common)
	b, err := s.AsJSON()
	if err != nil {
		t.Fatalf("AsJSON error: %v", err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("AsJSON output is not valid JSON: %v", err)
	}
	for _, key := range []string{"fields", "leader_fields", "field_008"} {
		if _, ok := out[key]; !ok {
			t.Errorf("AsJSON output missing key %q", key)
		}
	}
}

func TestLookupLeader(t *testing.T) {
	s := Build(Common)
	l, ok := s.LookupLeader("09")
	if !ok {
		t.Fatal("leader position 09 not found")
	}
	if _, ok := l.Values["a"]; !ok {
		t.Errorf("leader position 09 Values missing key \"a\": %v", l.Values)
	}
}
