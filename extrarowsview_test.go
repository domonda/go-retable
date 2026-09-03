package retable

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ExtraRowsView stacks views on top of each other, which is how a totals
// or summary row is appended without copying the base table. The row
// index has to be translated to the view that owns it, and getting that
// wrong silently shows one view's row in another view's position.
func TestExtraRowView(t *testing.T) {
	base := NewStringsView("Sales", [][]string{
		{"Product", "Amount"},
		{"Cable", "10"},
		{"Screen", "250"},
	})
	totals := NewStringsView("ignored title", [][]string{
		{"ignored", "columns"},
		{"Total", "260"},
	})
	stacked := ExtraRowsView{base, totals}

	t.Run("title and columns come from the first view only", func(t *testing.T) {
		require.Equal(t, "Sales", stacked.Title())
		require.Equal(t, []string{"Product", "Amount"}, stacked.ColumnNames(),
			"the later views contribute rows, not columns")
	})

	t.Run("rows are the sum of all views", func(t *testing.T) {
		require.Equal(t, 3, stacked.NumRows())
	})

	t.Run("every cell comes from the view owning its row", func(t *testing.T) {
		want := [][]any{
			{"Cable", "10"},
			{"Screen", "250"},
			{"Total", "260"},
		}
		for row := range want {
			for col := range want[row] {
				require.Equalf(t, want[row][col], stacked.Cell(row, col), "Cell(%d, %d)", row, col)
			}
		}
	})

	t.Run("out of range positions are nil, not a panic", func(t *testing.T) {
		require.Nil(t, stacked.Cell(3, 0), "one past the last row")
		require.Nil(t, stacked.Cell(0, 2), "one past the last column")
		require.Nil(t, stacked.Cell(-1, 0))
		require.Nil(t, stacked.Cell(0, -1))
	})
}

// A later view with more columns than the first must not widen the
// stack: the first view defines the columns, so the extra ones are not
// reachable and must not be reported either.
func TestExtraRowViewIgnoresExtraColumnsOfLaterViews(t *testing.T) {
	first := NewStringsView("", [][]string{
		{"A"},
		{"a1"},
	})
	wider := NewStringsView("", [][]string{
		{"A", "B"},
		{"a2", "b2"},
	})
	stacked := ExtraRowsView{first, wider}

	require.Equal(t, []string{"A"}, stacked.ColumnNames())
	require.Equal(t, 2, stacked.NumRows())
	require.Equal(t, "a1", stacked.Cell(0, 0))
	require.Equal(t, "a2", stacked.Cell(1, 0))
	require.Nil(t, stacked.Cell(1, 1), "a column the first view does not have is not reachable")
}

// An empty view in the middle must not consume a row position, or every
// row after it shifts by one.
func TestExtraRowViewWithEmptyView(t *testing.T) {
	first := NewStringsView("", [][]string{{"A"}, {"a1"}})
	empty := NewStringsView("", [][]string{{"A"}})
	last := NewStringsView("", [][]string{{"A"}, {"a2"}})

	stacked := ExtraRowsView{first, empty, last}

	require.Zero(t, empty.NumRows(), "the fixture has a header only")
	require.Equal(t, 2, stacked.NumRows())
	require.Equal(t, "a1", stacked.Cell(0, 0))
	require.Equal(t, "a2", stacked.Cell(1, 0), "the empty view must not shift this row")
}

// The zero value has to behave like an empty table rather than panic.
func TestExtraRowViewEmpty(t *testing.T) {
	var empty ExtraRowsView

	require.Equal(t, "", empty.Title())
	require.Empty(t, empty.ColumnNames())
	require.Zero(t, empty.NumRows())
	require.Nil(t, empty.Cell(0, 0))
}
