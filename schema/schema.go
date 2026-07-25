package schema

import "encoding/json"

// Build constructs a [Schema] for the given coverage level.
// Pass [Common] for the ~120 most-used bibliographic tags, or [All] for the
// complete MARC21 tag set.
func Build(coverage Coverage) *Schema {
	s := &Schema{
		LeaderFields: leaderFields,
		Field008:     field008Defs,
		Fields:       commonFields,
		Coverage:     "common",
	}
	if coverage == All {
		fields := make([]Field, 0, len(commonFields)+len(allFields))
		fields = append(fields, commonFields...)
		fields = append(fields, allFields...)
		s.Fields = fields
		s.Coverage = "all"
	}
	return s
}

// Lookup returns the [Field] description for the given 3-digit tag,
// or the zero value and false if not found.
func (s *Schema) Lookup(tag string) (Field, bool) {
	for _, f := range s.Fields {
		if f.Tag == tag {
			return f, true
		}
	}
	return Field{}, false
}

// LookupLeader returns the [LeaderField] description for the given position
// string (e.g. "05", "00-04").
func (s *Schema) LookupLeader(position string) (LeaderField, bool) {
	for _, l := range s.LeaderFields {
		if l.Position == position {
			return l, true
		}
	}
	return LeaderField{}, false
}

// AsJSON serializes the entire Schema to indented JSON, suitable for
// embedding directly in an LLM system prompt.
func (s *Schema) AsJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// MustJSON is like [Schema.AsJSON] but panics on error. Safe to call because
// Schema always marshals successfully.
func (s *Schema) MustJSON() []byte {
	b, err := s.AsJSON()
	if err != nil {
		panic(err)
	}
	return b
}
