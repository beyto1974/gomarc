// Package annotate pairs a MARC21 [marc.Record] with the machine-readable
// field descriptions from [schema], producing output suitable for embedding
// directly in an LLM prompt or context window.
package annotate

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	marc "github.com/beyto1974/gomarc"
	"github.com/beyto1974/gomarc/schema"
)

// Position describes one leader or 008 fixed-length position paired with the
// raw value found in a specific record and, if defined, its coded meaning.
type Position struct {
	Position string
	Label    string
	Value    string
	Meaning  string
}

// Field008 is the 008 control field broken down by the material type
// inferred from the record's leader.
type Field008 struct {
	MaterialType string
	Positions    []Position
}

// IndicatorLegend describes one indicator position's meaning, independent of
// any single field occurrence's value.
type IndicatorLegend struct {
	Label  string
	Values map[string]string
}

// SubfieldLegend describes one subfield code's meaning, independent of any
// single field occurrence's value.
type SubfieldLegend struct {
	Code       string
	Label      string
	Definition string
}

// FieldLegend is the schema description for one distinct tag appearing in a
// record. It appears once per tag in [Record.Legend] no matter how many
// times the tag repeats in [Record.Fields], so a record with e.g. ten 650
// fields carries the 650 definition/indicator/subfield meanings exactly once.
type FieldLegend struct {
	Tag        string
	Label      string
	Definition string
	Indicators []IndicatorLegend
	Subfields  []SubfieldLegend
}

// DataSubfield is one subfield's raw code/value, undecorated — its meaning
// lives once in the matching [FieldLegend].
type DataSubfield struct {
	Code  string
	Value string
}

// DataField is one field occurrence's raw content: tag, indicators, and
// subfields (or Data for a control field). Look up Tag in [Record.Legend]
// for its meaning.
type DataField struct {
	Tag       string
	Control   bool
	Data      string
	Ind1      string
	Ind2      string
	Subfields []DataSubfield
}

// Record is a MARC21 record with its leader and 008 field broken down
// positionally, a Legend of schema descriptions for every distinct tag
// present (each appearing once), and the record's Fields as raw values.
type Record struct {
	Leader   []Position
	Field008 *Field008
	Legend   []FieldLegend
	Fields   []DataField
}

// parsePosition parses a schema position string ("05" or "12-16") into a
// half-open [start,end) byte range.
func parsePosition(pos string) (start, end int) {
	if i := strings.IndexByte(pos, '-'); i != -1 {
		start, _ = strconv.Atoi(pos[:i])
		end, _ = strconv.Atoi(pos[i+1:])
		return start, end + 1
	}
	p, _ := strconv.Atoi(pos)
	return p, p + 1
}

// materialType picks the schema.Field008Defs.ByType key for a record, from
// Leader/06 (TypeOfRecord) and Leader/07 (BibliographicLevel), matching the
// table documented in schema/types.go. Returns "" if none apply.
func materialType(l *marc.Leader) string {
	typeOfRecord := l.Byte(6)
	bibLevel := l.Byte(7)
	switch {
	case (typeOfRecord == 'a' || typeOfRecord == 't') && strings.ContainsRune("acdm", rune(bibLevel)):
		return "Books"
	case typeOfRecord == 'a' && strings.ContainsRune("bis", rune(bibLevel)):
		return "Serials"
	case typeOfRecord == 'e' || typeOfRecord == 'f':
		return "Maps"
	case strings.ContainsRune("cdij", rune(typeOfRecord)):
		return "Music"
	case strings.ContainsRune("gkor", rune(typeOfRecord)):
		return "VisualMaterials"
	case typeOfRecord == 'm':
		return "ComputerFiles"
	case typeOfRecord == 'p':
		return "MixedMaterials"
	default:
		return ""
	}
}

// lookupMeaning resolves a raw position value against a schema Values map.
// LOC documents blank fixed-field codes as "#" but the raw MARC byte is a
// literal space, so an all-blank value is normalized to "#" before lookup.
func lookupMeaning(values map[string]string, val string) string {
	if values == nil {
		return ""
	}
	if strings.TrimSpace(val) == "" {
		return values["#"]
	}
	return values[val]
}

func positions(l *marc.Leader, defs []schema.LeaderField) []Position {
	out := make([]Position, 0, len(defs))
	for _, d := range defs {
		start, end := parsePosition(d.Position)
		val := l.Slice(start, end)
		out = append(out, Position{Position: d.Position, Label: d.Label, Value: val, Meaning: lookupMeaning(d.Values, val)})
	}
	return out
}

func field008Positions(data string, defs []schema.Field008Pos) []Position {
	out := make([]Position, 0, len(defs))
	for _, d := range defs {
		start, end := parsePosition(d.Position)
		if start < 0 || end > len(data) || start > end {
			continue
		}
		val := data[start:end]
		out = append(out, Position{Position: d.Position, Label: d.Label, Value: val, Meaning: lookupMeaning(d.Values, val)})
	}
	return out
}

// buildLegend builds the schema description for one tag's first occurrence.
// Indicators/subfields are populated only for data fields (isControl false).
func buildLegend(tag string, isControl bool, sch *schema.Schema, usedCodes map[string]bool) FieldLegend {
	fl := FieldLegend{Tag: tag}
	def, found := sch.Lookup(tag)
	if !found {
		fl.Label = "(undefined tag)"
		return fl
	}
	fl.Label = def.Label
	fl.Definition = def.Definition
	if isControl {
		return fl
	}
	for _, ind := range def.Indicators {
		fl.Indicators = append(fl.Indicators, IndicatorLegend{Label: ind.Label, Values: ind.Values})
	}
	for _, sdef := range def.Subfields {
		if usedCodes[sdef.Code] {
			fl.Subfields = append(fl.Subfields, SubfieldLegend{Code: sdef.Code, Label: sdef.Label, Definition: sdef.Definition})
		}
	}
	return fl
}

// Build breaks down r's leader and 008 field positionally, builds a Legend
// entry for every distinct tag present (schema description appears once per
// tag, not once per occurrence), and lists r's fields as raw values, at the
// given coverage level. The result is suitable for an LLM prompt without the
// token cost of repeating a definition for every occurrence of a repeated
// tag (e.g. multiple 650 subject fields).
func Build(r *marc.Record, coverage schema.Coverage) *Record {
	sch := schema.Build(coverage)
	ar := &Record{
		Leader: positions(r.Leader, sch.LeaderFields),
	}

	if f008 := r.Get("008"); f008 != nil && f008.ControlField {
		matType := materialType(r.Leader)
		pos := field008Positions(f008.Data, sch.Field008.Common)
		if byType, ok := sch.Field008.ByType[matType]; ok {
			pos = append(pos, field008Positions(f008.Data, byType)...)
		}
		ar.Field008 = &Field008{MaterialType: matType, Positions: pos}
	}

	var tagOrder []string
	isControl := make(map[string]bool)
	usedCodes := make(map[string]map[string]bool)
	ar.Fields = make([]DataField, 0, len(r.Fields))
	for _, f := range r.Fields {
		if _, ok := isControl[f.Tag]; !ok {
			tagOrder = append(tagOrder, f.Tag)
			isControl[f.Tag] = f.ControlField
		}

		df := DataField{Tag: f.Tag, Control: f.ControlField}
		if f.ControlField {
			df.Data = f.Data
		} else {
			df.Ind1 = f.Indicators.First
			df.Ind2 = f.Indicators.Second
			df.Subfields = make([]DataSubfield, 0, len(f.Subfields))
			codes := usedCodes[f.Tag]
			if codes == nil {
				codes = make(map[string]bool)
				usedCodes[f.Tag] = codes
			}
			for _, sf := range f.Subfields {
				if sf.Value == "" {
					continue
				}
				df.Subfields = append(df.Subfields, DataSubfield{Code: sf.Code, Value: sf.Value})
				codes[sf.Code] = true
			}
		}
		ar.Fields = append(ar.Fields, df)
	}

	ar.Legend = make([]FieldLegend, 0, len(tagOrder))
	for _, tag := range tagOrder {
		ar.Legend = append(ar.Legend, buildLegend(tag, isControl[tag], sch, usedCodes[tag]))
	}
	return ar
}

func writePositionLine(b *strings.Builder, p Position) {
	fmt.Fprintf(b, "- **%s** %s: `%s`", p.Position, p.Label, p.Value)
	if p.Meaning != "" {
		b.WriteString(" -> " + p.Meaning)
	}
	b.WriteByte('\n')
}

// joinValues renders an indicator's coded values as one compact line,
// sorted by code for stable output.
func joinValues(values map[string]string) string {
	codes := make([]string, 0, len(values))
	for k := range values {
		codes = append(codes, k)
	}
	sort.Strings(codes)
	parts := make([]string, len(codes))
	for i, c := range codes {
		parts[i] = c + "=" + values[c]
	}
	return strings.Join(parts, "; ")
}

// dispInd renders a raw indicator character for display, using "#" for
// blank to match LOC documentation convention.
func dispInd(v string) string {
	if v == " " || v == "" {
		return "#"
	}
	return v
}

// Markdown renders the record as an LLM-friendly Markdown document: a
// leader section, an 008 section, a Legend section (one entry per distinct
// tag, with its indicator/subfield meanings), and a Data section (one terse
// line per field occurrence, raw values only).
func (ar *Record) Markdown() string {
	var b strings.Builder

	b.WriteString("## Leader\n")
	for _, p := range ar.Leader {
		writePositionLine(&b, p)
	}

	if ar.Field008 != nil {
		fmt.Fprintf(&b, "\n## 008 (material type: %s)\n", ar.Field008.MaterialType)
		for _, p := range ar.Field008.Positions {
			writePositionLine(&b, p)
		}
	}

	b.WriteString("\n## Legend\n")
	for _, fl := range ar.Legend {
		fmt.Fprintf(&b, "\n### %s — %s\n", fl.Tag, fl.Label)
		if fl.Definition != "" {
			fmt.Fprintf(&b, "%s\n", fl.Definition)
		}
		for i, ind := range fl.Indicators {
			label := ind.Label
			if label == "" {
				label = fmt.Sprintf("Indicator %d", i+1)
			}
			if len(ind.Values) > 0 {
				fmt.Fprintf(&b, "- Ind%d (%s): %s\n", i+1, label, joinValues(ind.Values))
			} else {
				fmt.Fprintf(&b, "- Ind%d (%s)\n", i+1, label)
			}
		}
		for _, sf := range fl.Subfields {
			label := sf.Label
			if label == "" {
				label = "(undefined subfield)"
			}
			line := fmt.Sprintf("- $%s %s", sf.Code, label)
			if sf.Definition != "" {
				line += " — " + sf.Definition
			}
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n## Data\n")
	for _, f := range ar.Fields {
		if f.Control {
			fmt.Fprintf(&b, "%s %s\n", f.Tag, f.Data)
			continue
		}
		fmt.Fprintf(&b, "%s %s%s", f.Tag, dispInd(f.Ind1), dispInd(f.Ind2))
		for _, sf := range f.Subfields {
			fmt.Fprintf(&b, " $%s %s", sf.Code, sf.Value)
		}
		b.WriteByte('\n')
	}

	return b.String()
}
