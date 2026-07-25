package marc

import (
	"bytes"
	"errors"
	"io"
	"os"
	"regexp"
	"testing"
)

// Ported from test/test_reader.py, using fixtures copied verbatim from pymarc's
// test/ directory into testdata/.

func mustReadFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReaderIteratesTestDat(t *testing.T) {
	rdr := NewReaderFromBytes(mustReadFile(t, "test.dat"))
	startsWithLeader := regexp.MustCompile(`^=LDR`)
	hasNumericTag := regexp.MustCompile(`\n=\d\d\d `)
	count := 0
	for {
		rec, err := rdr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error at record %d: %v", count, err)
		}
		text := rec.String()
		if !startsWithLeader.MatchString(text) {
			t.Errorf("record %d: want leader prefix, got %q", count, text)
		}
		if !hasNumericTag.MatchString(text) {
			t.Errorf("record %d: want a numeric tag line", count)
		}
		count++
	}
	if count != 10 {
		t.Errorf("want 10 records, got %d", count)
	}
}

func TestReaderOneDat(t *testing.T) {
	rdr := NewReaderFromBytes(mustReadFile(t, "one.dat"))
	rec, err := rdr.Next()
	if err != nil {
		t.Fatal(err)
	}
	if rec.Get("245") == nil {
		t.Error("want a 245 field")
	}
	if _, err := rdr.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("want io.EOF after single record, got %v", err)
	}
}

func TestReaderBadIndicator(t *testing.T) {
	rdr := NewReaderFromBytes(mustReadFile(t, "bad_indicator.dat"))
	rec, err := rdr.Next()
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := rec.Get("245").Subfield("a"); v != "Aristocrats of color :" {
		t.Errorf("got %q", v)
	}
}

func TestReaderPermissiveBadRecords(t *testing.T) {
	rdr := NewReaderFromBytes(mustReadFile(t, "bad_records.mrc"))
	// Working records occur at (0-indexed) positions 0 and 7 of 9 total yields;
	// see pymarc's MARCReaderFilePermissiveTest.test_permissive_mode.
	wantGood := map[int]bool{0: true, 7: true}
	count := 0
	for {
		rec, err := rdr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if rdr.CurrentChunk() == nil {
			t.Errorf("record %d: want non-nil current chunk", count)
		}
		if wantGood[count] {
			if err != nil {
				t.Errorf("record %d: want valid record, got error %v", count, err)
			} else {
				if v, _ := rec.Get("245").Subfield("a"); v != "The pragmatic programmer : " {
					t.Errorf("record %d: $a got %q", count, v)
				}
				if v, _ := rec.Get("245").Subfield("b"); v != "from journeyman to master /" {
					t.Errorf("record %d: $b got %q", count, v)
				}
				if v, _ := rec.Get("245").Subfield("c"); v != "Andrew Hunt, David Thomas." {
					t.Errorf("record %d: $c got %q", count, v)
				}
			}
		} else if err == nil {
			t.Errorf("record %d: want error, got valid record", count)
		}
		count++
	}
	if count != 9 {
		t.Errorf("want 9 yielded records, got %d", count)
	}
}

func TestReaderTruncatedDataCases(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		rdr := NewReaderFromBytes([]byte(""))
		if _, err := rdr.Next(); !errors.Is(err, io.EOF) {
			t.Errorf("want io.EOF, got %v", err)
		}
	})

	t.Run("partial length", func(t *testing.T) {
		data := []byte("0012")
		rdr := NewReaderFromBytes(data)
		_, err := rdr.Next()
		if !errors.Is(err, ErrTruncatedRecord) {
			t.Errorf("want ErrTruncatedRecord, got %v", err)
		}
		if !bytes.Equal(rdr.CurrentChunk(), data) {
			t.Errorf("got chunk %q want %q", rdr.CurrentChunk(), data)
		}
	})

	t.Run("bad length", func(t *testing.T) {
		data := []byte("0012X")
		rdr := NewReaderFromBytes(data)
		_, err := rdr.Next()
		if !errors.Is(err, ErrRecordLengthInvalid) {
			t.Errorf("want ErrRecordLengthInvalid, got %v", err)
		}
		if !bytes.Equal(rdr.CurrentChunk(), data) {
			t.Errorf("got chunk %q want %q", rdr.CurrentChunk(), data)
		}
	})

	t.Run("partial data", func(t *testing.T) {
		data := []byte("00120cam")
		rdr := NewReaderFromBytes(data)
		_, err := rdr.Next()
		if !errors.Is(err, ErrTruncatedRecord) {
			t.Errorf("want ErrTruncatedRecord, got %v", err)
		}
		if !bytes.Equal(rdr.CurrentChunk(), data) {
			t.Errorf("got chunk %q want %q", rdr.CurrentChunk(), data)
		}
	})

	t.Run("missing end of record", func(t *testing.T) {
		data := []byte("00006 ")
		rdr := NewReaderFromBytes(data)
		_, err := rdr.Next()
		if !errors.Is(err, ErrEndOfRecordNotFound) {
			t.Errorf("want ErrEndOfRecordNotFound, got %v", err)
		}
		if !bytes.Equal(rdr.CurrentChunk(), data) {
			t.Errorf("got chunk %q want %q", rdr.CurrentChunk(), data)
		}
	})
}

func BenchmarkReader(b *testing.B) {
	data, err := os.ReadFile("testdata/test.dat")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rdr := NewReaderFromBytes(data)
		for {
			_, err := rdr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
