package retable

import (
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

func assignableValue[T any]() reflect.Value {
	ptr := new(T)
	return reflect.ValueOf(ptr).Elem()
}
