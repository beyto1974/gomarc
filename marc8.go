package marc

// marc8ToUnicode is a placeholder MARC-8 to Unicode decoder used until the real
// escape-sequence-driven converter (ported from pymarc/marc8.py + marc8_mapping.py)
// lands. It treats bytes as Latin-1 code points, which is wrong for actual MARC-8
// escape sequences but keeps the pipeline compiling and producing valid Unicode in
// the meantime. TODO(phase 5): replace with the real G0/G1 state machine + codeset
// tables.
func marc8ToUnicode(data []byte, hideWarnings bool) (string, error) {
	_ = hideWarnings
	runes := make([]rune, len(data))
	for i, b := range data {
		runes[i] = rune(b)
	}
	return string(runes), nil
}
