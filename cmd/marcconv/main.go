// Command marcconv streams MARC21 records between binary MARC, MARCXML, and
// MARC-in-JSON (and out to MARCMaker text), converting one record at a time
// rather than loading the whole input into memory.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	marc "github.com/beyto1974/gomarc"
)

func main() {
	from := flag.String("from", "", "input format: marc, xml, json (default: guessed from the input file's extension)")
	to := flag.String("to", "", "output format: marc, xml, json, text (default: guessed from -o's extension, else text)")
	out := flag.String("o", "", "output file (default: stdout)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: marcconv [-from marc|xml|json] [-to marc|xml|json|text] [-o output] [input]\n\n"+
			"Streams MARC21 records from input (or stdin) to the target format, one\n"+
			"record at a time. With neither -from nor -to given, formats are guessed\n"+
			"from the .mrc/.marc/.xml/.json/.txt extensions of the input file and -o.\n"+
			"Flags may appear before or after the input file.\n\n")
		flag.PrintDefaults()
	}
	// flag.Parse stops at the first non-flag argument, so flags placed after
	// the input file (e.g. "marcconv in.dat -o out.json", as shown in the
	// README) would otherwise be misread as extra positional arguments.
	// Reorder so flags always reach the parser regardless of where they sit.
	if err := flag.CommandLine.Parse(reorderArgs(os.Args[1:])); err != nil {
		os.Exit(2)
	}

	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}
	var inPath string
	if flag.NArg() == 1 {
		inPath = flag.Arg(0)
	}

	in := io.Reader(os.Stdin)
	if inPath != "" {
		f, err := os.Open(inPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "marcconv:", err)
			os.Exit(1)
		}
		defer func() { _ = f.Close() }()
		in = f
	}

	fromFmt := *from
	if fromFmt == "" {
		fromFmt = guessFormat(inPath)
		if fromFmt == "" {
			fmt.Fprintln(os.Stderr, "marcconv: -from is required (input format could not be guessed)")
			os.Exit(2)
		}
	}

	toFmt := *to
	if toFmt == "" {
		toFmt = guessFormat(*out)
		if toFmt == "" {
			toFmt = "text"
		}
	}

	w := io.Writer(os.Stdout)
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "marcconv:", err)
			os.Exit(1)
		}
		defer func() { _ = f.Close() }()
		w = f
	}
	bw := bufio.NewWriter(w)

	err := convert(in, bw, fromFmt, toFmt)
	if flushErr := bw.Flush(); flushErr != nil && err == nil {
		err = flushErr
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "marcconv:", err)
		os.Exit(1)
	}
}

// reorderArgs moves recognized flag tokens (and, for flags that take a
// value, the token following them) ahead of positional arguments, so that
// flag.Parse — which stops scanning at the first non-flag token — sees them
// regardless of where the user placed them relative to the input file.
func reorderArgs(args []string) []string {
	valueFlags := map[string]bool{"from": true, "to": true, "o": true}
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		name, isFlag := strings.CutPrefix(a, "--")
		if !isFlag {
			name, isFlag = strings.CutPrefix(a, "-")
		}
		if !isFlag || name == "" {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		if !strings.Contains(name, "=") && valueFlags[name] && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

// guessFormat maps a file extension to a format name, or "" if unrecognized
// (including when path is empty, e.g. stdin/stdout).
func guessFormat(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mrc", ".marc", ".dat":
		return "marc"
	case ".xml":
		return "xml"
	case ".json":
		return "json"
	case ".txt":
		return "text"
	default:
		return ""
	}
}

// recordSource is the shape shared by marc.Reader, marc.XMLReader, and
// marc.JSONReader.
type recordSource interface {
	Next() (*marc.Record, error)
}

// recordSink is the shape shared by marc.Writer, marc.TextWriter,
// marc.XMLWriter, and marc.JSONWriter.
type recordSink interface {
	Write(*marc.Record) error
}

func newSource(r io.Reader, format string) (recordSource, error) {
	switch format {
	case "marc":
		return marc.NewReader(r), nil
	case "xml":
		return marc.NewXMLReader(r), nil
	case "json":
		return marc.NewJSONReader(r), nil
	default:
		return nil, fmt.Errorf("unsupported input format %q (want marc, xml, or json)", format)
	}
}

// newSink builds a recordSink for format, plus a close func that flushes any
// closing syntax (e.g. "</collection>", "]"). close is a no-op for formats
// that don't need one.
func newSink(w io.Writer, format string) (sink recordSink, closeFn func() error, err error) {
	noop := func() error { return nil }
	switch format {
	case "marc":
		return marc.NewWriter(w), noop, nil
	case "text":
		return marc.NewTextWriter(w), noop, nil
	case "xml":
		xw, err := marc.NewXMLWriter(w)
		if err != nil {
			return nil, nil, err
		}
		return xw, xw.Close, nil
	case "json":
		jw, err := marc.NewJSONWriter(w)
		if err != nil {
			return nil, nil, err
		}
		return jw, jw.Close, nil
	default:
		return nil, nil, fmt.Errorf("unsupported output format %q (want marc, xml, json, or text)", format)
	}
}

// convert streams every record from r (in fromFmt) to w (in toFmt). It stops
// at the first decode or write error rather than skipping bad records, since
// only the binary marc.Reader documents that it's safe to keep calling Next
// after a non-fatal error.
func convert(r io.Reader, w io.Writer, fromFmt, toFmt string) error {
	src, err := newSource(r, fromFmt)
	if err != nil {
		return err
	}
	sink, closeSink, err := newSink(w, toFmt)
	if err != nil {
		return err
	}

	count := 0
	for {
		rec, err := src.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("reading record %d: %w", count+1, err)
		}
		count++
		if err := sink.Write(rec); err != nil {
			return fmt.Errorf("writing record %d: %w", count, err)
		}
	}
	return closeSink()
}
