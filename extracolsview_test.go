package retable

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ExtraColsView joins views side by side, which is how a computed
// column is added next to a base table without copying it. The column
// index therefore has to be translated to the view that owns it, and
// getting that translation wrong silently shows one column's values
// under another column's heading.
func TestExtraColsView(t *testing.T) {
	base := NewStringsView("People", [][]string{
		{"Name", "Age"},
		{"Erik", "42"},
		{"Ann", "7"},
	})
	computed := NewStringsView("ignored title", [][]string{
		{"Adult", "Initial"},
		{"yes", "E"},
		{"no", "A"},
	})
	combined := ExtraColsView{base, computed}

	t.Run("the title comes from the first view only", func(t *testing.T) {
		require.Equal(t, "People", combined.Title(),
			"the title is not combined, wrap in ViewWithTitle for a custom one")
	})

	t.Run("columns are concatenated in view order", func(t *testing.T) {
		require.Equal(t, []string{"Name", "Age", "Adult", "Initial"}, combined.ColumnNames())
	})

	t.Run("every cell comes from the view owning its column", func(t *testing.T) {
		want := [][]any{
			{"Erik", "42", "yes", "E"},
			{"Ann", "7", "no", "A"},
		}
		require.Equal(t, len(want), combined.NumRows())
		for row := range want {
			for col := range want[row] {
				require.Equalf(t, want[row][col], combined.Cell(row, col), "Cell(%d, %d)", row, col)
			}
		}
	})

	t.Run("out of range positions are nil, not a panic", func(t *testing.T) {
		require.Nil(t, combined.Cell(0, 4), "one past the last column")
		require.Nil(t, combined.Cell(0, -1))
		require.Nil(t, combined.Cell(-1, 0))
	})
}

// A shorter view does not truncate the combined view: its rows are
// padded, so a computed column that only covers the first rows still
// lines up with the base table instead of cutting it short.
func TestExtraColsViewDifferentRowCounts(t *testing.T) {
	long := NewStringsView("", [][]string{
		{"Name"},
		{"Erik"},
		{"Ann"},
		{"Bo"},
	})
	short := NewStringsView("", [][]string{
		{"Note"},
		{"first only"},
	})
	combined := ExtraColsView{long, short}

	require.Equal(t, 3, combined.NumRows(), "the longest view decides the row count")
	require.Equal(t, "Bo", combined.Cell(2, 0))
	require.Equal(t, "first only", combined.Cell(0, 1))
	require.Nil(t, combined.Cell(2, 1), "rows past a shorter view read as nil")
}

// The zero value has to behave like an empty table rather than panic,
// because it is what a caller gets from a slice that was never filled.
func TestExtraColsViewEmpty(t *testing.T) {
	var empty ExtraColsView

	require.Equal(t, "", empty.Title())
	require.Empty(t, empty.ColumnNames())
	require.Zero(t, empty.NumRows())
	require.Nil(t, empty.Cell(0, 0))
}

// Nesting is the documented way to add more than one computed column,
// so the index translation has to survive a view that is itself an
// ExtraColsView.
func TestExtraColsViewNested(t *testing.T) {
	a := NewStringsView("A", [][]string{{"A"}, {"a1"}})
	b := NewStringsView("B", [][]string{{"B"}, {"b1"}})
	c := NewStringsView("C", [][]string{{"C"}, {"c1"}})

	nested := ExtraColsView{ExtraColsView{a, b}, c}

	require.Equal(t, "A", nested.Title(), "the title still comes from the innermost first view")
	require.Equal(t, []string{"A", "B", "C"}, nested.ColumnNames())
	require.Equal(t, "a1", nested.Cell(0, 0))
	require.Equal(t, "b1", nested.Cell(0, 1))
	require.Equal(t, "c1", nested.Cell(0, 2))
	require.Nil(t, nested.Cell(0, 3))
}
