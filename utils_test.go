package retable

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSpacePascalCase(t *testing.T) {
	tests := []struct {
		testName string
		name     string
		want     string
	}{
		{testName: "", name: "", want: ""},
		{testName: "HelloWorld", name: "HelloWorld", want: "Hello World"},
		{testName: "_Hello_World", name: "_Hello_World", want: "Hello World"},
		{testName: "helloWorld", name: "helloWorld", want: "hello World"},
		{testName: "helloWorld_", name: "helloWorld_", want: "hello World"},
		{testName: "ThisHasMoreSpacesForSure", name: "ThisHasMoreSpacesForSure", want: "This Has More Spaces For Sure"},
		{testName: "ThisHasMore_Spaces__ForSure", name: "ThisHasMore_Spaces__ForSure", want: "This Has More Spaces For Sure"},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			if got := SpacePascalCase(tt.name); got != tt.want {
				t.Errorf("SpacePascalCase() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStructFieldIndex(t *testing.T) {
	type innerStruct struct {
		B int
		C bool
	}
	type testStruct struct {
		A int
		innerStruct
		private string
		D       bool
	}
	var s testStruct

	tests := []struct {
		name      string
		structPtr any
		fieldPtr  any
		want      int
		wantErr   bool
	}{
		{name: "A", structPtr: &s, fieldPtr: &s.A, want: 0},
		{name: "B", structPtr: &s, fieldPtr: &s.B, want: 1},
		{name: "C", structPtr: &s, fieldPtr: &s.C, want: 2},
		{name: "D", structPtr: &s, fieldPtr: &s.D, want: 3},

		// Errors
		{name: "nil, nil", structPtr: nil, fieldPtr: nil, wantErr: true},
		{name: "unexported field", structPtr: &s, fieldPtr: &s.private, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StructFieldIndex(tt.structPtr, tt.fieldPtr)
			if (err != nil) != tt.wantErr {
				t.Errorf("StructFieldIndex() error = '%s', wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("StructFieldIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValueLikeNil(t *testing.T) {
	var nilInterface any
	var nilInt *int
	var nilSlice []int
	tests := []struct {
		name string
		val  reflect.Value
		want bool
	}{
		// true
		{name: "reflect.Value{}", val: reflect.Value{}, want: true},
		{name: "<nil> interface{}", val: reflect.ValueOf(nilInterface), want: true},
		{name: "<nil> int", val: reflect.ValueOf(nilInt), want: true},
		{name: "<nil> []int", val: reflect.ValueOf(nilSlice), want: true},
		{name: "struct{}{}", val: reflect.ValueOf(struct{}{}), want: true},

		// false
		{name: "any(0)", val: reflect.ValueOf(any(0)), want: false},
		{name: "empty string", val: reflect.ValueOf(""), want: false},
		{name: "empty slice", val: reflect.ValueOf([]int{}), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNullLike(tt.val); got != tt.want {
				t.Errorf("ValueLikeNil() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveEmptyStringRows(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name string
		rows [][]string
		want [][]string
	}{
		{name: "nil", rows: nil, want: nil},
		{name: "empty", rows: [][]string{}, want: [][]string{}},
		{
			name: "1 empty row",
			rows: [][]string{
				{"", "", ""},
			},
			want: [][]string{},
		},
		{
			name: "0 1 0",
			rows: [][]string{
				{"", "", ""},
				{"", "X", ""},
				{"", "", ""},
			},
			want: [][]string{
				{"", "X", ""},
			},
		},
		{
			name: "nil 1 nil",
			rows: [][]string{
				nil,
				{"", "X", ""},
				nil,
			},
			want: [][]string{
				{"", "X", ""},
			},
		},
		{
			name: "mixed",
			rows: [][]string{
				{""},
				{"", "X", ""},
				{"", "", ""},
				nil,
				{"A", "B", "C", "D"},
				{"", ""},
			},
			want: [][]string{
				{"", "X", ""},
				{"A", "B", "C", "D"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RemoveEmptyStringRows(tt.rows); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RemoveEmptyStringRows() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestRemoveEmptyStringColumns(t *testing.T) {
	tests := []struct {
		name        string
		rows        [][]string
		wantNumCols int
		wantRows    [][]string
	}{
		{name: "nil", rows: nil, wantNumCols: 0, wantRows: nil},
		{name: "empty", rows: [][]string{}, wantNumCols: 0, wantRows: [][]string{}},
		{
			name: "1 empty row",
			rows: [][]string{
				{"", "", ""},
			},
			wantNumCols: 0,
			wantRows:    [][]string{{}},
		},
		{
			name: "mixed rem col 0 and 2",
			rows: [][]string{
				{""},
				{"", "X", ""},
				{"", "", ""},
				nil,
				{"", "A", "", "B"},
				{"", "", "", ""},
			},
			wantNumCols: 2,
			wantRows: [][]string{
				{},
				{"X"},
				{""},
				nil,
				{"A", "B"},
				{"", ""},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNumCols := RemoveEmptyStringColumns(tt.rows)
			require.Equal(t, tt.wantNumCols, gotNumCols, "number of columns")
			require.True(t, equalStringRows(tt.wantRows, tt.rows), "rows are equal")
		})
	}
}

func equalStringRows(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for row := range a {
		if len(a[row]) != len(b[row]) {
			return false
		}
		for col := range a[row] {
			if a[row][col] != b[row][col] {
				return false
			}
		}
	}
	return true
}

func ExamplePrintlnView() {
	PrintlnView(&StringsView{
		Tit:  "ExamplePrintlnView",
		Cols: []string{"A", "B", "C"},
		Rows: [][]string{
			{"1", "2222222222", "3"},
			{"", "", "3333"},
			{"Last row"},
		},
	})

	// Output:
	// ExamplePrintlnView:
	// | A        | B          | C    |
	// | 1        | 2222222222 | 3    |
	// |          |            | 3333 |
	// | Last row |            |      |
}

func ExamplePrintlnTable() {
	type Row struct {
		A string
		B int
		C *time.Time
	}
	t := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	PrintlnTable("ExamplePrintlnTable", []Row{
		{A: "1", B: -1, C: &t},
		{A: "", B: 2222222222, C: nil},
		{A: "Last row", B: 0, C: nil},
	})

	// Output:
	// ExamplePrintlnTable:
	// | A        | B          | C                             |
	// | 1        | -1         | 2024-01-02 03:04:05 +0000 UTC |
	// |          | 2222222222 |                               |
	// | Last row | 0          |                               |

}
