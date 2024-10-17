package retable

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStructFieldNaming_Columns(t *testing.T) {
	type StructWithFloat struct {
		Float float64 `col:"float"`
	}
	tests := []struct {
		name   string
		naming *StructFieldNaming
		strct  any
		want   []string
	}{
		{
			name:   "empty struct, nil naming",
			naming: nil,
			strct:  struct{}{},
			want:   []string{},
		},
		{
			name:   "exported names, nil naming",
			naming: nil,
			strct: struct {
				Int  int
				Bool bool
			}{},
			want: []string{"Int", "Bool"},
		},
		{
			name:   "exported and private names, nil naming",
			naming: nil,
			strct: struct {
				Int    int
				Bool   bool
				hidden string
			}{},
			want: []string{"Int", "Bool"},
		},
		{
			name:   "mixed, nil naming",
			naming: nil,
			strct: struct {
				Int int
				StructWithFloat
				Struct struct {
					Sub bool
				}
				hidden string
			}{},
			want: []string{"Int", "Float", "Struct"},
		},

		{
			name:   "empty struct, DefaultStructFieldNaming",
			naming: &DefaultStructFieldNaming,
			strct:  struct{}{},
			want:   []string{},
		},
		{
			name:   "exported names, DefaultStructFieldNaming",
			naming: &DefaultStructFieldNaming,
			strct: struct {
				Int  int
				Bool bool `col:"boolean"`
			}{},
			want: []string{"Int", "boolean"},
		},
		{
			name:   "exported and private names, DefaultStructFieldNaming",
			naming: &DefaultStructFieldNaming,
			strct: struct {
				Int        int  `col:"Integer"`
				Bool       bool `col:"-"`
				hidden     string
				HelloWorld string
			}{},
			want: []string{"Integer", "Hello World"},
		},
		{
			name:   "mixed, DefaultStructFieldNaming",
			naming: &DefaultStructFieldNaming,
			strct: struct {
				hidden string `col:"-"`
				Int    int
				StructWithFloat
				Struct struct {
					Sub bool
				}
			}{},
			want: []string{"Int", "float", "Struct"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.naming.Columns(tt.strct)
			require.Equal(t, tt.want, got, "StructFieldNaming.Columns()")
		})
	}
}
