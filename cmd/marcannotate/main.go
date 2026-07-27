// Command marcannotate reads a single MARC21 biblio record (ISO 2709,
// MARCXML, or MARC-in-JSON, auto-detected) and prints it annotated with the
// gomarc/schema field/indicator/subfield descriptions as Markdown, for use
// as LLM context.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	marc "github.com/beyto1974/gomarc"
	"github.com/beyto1974/gomarc/annotate"
	"github.com/beyto1974/gomarc/schema"
)

func main() {
	coverage := flag.String("coverage", "common", "schema coverage: common or all")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: marcannotate [-coverage common|all] <file|->\n")
		os.Exit(2)
	}

	var data []byte
	var err error
	if flag.Arg(0) == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(flag.Arg(0))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rec, err := marc.ParseAny(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cov := schema.Common
	if *coverage == "all" {
		cov = schema.All
	}

	ar := annotate.Build(rec, cov)
	fmt.Print(ar.Markdown())
}
