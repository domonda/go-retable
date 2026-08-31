package retable

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// Sources of rows like CSV files or Excel sheets omit trailing empty cells,
// so a data row is routinely shorter than the column count. Such a cell is
// still within the range of ColumnNames() and must therefore behave like an empty
// cell instead of like an out-of-range coordinate: reflection based formatters
// call reflect.Value.Type on the result of ReflectCell, which panics for an
// invalid reflect.Value, and ViewToStructSlice skips cells with an invalid
// reflect.Value, which would silently bypass its validate func.
func TestStringsViewReflectCellSparseRow(t *testing.T) {
	view := NewStringsView("T", [][]string{{"1"}}, "A", "B")

	require.Equal(t, "", view.Cell(0, 1), "missing trailing cell must be an empty string")

	cell := view.ReflectCell(0, 1)
	require.True(t, cell.IsValid(), "missing trailing cell must yield a valid reflect.Value")
	require.Equal(t, reflect.String, cell.Kind())
	require.Equal(t, "", cell.String())

	require.False(t, view.ReflectCell(0, 2).IsValid(), "column index out of range must be invalid")
	require.False(t, view.ReflectCell(1, 0).IsValid(), "row index out of range must be invalid")
	require.False(t, view.ReflectCell(-1, 0).IsValid())
	require.False(t, view.ReflectCell(0, -1).IsValid())
}

// Implementing ReflectCellView natively saves AsReflectCellView the wrapper
// allocation it would otherwise make on every call, and those calls happen
// once per cell in the formatter paths.
func TestStringsViewIsReflectCellView(t *testing.T) {
	view := NewStringsView("T", [][]string{{"1"}}, "A")

	require.Same(t, view, AsReflectCellView(view), "must not be wrapped")
}

// A header row shorter than the widest data row is not a statement that the
// view has fewer columns, it is the same omitted-trailing-cells artifact that
// makes data rows short. Without widening, the cells past the end of the
// header row would be unreachable through both Cell and ReflectCell.
func TestNewStringsViewWidensShortHeaderRow(t *testing.T) {
	view := NewStringsView("T", [][]string{
		{"A"}, // Header row missing the name of the second column
		{"1", "2"},
	})

	require.Equal(t, []string{"A", ""}, view.ColumnNames())
	require.Equal(t, "1", view.Cell(0, 0))
	require.Equal(t, "2", view.Cell(0, 1), "cell past the end of the header row must be reachable")
	require.Equal(t, "2", view.ReflectCell(0, 1).String())
}

// Explicitly passed cols state the columns of the view, so a wider data row
// must not silently add columns the caller did not ask for.
func TestNewStringsViewDoesNotWidenExplicitCols(t *testing.T) {
	view := NewStringsView("T", [][]string{{"1", "2"}}, "A")

	require.Equal(t, []string{"A"}, view.ColumnNames())
	require.Nil(t, view.Cell(0, 1))
}

// StringsViewer.NewView passes its Cols field as the cols argument, so
// trimming in place would mutate the viewer and make it non-reusable.
func TestNewStringsViewDoesNotModifyPassedCols(t *testing.T) {
	cols := []string{" A ", " B "}
	view := NewStringsView("T", [][]string{{"1", "2"}}, cols...)

	require.Equal(t, []string{"A", "B"}, view.ColumnNames())
	require.Equal(t, []string{" A ", " B "}, cols, "passed cols must not be modified")
}

// The rows are owned by the caller, so using the first row as column names
// must not trim that row in place.
func TestNewStringsViewDoesNotModifyHeaderRow(t *testing.T) {
	rows := [][]string{{" A ", " B "}, {"1", "2"}}
	view := NewStringsView("T", rows)

	require.Equal(t, []string{"A", "B"}, view.ColumnNames())
	require.Equal(t, []string{" A ", " B "}, rows[0], "header row must not be modified")
}
