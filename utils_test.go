package retable

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"testing"
	"time"
	"unsafe"

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

// Types for TestIsNullLike, one per method rule of IsNullLike.
// The value receivers report null for an empty string,
// the pointer receivers for an empty string as well.
type (
	nullLikeValueIsNull struct{ s string }
	nullLikePtrIsNull   struct{ s string }
	nullLikeValueIsZero struct{ s string }
	nullLikePtrIsZero   struct{ s string }
	nullLikeNotNullZero struct{}
	nullLikeValuer      struct {
		v   driver.Value
		err error
	}
)

func (v nullLikeValueIsNull) IsNull() bool            { return v.s == "" }
func (p *nullLikePtrIsNull) IsNull() bool             { return p.s == "" }
func (v nullLikeValueIsZero) IsZero() bool            { return v.s == "" }
func (p *nullLikePtrIsZero) IsZero() bool             { return p.s == "" }
func (nullLikeNotNullZero) IsNull() bool              { return false }
func (nullLikeNotNullZero) IsZero() bool              { return true }
func (v nullLikeValuer) Value() (driver.Value, error) { return v.v, v.err }

// addressable returns val as an addressable reflect.Value,
// like a field read from a struct behind a pointer.
func addressable(val any) reflect.Value {
	ptr := reflect.New(reflect.TypeOf(val))
	ptr.Elem().Set(reflect.ValueOf(val))
	return ptr.Elem()
}

// interfaceHeld returns val as the reflect.Value of a struct field of type any,
// which is what a view of structs with an any field yields.
func interfaceHeld(val any) reflect.Value {
	holder := struct{ A any }{A: val}
	return reflect.ValueOf(&holder).Elem().Field(0)
}

// TestIsNullLike covers every rule of IsNullLike positive and negative,
// in value, pointer, pointer-to-pointer and interface-held form,
// and pins the cases that changed when IsNullLike started to look
// through non-nil pointers (marked "changed").
func TestIsNullLike(t *testing.T) {
	var (
		nilInterface any
		nilInt       *int
		nilIntPtr    = &nilInt
		nilSlice     []int
		nilMap       map[string]int
		nilChan      chan int
		nilFunc      func()
		zeroTime     time.Time
		someTime     = time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
		one          = 1
		onePtr       = &one
	)
	unexported := struct{ v nullLikeValueIsNull }{}
	// A self referential value is legal Go and has no end to unwrap
	selfAny := new(any)
	*selfAny = selfAny
	var selfPointer selfPtr
	selfPointer = &selfPointer
	tests := []struct {
		name string
		val  reflect.Value
		want bool
	}{
		// Rule 1: pointer and interface chain
		{name: "nil *int", val: reflect.ValueOf(nilInt), want: true},
		{name: "**int to nil *int (changed)", val: reflect.ValueOf(nilIntPtr), want: true},
		{name: "**int to *int to 1", val: reflect.ValueOf(&onePtr), want: false},
		{name: "*int to 1", val: reflect.ValueOf(onePtr), want: false},
		{name: "any holding typed nil pointer (changed)", val: interfaceHeld((*nullLikeValueIsNull)(nil)), want: true},
		{name: "any holding nil", val: interfaceHeld(nil), want: true},
		{name: "any holding 1", val: interfaceHeld(1), want: false},
		{name: "pointer to zero time.Time (changed)", val: reflect.ValueOf(&zeroTime), want: true},
		{name: "pointer to non-zero time.Time", val: reflect.ValueOf(&someTime), want: false},
		{name: "any holding a pointer to itself", val: reflect.ValueOf(selfAny), want: false},
		{name: "self referential pointer type", val: reflect.ValueOf(selfPointer), want: false},

		// Rule 2: invalid reflect.Value
		{name: "reflect.Value{}", val: reflect.Value{}, want: true},
		{name: "reflect.ValueOf(nil interface)", val: reflect.ValueOf(nilInterface), want: true},

		// Rule 3: nil-able kinds
		{name: "nil []int", val: reflect.ValueOf(nilSlice), want: true},
		{name: "empty []int", val: reflect.ValueOf([]int{}), want: false},
		{name: "nil map", val: reflect.ValueOf(nilMap), want: true},
		{name: "empty map", val: reflect.ValueOf(map[string]int{}), want: false},
		{name: "nil chan", val: reflect.ValueOf(nilChan), want: true},
		{name: "nil func", val: reflect.ValueOf(nilFunc), want: true},
		{name: "nil unsafe.Pointer", val: reflect.ValueOf(unsafe.Pointer(nil)), want: true},

		// Rule 4: empty struct
		{name: "struct{}{}", val: reflect.ValueOf(struct{}{}), want: true},
		{name: "pointer to struct{}{} (changed)", val: reflect.ValueOf(&struct{}{}), want: true},

		// Rule 5: unexported field (used to panic)
		{name: "unexported field (changed)", val: reflect.ValueOf(unexported).Field(0), want: false},

		// Rule 6: IsNull with value receiver
		{name: "IsNull value receiver true", val: reflect.ValueOf(nullLikeValueIsNull{}), want: true},
		{name: "IsNull value receiver false", val: reflect.ValueOf(nullLikeValueIsNull{"x"}), want: false},
		{name: "pointer to IsNull true (changed)", val: reflect.ValueOf(&nullLikeValueIsNull{}), want: true},
		{name: "pointer to IsNull false", val: reflect.ValueOf(&nullLikeValueIsNull{"x"}), want: false},
		{name: "any holding IsNull true (changed)", val: interfaceHeld(nullLikeValueIsNull{}), want: true},

		// Rule 6: IsNull with pointer receiver
		{name: "IsNull pointer receiver, addressable, true (changed)", val: addressable(nullLikePtrIsNull{}), want: true},
		{name: "IsNull pointer receiver, addressable, false", val: addressable(nullLikePtrIsNull{"x"}), want: false},
		{name: "IsNull pointer receiver, not addressable", val: reflect.ValueOf(nullLikePtrIsNull{}), want: false},
		{name: "pointer with IsNull pointer receiver true (changed)", val: reflect.ValueOf(&nullLikePtrIsNull{}), want: true},

		// Rule 6: IsZero
		{name: "IsZero value receiver true", val: reflect.ValueOf(nullLikeValueIsZero{}), want: true},
		{name: "IsZero value receiver false", val: reflect.ValueOf(nullLikeValueIsZero{"x"}), want: false},
		{name: "IsZero pointer receiver, addressable, true (changed)", val: addressable(nullLikePtrIsZero{}), want: true},
		{name: "IsZero pointer receiver, not addressable", val: reflect.ValueOf(nullLikePtrIsZero{}), want: false},
		{name: "zero time.Time", val: reflect.ValueOf(zeroTime), want: true},
		{name: "non-zero time.Time", val: reflect.ValueOf(someTime), want: false},
		{name: "IsNull false but IsZero true", val: reflect.ValueOf(nullLikeNotNullZero{}), want: true},

		// Rule 6: driver.Valuer
		{name: "Valuer (nil, nil)", val: reflect.ValueOf(nullLikeValuer{}), want: true},
		{name: "Valuer (x, nil)", val: reflect.ValueOf(nullLikeValuer{v: "x"}), want: false},
		{name: "Valuer (nil, err)", val: reflect.ValueOf(nullLikeValuer{err: errors.New("e")}), want: false},
		{name: "sql.NullString{}", val: reflect.ValueOf(sql.NullString{}), want: true},
		{name: "sql.NullString valid", val: reflect.ValueOf(sql.NullString{String: "", Valid: true}), want: false},
		{name: "pointer to sql.NullInt64{} (changed)", val: reflect.ValueOf(&sql.NullInt64{}), want: true},

		// Not null-like
		{name: "0", val: reflect.ValueOf(0), want: false},
		{name: "false", val: reflect.ValueOf(false), want: false},
		{name: "empty string", val: reflect.ValueOf(""), want: false},
		{name: "42", val: reflect.ValueOf(42), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bool
			require.NotPanics(t, func() { got = IsNullLike(tt.val) })
			require.Equal(t, tt.want, got)
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
	require.Equal(t, []string{"Name", "Age"}, view.ColumnNames(), "the col tags must be ignored")
	require.Equal(t, 1, view.NumRows())
}

// TestStructFieldValues_NilPointer: a nil struct pointer has no field values,
// so every field is nil or an invalid reflect.Value, including the fields
// of embedded structs, and the count still matches StructFieldTypes.
func TestStructFieldValues_NilPointer(t *testing.T) {
	type inner struct {
		B int
		C bool
	}
	type outer struct {
		A int
		*inner
		D string
	}
	var nilOuter *outer

	require.Equal(t, []any{nil, nil, nil, nil}, StructFieldAnyValues(reflect.ValueOf(nilOuter)))
	reflectVals := StructFieldReflectValues(reflect.ValueOf(nilOuter))
	require.Len(t, reflectVals, 4)
	for i, val := range reflectVals {
		require.False(t, val.IsValid(), "field %d", i)
	}

	// A nil embedded pointer inside a non-nil struct
	s := outer{A: 1, D: "d"}
	require.Equal(t, []any{1, nil, nil, "d"}, StructFieldAnyValues(reflect.ValueOf(s)))
	vals := IndexedStructFieldReflectValues(reflect.ValueOf(s), 4, []int{3, 2, 1, 0})
	require.Equal(t, "d", vals[0].Interface())
	require.False(t, vals[1].IsValid())
	require.False(t, vals[2].IsValid())
	require.Equal(t, 1, vals[3].Interface())
	require.Equal(t, []any{"d", nil, nil, 1}, IndexedStructFieldAnyValues(reflect.ValueOf(s), 4, []int{3, 2, 1, 0}))

	// The index count check still sees all flattened fields
	require.Panics(t, func() { IndexedStructFieldReflectValues(reflect.ValueOf(nilOuter), 1, []int{0}) })
}

// embeddedCode is a non struct type embedded in a row struct.
type embeddedCode string

// TestStructFieldValues_EmbeddedNonStructType covers that the struct field
// walkers skip an embedded type that is no struct, like the read side does,
// instead of panicking with "Fields of non-struct type".
func TestStructFieldValues_EmbeddedNonStructType(t *testing.T) {
	type row struct {
		Name string
		embeddedCode
		error // an embedded interface
		Count int
	}
	r := row{Name: "x", embeddedCode: "c", Count: 2}
	v := reflect.ValueOf(r)

	fields := StructFieldTypes(v.Type())
	require.Len(t, fields, 2)
	require.Equal(t, "Name", fields[0].Name)
	require.Equal(t, "Count", fields[1].Name)
	require.Equal(t, []any{"x", 2}, StructFieldAnyValues(v))
	require.Len(t, StructFieldReflectValues(v), 2)
	vals := IndexedStructFieldReflectValues(v, 2, []int{1, 0})
	require.Equal(t, 2, vals[0].Interface())
	require.Equal(t, "x", vals[1].Interface())
	require.Equal(t, []any{2, "x"}, IndexedStructFieldAnyValues(v, 2, []int{1, 0}))

	view, err := DefaultStructRowsViewer().NewView("", []row{r})
	require.NoError(t, err)
	require.Equal(t, []string{"Name", "Count"}, view.ColumnNames())
	require.Equal(t, "x", view.Cell(0, 0))
	require.Equal(t, 2, view.Cell(0, 1))
}
