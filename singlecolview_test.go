package retable

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// SingleColView turns a plain slice into a one column table, which is
// the cheapest way to print or write a list of values.
func TestSingleColView(t *testing.T) {
	view := SingleColView("Name", []string{"Erik", "Ann", "Bo"})

	require.Equal(t, []string{"Name"}, view.Columns())
	require.Equal(t, 3, view.NumRows())
	require.Equal(t, "Erik", view.Cell(0, 0))
	require.Equal(t, "Bo", view.Cell(2, 0))

	// SingleColView takes no title, so the column name is used for it
	require.Equal(t, "Name", view.Title())

	t.Run("only column 0 exists", func(t *testing.T) {
		require.Nil(t, view.Cell(0, 1))
		require.Nil(t, view.Cell(0, -1))
	})

	t.Run("rows outside the slice are nil, not an index panic", func(t *testing.T) {
		require.Nil(t, view.Cell(3, 0))
		require.Nil(t, view.Cell(-1, 0))
	})

	t.Run("the cell type is preserved", func(t *testing.T) {
		ints := SingleColView("N", []int{1, 2})
		require.Equal(t, 1, ints.Cell(0, 0))
		require.Equal(t, reflect.TypeFor[int](),
			AsReflectCellView(ints).ReflectCell(0, 0).Type(),
			"an int column must not be flattened to a string")
	})

	t.Run("an empty slice is an empty table", func(t *testing.T) {
		empty := SingleColView("N", []string{})
		require.Zero(t, empty.NumRows())
		require.Nil(t, empty.Cell(0, 0))
	})
}

// A slice of reflect.Value is the one case that needs unwrapping rather
// than wrapping, or every cell would be reported as a reflect.Value
// instead of the value it holds.
func TestSingleColViewOfReflectValues(t *testing.T) {
	view := SingleColView("V", []reflect.Value{
		reflect.ValueOf("text"),
		reflect.ValueOf(42),
		{}, // a value that is not there
	})
	reflectView := AsReflectCellView(view)

	require.Equal(t, "text", view.Cell(0, 0), "the held value, not the reflect.Value wrapping it")
	require.Equal(t, 42, view.Cell(1, 0))
	require.Equal(t, reflect.TypeFor[int](), reflectView.ReflectCell(1, 0).Type())

	require.Nil(t, view.Cell(2, 0), "an invalid reflect.Value has no value to unwrap")
	require.False(t, reflectView.ReflectCell(2, 0).IsValid())
}

// SingleCellView wraps one value as a one by one table.
func TestSingleCellView(t *testing.T) {
	view := SingleCellView("Count", "Total", 42)

	require.Equal(t, []string{"Total"}, view.Columns())
	require.Equal(t, 1, view.NumRows())
	require.Equal(t, 42, view.Cell(0, 0))

	// The title argument is the title, which is what the documented
	// example of this constructor has always shown. It used to be
	// dropped and the column name reported instead.
	require.Equal(t, "Count", view.Title())

	require.Nil(t, view.Cell(1, 0))
	require.Nil(t, view.Cell(0, 1))

	// A title and a column name that differ must not be confused for
	// each other, which a view reporting the column as its title would.
	require.NotEqual(t, view.Title(), view.Columns()[0])

	t.Run("an empty title stays empty", func(t *testing.T) {
		untitled := SingleCellView("", "Total", 42)
		require.Equal(t, "", untitled.Title(), "no title means no title, not the column name")
		require.Equal(t, []string{"Total"}, untitled.Columns())
	})
}

func TestSingleCellViewOfReflectValue(t *testing.T) {
	view := SingleCellView("", "V", reflect.ValueOf("text"))
	require.Equal(t, "text", view.Cell(0, 0), "the held value, not the reflect.Value wrapping it")

	invalid := SingleCellView("", "V", reflect.Value{})
	require.Nil(t, invalid.Cell(0, 0))
}
