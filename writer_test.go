package marc

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestWriterRoundTrip(t *testing.T) {
	r := mustRecord(t)
	r.AddField(NewDataField("245", "0", "1", Subfield{Code: "a", Value: "Test Title"}))

	want, err := r.AsMARC()
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.Write(r); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("got %q want %q", buf.Bytes(), want)
	}

	rdr := NewReaderFromBytes(buf.Bytes())
	rec, err := rdr.Next()
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := rec.Get("245").Subfield("a"); v != "Test Title" {
		t.Errorf("got %q", v)
	}
	if _, err := rdr.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("want io.EOF, got %v", err)
	}
}

func TestTextWriter(t *testing.T) {
	r1 := mustRecord(t)
	r1.AddField(NewDataField("245", "0", "1", Subfield{Code: "a", Value: "First"}))
	r2 := mustRecord(t)
	r2.AddField(NewDataField("245", "0", "1", Subfield{Code: "a", Value: "Second"}))

	var buf strings.Builder
	w := NewTextWriter(&buf)
	if err := w.Write(r1); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(r2); err != nil {
		t.Fatal(err)
	}
	want := r1.String() + "\n" + r2.String()
	if buf.String() != want {
		t.Errorf("got %q want %q", buf.String(), want)
	}
}
