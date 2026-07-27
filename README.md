# gomarc

`gomarc` reads, writes, and modifies bibliographic records encoded in
[MARC21](https://en.wikipedia.org/wiki/MARC_standards). It's a Go port of the
Python library [pymarc](https://gitlab.com/pymarc/pymarc), covering the binary
MARC21 transmission format, MARC-8 to Unicode conversion, MARC-in-JSON, and
MARCXML.

## Installation

```sh
go get github.com/beyto1974/gomarc@v0.1.0
```

```go
import marc "github.com/beyto1974/gomarc"
```

Public repo, so the normal module proxy (proxy.golang.org) and checksum
database (sum.golang.org) resolve it with no extra setup.

## Reading

```go
f, err := os.Open("marc.dat")
if err != nil {
	log.Fatal(err)
}
defer f.Close()

reader := marc.NewReader(f)
for {
	record, err := reader.Next()
	if errors.Is(err, io.EOF) {
		break
	}
	if err != nil {
		log.Println(err) // Reader is permissive: bad records are skipped, not fatal
		continue
	}
	title, _ := record.Title()
	fmt.Println(title)
}
```

A `*marc.Record` has convenience methods for common fields: `Title`,
`Author`, `ISBN`, `ISSN`, `Subjects`, `Location`, `Notes`,
`PhysicalDescription`, `Publisher`, `PubYear`. For anything else you need the
numeric field tag and subfield code directly:

```go
value, ok := record.Get("245").Subfield("a")
```

Repeating fields (e.g. subjects) come back as a slice via `GetFields`:

```go
for _, f := range record.GetFields("650") {
	fmt.Println(f)
}
```

## Writing

```go
record, err := marc.NewRecord()
if err != nil {
	log.Fatal(err)
}
record.AddField(marc.NewDataField("245", "0", "1",
	marc.Subfield{Code: "a", Value: "The pragmatic programmer : "},
	marc.Subfield{Code: "b", Value: "from journeyman to master /"},
	marc.Subfield{Code: "c", Value: "Andrew Hunt, David Thomas."},
))

out, err := os.Create("file.dat")
if err != nil {
	log.Fatal(err)
}
defer out.Close()

writer := marc.NewWriter(out)
if err := writer.Write(record); err != nil {
	log.Fatal(err)
}
```

## Updating

Read a record in, modify it, write it back out:

```go
record, err := reader.Next()
if err != nil {
	log.Fatal(err)
}
if err := record.Get("245").SetSubfield("a", "The Zombie Programmer : "); err != nil {
	log.Fatal(err)
}

data, err := record.AsMARC()
if err != nil {
	log.Fatal(err)
}
os.WriteFile("file.dat", data, 0o644)
```

## JSON and XML

The main benefit of JSON or XML over binary MARC21 is that they use UTF-8
throughout, rather than the archaic MARC-8 encoding, and can be read with
standard tooling instead of a MARC-specific library.

**JSON**

```go
s, err := record.AsJSON()
```

```go
records, err := marc.ParseJSON(data)
```

**XML**

```go
records, err := marc.ParseXML(r) // r is an io.Reader
```

To stream a large MARCXML file one record at a time instead of loading it all
into memory:

```go
xr := marc.NewXMLReader(r)
for {
	record, err := xr.Next()
	if errors.Is(err, io.EOF) {
		break
	}
	title, _ := record.Title()
	fmt.Println(title)
}
```

## Example CLI

`cmd/marcdump` is a small example program that dumps a binary MARC21 file as
MARCMaker-style text (default) or MARC-in-JSON (`-json`):

```sh
go run ./cmd/marcdump testdata/marc.dat
go run ./cmd/marcdump -json testdata/marc.dat
```

`cmd/marcannotate` reads a single MARC21 record (ISO 2709, MARCXML, or
MARC-in-JSON, auto-detected) and prints it as an LLM-ready Markdown document:
the leader and 008 broken down position-by-position with coded meanings, a
legend of field/indicator/subfield descriptions (one entry per distinct tag,
not per occurrence), and the record's raw field data.

```sh
go run ./cmd/marcannotate testdata/marc.dat
go run ./cmd/marcannotate -coverage all testdata/marc.dat
cat testdata/marc.dat | go run ./cmd/marcannotate -
```

`-coverage` selects the schema depth: `common` (default) or `all`.

## Performance & Comparison with pymarc

`gomarc` features a high-performance, streaming parser architecture designed for high-throughput processing, featuring zero-allocation field tag normalization, direct byte-slice integer parsing, and reflection-free MARCXML decoding.

### Benchmarks (`gomarc` vs. `pymarc`)

| Task / File Format | `pymarc` Execution Time | `gomarc` Execution Time | `pymarc` Memory Footprint | `gomarc` Memory Footprint | Speedup |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **NLM MARCXML Dataset** (`catplus.marcxml.xml`, 2,663 recs, 15.2 MB) | 858.86 ms | **367.78 ms** (138 µs/rec) | ~140 MB | **~25 MB** | **~2.3x faster** |
| **MARCXML Parsing** (`batch.xml`, 2 recs) | 493.92 µs | **155.41 µs** | 3.43 MB | **61.2 KB** (1,559 allocs) | **~3.2x faster** (~56x less memory) |
| **ISO 2709 MARC** (`test.dat`, 10 recs) | 1,922.14 µs | **123.46 µs** | 83.9 KB | **78.6 KB** (2,155 allocs) | **~15.5x faster** |

To run the built-in benchmarks with memory profiling:

```sh
go test -bench=Benchmark -benchmem ./...
```

## Testing

```sh
go test ./...
```

Test fixtures under `testdata/` are copied verbatim from pymarc's own test
suite, so behavior can be cross-checked against the original Python
implementation.
