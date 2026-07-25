#!/usr/bin/env python3
"""One-time generator: dumps pymarc.marc8_mapping.CODESETS/ODD_MAP as Go source.

Run with pymarc importable (e.g. PYTHONPATH pointing at a pip --target install)
and redirect stdout to marc8_mapping.go. Kept for future regeneration if pymarc's
mapping tables change; not part of the Go build.
"""
import sys

import pymarc.marc8_mapping as m

out = sys.stdout

out.write("// Code generated from pymarc/marc8_mapping.py by "
          "tools/gen_marc8_mapping.py. DO NOT EDIT.\n\n")
out.write("package marc\n\n")
out.write("// marc8CodePoint is a single MARC-8 -> Unicode mapping entry.\n")
out.write("type marc8CodePoint struct {\n\tUnicode   rune\n\tCombining bool\n}\n\n")

out.write("// marc8Codesets maps a G0/G1 charset designator byte to its byte\n"
          "// (or, for charset 0x31/EACC, 3-byte-combined) code point table.\n"
          "// Ported from pymarc/marc8_mapping.py CODESETS.\n")
out.write("var marc8Codesets = map[byte]map[int]marc8CodePoint{\n")
for charset in sorted(m.CODESETS):
    table = m.CODESETS[charset]
    out.write(f"\t0x{charset:x}: {{\n")
    for code_point in sorted(table):
        uni, cflag = table[code_point]
        out.write(f"\t\t{code_point}: {{Unicode: {uni}, Combining: {'true' if cflag else 'false'}}},\n")
    out.write("\t},\n")
out.write("}\n\n")

out.write("// marc8OddMap is a charset-independent fallback table for code points\n"
          "// that CODESETS doesn't cover. Ported from pymarc/marc8_mapping.py ODD_MAP.\n")
out.write("var marc8OddMap = map[int]rune{\n")
for code_point in sorted(m.ODD_MAP):
    out.write(f"\t{code_point}: {m.ODD_MAP[code_point]},\n")
out.write("}\n")
