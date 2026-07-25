package marc

import (
	"errors"
	"math/rand"
	"strings"
	"testing"
)

// Ported from test/test_leader.py.
const testLeaderStr = "00475casaa2200169 ib4500"

type leaderField struct {
	name  string
	start int
	end   int // exclusive; end == start+1 for single-byte fields
	get   func(*Leader) string
	set   func(*Leader, string) error
	want  string
}

func byteField(name string, pos int, get func(*Leader) byte, set func(*Leader, string) error, want string) leaderField {
	return leaderField{
		name:  name,
		start: pos,
		end:   pos + 1,
		get:   func(l *Leader) string { return string(get(l)) },
		set:   set,
		want:  want,
	}
}

func leaderTestFields() []leaderField {
	return []leaderField{
		{"record_length", 0, 5, func(l *Leader) string { return l.RecordLength() }, (*Leader).SetRecordLength, "00475"},
		byteField("record_status", 5, (*Leader).RecordStatus, (*Leader).SetRecordStatus, "c"),
		byteField("type_of_record", 6, (*Leader).TypeOfRecord, (*Leader).SetTypeOfRecord, "a"),
		byteField("bibliographic_level", 7, (*Leader).BibliographicLevel, (*Leader).SetBibliographicLevel, "s"),
		byteField("type_of_control", 8, (*Leader).TypeOfControl, (*Leader).SetTypeOfControl, "a"),
		byteField("coding_scheme", 9, (*Leader).CodingScheme, (*Leader).SetCodingScheme, "a"),
		byteField("indicator_count", 10, (*Leader).IndicatorCount, (*Leader).SetIndicatorCount, "2"),
		byteField("subfield_code_count", 11, (*Leader).SubfieldCodeCount, (*Leader).SetSubfieldCodeCount, "2"),
		{"base_address", 12, 17, func(l *Leader) string { return l.BaseAddress() }, (*Leader).SetBaseAddress, "00169"},
		byteField("encoding_level", 17, (*Leader).EncodingLevel, (*Leader).SetEncodingLevel, " "),
		byteField("cataloging_form", 18, (*Leader).CatalogingForm, (*Leader).SetCatalogingForm, "i"),
		byteField("multipart_resource", 19, (*Leader).MultipartResource, (*Leader).SetMultipartResource, "b"),
		byteField("length_of_field_length", 20, (*Leader).LengthOfFieldLength, (*Leader).SetLengthOfFieldLength, "4"),
		byteField("starting_character_position_length", 21, (*Leader).StartingCharacterPositionLength, (*Leader).SetStartingCharacterPositionLength, "5"),
		byteField("implementation_defined_length", 22, (*Leader).ImplementationDefinedLength, (*Leader).SetImplementationDefinedLength, "0"),
	}
}

func randomLowerString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(letters[rand.Intn(len(letters))])
	}
	return b.String()
}

func TestLeaderInvalidLength(t *testing.T) {
	_, err := NewLeader(testLeaderStr[:len(testLeaderStr)-1])
	if !errors.Is(err, ErrRecordLeaderInvalid) {
		t.Fatalf("want ErrRecordLeaderInvalid, got %v", err)
	}
}

func TestLeaderValue(t *testing.T) {
	l, err := NewLeader(testLeaderStr)
	if err != nil {
		t.Fatal(err)
	}
	if l.String() != testLeaderStr {
		t.Fatalf("got %q want %q", l.String(), testLeaderStr)
	}
}

func TestLeaderSliceConcat(t *testing.T) {
	l, err := NewLeader(testLeaderStr)
	if err != nil {
		t.Fatal(err)
	}
	got := l.Slice(0, 9) + "b" + l.Slice(10, LeaderLen)
	want := "00475casab2200169 ib4500"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLeaderGetters(t *testing.T) {
	l, err := NewLeader(testLeaderStr)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range leaderTestFields() {
		got := f.get(l)
		if got != f.want {
			t.Errorf("%s: got %q want %q", f.name, got, f.want)
		}
		sliceVal := l.Slice(f.start, f.end)
		if sliceVal != f.want {
			t.Errorf("%s: slice got %q want %q", f.name, sliceVal, f.want)
		}
	}
}

func TestLeaderSetters(t *testing.T) {
	for _, f := range leaderTestFields() {
		l, err := NewLeader(testLeaderStr)
		if err != nil {
			t.Fatal(err)
		}
		value := randomLowerString(len(f.want))
		if err := f.set(l, value); err != nil {
			t.Fatalf("%s: set: %v", f.name, err)
		}
		if got := f.get(l); got != value {
			t.Errorf("%s: got %q want %q", f.name, got, value)
		}
	}
}

func TestLeaderSettersErrors(t *testing.T) {
	for _, f := range leaderTestFields() {
		l, err := NewLeader(testLeaderStr)
		if err != nil {
			t.Fatal(err)
		}
		value := randomLowerString(len(f.want) + 1)
		if err := f.set(l, value); !errors.Is(err, ErrBadLeaderValue) {
			t.Errorf("%s: want ErrBadLeaderValue, got %v", f.name, err)
		}
	}
}
