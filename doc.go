// Package marc reads, writes, and modifies bibliographic records encoded in
// MARC21 (https://en.wikipedia.org/wiki/MARC_standards).
//
// It is a Go port of the Python library pymarc (https://gitlab.com/pymarc/pymarc),
// covering the binary MARC21 transmission format, MARC-8 to Unicode conversion,
// MARC-in-JSON, and MARCXML.
//
// # Reading
//
// Read a batch of binary MARC21 records and print each title:
//
//	f, _ := os.Open("marc.dat")
//	reader := marc.NewReader(f)
//	for {
//		record, err := reader.Next()
//		if errors.Is(err, io.EOF) {
//			break
//		}
//		title, _ := record.Title()
//		fmt.Println(title)
//	}
//
// # Writing
//
// Build a record and write it out:
//
//	record, _ := marc.NewRecord()
//	record.AddField(marc.NewDataField("245", "0", "1",
//		marc.Subfield{Code: "a", Value: "The pragmatic programmer : "},
//		marc.Subfield{Code: "b", Value: "from journeyman to master /"},
//	))
//	out, _ := os.Create("file.dat")
//	writer := marc.NewWriter(out)
//	writer.Write(record)
//
// # JSON and XML
//
// Records can also be (de)serialized as MARC-in-JSON or MARCXML, which use
// UTF-8 throughout rather than MARC-8:
//
//	records, _ := marc.ParseJSON(data)
//	records, _ := marc.ParseXML(r)
package marc
