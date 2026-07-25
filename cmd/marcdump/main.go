// Command marcdump reads a binary MARC21 file and prints each record, either
// as MARCMaker-style text (default) or MARC-in-JSON (-json).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	marc "marc21"
)

func main() {
	jsonOut := flag.Bool("json", false, "print records as MARC-in-JSON instead of MARCMaker text")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: marcdump [-json] <file.mrc>\n")
		os.Exit(2)
	}

	f, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()

	if *jsonOut {
		fmt.Print("[")
	}
	first := true
	rdr := marc.NewReader(f)
	for {
		rec, err := rdr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		if *jsonOut {
			s, err := rec.AsJSON()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			if !first {
				fmt.Print(",")
			}
			fmt.Print(s)
			first = false
		} else {
			fmt.Println(rec.String())
		}
	}
	if *jsonOut {
		fmt.Println("]")
	}
}
