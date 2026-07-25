package marc

import (
	"bytes"
	"io"
	"strconv"
)

// ReaderOption configures a Reader; shares option constructors with NewRecord
// (leader/fields/data options are ignored by the reader).
type ReaderOption = RecordOption

// Reader iterates over MARC21 records in transmission format read from an
// io.Reader. Ported from pymarc.reader.MARCReader.
//
// It is permissive: a bad record yields (nil, err) from Next but does not stop
// iteration, unless the error is fatal (the record's length/boundary could not
// be determined), in which case every subsequent Next call returns io.EOF.
type Reader struct {
	src              io.Reader
	toUnicode        bool
	forceUTF8        bool
	hideUTF8Warnings bool
	utf8Handling     string
	fileEncoding     string

	currentChunk []byte
	fatal        bool
}

// NewReader builds a Reader over r (or over raw bytes via NewReaderFromBytes).
func NewReader(r io.Reader, opts ...ReaderOption) *Reader {
	cfg := &recordConfig{toUnicode: true, utf8Handling: "strict", fileEncoding: "iso8859-1"}
	for _, opt := range opts {
		opt(cfg)
	}
	return &Reader{
		src:              r,
		toUnicode:        cfg.toUnicode,
		forceUTF8:        cfg.forceUTF8,
		hideUTF8Warnings: cfg.hideUTF8Warnings,
		utf8Handling:     cfg.utf8Handling,
		fileEncoding:     cfg.fileEncoding,
	}
}

// NewReaderFromBytes builds a Reader over an in-memory MARC blob.
func NewReaderFromBytes(data []byte, opts ...ReaderOption) *Reader {
	return NewReader(bytes.NewReader(data), opts...)
}

// CurrentChunk returns the raw bytes of the most recently attempted record.
func (rd *Reader) CurrentChunk() []byte { return rd.currentChunk }

// Next reads and decodes the next record. It returns (nil, io.EOF) once the
// underlying reader is exhausted, or once a fatal boundary error has occurred.
// A non-fatal decode error is returned as (nil, err); the reader remains usable
// for subsequent Next calls.
func (rd *Reader) Next() (*Record, error) {
	if rd.fatal {
		return nil, io.EOF
	}
	rd.currentChunk = nil

	first5 := make([]byte, 5)
	n, err := io.ReadFull(rd.src, first5)
	if n == 0 && err == io.EOF {
		return nil, io.EOF
	}
	if err != nil {
		rd.currentChunk = first5[:n]
		rd.fatal = true
		return nil, ErrTruncatedRecord
	}
	rd.currentChunk = first5

	length, convErr := strconv.Atoi(string(first5))
	if convErr != nil {
		rd.fatal = true
		return nil, ErrRecordLengthInvalid
	}

	rest := make([]byte, length-5)
	n2, err2 := io.ReadFull(rd.src, rest)
	chunk := append(first5, rest[:n2]...)
	rd.currentChunk = chunk
	if err2 != nil || len(chunk) < length {
		rd.fatal = true
		return nil, ErrTruncatedRecord
	}

	if chunk[len(chunk)-1] != EndOfRecord {
		rd.fatal = true
		return nil, ErrEndOfRecordNotFound
	}

	rec, err := NewRecord(
		WithData(chunk),
		WithToUnicode(rd.toUnicode),
		WithForceUTF8(rd.forceUTF8),
		WithHideUTF8Warnings(rd.hideUTF8Warnings),
		WithUTF8Handling(rd.utf8Handling),
		WithFileEncoding(rd.fileEncoding),
	)
	if err != nil {
		return nil, err
	}
	return rec, nil
}
