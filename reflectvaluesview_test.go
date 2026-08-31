package retable

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// ReflectValuesView materializes a source view as reflect.Values, which
// is what lets a formatter dispatch on the cell type without re-reading
// the source. The copy has to preserve the types, not just the values.
func TestNewReflectValuesViewFrom(t *testing.T) {
	source := &AnyValuesView{
		TableTitle: "Mixed",
		Cols:       []string{"Name", "Age", "Active"},
		Rows: [][]any{
			{"Erik", 42, true},
			{"Ann", 7, false},
		},
	}
	view, err := NewReflectValuesViewFrom(source)
	require.NoError(t, err)

	require.Equal(t, "Mixed", view.Title())
	require.Equal(t, []string{"Name", "Age", "Active"}, view.ColumnNames())
	require.Equal(t, 2, view.NumRows())

	t.Run("values and their types survive the copy", func(t *testing.T) {
		require.Equal(t, "Erik", view.Cell(0, 0))
		require.Equal(t, 42, view.Cell(0, 1))
		require.Equal(t, true, view.Cell(0, 2))
		require.Equal(t, reflect.TypeFor[int](), view.ReflectCell(0, 1).Type(),
			"an int cell must not be flattened to a string")
	})

	t.Run("out of range positions are reported, not indexed", func(t *testing.T) {
		require.Nil(t, view.Cell(2, 0))
		require.Nil(t, view.Cell(0, 3))
		require.Nil(t, view.Cell(-1, 0))
		require.Nil(t, view.Cell(0, -1))
		require.False(t, view.ReflectCell(2, 0).IsValid())
		require.False(t, view.ReflectCell(0, -1).IsValid())
	})

	t.Run("a nil source is an error, not a panic", func(t *testing.T) {
		_, err := NewReflectValuesViewFrom(nil)
		require.Error(t, err)
	})
}

// A nil interface cell has no reflect.Value to unwrap, and
// reflect.Value.Interface panics for a zero Value. Cell has to report
// it as no value, the way it already reports an out of range position,
// or a single nil in a source table takes the caller down.
func TestReflectValuesViewNilCell(t *testing.T) {
	source := &AnyValuesView{
		Cols: []string{"a", "b"},
		Rows: [][]any{{nil, "present"}},
	}
	view, err := NewReflectValuesViewFrom(source)
	require.NoError(t, err)

	require.False(t, view.ReflectCell(0, 0).IsValid(), "the fixture needs an invalid stored value")
	require.NotPanics(t, func() {
		require.Nil(t, view.Cell(0, 0))
	})
	require.Equal(t, "present", view.Cell(0, 1), "the neighbouring cell is unaffected")
}

// SingleReflectValueView wraps one cell as a one by one view, which is
// how a single value is handed to something that expects a table.
func TestNewSingleReflectValueView(t *testing.T) {
	source := NewStringsView("People", [][]string{
		{"Name", "Age"},
		{"Erik", "42"},
		{"Ann", "7"},
	})

	view := NewSingleReflectValueView(source, 1, 1)
	require.Equal(t, "People", view.Title())
	require.Equal(t, []string{"Age"}, view.ColumnNames(), "only the selected column is named")
	require.Equal(t, 1, view.NumRows())
	require.Equal(t, "7", view.Cell(0, 0))
	require.Equal(t, "7", view.ReflectCell(0, 0).Interface())

	t.Run("only position 0,0 exists", func(t *testing.T) {
		require.Nil(t, view.Cell(1, 0))
		require.Nil(t, view.Cell(0, 1))
		require.Nil(t, view.Cell(-1, 0))
		require.False(t, view.ReflectCell(1, 0).IsValid())
	})
}

// A position outside the source produces a view with no value in it.
// Reading that value must report no value rather than panic, because
// the constructor accepts the out of range position silently.
func TestNewSingleReflectValueViewOutOfRange(t *testing.T) {
	source := NewStringsView("T", [][]string{{"A"}, {"a0"}})

	for _, tt := range []struct {
		name     string
		row, col int
	}{
		{name: "row past the end", row: 5, col: 0},
		{name: "col past the end", row: 0, col: 5},
		{name: "negative row", row: -1, col: 0},
		{name: "negative col", row: 0, col: -1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			view := NewSingleReflectValueView(source, tt.row, tt.col)
			require.Equal(t, "T", view.Title(), "the title of the source is still known")
			require.NotPanics(t, func() {
				require.Nil(t, view.Cell(0, 0))
			})
			require.False(t, view.ReflectCell(0, 0).IsValid())
		})
	}
}

// A nil source must not be dereferenced by the very branch that guards
// against it.
func TestNewSingleReflectValueViewNilSource(t *testing.T) {
	var view *SingleReflectValueView
	require.NotPanics(t, func() {
		view = NewSingleReflectValueView(nil, 0, 0)
	})
	require.NotNil(t, view)
	require.Equal(t, "", view.Title())
	require.Nil(t, view.Cell(0, 0))
}

// A cell that is a nil interface has no value to unwrap either.
func TestNewSingleReflectValueViewNilCell(t *testing.T) {
	source := &AnyValuesView{Cols: []string{"a"}, Rows: [][]any{{nil}}}

	view := NewSingleReflectValueView(source, 0, 0)
	require.NotPanics(t, func() {
		require.Nil(t, view.Cell(0, 0))
	})
	require.False(t, view.ReflectCell(0, 0).IsValid())
}
