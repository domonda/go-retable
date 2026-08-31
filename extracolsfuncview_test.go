package retable

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// ExtraColsAnyValueFuncView appends columns whose values are computed
// on demand, so nothing is materialized for them. The column index
// passed to the function is local to the appended columns, and getting
// that translation wrong feeds the function a base-table index and
// silently computes the wrong column.
func TestExtraColsAnyValueFuncView(t *testing.T) {
	base := NewStringsView("People", [][]string{
		{"Name", "Age"},
		{"Erik", "42"},
		{"Ann", "7"},
	})
	var gotCalls [][2]int
	view := ExtraColsAnyValueFuncView(base, []string{"Row", "Doubled"},
		func(row, col int) any {
			gotCalls = append(gotCalls, [2]int{row, col})
			if col == 0 {
				return row
			}
			return row * 2
		},
	)

	require.Equal(t, "People", view.Title(), "the title comes from the left view")
	require.Equal(t, []string{"Name", "Age", "Row", "Doubled"}, view.ColumnNames())
	require.Equal(t, 2, view.NumRows(), "the left view decides the row count")

	t.Run("the left columns are passed through", func(t *testing.T) {
		require.Equal(t, "Erik", view.Cell(0, 0))
		require.Equal(t, "42", view.Cell(0, 1))
		require.Equal(t, "Ann", view.Cell(1, 0))
	})

	t.Run("the function gets a column index local to the extra columns", func(t *testing.T) {
		gotCalls = nil
		require.Equal(t, 1, view.Cell(1, 2), "the first extra column is col 0 for the function")
		require.Equal(t, 2, view.Cell(1, 3), "the second extra column is col 1 for the function")
		require.Equal(t, [][2]int{{1, 0}, {1, 1}}, gotCalls)
	})

	t.Run("ReflectCell wraps the same computed value", func(t *testing.T) {
		rv := view.ReflectCell(1, 2)
		require.True(t, rv.IsValid())
		require.Equal(t, 1, rv.Interface())

		left := view.ReflectCell(0, 0)
		require.True(t, left.IsValid())
		require.Equal(t, "Erik", left.Interface())
	})
}

// ExtraColsReflectValueFuncView is the reflect-native counterpart. Its
// Cell has to unwrap the reflect.Value, and an invalid one means "no
// value" rather than a panic, which is what a function returns for a
// row it cannot compute.
func TestExtraColsReflectValueFuncView(t *testing.T) {
	base := NewStringsView("", [][]string{
		{"Name"},
		{"Erik"},
		{"Ann"},
	})
	view := ExtraColsReflectValueFuncView(base, []string{"Len"},
		func(row, col int) reflect.Value {
			if row == 1 {
				// Nothing to report for this row
				return reflect.Value{}
			}
			return reflect.ValueOf(strconv.Itoa(row))
		},
	)

	require.Equal(t, []string{"Name", "Len"}, view.ColumnNames())
	require.Equal(t, 2, view.NumRows())

	t.Run("a valid value is unwrapped by Cell", func(t *testing.T) {
		require.Equal(t, "0", view.Cell(0, 1))
		require.Equal(t, "0", view.ReflectCell(0, 1).Interface())
	})

	t.Run("an invalid value is nil rather than a panic", func(t *testing.T) {
		require.Nil(t, view.Cell(1, 1))
		require.False(t, view.ReflectCell(1, 1).IsValid())
	})
}

// A view with no extra columns still has to behave, because a caller
// building the column list dynamically can end up with an empty one.
func TestExtraColsFuncViewNoExtraColumns(t *testing.T) {
	base := NewStringsView("T", [][]string{{"A"}, {"a0"}})
	view := ExtraColsAnyValueFuncView(base, nil, func(row, col int) any {
		t.Fatal("the value function must not be called when there are no extra columns")
		return nil
	})

	require.Equal(t, []string{"A"}, view.ColumnNames())
	require.Equal(t, 1, view.NumRows())
	require.Equal(t, "a0", view.Cell(0, 0))
}
