package marc

import "errors"

// Sentinel errors ported from pymarc/exceptions.py. Wrap with fmt.Errorf("%w: ...", ErrX)
// for dynamic detail, and check with errors.Is.
var (
	ErrRecordLengthInvalid    = errors.New("invalid record length in first 5 bytes of record")
	ErrTruncatedRecord        = errors.New("record length in leader is greater than the length of data")
	ErrEndOfRecordNotFound    = errors.New("unable to locate end of record marker")
	ErrRecordLeaderInvalid    = errors.New("unable to extract record leader")
	ErrRecordDirectoryInvalid = errors.New("invalid directory")
	ErrNoFieldsFound          = errors.New("unable to locate fields in record data")
	ErrBaseAddressInvalid     = errors.New("base address exceeds size of record")
	ErrBaseAddressNotFound    = errors.New("unable to locate base address of record")
	ErrWriteNeedsRecord       = errors.New("write requires a *marc.Record argument")
	ErrNoActiveFile           = errors.New("there is no active file to write to")
	ErrFieldNotFound          = errors.New("record does not contain the specified field")
	ErrBadLeaderValue         = errors.New("bad leader value")
	ErrMissingLinkedFields    = errors.New("field includes a subfield 6 but no linked fields could be found")
)
