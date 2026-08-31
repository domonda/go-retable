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
		{testName: "HTTPServer", name: "HTTPServer", want: "HTTPServer"},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			if got := SpacePascalCase(tt.name); got != tt.want {
				t.Errorf("SpacePascalCase(%#v) = %#v, want %#v", tt.name, got, tt.want)
			}
		})
	}
}

func TestSpaceGoCase(t *testing.T) {
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
		{testName: "HTTPServer", name: "HTTPServer", want: "HTTP Server"},
		{testName: "MyXMLFile", name: "MyXMLFile", want: "My XML File"},
		{testName: "wantJPEG", name: "wantJPEG", want: "want JPEG"},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			if got := SpaceGoCase(tt.name); got != tt.want {
				t.Errorf("SpaceGoCase(%#v) = %#v, want %#v", tt.name, got, tt.want)
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

func TestIndexedStructFieldReflectValues(t *testing.T) {
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
	s := testStruct{A: 1, innerStruct: innerStruct{B: 2, C: true}, private: "ignored", D: false}

	tests := []struct {
		name        string
		structValue reflect.Value
		numVals     int
		indices     []int
		// want holds the values of the returned reflect.Values,
		// nil for positions that must stay the zero reflect.Value
		want []any
	}{
		{name: "reorder and skip", structValue: reflect.ValueOf(s), numVals: 3, indices: []int{1, -1, 2, 0}, want: []any{false, 1, true}},
		{name: "struct pointer", structValue: reflect.ValueOf(&s), numVals: 3, indices: []int{1, -1, 2, 0}, want: []any{false, 1, true}},
		{name: "all fields skipped", structValue: reflect.ValueOf(s), numVals: 2, indices: []int{-1, -1, -1, -1}, want: []any{nil, nil}},
		{name: "more values than indexed fields", structValue: reflect.ValueOf(s), numVals: 3, indices: []int{0, -1, -1, -1}, want: []any{1, nil, nil}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vals := IndexedStructFieldReflectValues(tt.structValue, tt.numVals, tt.indices)
			require.Len(t, vals, tt.numVals)
			for i, val := range vals {
				var got any
				if val.IsValid() {
					got = val.Interface()
				}
				require.Equalf(t, tt.want[i], got, "value at index %d", i)
			}
		})
	}

	// The number of indices must match the number of flattened fields
	// of the struct, else the mapping from fields to values is undefined.
	require.PanicsWithError(t, "got 3 indices for struct with 4 fields", func() {
		IndexedStructFieldReflectValues(reflect.ValueOf(s), 3, []int{0, 1, 2})
	})
	require.PanicsWithError(t, "got 5 indices for struct with 4 fields", func() {
		IndexedStructFieldReflectValues(reflect.ValueOf(s), 5, []int{0, 1, 2, 3, 4})
	})
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
		TableTitle: "ExamplePrintlnView",
		Cols:       []string{"A", "B", "C"},
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

// TestSprintlnViewAndTable covers the two Sprintln helpers, which had no
// test. They build the result into a strings.Builder and return it, so
// unlike the Fprintln pair they take no writer.
func TestSprintlnViewAndTable(t *testing.T) {
	view := NewStringsView("People", [][]string{
		{"Name", "Age"},
		{"Erik", "42"},
	})

	str, err := SprintlnView(view)
	require.NoError(t, err)
	require.Contains(t, str, "Name")
	require.Contains(t, str, "Erik")
	require.Contains(t, str, "42")

	type Person struct {
		Name string
		Age  int
	}
	str, err = SprintlnTable("People", []Person{{Name: "Erik", Age: 42}})
	require.NoError(t, err)
	require.Contains(t, str, "People")
	require.Contains(t, str, "Name")
	require.Contains(t, str, "Erik")

	// A table type that has no viewer is reported
	_, err = SprintlnTable("", 42)
	require.Error(t, err)
}

// TestMustStructFieldIndex covers the panicking field-index lookup,
// which had no test. It resolves a field pointer to its index so that a
// column can be named by the field itself instead of by a string.
func TestMustStructFieldIndex(t *testing.T) {
	type Person struct {
		Name string
		Age  int
		City string
	}
	p := new(Person)

	require.Equal(t, 0, MustStructFieldIndex(p, &p.Name))
	require.Equal(t, 1, MustStructFieldIndex(p, &p.Age))
	require.Equal(t, 2, MustStructFieldIndex(p, &p.City))

	// A pointer that is not a field of the struct has no index
	other := new(Person)
	require.Panics(t, func() { MustStructFieldIndex(p, &other.Name) })
}

// TestNoTagsStructRowsViewer covers the viewer that ignores struct tags,
// which had no test. It must use the field names even when a tag would
// have named the column differently.
func TestNoTagsStructRowsViewer(t *testing.T) {
	type Person struct {
		Name string `col:"Vorname"`
		Age  int    `col:"Alter"`
	}
	view, err := NoTagsStructRowsViewer().NewView("", []Person{{Name: "Erik", Age: 42}})
	require.NoError(t, err)
	require.Equal(t, []string{"Name", "Age"}, view.ColNames(), "the col tags must be ignored")
	require.Equal(t, 1, view.NumRows())
}
