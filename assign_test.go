package retable

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSmartAssign(t *testing.T) {
	tests := []struct {
		name      string
		dst       reflect.Value
		src       reflect.Value
		scanner   Scanner
		parser    Parser
		formatter Formatter
		wantErr   bool
		wantDst   any
	}{
		{
			name:    "int to int",
			dst:     assignableValue[int](),
			src:     reflect.ValueOf(int(1)),
			wantDst: int(1),
		},
		{
			name:    "string to string",
			dst:     assignableValue[string](),
			src:     reflect.ValueOf("S"),
			wantDst: "S",
		},

		{
			name:    "int to *int",
			dst:     assignableValue[*int](),
			src:     reflect.ValueOf(int(1)),
			wantDst: pointerTo(int(1)),
		},
		{
			name:    "*int to int",
			dst:     assignableValue[int](),
			src:     reflect.ValueOf(pointerTo(int(1))),
			wantDst: int(1),
		},

		// An empty source string means "no value" and assigns the zero
		// value, like a null source does. Excel and CSV files have empty
		// cells for optional columns of any type, and reading one must
		// not fail the whole file.
		{
			name:    "empty string to uint64",
			dst:     assignableValue[uint64](),
			src:     reflect.ValueOf(""),
			wantDst: uint64(0),
		},
		{
			name:    "empty string to int",
			dst:     assignableValue[int](),
			src:     reflect.ValueOf(""),
			wantDst: int(0),
		},
		{
			name:    "empty string to float64",
			dst:     assignableValue[float64](),
			src:     reflect.ValueOf(""),
			wantDst: float64(0),
		},
		{
			name:    "empty string to bool",
			dst:     assignableValue[bool](),
			src:     reflect.ValueOf(""),
			wantDst: false,
		},
		{
			name:    "empty string to time.Time",
			dst:     assignableValue[time.Time](),
			src:     reflect.ValueOf(""),
			wantDst: time.Time{},
		},
		{
			name:    "empty string to *int",
			dst:     assignableValue[*int](),
			src:     reflect.ValueOf(""),
			wantDst: (*int)(nil),
		},
		{
			// A string destination keeps the empty string
			// instead of being reset to its zero value,
			// which happens to be the same value here but
			// must not go through the zero value branch.
			name:    "empty string to string",
			dst:     assignableValue[string](),
			src:     reflect.ValueOf(""),
			wantDst: "",
		},
		{
			// A non-empty string that cannot be parsed
			// still has to be reported as an error.
			name:    "non numeric string to uint64",
			dst:     assignableValue[uint64](),
			src:     reflect.ValueOf("not a number"),
			wantErr: true,
		},
		{
			// A destination that can hold the string itself
			// keeps the empty string instead of the zero value,
			// so an optional text column stays distinguishable
			// from a missing pointer.
			name:    "empty string to *string",
			dst:     assignableValue[*string](),
			src:     reflect.ValueOf(""),
			wantDst: pointerTo(""),
		},
		{
			name:    "empty string to any",
			dst:     assignableValue[any](),
			src:     reflect.ValueOf(""),
			wantDst: any(""),
		},
		{
			name:    "empty string to []byte",
			dst:     assignableValue[[]byte](),
			src:     reflect.ValueOf(""),
			wantDst: []byte{},
		},
		{
			// Every pointer level is allocated, so a **string
			// column is as distinguishable from a missing value
			// as a *string one instead of collapsing to nil.
			name:    "empty string to **string",
			dst:     assignableValue[**string](),
			src:     reflect.ValueOf(""),
			wantDst: pointerTo(pointerTo("")),
		},
		{
			name:    "empty string to **int",
			dst:     assignableValue[**int](),
			src:     reflect.ValueOf(""),
			wantDst: (**int)(nil),
		},

		// A destination that cannot hold a string of any content is
		// a type mismatch, not an empty cell. Assigning the zero
		// value would make the error depend on the data: a struct
		// field wired to the wrong column type would parse every row
		// with an empty cell and only fail on the first non-empty
		// one, so a sparse column could hide the mismatch entirely.
		{
			name:    "empty string to chan",
			dst:     assignableValue[chan int](),
			src:     reflect.ValueOf(""),
			wantErr: true,
		},
		{
			name:    "empty string to map",
			dst:     assignableValue[map[string]int](),
			src:     reflect.ValueOf(""),
			wantErr: true,
		},
		{
			name:    "empty string to struct",
			dst:     assignableValue[struct{ X int }](),
			src:     reflect.ValueOf(""),
			wantErr: true,
		},
		{
			name:    "empty string to array",
			dst:     assignableValue[[4]int](),
			src:     reflect.ValueOf(""),
			wantErr: true,
		},
		{
			// An interface that a string does not implement,
			// unlike any, which is assigned the empty string above.
			name:    "empty string to non-empty interface",
			dst:     assignableValue[fmt.Stringer](),
			src:     reflect.ValueOf(""),
			wantErr: true,
		},
		{
			name:    "empty string to pointer to chan",
			dst:     assignableValue[*chan int](),
			src:     reflect.ValueOf(""),
			wantErr: true,
		},

		// The empty string is not the only spelling of "no value":
		// exports of database tables write a literal NULL for it, and
		// the strings that mean it are configured on the Parser through
		// StringParser.NilStrings, not hardcoded here.
		{
			name:    "NULL string to int",
			dst:     assignableValue[int](),
			src:     reflect.ValueOf("NULL"),
			wantDst: int(0),
		},
		{
			name:    "NULL string to *int is nil",
			dst:     assignableValue[*int](),
			src:     reflect.ValueOf("NULL"),
			wantDst: (*int)(nil),
		},
		{
			name:    "null string to time.Time",
			dst:     assignableValue[time.Time](),
			src:     reflect.ValueOf("null"),
			wantDst: time.Time{},
		},
		// Destinations that can hold the string keep it, because only
		// the source format knows whether a cell reading NULL is a null
		// value or that text, and a string column can hold the text.
		{
			name:    "NULL string to string keeps the text",
			dst:     assignableValue[string](),
			src:     reflect.ValueOf("NULL"),
			wantDst: "NULL",
		},
		{
			name:    "NULL string to *string keeps the text",
			dst:     assignableValue[*string](),
			src:     reflect.ValueOf("NULL"),
			wantDst: pointerTo("NULL"),
		},
		// An explicitly empty NilStrings makes every string a value
		// again. It has to be empty and non-nil: a nil field falls back
		// to the defaults so that a partially built StringParser still
		// parses, see StringParser.nilStrings.
		{
			name:    "NULL string to int without nil strings",
			dst:     assignableValue[int](),
			src:     reflect.ValueOf("NULL"),
			parser:  &StringParser{NilStrings: []string{}},
			wantErr: true,
		},
		// The zero value of StringParser uses the defaults for every
		// field it does not set, so a config that only sets TimeFormats
		// does not silently stop parsing numbers and booleans.
		{
			name:    "NULL string to int with a partially set parser",
			dst:     assignableValue[int](),
			src:     reflect.ValueOf("NULL"),
			parser:  &StringParser{TimeFormats: []string{"2006-01-02"}},
			wantDst: int(0),
		},
		{
			name:    "true string to bool with a partially set parser",
			dst:     assignableValue[bool](),
			src:     reflect.ValueOf("true"),
			parser:  &StringParser{TimeFormats: []string{"2006-01-02"}},
			wantDst: true,
		},
		// PostgreSQL writes booleans as t and f in CSV exports
		{
			name:    "t string to bool",
			dst:     assignableValue[bool](),
			src:     reflect.ValueOf("t"),
			wantDst: true,
		},
		{
			name:    "f string to bool",
			dst:     assignableValue[bool](),
			src:     reflect.ValueOf("f"),
			wantDst: false,
		},

		// reflect.Value.Convert applies Go's string(rune) conversion
		// for integer sources, which would assign the character with
		// that code point ("*" for 42) instead of the decimal digits.
		// Numbers read from a table must render as their digits.
		{
			name:    "int to string",
			dst:     assignableValue[string](),
			src:     reflect.ValueOf(int(42)),
			wantDst: "42",
		},
		{
			name:    "negative int to string",
			dst:     assignableValue[string](),
			src:     reflect.ValueOf(int(-1)),
			wantDst: "-1",
		},
		{
			name:    "uint8 to string",
			dst:     assignableValue[string](),
			src:     reflect.ValueOf(uint8(65)),
			wantDst: "65",
		},
		{
			// A Stringer with an integer kind must use its
			// String method instead of the rune conversion.
			name:    "time.Month to string",
			dst:     assignableValue[string](),
			src:     reflect.ValueOf(time.January),
			wantDst: "January",
		},
		{
			// []byte and []rune conversions to string are the
			// intended direct conversion and must be preserved.
			name:    "[]byte to string",
			dst:     assignableValue[string](),
			src:     reflect.ValueOf([]byte("abc")),
			wantDst: "abc",
		},

		// bool and numbers convert in both directions and must
		// report success, not assign the value and then return
		// an unsupported operation error.
		{
			name:    "bool to int",
			dst:     assignableValue[int](),
			src:     reflect.ValueOf(true),
			wantDst: int(1),
		},
		{
			name:    "bool to uint8",
			dst:     assignableValue[uint8](),
			src:     reflect.ValueOf(false),
			wantDst: uint8(0),
		},
		{
			name:    "bool to float64",
			dst:     assignableValue[float64](),
			src:     reflect.ValueOf(true),
			wantDst: float64(1),
		},
		{
			name:    "bool to string",
			dst:     assignableValue[string](),
			src:     reflect.ValueOf(true),
			wantDst: "true",
		},
		{
			name:    "int to bool",
			dst:     assignableValue[bool](),
			src:     reflect.ValueOf(int(3)),
			wantDst: true,
		},
		{
			name:    "uint to bool",
			dst:     assignableValue[bool](),
			src:     reflect.ValueOf(uint(0)),
			wantDst: false,
		},
		{
			name:    "float64 to bool",
			dst:     assignableValue[bool](),
			src:     reflect.ValueOf(1.5),
			wantDst: true,
		},
		{
			name:    "string to bool",
			dst:     assignableValue[bool](),
			src:     reflect.ValueOf("true"),
			wantDst: true,
		},
		// A nil Parser uses DefaultParser, whose configurable boolean
		// strings cover spellings that strconv.ParseBool rejects.
		{
			name:    "yes string to bool",
			dst:     assignableValue[bool](),
			src:     reflect.ValueOf("yes"),
			wantDst: true,
		},
		// Numbers are parsed by the Parser, so a cell written by a
		// spreadsheet in a locale that groups thousands is converted
		// instead of reported as an unsupported assignment.
		{
			name:    "dot thousands separator string to float64",
			dst:     assignableValue[float64](),
			src:     reflect.ValueOf("1.234,56"),
			wantDst: float64(1234.56),
		},
		{
			name:    "comma thousands separator string to float64",
			dst:     assignableValue[float64](),
			src:     reflect.ValueOf("1,234.56"),
			wantDst: float64(1234.56),
		},

		// Converting a slice to a longer array panics in package
		// reflect, so it has to be reported as an error for the
		// array itself and for a pointer to it.
		{
			name:    "short slice to array",
			dst:     assignableValue[[4]byte](),
			src:     reflect.ValueOf([]byte{1, 2}),
			wantErr: true,
		},
		{
			name:    "short slice to array pointer",
			dst:     assignableValue[*[4]byte](),
			src:     reflect.ValueOf([]byte{1, 2, 3}),
			wantErr: true,
		},
		{
			name:    "long slice to array",
			dst:     assignableValue[[2]byte](),
			src:     reflect.ValueOf([]byte{7, 8, 9}),
			wantDst: [2]byte{7, 8},
		},
		{
			name:    "long slice to array pointer",
			dst:     assignableValue[*[2]byte](),
			src:     reflect.ValueOf([]byte{7, 8, 9}),
			wantDst: pointerTo([2]byte{7, 8}),
		},

		// time.Duration has an int64 kind, so without explicit
		// duration parsing only a plain number of nanoseconds
		// would be accepted.
		{
			name:    "duration string to time.Duration",
			dst:     assignableValue[time.Duration](),
			src:     reflect.ValueOf("1h30m"),
			wantDst: 90 * time.Minute,
		},
		// An optional date column is declared as *time.Time, so the
		// pointer arm of the time branch has to parse like the value one.
		{
			name:    "date string to *time.Time",
			dst:     assignableValue[*time.Time](),
			src:     reflect.ValueOf("2024-03-15"),
			wantDst: pointerTo(time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)),
		},
		{
			name:    "duration string to *time.Duration",
			dst:     assignableValue[*time.Duration](),
			src:     reflect.ValueOf("5s"),
			wantDst: pointerTo(5 * time.Second),
		},
		{
			// A plain number without a unit stays nanoseconds
			// so the integer parsing below is not shadowed.
			name:    "nanoseconds string to time.Duration",
			dst:     assignableValue[time.Duration](),
			src:     reflect.ValueOf("90"),
			wantDst: time.Duration(90),
		},

		// A type defined as time.Time or time.Duration parses from the
		// same strings as the type it is defined as, because it has the
		// same underlying type and a string cannot be converted to it
		// directly. Applications declare such types for domain specific
		// columns, and before this they only reported an unsupported
		// operation unless a Scanner was passed for them.
		{
			name:    "date string to defined time.Time type",
			dst:     assignableValue[date](),
			src:     reflect.ValueOf("2024-03-15"),
			wantDst: date(time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)),
		},
		{
			name:    "date string to pointer to defined time.Time type",
			dst:     assignableValue[*date](),
			src:     reflect.ValueOf("2024-03-15"),
			wantDst: pointerTo(date(time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC))),
		},
		{
			// The empty cell of an optional column means "no value"
			// for a defined type too, not only for time.Time itself.
			name:    "empty string to defined time.Time type",
			dst:     assignableValue[date](),
			src:     reflect.ValueOf(""),
			wantDst: date{},
		},
		{
			// A struct that embeds time.Time has a different underlying
			// type, so it is not a time.Time and stays unsupported.
			name:    "date string to struct embedding time.Time",
			dst:     assignableValue[embeddedTime](),
			src:     reflect.ValueOf("2024-03-15"),
			wantErr: true,
		},
		{
			name:    "duration string to defined time.Duration type",
			dst:     assignableValue[timeout](),
			src:     reflect.ValueOf("1h30m"),
			wantDst: timeout(90 * time.Minute),
		},
		{
			name:    "duration string to pointer to defined time.Duration type",
			dst:     assignableValue[*timeout](),
			src:     reflect.ValueOf("5s"),
			wantDst: pointerTo(timeout(5 * time.Second)),
		},
		{
			// A plain number without a unit stays nanoseconds
			// for a defined type as well.
			name:    "nanoseconds string to defined time.Duration type",
			dst:     assignableValue[timeout](),
			src:     reflect.ValueOf("90"),
			wantDst: timeout(90),
		},
		{
			// Reflection cannot tell a type defined as time.Duration
			// from any other defined int64 type, so both parse duration
			// strings. This is the cost documented at durationType and
			// the case that makes it visible.
			name:    "duration string to defined int64 type",
			dst:     assignableValue[byteCount](),
			src:     reflect.ValueOf("5m"),
			wantDst: byteCount(5 * time.Minute),
		},
		{
			// The predeclared int64 is excluded from that, so an
			// ordinary numeric column keeps rejecting duration strings.
			name:    "duration string to int64",
			dst:     assignableValue[int64](),
			src:     reflect.ValueOf("1h30m"),
			wantErr: true,
		},

		// dstScanner overrides the built-in string parsing and
		// falls through to it for errors.ErrUnsupported.
		{
			name:    "dstScanner handles destination",
			dst:     assignableValue[int](),
			src:     reflect.ValueOf("42"),
			scanner: intScanner(99),
			wantDst: int(99),
		},
		{
			name:    "dstScanner falls through for unsupported destination",
			dst:     assignableValue[float64](),
			src:     reflect.ValueOf("1.5"),
			scanner: intScanner(99),
			wantDst: float64(1.5),
		},
		{
			name:    "dstScanner error is returned",
			dst:     assignableValue[int](),
			src:     reflect.ValueOf("42"),
			scanner: failingScanner(),
			wantErr: true,
		},
		{
			// The empty string has to reach dstScanner before the
			// zero value branch, because a custom scanner is the
			// only way to give an empty cell a different meaning
			// than the zero value of the destination.
			name:    "dstScanner handles empty string",
			dst:     assignableValue[int](),
			src:     reflect.ValueOf(""),
			scanner: intScanner(99),
			wantDst: int(99),
		},
		{
			// A scanner that does not handle the destination still
			// falls through to the zero value for an empty string.
			name:    "dstScanner falls through for empty string",
			dst:     assignableValue[float64](),
			src:     reflect.ValueOf(""),
			scanner: intScanner(99),
			wantDst: float64(0),
		},
		{
			// The parser passed to SmartAssign is the one dstScanner
			// receives, so parsing stays configurable per call instead
			// of going through a shared package level default.
			name:    "parser is passed to dstScanner",
			dst:     assignableValue[bool](),
			src:     reflect.ValueOf("ja"),
			scanner: boolParsingScanner(),
			parser:  germanBoolParser(),
			wantDst: true,
		},
		{
			// Without a parser SmartAssign allocates a default
			// StringParser, which does not know "ja".
			name:    "default parser is used without a parser",
			dst:     assignableValue[bool](),
			src:     reflect.ValueOf("ja"),
			scanner: boolParsingScanner(),
			wantErr: true,
		},

		// srcFormatter formats any source into a string destination,
		// including the integer sources excluded from the direct
		// conversion above.
		{
			name:      "srcFormatter formats int to string",
			dst:       assignableValue[string](),
			src:       reflect.ValueOf(int(42)),
			formatter: prefixFormatter("#"),
			wantDst:   "#42",
		},

		// The false branch of the boolean conversions has to write the
		// zero number, not leave the destination at whatever it held.
		// A row struct is reused across rows in some callers, so a
		// false cell that skipped the assignment would keep the 1 of
		// the previous row.
		{
			name:    "false bool to int",
			dst:     reflect.ValueOf(pointerTo(int(7))).Elem(),
			src:     reflect.ValueOf(false),
			wantDst: int(0),
		},
		{
			name:    "true bool to uint8",
			dst:     assignableValue[uint8](),
			src:     reflect.ValueOf(true),
			wantDst: uint8(1),
		},
		{
			name:    "false bool to float64",
			dst:     reflect.ValueOf(pointerTo(float64(7))).Elem(),
			src:     reflect.ValueOf(false),
			wantDst: float64(0),
		},

		// A null source means "no value" and must overwrite the
		// destination with its zero value. A database driver reports a
		// NULL column as a value that is present but null, and the
		// wrapped value it still carries must not be assigned.
		{
			name:    "null source to int",
			dst:     reflect.ValueOf(pointerTo(int(7))).Elem(),
			src:     reflect.ValueOf(nullableInt{value: 42, null: true}),
			wantDst: int(0),
		},
		{
			name:    "non null source to int",
			dst:     assignableValue[int](),
			src:     reflect.ValueOf(nullableInt{value: 42}),
			wantErr: true, // not convertible, IsNull only shortcuts a null
		},
		{
			name:    "nil pointer to int",
			dst:     reflect.ValueOf(pointerTo(int(7))).Elem(),
			src:     reflect.ValueOf((*int)(nil)),
			wantDst: int(0),
		},
		// An empty struct carries no data, so it is the zero value of
		// whatever it is assigned to rather than an unsupported type.
		{
			name:    "empty struct to int",
			dst:     reflect.ValueOf(pointerTo(int(7))).Elem(),
			src:     reflect.ValueOf(struct{}{}),
			wantDst: int(0),
		},

		// Error cases
		{
			name:    "invalid src",
			dst:     assignableValue[int](),
			src:     reflect.Value{},
			wantErr: true,
		},
		{
			name:    "invalid dst",
			dst:     reflect.Value{},
			src:     reflect.ValueOf(int(1)),
			wantErr: true,
		},
		// A Formatter reports an unsupported source with
		// errors.ErrUnsupported so that the conversions below it still
		// run. Every other error is a real formatting failure and must
		// abort instead of falling through to the fmt.Sprint fallback,
		// which would write a string the Formatter refused to produce.
		{
			name:      "formatter error to string",
			dst:       assignableValue[string](),
			src:       reflect.ValueOf(float64(1.5)),
			formatter: failingFormatter(),
			wantErr:   true,
		},
		// The pointer destination allocates and assigns through to the
		// pointed to type, so an error from that assignment is the
		// caller's error and must not be swallowed into an allocated
		// pointer to a zero value.
		{
			name:    "overflowing string to *int8",
			dst:     assignableValue[*int8](),
			src:     reflect.ValueOf("300"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Copy value in tt.dst to gotDst to be used by SmartAssign
			// to not modify the original value in tt.dst
			var gotDst reflect.Value
			if tt.dst.IsValid() {
				gotDst = reflect.New(tt.dst.Type()).Elem()
				gotDst.Set(tt.dst)
			}
			err := SmartAssign(gotDst, tt.src, tt.scanner, tt.parser, tt.formatter)
			require.Equalf(t, tt.wantErr, err != nil, "SmartAssign(%s, %s) error = %#v, wantErr %t", tt.dst, tt.src, err, tt.wantErr)
			if err != nil {
				return
			}
			require.Equalf(t, tt.wantDst, gotDst.Interface(), "SmartAssign(%s, %s) gotDst = %#v, wantDst %#v", tt.dst, tt.src, gotDst.Interface(), tt.wantDst)
		})
	}
}

func pointerTo[T any](v T) *T {
	return &v
}

// nullableInt is a source that reports whether it holds a value, like
// the nullable column types of a database driver do. SmartAssign has to
// ask before it converts, because the wrapped value of a null is
// meaningless.
type nullableInt struct {
	value int
	null  bool
}

func (n nullableInt) IsNull() bool { return n.null }

// failingFormatter returns a Formatter whose error is not
// errors.ErrUnsupported and must abort the assignment.
func failingFormatter() Formatter {
	return FormatterFunc(func(reflect.Value) (string, error) {
		return "", errors.New("formatter failed")
	})
}

// TestSmartAssignUnsettableDst covers the guard in front of every
// reflect.Value.Set of this package. A destination reached through
// reflection is easily not settable, an unexported struct field or a
// value that was never addressed, and reflect panics on writing one.
// The caller has to get an error instead of a crashing process.
func TestSmartAssignUnsettableDst(t *testing.T) {
	notSettable := reflect.ValueOf(42)
	require.True(t, notSettable.IsValid(), "the value is valid, only not settable")
	require.False(t, notSettable.CanSet())

	err := SmartAssign(notSettable, reflect.ValueOf(1), nil, nil, nil)
	require.ErrorContains(t, err, "cannot set dst value")
}

// date, timeout and byteCount are types defined as time.Time,
// time.Duration and int64, like an application declares for a domain
// specific column. embeddedTime is the case that must not be mistaken
// for one, because embedding is not the same underlying type.
type (
	date         time.Time
	timeout      time.Duration
	byteCount    int64
	embeddedTime struct{ time.Time }
)

// intScanner returns a Scanner that assigns value to int
// destinations and reports errors.ErrUnsupported for all
// others so the built-in parsing takes over.
func intScanner(value int64) Scanner {
	return ScannerFunc(func(dest reflect.Value, str string, parser Parser) error {
		if dest.Kind() != reflect.Int {
			return fmt.Errorf("%w: %s", errors.ErrUnsupported, dest.Type())
		}
		dest.SetInt(value)
		return nil
	})
}

// failingScanner returns a Scanner whose error is not
// errors.ErrUnsupported and must abort the assignment.
func failingScanner() Scanner {
	return ScannerFunc(func(dest reflect.Value, str string, parser Parser) error {
		return errors.New("scanner failed")
	})
}

// boolParsingScanner returns a Scanner that parses bool
// destinations with the Parser it receives, so that the
// Parser passed to SmartAssign becomes observable.
func boolParsingScanner() Scanner {
	return ScannerFunc(func(dest reflect.Value, str string, parser Parser) error {
		if dest.Kind() != reflect.Bool {
			return fmt.Errorf("%w: %s", errors.ErrUnsupported, dest.Type())
		}
		b, err := parser.ParseBool(str)
		if err != nil {
			return err
		}
		dest.SetBool(b)
		return nil
	})
}

// germanBoolParser returns a Parser that recognizes
// German boolean strings instead of the default English ones.
func germanBoolParser() Parser {
	p := NewStringParser()
	p.TrueStrings = []string{"ja"}
	p.FalseStrings = []string{"nein"}
	return p
}

func prefixFormatter(prefix string) Formatter {
	return FormatterFunc(func(v reflect.Value) (string, error) {
		return prefix + fmt.Sprint(v.Interface()), nil
	})
}

func assignableValue[T any]() reflect.Value {
	ptr := new(T)
	return reflect.ValueOf(ptr).Elem()
}

// TestSmartAssignUsesPassedParser ensures the string conversions go
// through the passed Parser instead of fixed strconv and time functions,
// which is what makes parsing configurable per call. The parser below
// configures spellings that the standard library functions reject, and
// drops the ones they accept, so each case can only pass through it.
func TestSmartAssignUsesPassedParser(t *testing.T) {
	parser := &StringParser{
		TrueStrings:  []string{"ja"},
		FalseStrings: []string{"nein"},
		TimeFormats:  []string{"02/01/2006"},
	}

	var b bool
	err := SmartAssign(reflect.ValueOf(&b).Elem(), reflect.ValueOf("ja"), nil, parser, nil)
	require.NoError(t, err)
	require.True(t, b)

	err = SmartAssign(reflect.ValueOf(&b).Elem(), reflect.ValueOf("nein"), nil, parser, nil)
	require.NoError(t, err)
	require.False(t, b)

	// "true" is not configured on this parser, so it must not be parsed
	err = SmartAssign(reflect.ValueOf(&b).Elem(), reflect.ValueOf("true"), nil, parser, nil)
	require.ErrorIs(t, err, errors.ErrUnsupported)

	var tm time.Time
	err = SmartAssign(reflect.ValueOf(&tm).Elem(), reflect.ValueOf("15/03/2024"), nil, parser, nil)
	require.NoError(t, err)
	require.Equal(t, time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC), tm)

	// TimeFormats replaces the default formats instead of extending them
	err = SmartAssign(reflect.ValueOf(&tm).Elem(), reflect.ValueOf("2024-03-15"), nil, parser, nil)
	require.ErrorIs(t, err, errors.ErrUnsupported)
}

// TestSmartAssignNumericOverflow covers that a value which does not fit
// the destination is reported instead of assigned as a different
// number. The Parser always parses 64 bits and reflect.Value.SetInt,
// SetUint and SetFloat truncate silently to the destination width, so
// without the check a cell reading 300 became 44 in an int8 column with
// no error anywhere, which is the shape that corrupts data quietly.
func TestSmartAssignNumericOverflow(t *testing.T) {
	overflowing := []struct {
		name string
		dst  any
		src  string
	}{
		{name: "int8", dst: new(int8), src: "300"},
		{name: "negative int8", dst: new(int8), src: "-300"},
		{name: "int16", dst: new(int16), src: "40000"},
		{name: "int32", dst: new(int32), src: "3000000000"},
		{name: "uint8", dst: new(uint8), src: "256"},
		{name: "uint16", dst: new(uint16), src: "70000"},
		{name: "float32", dst: new(float32), src: "1e39"},
	}
	for _, tt := range overflowing {
		t.Run(tt.name+" overflow is an error", func(t *testing.T) {
			dst := reflect.ValueOf(tt.dst).Elem()
			err := SmartAssign(dst, reflect.ValueOf(tt.src), nil, nil, nil)
			require.ErrorContains(t, err, "overflows "+dst.Type().String())
			require.True(t, dst.IsZero(), "the destination must not be written when the value does not fit")
		})
	}

	fitting := []struct {
		name string
		dst  any
		src  string
		want any
	}{
		{name: "int8 max", dst: new(int8), src: "127", want: int8(127)},
		{name: "int8 min", dst: new(int8), src: "-128", want: int8(-128)},
		{name: "uint8 max", dst: new(uint8), src: "255", want: uint8(255)},
		{name: "int64 max", dst: new(int64), src: "9223372036854775807", want: int64(math.MaxInt64)},
		{name: "float32", dst: new(float32), src: "3.14", want: float32(3.14)},
		{name: "float64 large", dst: new(float64), src: "1e300", want: 1e300},
		// An infinity that the source really said is representable and
		// must stay assignable, unlike a finite value that becomes one.
		{name: "float32 infinity", dst: new(float32), src: "Inf", want: float32(math.Inf(1))},
	}
	for _, tt := range fitting {
		t.Run(tt.name+" fits", func(t *testing.T) {
			dst := reflect.ValueOf(tt.dst).Elem()
			require.NoError(t, SmartAssign(dst, reflect.ValueOf(tt.src), nil, nil, nil))
			require.Equal(t, tt.want, dst.Interface())
		})
	}
}

// TestStringParserZeroValueUsesDefaults covers that a StringParser which
// only sets some of its fields still parses the rest. Every SmartAssign
// string conversion goes through the Parser, so a parser built from a
// configuration file that only names TimeFormats used to silently stop
// parsing numbers and booleans for the whole file.
func TestStringParserZeroValueUsesDefaults(t *testing.T) {
	partial := &StringParser{TimeFormats: []string{"2006-01-02"}}

	b, err := partial.ParseBool("true")
	require.NoError(t, err)
	require.True(t, b)
	require.True(t, partial.IsNil(""), "an empty cell must still mean no value")
	require.True(t, partial.IsNil("NULL"))

	// The field it did set is still the one that is used
	_, err = partial.ParseTime("2024-03-15T14:30:00Z")
	require.Error(t, err, "TimeFormats was set explicitly, so RFC3339 is not accepted")
	_, err = partial.ParseTime("2024-03-15")
	require.NoError(t, err)

	// An explicitly empty slice means "accept nothing", unlike nil
	empty := &StringParser{TrueStrings: []string{}, FalseStrings: []string{}}
	_, err = empty.ParseBool("true")
	require.Error(t, err)
}

// TestNewStringParserDoesNotShareSlices covers that two parsers, and the
// shared DefaultParser, do not alias one backing array. NewStringParser
// used to assign the package level timeFormats by reference, so writing
// through a parser a caller believed it owned reconfigured every other
// parser in the process, defeating the isolation that ViewToStructSlice
// documents for a Scanner that reconfigures its Parser.
func TestNewStringParserDoesNotShareSlices(t *testing.T) {
	a := NewStringParser()
	b := NewStringParser()
	def, ok := DefaultParser.(*StringParser)
	require.True(t, ok)

	// Compare the backing arrays instead of writing through one of them.
	// A destructive check corrupts the package defaults for every later
	// test in this package when it fails, turning one real failure into
	// a cascade of unrelated ones in tests that never touch a parser.
	sameArray := func(x, y []string) bool { return len(x) > 0 && len(y) > 0 && &x[0] == &y[0] }

	require.False(t, sameArray(a.TimeFormats, b.TimeFormats), "two parsers must not share TimeFormats")
	require.False(t, sameArray(a.TrueStrings, b.TrueStrings), "two parsers must not share TrueStrings")
	require.False(t, sameArray(a.FalseStrings, b.FalseStrings), "two parsers must not share FalseStrings")
	require.False(t, sameArray(a.NilStrings, b.NilStrings), "two parsers must not share NilStrings")

	require.False(t, sameArray(a.TimeFormats, def.TimeFormats), "a new parser must not share TimeFormats with DefaultParser")
	require.False(t, sameArray(a.TimeFormats, defaultTimeFormats), "a new parser must not share the package defaults")
	require.False(t, sameArray(a.NilStrings, defaultNilStrings), "a new parser must not share the package defaults")

	// The clone still carries the same values
	require.Equal(t, defaultTimeFormats, a.TimeFormats)
	require.Equal(t, defaultNilStrings, a.NilStrings)
}

// selfPtr is a self referential pointer type, which is legal Go: its
// element type is itself. Walking it to a non pointer type never
// terminates, so it is the shape that made zeroValueForNilString spin
// forever and the pointer branch of SmartAssign recurse until the stack
// overflowed, which is fatal and no deferred recover can catch.
type selfPtr *selfPtr

func TestSmartAssignSelfReferentialPointerType(t *testing.T) {
	// Run in a goroutine so that a regression fails the test instead of
	// hanging the whole suite until the go test timeout.
	for _, src := range []string{"", "42", "NULL"} {
		t.Run("src "+src, func(t *testing.T) {
			done := make(chan error, 1)
			go func() {
				var dst selfPtr
				done <- SmartAssign(reflect.ValueOf(&dst).Elem(), reflect.ValueOf(src), nil, nil, nil)
			}()
			select {
			case err := <-done:
				require.ErrorIs(t, err, errors.ErrUnsupported)
			case <-time.After(5 * time.Second):
				t.Fatal("SmartAssign did not return for a self referential pointer type")
			}
		})
	}

	require.False(t, zeroValueForNilString(reflect.TypeFor[selfPtr]()))

	// A pointer chain that does end is still followed
	_, ok := derefPointerType(reflect.TypeFor[***int]())
	require.True(t, ok)
	elem, ok := derefPointerType(reflect.TypeFor[**string]())
	require.True(t, ok)
	require.Equal(t, reflect.TypeFor[string](), elem)

	// Both sides of the bound, so changing the loop is a visible failure:
	// one level below the bound still resolves, at it the walk gives up
	// rather than spinning. The fixtures are derived from
	// maxPointerDepth, so pin the constant separately or moving it would
	// move the fixtures with it and neither assertion could fail.
	require.Equal(t, 32, maxPointerDepth,
		"32 is deep enough for any real type and shallow enough to fail fast")
	pointerTypeOfDepth := func(n int) reflect.Type {
		typ := reflect.TypeFor[int]()
		for range n {
			typ = reflect.PointerTo(typ)
		}
		return typ
	}
	_, ok = derefPointerType(pointerTypeOfDepth(maxPointerDepth - 1))
	require.True(t, ok, "a chain shorter than the bound still resolves")
	_, ok = derefPointerType(pointerTypeOfDepth(maxPointerDepth))
	require.False(t, ok, "at the bound the walk gives up")

	// The user visible effect of giving up
	deep := reflect.New(pointerTypeOfDepth(maxPointerDepth)).Elem()
	require.ErrorIs(t, SmartAssign(deep, reflect.ValueOf("42"), nil, nil, nil), errors.ErrUnsupported)
	shallow := reflect.New(pointerTypeOfDepth(maxPointerDepth - 1)).Elem()
	require.NoError(t, SmartAssign(shallow, reflect.ValueOf("42"), nil, nil, nil))
}

// textNumber has an integer kind and a MarshalText method, so assigning
// it to a string destination must use the text and not Go's
// string(rune) conversion, which would produce "*" for 42.
type textNumber int

func (t textNumber) MarshalText() ([]byte, error) {
	if t < 0 {
		return nil, errors.New("negative textNumber")
	}
	return []byte(strconv.Itoa(int(t))), nil
}

func TestSmartAssignTextMarshaler(t *testing.T) {
	var s string
	require.NoError(t, SmartAssign(reflect.ValueOf(&s).Elem(), reflect.ValueOf(textNumber(42)), nil, nil, nil))
	require.Equal(t, "42", s)

	// The text is parsed further, so a numeric destination works too
	var i int
	require.NoError(t, SmartAssign(reflect.ValueOf(&i).Elem(), reflect.ValueOf(textNumber(7)), nil, nil, nil))
	require.Equal(t, 7, i)

	// A failing MarshalText is reported, not swallowed
	err := SmartAssign(reflect.ValueOf(&s).Elem(), reflect.ValueOf(textNumber(-1)), nil, nil, nil)
	require.ErrorContains(t, err, "negative textNumber")
}

// TestSmartAssignRecoversPanic covers the deferred recover, which is
// load bearing: package reflect panics for a value read from an
// unexported field, and SmartAssign calls src.Interface() on every
// source. A table row must not take the whole program down.
func TestSmartAssignRecoversPanic(t *testing.T) {
	type hidden struct{ n int }
	src := reflect.ValueOf(hidden{n: 7}).Field(0)
	require.False(t, src.CanInterface(), "the test needs a value that panics on Interface()")

	var s string
	require.NotPanics(t, func() {
		err := SmartAssign(reflect.ValueOf(&s).Elem(), src, nil, nil, nil)
		require.Error(t, err)
	})
}

// TestParseTime covers the standalone ParseTime, which lost its only
// in-package caller when SmartAssign moved to parser.ParseTime. It
// duplicates the loop of StringParser.ParseTime over the same formats,
// so without a test the two can drift apart unnoticed while ParseTime
// stays part of the public API.
func TestParseTime(t *testing.T) {
	for _, tt := range []struct{ str, format string }{
		// RFC3339Nano is tried before RFC3339 and also matches a
		// timestamp without fractional seconds, so it is the layout
		// that is reported back.
		{str: "2024-03-15T14:30:00Z", format: time.RFC3339Nano},
		{str: "2024-03-15", format: time.DateOnly},
	} {
		t.Run(tt.str, func(t *testing.T) {
			got, format, err := ParseTime(tt.str)
			require.NoError(t, err)
			require.Equal(t, tt.format, format)

			viaParser, err := NewStringParser().ParseTime(tt.str)
			require.NoError(t, err)
			require.Equal(t, viaParser, got, "the two implementations must not diverge")
		})
	}

	_, _, err := ParseTime("not a date")
	require.Error(t, err)
}

// TestSmartAssignSelfReferentialPointerSource covers the source side of
// the self referential pointer walk. Bounding only the destination left
// this one: src.Elem() of a value pointing at itself is the same value,
// so the recursion never ends and overflows the stack, which is a fatal
// error that the deferred recover cannot turn into an error.
//
// A regression here crashes the whole test binary rather than failing
// this test, with a stack full of the SmartAssign source pointer branch.
func TestSmartAssignSelfReferentialPointerSource(t *testing.T) {
	var src selfPtr
	src = &src // legal Go: the value points at itself

	// An int destination has no catch-all below the pointer branch, so
	// this is where the guard is observable as an error.
	var dst int
	err := SmartAssign(reflect.ValueOf(&dst).Elem(), reflect.ValueOf(src), nil, nil, nil)
	require.ErrorIs(t, err, errors.ErrUnsupported)

	// A string destination reaches the fmt.Sprint fallback and renders
	// the pointer itself. That is not useful, but it returns, which is
	// the property under test.
	var str string
	require.NoError(t, SmartAssign(reflect.ValueOf(&str).Elem(), reflect.ValueOf(src), nil, nil, nil))
	require.NotEmpty(t, str)

	// A pointer source that does terminate is still dereferenced
	s := "text"
	require.NoError(t, SmartAssign(reflect.ValueOf(&str).Elem(), reflect.ValueOf(&s), nil, nil, nil))
	require.Equal(t, "text", str)
}
