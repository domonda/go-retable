package retable

import (
	"errors"
	"fmt"
	"reflect"
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
			err := SmartAssign(gotDst, tt.src, tt.scanner, tt.formatter)
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

func prefixFormatter(prefix string) Formatter {
	return FormatterFunc(func(v reflect.Value) (string, error) {
		return prefix + fmt.Sprint(v.Interface()), nil
	})
}

func assignableValue[T any]() reflect.Value {
	ptr := new(T)
	return reflect.ValueOf(ptr).Elem()
}
