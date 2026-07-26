// Package schema provides machine-readable semantic descriptions of MARC21
// record structures for use with LLMs and other schema-aware tooling.
//
// It describes every named leader position, the 008 fixed-length field broken
// down by material type, and the field/indicator/subfield vocabulary for all
// defined MARC21 bibliographic tags.
//
// # Usage
//
// Build a schema at the coverage level you need and serialize it to JSON for
// inclusion in an LLM system prompt:
//
//	s := schema.Build(schema.Common) // or schema.All
//	jsonBytes := s.MustJSON()
//
// Look up an individual tag at runtime:
//
//	f, ok := s.Lookup("245")
//	if ok {
//	    fmt.Println(f.Label, f.Definition)
//	}
//
// Look up a leader position:
//
//	lf, ok := s.LookupLeader("09")
//	if ok {
//	    fmt.Println(lf.Label, lf.Values)
//	}
//
// # Coverage
//
// [Common] (the default) includes approximately 120 tags covering the most
// common bibliographic fields — control fields, main entries, title/edition/
// publication fields, notes, subjects, added entries, series, and holdings.
// Each tag carries full indicator and subfield definitions.
//
// [All] adds every remaining LOC-defined MARC21 bibliographic tag on top of
// the common set. Less-common tags carry at minimum a label and definition;
// frequently-used subfields are populated where practical.
//
// # 008 Field
//
// The MARC 008 fixed-length data element field is broken down by material
// type. The correct material-type key is determined from Leader byte 06
// (TypeOfRecord) and byte 07 (BibliographicLevel). [Field008Defs] carries a
// [Field008Defs.Common] slice (positions 00-05 and 35-39, identical for all
// types) and a [Field008Defs.ByType] map keyed by material type name.
package schema
