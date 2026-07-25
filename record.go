package marc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

var isbnRegex = regexp.MustCompile(`[0-9\-xX]+`)

// namedCharsets maps a lowercased file_encoding name to its decoder, for the
// non-MARC8, non-UTF-8 charsets DecodeMARC supports.
var namedCharsets = map[string]encoding.Encoding{
	"cp1251":       charmap.Windows1251,
	"windows-1251": charmap.Windows1251,
}

// Record represents a MARC record: a Leader plus an ordered list of Fields.
// Ported from pymarc/record.py.
type Record struct {
	Leader    *Leader
	Fields    []*Field
	ToUnicode bool
	ForceUTF8 bool
}

// RecordOption configures NewRecord, mirroring pymarc's Record.__init__ keyword args.
type RecordOption func(*recordConfig)

type recordConfig struct {
	leader           string
	fields           []*Field
	toUnicode        bool
	forceUTF8        bool
	hideUTF8Warnings bool
	utf8Handling     string
	fileEncoding     string
	data             []byte
}

// WithLeaderString sets the initial 24-byte leader input (default: 24 spaces).
func WithLeaderString(s string) RecordOption { return func(c *recordConfig) { c.leader = s } }

// WithFields sets the record's fields directly, skipping MARC decoding.
func WithFields(fields ...*Field) RecordOption { return func(c *recordConfig) { c.fields = fields } }

// WithData supplies raw MARC transmission-format bytes to decode.
func WithData(data []byte) RecordOption { return func(c *recordConfig) { c.data = data } }

// WithForceUTF8 forces UTF-8 decoding/encoding regardless of the leader's coding scheme.
func WithForceUTF8(b bool) RecordOption { return func(c *recordConfig) { c.forceUTF8 = b } }

// WithToUnicode controls whether subfield/control-field data is decoded to Go strings
// (true, default) — to_unicode=false (raw byte passthrough) is not yet implemented.
func WithToUnicode(b bool) RecordOption { return func(c *recordConfig) { c.toUnicode = b } }

// WithUTF8Handling sets the UTF-8 decode error mode: "strict" (default), "replace", or "ignore".
func WithUTF8Handling(mode string) RecordOption {
	return func(c *recordConfig) { c.utf8Handling = mode }
}

// WithFileEncoding sets the non-UTF-8, non-MARC8 charset to assume (default "iso8859-1").
func WithFileEncoding(enc string) RecordOption { return func(c *recordConfig) { c.fileEncoding = enc } }

// WithHideUTF8Warnings suppresses MARC8 conversion warnings.
func WithHideUTF8Warnings(b bool) RecordOption {
	return func(c *recordConfig) { c.hideUTF8Warnings = b }
}

func defaultLeaderInput() string { return strings.Repeat(" ", LeaderLen) }

// buildDefaultLeader replicates pymarc's Record.__init__ leader construction:
// leader[0:10] + "22" + leader[12:20] + "4500", fixing indicator/subfield-code
// counts and the standard MARC21 directory entry map.
func buildDefaultLeader(input string) (*Leader, error) {
	if len(input) != LeaderLen {
		return nil, fmt.Errorf("%w: leader must be %d chars, got %d", ErrRecordLeaderInvalid, LeaderLen, len(input))
	}
	s := input[0:10] + "22" + input[12:20] + "4500"
	return NewLeader(s)
}

// NewRecord builds a Record from options, mirroring pymarc's Record.__init__.
// If WithData is given (and WithFields is not), the data is decoded via DecodeMARC.
func NewRecord(opts ...RecordOption) (*Record, error) {
	cfg := &recordConfig{
		leader:       defaultLeaderInput(),
		toUnicode:    true,
		utf8Handling: "strict",
		fileEncoding: "iso8859-1",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	leader, err := buildDefaultLeader(cfg.leader)
	if err != nil {
		return nil, err
	}
	r := &Record{
		Leader:    leader,
		ToUnicode: cfg.toUnicode,
		ForceUTF8: cfg.forceUTF8,
	}

	switch {
	case len(cfg.fields) > 0:
		r.Fields = cfg.fields
	case len(cfg.data) > 0:
		if err := r.DecodeMARC(cfg.data, decodeOptions{
			toUnicode:        cfg.toUnicode,
			forceUTF8:        cfg.forceUTF8,
			hideUTF8Warnings: cfg.hideUTF8Warnings,
			utf8Handling:     cfg.utf8Handling,
			encoding:         cfg.fileEncoding,
		}); err != nil {
			return nil, err
		}
	case cfg.forceUTF8:
		if err := r.Leader.replaceValues(9, "a"); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// String returns the record in MARCMaker format (leader line + one line per field).
func (r *Record) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "=LDR  %s\n", r.Leader.String())
	for _, f := range r.Fields {
		b.WriteString(f.String())
		b.WriteString("\n")
	}
	return b.String()
}

// Get returns the first field with the given tag, or nil if absent.
func (r *Record) Get(tag string) *Field {
	for _, f := range r.Fields {
		if f.Tag == tag {
			return f
		}
	}
	return nil
}

// Contains reports whether the record has a field with the given tag.
func (r *Record) Contains(tag string) bool {
	return r.Get(tag) != nil
}

// AddField appends one or more fields to the record.
func (r *Record) AddField(fields ...*Field) {
	r.Fields = append(r.Fields, fields...)
}

func (r *Record) sortField(f *Field, grouped bool) {
	tagKey := func(tag string) int {
		if grouped {
			n, _ := strconv.Atoi(tag[0:1])
			return n
		}
		n, _ := strconv.Atoi(tag)
		return n
	}
	tag := tagKey(f.Tag)
	for i, existing := range r.Fields {
		if !isAllDigits(existing.Tag) {
			r.Fields = append(r.Fields[:i], append([]*Field{f}, r.Fields[i:]...)...)
			return
		}
		if tagKey(existing.Tag) > tag {
			r.Fields = append(r.Fields[:i], append([]*Field{f}, r.Fields[i:]...)...)
			return
		}
		if i == len(r.Fields)-1 {
			r.Fields = append(r.Fields, f)
			return
		}
	}
}

// AddGroupedField adds fields, keeping a loose numeric order per the MARC "organization
// of the record" convention (grouped by first tag digit).
func (r *Record) AddGroupedField(fields ...*Field) {
	for _, f := range fields {
		if len(r.Fields) == 0 || !isAllDigits(f.Tag) {
			r.Fields = append(r.Fields, f)
			continue
		}
		r.sortField(f, true)
	}
}

// AddOrderedField adds fields, keeping a strict numeric tag order.
func (r *Record) AddOrderedField(fields ...*Field) {
	for _, f := range fields {
		if len(r.Fields) == 0 || !isAllDigits(f.Tag) {
			r.Fields = append(r.Fields, f)
			continue
		}
		r.sortField(f, false)
	}
}

// RemoveField removes a field by identity (pointer equality). Returns
// ErrFieldNotFound if the field isn't present.
func (r *Record) RemoveField(f *Field) error {
	for i, existing := range r.Fields {
		if existing == f {
			r.Fields = append(r.Fields[:i], r.Fields[i+1:]...)
			return nil
		}
	}
	return ErrFieldNotFound
}

// RemoveFields removes all fields whose tag matches any of the given tags.
func (r *Record) RemoveFields(tags ...string) {
	if len(tags) == 0 {
		return
	}
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}
	out := r.Fields[:0]
	for _, f := range r.Fields {
		if !tagSet[f.Tag] {
			out = append(out, f)
		}
	}
	r.Fields = out
}

// GetFields returns all fields matching any of the given tags, in record order.
// With no tags, returns all fields.
func (r *Record) GetFields(tags ...string) []*Field {
	if len(tags) == 0 {
		return r.Fields
	}
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}
	var out []*Field
	for _, f := range r.Fields {
		if tagSet[f.Tag] {
			out = append(out, f)
		}
	}
	return out
}

// GetLinkedFields returns the 880 fields linked to f via subfield 6's occurrence number.
// Returns ErrMissingLinkedFields if f has a subfield 6 but no 880 matches it.
func (r *Record) GetLinkedFields(f *Field) ([]*Field, error) {
	num, hasNum := f.LinkageOccurrenceNum()
	var linked []*Field
	for _, cand := range r.GetFields("880") {
		if n, ok := cand.LinkageOccurrenceNum(); ok && n == num {
			linked = append(linked, cand)
		}
	}
	if hasNum && len(linked) == 0 {
		return nil, fmt.Errorf("%w: %s field includes a subfield 6 but no linked fields could be found", ErrMissingLinkedFields, f.Tag)
	}
	return linked, nil
}

// AsDict turns the record into a plain map, matching pymarc's as_dict()/MARC-in-JSON shape.
func (r *Record) AsDict() map[string]any {
	fields := make([]map[string]any, len(r.Fields))
	for i, f := range r.Fields {
		if f.ControlField {
			fields[i] = map[string]any{f.Tag: f.Data}
			continue
		}
		subs := make([]map[string]string, len(f.Subfields))
		for j, s := range f.Subfields {
			subs[j] = map[string]string{s.Code: s.Value}
		}
		fields[i] = map[string]any{
			f.Tag: map[string]any{
				"ind1":      f.Indicator1(),
				"ind2":      f.Indicator2(),
				"subfields": subs,
			},
		}
	}
	return map[string]any{"leader": r.Leader.String(), "fields": fields}
}

// AsJSON serializes the record as MARC-in-JSON.
func (r *Record) AsJSON() (string, error) {
	b, err := json.Marshal(r.AsDict())
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func fieldValue(f *Field, code string) (string, bool) {
	if f == nil {
		return "", false
	}
	return f.Subfield(code)
}

// Title returns the title of the record (245 $a and $b).
func (r *Record) Title() (string, bool) {
	f := r.Get("245")
	a, ok := fieldValue(f, "a")
	if !ok {
		return "", false
	}
	if b, ok := fieldValue(f, "b"); ok && b != "" {
		return a + " " + b, true
	}
	return a, true
}

// IssnTitle returns the key title of the record (222 $a and $b).
func (r *Record) IssnTitle() (string, bool) {
	f := r.Get("222")
	a, ok := fieldValue(f, "a")
	if !ok {
		return "", false
	}
	if b, ok := fieldValue(f, "b"); ok && b != "" {
		return a + " " + b, true
	}
	return a, true
}

// ISBN returns the first ISBN in the record (from 020 $a), with dashes and
// extraneous text stripped, or ok=false if absent/unparseable.
func (r *Record) ISBN() (string, bool) {
	a, ok := fieldValue(r.Get("020"), "a")
	if !ok || a == "" {
		return "", false
	}
	m := isbnRegex.FindString(a)
	if m == "" {
		return "", false
	}
	return strings.ReplaceAll(m, "-", ""), true
}

// ISSN returns the ISSN number (022 $a), or ok=false if absent.
func (r *Record) ISSN() (string, bool) {
	f := r.Get("022")
	if f == nil || !f.Contains("a") {
		return "", false
	}
	return fieldValue(f, "a")
}

// ISSNL returns the ISSN-L number (022 $l), or ok=false if absent.
func (r *Record) ISSNL() (string, bool) {
	f := r.Get("022")
	if f == nil || !f.Contains("l") {
		return "", false
	}
	return fieldValue(f, "l")
}

// SUDOC returns the Superintendent of Documents classification number (086), or ok=false if absent.
func (r *Record) SUDOC() (string, bool) {
	f := r.Get("086")
	if f == nil {
		return "", false
	}
	return f.FormatField(), true
}

// Author returns the author from field 100, 110, or 111, or ok=false if none present.
func (r *Record) Author() (string, bool) {
	f := r.Get("100")
	if f == nil {
		f = r.Get("110")
	}
	if f == nil {
		f = r.Get("111")
	}
	if f == nil {
		return "", false
	}
	return f.FormatField(), true
}

// UniformTitle returns the uniform title from field 130 or 240, or ok=false if none present.
func (r *Record) UniformTitle() (string, bool) {
	f := r.Get("130")
	if f == nil {
		f = r.Get("240")
	}
	if f == nil {
		return "", false
	}
	return f.FormatField(), true
}

// Series returns series fields (440, 490, 800, 810, 811, 830).
func (r *Record) Series() []*Field {
	return r.GetFields("440", "490", "800", "810", "811", "830")
}

// Subjects returns subject fields (6XX).
func (r *Record) Subjects() []*Field {
	return r.GetFields(
		"600", "610", "611", "630", "648", "650", "651", "653", "654", "655",
		"656", "657", "658", "662", "690", "691", "696", "697", "698", "699",
	)
}

// AddedEntries returns added-entry fields (7XX).
func (r *Record) AddedEntries() []*Field {
	return r.GetFields(
		"700", "710", "711", "720", "730", "740", "752", "753", "754", "790",
		"791", "792", "793", "796", "797", "798", "799",
	)
}

// Location returns location fields (852).
func (r *Record) Location() []*Field {
	return r.GetFields("852")
}

// Notes returns note fields (5XX).
func (r *Record) Notes() []*Field {
	return r.GetFields(
		"500", "501", "502", "504", "505", "506", "507", "508", "510", "511",
		"513", "514", "515", "516", "518", "520", "521", "522", "524", "525",
		"526", "530", "533", "534", "535", "536", "538", "540", "541", "544",
		"545", "546", "547", "550", "552", "555", "556", "561", "562", "563",
		"565", "567", "580", "581", "583", "584", "585", "586", "590", "591",
		"592", "593", "594", "595", "596", "597", "598", "599",
	)
}

// PhysicalDescription returns physical-description fields (300).
func (r *Record) PhysicalDescription() []*Field {
	return r.GetFields("300")
}

// Publisher returns the publisher from 260 $b, or from 264 $b when the 264's
// second indicator is "1", or ok=false if neither is present.
func (r *Record) Publisher() (string, bool) {
	for _, f := range r.GetFields("260", "264") {
		if f.Tag == "260" {
			return fieldValue(f, "b")
		}
		if f.Tag == "264" && f.Indicator2() == "1" {
			return fieldValue(f, "b")
		}
	}
	return "", false
}

// PubYear returns the publication year from 260 $c, or from 264 $c when the
// 264's second indicator is "1", or ok=false if neither is present.
func (r *Record) PubYear() (string, bool) {
	for _, f := range r.GetFields("260", "264") {
		if f.Tag == "260" {
			return fieldValue(f, "c")
		}
		if f.Tag == "264" && f.Indicator2() == "1" {
			return fieldValue(f, "c")
		}
	}
	return "", false
}

// decodeOptions configures DecodeMARC.
type decodeOptions struct {
	toUnicode        bool
	forceUTF8        bool
	hideUTF8Warnings bool
	utf8Handling     string
	encoding         string
}

func parseDec(b []byte) (int, error) {
	b = bytes.TrimRight(b, " ")
	if len(b) == 0 {
		return 0, strconv.ErrSyntax
	}
	n := 0
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, strconv.ErrSyntax
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// DecodeMARC populates the record from data in MARC transmission format,
// matching pymarc's Record.decode_marc. Only to_unicode=true is currently
// supported; RawField / to_unicode=false is not yet ported.
func (r *Record) DecodeMARC(marc []byte, opts decodeOptions) error {
	if !opts.toUnicode {
		return fmt.Errorf("marc: to_unicode=false (RawField) is not yet implemented")
	}
	if len(marc) < LeaderLen {
		return ErrRecordLeaderInvalid
	}
	leaderStr := string(marc[0:LeaderLen])

	encoding := opts.encoding
	if leaderStr[9] == 'a' || opts.forceUTF8 {
		encoding = "utf-8"
	}

	leader, err := NewLeader(leaderStr)
	if err != nil {
		return err
	}
	r.Leader = leader
	r.ToUnicode = opts.toUnicode
	r.ForceUTF8 = opts.forceUTF8

	baseAddress, err := parseDec(marc[12:17])
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBaseAddressNotFound, err)
	}
	if baseAddress <= 0 {
		return ErrBaseAddressNotFound
	}
	if baseAddress >= len(marc) {
		return ErrBaseAddressInvalid
	}
	recordLen, err := parseDec(marc[0:5])
	if err == nil && len(marc) < recordLen {
		return ErrTruncatedRecord
	}

	dirBytes := marc[LeaderLen : baseAddress-1]
	if len(dirBytes)%DirectoryEntryLen != 0 {
		return ErrRecordDirectoryInvalid
	}
	fieldTotal := len(dirBytes) / DirectoryEntryLen

	r.Fields = make([]*Field, 0, fieldTotal)
	for i := 0; i < fieldTotal; i++ {
		entry := dirBytes[i*DirectoryEntryLen : (i+1)*DirectoryEntryLen]
		entryTag := string(entry[0:3])
		entryLength, err := parseDec(entry[3:7])
		if err != nil {
			return fmt.Errorf("%w: bad field length in directory entry %q", ErrRecordDirectoryInvalid, string(entry))
		}
		entryOffset, err := parseDec(entry[7:12])
		if err != nil {
			return fmt.Errorf("%w: bad field offset in directory entry %q", ErrRecordDirectoryInvalid, string(entry))
		}
		start := baseAddress + entryOffset
		end := start + entryLength - 1
		if start < 0 || end > len(marc) || start > end {
			return fmt.Errorf("%w: field data out of range for entry %q", ErrRecordDirectoryInvalid, string(entry))
		}
		entryData := marc[start:end]

		var field *Field
		if isAllDigits(entryTag) && entryTag < "010" {
			field = NewControlField(entryTag, string(entryData))
		} else {
			field, err = decodeDataField(entryTag, entryData, r.Leader, encoding, opts)
			if err != nil {
				return err
			}
		}
		r.Fields = append(r.Fields, field)
	}

	if fieldTotal == 0 {
		return ErrNoFieldsFound
	}
	return nil
}

func decodeDataField(tag string, entryData []byte, leader *Leader, encoding string, opts decodeOptions) (*Field, error) {
	firstInd, secondInd := " ", " "
	if len(entryData) > 0 {
		firstInd = string(entryData[0])
	}
	if len(entryData) > 1 {
		secondInd = string(entryData[1])
	}

	var subfields []Subfield
	idx := bytes.IndexByte(entryData, SubfieldIndicator)
	if idx != -1 {
		rest := entryData[idx:]
		for len(rest) > 0 && rest[0] == SubfieldIndicator {
			rest = rest[1:]
			if len(rest) == 0 {
				break
			}
			code := string(rest[0])
			rest = rest[1:]

			nextIdx := bytes.IndexByte(rest, SubfieldIndicator)
			var data []byte
			if nextIdx == -1 {
				data = rest
				rest = nil
			} else {
				data = rest[:nextIdx]
				rest = rest[nextIdx:]
			}

			var value string
			var err error
			if leader.Byte(9) == 'a' || opts.forceUTF8 {
				value, err = decodeUTF8(data, opts.utf8Handling)
			} else if encoding == "iso8859-1" {
				value, err = marc8ToUnicode(data, opts.hideUTF8Warnings)
			} else {
				value, err = decodeCharset(data, encoding)
			}
			if err != nil {
				return nil, err
			}
			subfields = append(subfields, Subfield{Code: code, Value: value})
		}
	}

	return NewField(tag, Indicators{First: firstInd, Second: secondInd}, subfields, ""), nil
}

func decodeUTF8(data []byte, mode string) (string, error) {
	if utf8.Valid(data) {
		return string(data), nil
	}
	switch mode {
	case "replace":
		return strings.ToValidUTF8(string(data), "�"), nil
	case "ignore":
		return strings.ToValidUTF8(string(data), ""), nil
	default:
		return "", fmt.Errorf("marc: invalid utf-8 in subfield data")
	}
}

// decodeCharset decodes data using a non-MARC8, non-UTF-8 file encoding, matching
// pymarc's file_encoding parameter. Only a small set of charsets are supported.
func decodeCharset(data []byte, encoding string) (string, error) {
	enc, ok := namedCharsets[strings.ToLower(encoding)]
	if !ok {
		return "", fmt.Errorf("marc: unsupported file encoding %q", encoding)
	}
	out, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		return "", fmt.Errorf("marc: decoding %q: %w", encoding, err)
	}
	return string(out), nil
}

// AsMARC serializes the record into MARC transmission-format bytes,
// matching pymarc's Record.as_marc().
func (r *Record) AsMARC() ([]byte, error) {
	if r.ToUnicode {
		if err := r.Leader.SetCodingScheme("a"); err != nil {
			return nil, err
		}
	}
	encoding := "iso8859-1"
	if r.Leader.Byte(9) == 'a' || r.ForceUTF8 {
		encoding = "utf-8"
	}
	if encoding != "utf-8" {
		return nil, fmt.Errorf("marc: as_marc: only utf-8 output is currently supported (got leader coding scheme %q)", string(r.Leader.Byte(9)))
	}

	var fields strings.Builder
	var directory strings.Builder
	offset := 0

	for _, f := range r.Fields {
		data, err := f.AsMarc(encoding)
		if err != nil {
			return nil, err
		}
		fields.Write(data)
		tag := f.Tag
		if len(tag) < 3 {
			tag = strings.Repeat(" ", 3-len(tag)) + tag
		}
		fmt.Fprintf(&directory, "%s%04d%05d", tag, len(data), offset)
		offset += len(data)
	}
	directory.WriteByte(EndOfField)
	fields.WriteByte(EndOfRecord)

	baseAddress := LeaderLen + directory.Len()
	recordLength := baseAddress + fields.Len()

	newLeader := fmt.Sprintf("%05d%s%05d%s",
		recordLength,
		r.Leader.Slice(5, 12),
		baseAddress,
		r.Leader.Slice(17, LeaderLen),
	)

	var out strings.Builder
	out.WriteString(newLeader)
	out.WriteString(directory.String())
	out.WriteString(fields.String())
	return []byte(out.String()), nil
}
