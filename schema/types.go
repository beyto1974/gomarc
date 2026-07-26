package schema

// Coverage selects how many MARC21 tags are included in the built [Schema].
type Coverage int

const (
	// Common includes approximately 120 tags covering the most-used
	// bibliographic fields. Every tag carries full indicator and subfield
	// definitions.
	Common Coverage = iota

	// All includes every LOC-defined MARC21 bibliographic tag on top of the
	// common set. Less-common tags carry at minimum a label and definition.
	All
)

// Schema is the top-level object returned by [Build]. It contains the leader
// field definitions, the 008 fixed-length field breakdown by material type,
// and a list of MARC21 field descriptions at the requested [Coverage] level.
type Schema struct {
	// LeaderFields describes all named positions/ranges of the 24-byte leader.
	LeaderFields []LeaderField `json:"leader_fields"`

	// Field008 describes the 008 fixed-length field broken down by material type.
	Field008 Field008Defs `json:"field_008"`

	// Fields is the list of MARC field descriptions (control + data).
	Fields []Field `json:"fields"`

	// Coverage records which coverage level was used when building this schema.
	Coverage string `json:"coverage"`
}

// LeaderField describes one named position (or range) within the 24-byte
// MARC21 record leader.
type LeaderField struct {
	// Position is the 0-based character position or range, e.g. "00-04" or "05".
	Position string `json:"position"`

	// Name is the Go accessor method name on *Leader, e.g. "RecordLength".
	Name string `json:"name"`

	// Label is the official LOC label for this position.
	Label string `json:"label"`

	// Definition describes what the position means and how it is used.
	Definition string `json:"definition"`

	// Values maps each defined coded character to its meaning.
	// Nil when the position holds a free-form numeric or computed value.
	Values map[string]string `json:"values,omitempty"`
}

// Field008Defs holds the per-material-type sub-position breakdown of MARC 008.
type Field008Defs struct {
	// Common lists the sub-positions that are identical for every material type:
	// positions 00-05 (date entered on file, type of date/pub status, dates 1
	// and 2, place of publication) and 35-39 (language, cataloging source).
	Common []Field008Pos `json:"common"`

	// ByType maps a material-type label to its specific positions 06-34.
	// Valid keys: "Books", "Serials", "Maps", "Music", "VisualMaterials",
	// "ComputerFiles", "MixedMaterials".
	//
	// To pick the correct key, inspect the MARC leader:
	//   Leader[06] TypeOfRecord + Leader[07] BibliographicLevel
	//
	//   Books:           TypeOfRecord=a/t  AND BibliographicLevel=a/c/d/m
	//   Serials:         TypeOfRecord=a    AND BibliographicLevel=b/i/s
	//   Maps:            TypeOfRecord=e/f
	//   Music:           TypeOfRecord=c/d/i/j
	//   VisualMaterials: TypeOfRecord=g/k/o/r
	//   ComputerFiles:   TypeOfRecord=m
	//   MixedMaterials:  TypeOfRecord=p
	ByType map[string][]Field008Pos `json:"by_type"`
}

// Field008Pos describes one fixed character position within the MARC 008 field.
type Field008Pos struct {
	// Position is the 0-based character position or range, e.g. "00-05" or "18".
	Position string `json:"position"`

	// Label is the official LOC name for this sub-position.
	Label string `json:"label"`

	// Definition describes the semantics and usage of this position.
	Definition string `json:"definition"`

	// Values maps each coded character to its meaning.
	// Nil when the position holds a free-form or fill-character value.
	Values map[string]string `json:"values,omitempty"`
}

// Field describes one MARC21 field — either a control field (tag < "010")
// or a data field.
type Field struct {
	// Tag is the 3-digit MARC tag, e.g. "001", "245".
	Tag string `json:"tag"`

	// Label is the official LOC name for this field.
	Label string `json:"label"`

	// Definition describes the purpose and content of the field.
	Definition string `json:"definition"`

	// Repeatable reports whether the field may appear more than once per record.
	Repeatable bool `json:"repeatable"`

	// Control is true for control fields (tags 001-009), which carry raw Data
	// rather than indicators and subfields.
	Control bool `json:"control,omitempty"`

	// Common is true when this tag is included in [Common] coverage.
	Common bool `json:"common"`

	// Indicators describes the two indicator positions. Both are empty
	// (zero-value [Indicator]) for control fields.
	Indicators [2]Indicator `json:"indicators,omitempty"`

	// Subfields lists the defined subfield codes for this field.
	// Empty for control fields (their data is unstructured).
	Subfields []Subfield `json:"subfields,omitempty"`
}

// Indicator describes one indicator position (first or second) of a data field.
type Indicator struct {
	// Label is the official LOC label for this indicator position.
	Label string `json:"label,omitempty"`

	// Values maps each defined indicator character to its meaning.
	// "#" represents a blank (space) indicator.
	// Nil when the indicator is undefined or not applicable.
	Values map[string]string `json:"values,omitempty"`
}

// Subfield describes one subfield code within a MARC21 data field.
type Subfield struct {
	// Code is the single-character subfield delimiter code, e.g. "a", "b", "6".
	Code string `json:"code"`

	// Label is the official LOC name for this subfield.
	Label string `json:"label"`

	// Definition describes the content and usage of the subfield.
	Definition string `json:"definition,omitempty"`

	// Repeatable reports whether the subfield may appear more than once in the field.
	Repeatable bool `json:"repeatable"`
}
