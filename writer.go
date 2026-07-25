package marc

import "io"

// Writer writes MARC21 records in transmission format to an io.Writer.
// Ported from pymarc.writer.MARCWriter.
type Writer struct {
	dst io.Writer
}

// NewWriter builds a binary MARC21 Writer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{dst: w}
}

// Write serializes and writes a single record.
func (w *Writer) Write(r *Record) error {
	data, err := r.AsMARC()
	if err != nil {
		return err
	}
	_, err = w.dst.Write(data)
	return err
}

// TextWriter writes records in prettified MARCMaker text format, separated by
// a blank line. Ported from pymarc.writer.TextWriter.
type TextWriter struct {
	dst        io.Writer
	writeCount int
}

// NewTextWriter builds a MARCMaker-format TextWriter.
func NewTextWriter(w io.Writer) *TextWriter {
	return &TextWriter{dst: w}
}

// Write writes a single record, preceded by a blank line if this isn't the first.
func (w *TextWriter) Write(r *Record) error {
	if w.writeCount > 0 {
		if _, err := io.WriteString(w.dst, "\n"); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w.dst, r.String()); err != nil {
		return err
	}
	w.writeCount++
	return nil
}
