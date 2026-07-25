package marc

import (
	"fmt"
	"os"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// MARC-8 to Unicode conversion. Ported from pymarc/marc8.py, using the code
// tables generated into marc8_mapping.go (see tools/gen_marc8_mapping.py).
//
// Warning: MARC-8 EACC (East Asian characters) makes some distinctions that
// aren't captured in Unicode; like pymarc, we map such characters to a
// substitute character that (usually) conveys the sense. Treat the MARC-8
// bytes as primary and the Unicode as display-only.

const (
	marc8BasicLatin byte = 0x42
	marc8Ansel      byte = 0x45
	marc8EACC       byte = 0x31 // multibyte (3-byte) charset designator
)

var (
	marc8G0Escapes = map[byte]bool{'(': true, ',': true, '$': true}
	marc8G1Escapes = map[byte]bool{')': true, '-': true, '$': true}
)

// marc8Converter holds the G0/G1 charset state across a MARC8ToUnicode call,
// mirroring pymarc's MARC8ToUnicode class.
type marc8Converter struct {
	g0    byte
	g1    byte
	quiet bool
}

func newMARC8Converter(quiet bool) *marc8Converter {
	return &marc8Converter{g0: marc8BasicLatin, g1: marc8Ansel, quiet: quiet}
}

func byteAt(data []byte, i int) (byte, bool) {
	if i < 0 || i >= len(data) {
		return 0, false
	}
	return data[i], true
}

// translate converts MARC-8 bytes to a NFC-normalized Unicode string.
func (c *marc8Converter) translate(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var uniList []rune
	var combinings []rune
	pos := 0

	for pos < len(data) {
		if data[pos] == 0x1b {
			nextByte, hasNext := byteAt(data, pos+1)
			switch {
			case hasNext && marc8G0Escapes[nextByte]:
				if len(data) >= pos+3 {
					b2, _ := byteAt(data, pos+2)
					if nextByte == '$' && b2 == ',' {
						pos++
					}
					g0, _ := byteAt(data, pos+2)
					c.g0 = g0
					pos += 3
					continue
				}
				uniList = append(uniList, rune(data[pos]))
				pos++
				continue
			case hasNext && marc8G1Escapes[nextByte]:
				b2, _ := byteAt(data, pos+2)
				if nextByte == '$' && b2 == '-' {
					pos++
				}
				g1, _ := byteAt(data, pos+2)
				c.g1 = g1
				pos += 3
				continue
			default:
				if hasNext {
					charset := nextByte
					if _, ok := marc8Codesets[charset]; ok {
						c.g0 = charset
						pos += 2
					} else if charset == 0x73 {
						c.g0 = marc8BasicLatin
						pos += 2
						if pos == len(data) {
							goto doneLoop
						}
					}
					// else: matches pymarc's fallthrough with neither branch
					// taken - g0/pos untouched, falls into decoding below.
				}
			}
		}

		mbFlag := c.g0 == marc8EACC
		var codePoint int
		if mbFlag {
			if len(data) < pos+3 {
				fmt.Fprintf(os.Stderr, "Multi-byte position %d exceeds length of marc8 string %d\n", pos+3, len(data))
				codePoint = 32
			} else {
				codePoint = int(data[pos])*65536 + int(data[pos+1])*256 + int(data[pos+2])
			}
			pos += 3
		} else {
			codePoint = int(data[pos])
			pos++
		}

		if codePoint < 0x20 || (0x80 < codePoint && codePoint < 0xA0) {
			// Matches pymarc: the byte is dropped (not appended) but still consumed.
			continue
		}

		var uni rune
		var cflag bool
		var found bool
		if codePoint > 0x80 && !mbFlag {
			if entry, ok := marc8Codesets[c.g1][codePoint]; ok {
				uni, cflag, found = entry.Unicode, entry.Combining, true
			}
		} else {
			if entry, ok := marc8Codesets[c.g0][codePoint]; ok {
				uni, cflag, found = entry.Unicode, entry.Combining, true
			}
		}
		if !found {
			if odd, ok := marc8OddMap[codePoint]; ok {
				uniList = append(uniList, odd)
				continue
			}
			if !c.quiet {
				fmt.Fprintf(os.Stderr, "Unable to parse character 0x%x in g0=%d g1=%d\n", codePoint, c.g0, c.g1)
			}
			uni, cflag = ' ', false
		}

		if cflag {
			combinings = append(combinings, uni)
		} else {
			uniList = append(uniList, uni)
			if len(combinings) > 0 {
				uniList = append(uniList, combinings...)
				combinings = nil
			}
		}
	}
doneLoop:

	return norm.NFC.String(string(uniList))
}

// marc8ToUnicode converts MARC-8 encoded bytes to a Unicode string, matching
// pymarc's marc8_to_unicode().
func marc8ToUnicode(data []byte, hideWarnings bool) (string, error) {
	c := newMARC8Converter(hideWarnings)
	s := c.translate(data)
	if !utf8.ValidString(s) {
		return "", fmt.Errorf("marc8_to_unicode: invalid multibyte character encoding")
	}
	return s, nil
}
