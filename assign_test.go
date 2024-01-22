package retable

import (
	"reflect"
	"testing"

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
