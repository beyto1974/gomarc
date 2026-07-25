package marc

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"testing"
)

// Ported from test/test_json.py.

func TestJSONAsDictSingle(t *testing.T) {
	r := mustRecord(t)
	r.AddField(NewDataField("245", "1", "0", Subfield{Code: "a", Value: "Python"}, Subfield{Code: "c", Value: "Guido"}))
	got := r.AsDict()

	want := map[string]any{
		"leader": "          22        4500",
		"fields": []map[string]any{
			{
				"245": map[string]any{
					"ind1":      "1",
					"ind2":      "0",
					"subfields": []map[string]string{{"a": "Python"}, {"c": "Guido"}},
				},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v want %#v", got, want)
	}
}

func TestJSONAsJSONSimple(t *testing.T) {
	r := mustRecord(t)
	r.AddField(NewDataField("245", "1", "0", Subfield{Code: "a", Value: "Python"}, Subfield{Code: "c", Value: "Guido"}))
	s, err := r.AsJSON()
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(s), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["leader"] != "          22        4500" {
		t.Errorf("got leader %v", decoded["leader"])
	}
	fields, _ := decoded["fields"].([]any)
	if len(fields) != 1 {
		t.Fatalf("want 1 field, got %d", len(fields))
	}
	f0 := fields[0].(map[string]any)
	field245, ok := f0["245"]
	if !ok {
		t.Fatal("want 245 key")
	}
	want := map[string]any{
		"ind1": "1",
		"ind2": "0",
		"subfields": []any{
			map[string]any{"a": "Python"},
			map[string]any{"c": "Guido"},
		},
	}
	if !reflect.DeepEqual(field245, want) {
		t.Errorf("got %#v want %#v", field245, want)
	}
}

func TestParseJSONRoundtripFromFile(t *testing.T) {
	data := mustReadFile(t, "test.json")
	var original []any
	if err := json.Unmarshal(sanitizeLenientJSON(data), &original); err != nil {
		t.Fatal(err)
	}

	recs, err := ParseJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != len(original) {
		t.Fatalf("want %d records, got %d", len(original), len(recs))
	}
	for i, rec := range recs {
		serialized, err := rec.AsJSON()
		if err != nil {
			t.Fatal(err)
		}
		var deserialized any
		if err := json.Unmarshal([]byte(serialized), &deserialized); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(deserialized, original[i]) {
			t.Errorf("record %d: round trip mismatch\ngot  %#v\nwant %#v", i, deserialized, original[i])
		}
	}
}

func TestParseJSONOneRecordNotArray(t *testing.T) {
	all := mustReadFile(t, "test.json")
	var original []any
	if err := json.Unmarshal(sanitizeLenientJSON(all), &original); err != nil {
		t.Fatal(err)
	}
	single, err := json.Marshal(original[0])
	if err != nil {
		t.Fatal(err)
	}

	recs, err := ParseJSON(single)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d", len(recs))
	}
	var got any
	b, err := json.Marshal(recs[0].AsDict())
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, original[0]) {
		t.Errorf("got %#v want %#v", got, original[0])
	}
}

func TestParseJSONXMLEquivalence(t *testing.T) {
	jsonRecords, err := ParseJSON(mustReadFile(t, "batch.json"))
	if err != nil {
		t.Fatal(err)
	}
	xmlRecords, err := ParseXML(mustOpen(t, "batch.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(jsonRecords) != len(xmlRecords) {
		t.Fatalf("want equal counts, got json=%d xml=%d", len(jsonRecords), len(xmlRecords))
	}
	for i := range jsonRecords {
		jb, err := jsonRecords[i].AsMARC()
		if err != nil {
			t.Fatal(err)
		}
		xb, err := xmlRecords[i].AsMARC()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(jb, xb) {
			t.Errorf("record %d: as_marc mismatch between JSON and XML sources", i)
		}
	}
}

func TestJSONReaderArray(t *testing.T) {
	data := mustReadFile(t, "test.json")
	want, err := ParseJSON(data)
	if err != nil {
		t.Fatal(err)
	}

	jr := NewJSONReader(bytes.NewReader(data))
	var got []*Record
	for {
		rec, err := jr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, rec)
	}
	if len(got) != len(want) {
		t.Fatalf("want %d records, got %d", len(want), len(got))
	}
	for i := range got {
		gb, err := got[i].AsMARC()
		if err != nil {
			t.Fatal(err)
		}
		wb, err := want[i].AsMARC()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(gb, wb) {
			t.Errorf("record %d: as_marc mismatch between JSONReader and ParseJSON", i)
		}
	}

	if _, err := jr.Next(); err != io.EOF {
		t.Errorf("want io.EOF after exhausting reader, got %v", err)
	}
}

func TestJSONReaderSingleRecordNotArray(t *testing.T) {
	all := mustReadFile(t, "test.json")
	var original []any
	if err := json.Unmarshal(sanitizeLenientJSON(all), &original); err != nil {
		t.Fatal(err)
	}
	single, err := json.Marshal(original[0])
	if err != nil {
		t.Fatal(err)
	}

	jr := NewJSONReader(bytes.NewReader(single))
	rec, err := jr.Next()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jr.Next(); err != io.EOF {
		t.Errorf("want io.EOF after single bare record, got %v", err)
	}

	b, err := json.Marshal(rec.AsDict())
	if err != nil {
		t.Fatal(err)
	}
	var got any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, original[0]) {
		t.Errorf("got %#v want %#v", got, original[0])
	}
}

func TestJSONReaderLenientControlChars(t *testing.T) {
	// A literal (unescaped) newline embedded in a JSON string value, as
	// produced by some real-world MARC-in-JSON writers (see
	// sanitizeLenientJSON/lenientJSONFilter).
	raw := []byte("{\"leader\":\"          22        4500\",\"fields\":[" +
		"{\"245\":{\"ind1\":\"0\",\"ind2\":\"1\",\"subfields\":[{\"a\":\"line one\nline two\"}]}}]}")

	jr := NewJSONReader(bytes.NewReader(raw))
	rec, err := jr.Next()
	if err != nil {
		t.Fatal(err)
	}
	if want, got := "line one\nline two", rec.Fields[0].Subfields[0].Value; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if _, err := jr.Next(); err != io.EOF {
		t.Errorf("want io.EOF after single bare record, got %v", err)
	}
}

func TestJSONWriter(t *testing.T) {
	r1 := mustRecord(t)
	r1.AddField(NewDataField("245", "0", "1", Subfield{Code: "a", Value: "First"}))
	r2 := mustRecord(t)
	r2.AddField(NewDataField("245", "0", "1", Subfield{Code: "a", Value: "Second"}))

	var buf bytes.Buffer
	w, err := NewJSONWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(r1); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(r2); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	var decoded []any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v (%s)", err, buf.String())
	}
	if len(decoded) != 2 {
		t.Fatalf("want 2 records, got %d", len(decoded))
	}
}
