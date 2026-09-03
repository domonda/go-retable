package retable

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestViewWithTitle(t *testing.T) {
	source := NewStringsView("Original", [][]string{{"A", "B"}, {"1", "2"}})
	renamed := ViewWithTitle(source, "Renamed")

	require.Equal(t, "Renamed", renamed.Title())
	require.Equal(t, "Original", source.Title(), "source view must not be modified")
	require.Equal(t, []string{"A", "B"}, renamed.ColumnNames())
	require.Equal(t, 1, renamed.NumRows())
	require.Equal(t, "1", renamed.Cell(0, 0))
	require.Equal(t, "2", renamed.Cell(0, 1))
}

// ViewWithTitle only replaces the title, so ReflectCell has to pass cell values
// through unchanged just like Cell does. Dereferencing is the job of DerefView
// and would panic here for the string cells of the most common View types.
func TestViewWithTitleReflectCellPassesValuesThrough(t *testing.T) {
	source := NewStringsView("Original", [][]string{{"A"}, {"1"}})
	renamed := ViewWithTitle(source, "Renamed").(ReflectCellView)

	cell := renamed.ReflectCell(0, 0)
	require.True(t, cell.IsValid())
	require.Equal(t, reflect.String, cell.Kind())
	require.Equal(t, "1", cell.String())
}

func TestViewWithTitleReflectCellKeepsPointers(t *testing.T) {
	value := 42
	source := &AnyValuesView{
		TableTitle: "Original",
		Cols:       []string{"A"},
		Rows:       [][]any{{&value}},
	}
	renamed := ViewWithTitle(source, "Renamed").(ReflectCellView)

	cell := renamed.ReflectCell(0, 0)
	require.Equal(t, reflect.Pointer, cell.Kind(), "pointer cells must not be dereferenced")
	require.Equal(t, 42, cell.Elem().Interface())

	// DerefView is the decorator that dereferences pointer cells.
	require.Equal(t, 42, DerefView(renamed).Cell(0, 0))
}
