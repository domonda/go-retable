package retable

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func filteredSource() View {
	return NewStringsView("Source", [][]string{
		{"A", "B", "C"},
		{"a0", "b0", "c0"},
		{"a1", "b1", "c1"},
		{"a2", "b2", "c2"},
		{"a3", "b3", "c3"},
	})
}

// RowOffset and RowLimit are what paginate a table, so an off-by-one
// here silently drops or repeats a row at every page boundary.
func TestFilteredViewRows(t *testing.T) {
	tests := []struct {
		name      string
		offset    int
		limit     int
		wantRows  int
		wantFirst any
		wantLast  any
	}{
		{name: "no offset and no limit is the whole source", wantRows: 4, wantFirst: "a0", wantLast: "a3"},
		{name: "offset skips from the front", offset: 2, wantRows: 2, wantFirst: "a2", wantLast: "a3"},
		{name: "limit caps the count", limit: 2, wantRows: 2, wantFirst: "a0", wantLast: "a1"},
		{name: "offset and limit together are a page", offset: 1, limit: 2, wantRows: 2, wantFirst: "a1", wantLast: "a2"},
		{name: "a limit larger than the source is not padding", limit: 99, wantRows: 4, wantFirst: "a0", wantLast: "a3"},
		// A negative offset is documented as 0 rather than an error,
		// so a computed offset that underflows shows the first page
		// instead of panicking or reading before the start.
		{name: "a negative offset is treated as zero", offset: -5, wantRows: 4, wantFirst: "a0", wantLast: "a3"},
		{name: "a negative limit means no limit", limit: -1, wantRows: 4, wantFirst: "a0", wantLast: "a3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := &FilteredView{Source: filteredSource(), RowOffset: tt.offset, RowLimit: tt.limit}

			require.Equal(t, tt.wantRows, view.NumRows())
			require.Equal(t, tt.wantFirst, view.Cell(0, 0))
			require.Equal(t, tt.wantLast, view.Cell(view.NumRows()-1, 0))
			require.Nil(t, view.Cell(view.NumRows(), 0), "one past the last row")
		})
	}
}

// An offset past the end is an empty page, not a negative row count,
// which is what a caller gets by paging one step too far.
func TestFilteredViewOffsetPastEnd(t *testing.T) {
	for _, offset := range []int{4, 5, 100} {
		view := &FilteredView{Source: filteredSource(), RowOffset: offset}

		require.Zerof(t, view.NumRows(), "offset %d", offset)
		require.Nil(t, view.Cell(0, 0))
	}
}

// ColumnMapping is what selects and reorders columns. Its indices point
// into the source, so a wrong translation shows the right heading over
// the wrong column's values.
func TestFilteredViewColumnMapping(t *testing.T) {
	t.Run("nil mapping passes every column through unchanged", func(t *testing.T) {
		view := &FilteredView{Source: filteredSource()}

		require.Equal(t, []string{"A", "B", "C"}, view.ColNames())
		require.Equal(t, 3, view.NumCols())
		require.Equal(t, "b0", view.Cell(0, 1))
	})

	t.Run("a mapping selects a subset", func(t *testing.T) {
		view := &FilteredView{Source: filteredSource(), ColumnMapping: []int{1}}

		require.Equal(t, []string{"B"}, view.ColNames())
		require.Equal(t, 1, view.NumCols())
		require.Equal(t, "b0", view.Cell(0, 0), "the heading and the value must come from the same source column")
		require.Nil(t, view.Cell(0, 1), "the unmapped columns are not reachable")
	})

	t.Run("a mapping reorders", func(t *testing.T) {
		view := &FilteredView{Source: filteredSource(), ColumnMapping: []int{2, 0}}

		require.Equal(t, []string{"C", "A"}, view.ColNames())
		require.Equal(t, "c0", view.Cell(0, 0))
		require.Equal(t, "a0", view.Cell(0, 1))
	})

	t.Run("a mapping may repeat a column", func(t *testing.T) {
		view := &FilteredView{Source: filteredSource(), ColumnMapping: []int{1, 1}}

		require.Equal(t, []string{"B", "B"}, view.ColNames())
		require.Equal(t, "b0", view.Cell(0, 0))
		require.Equal(t, "b0", view.Cell(0, 1))
	})

	// An empty non-nil mapping is not the same as nil: it selects no
	// column at all, where nil selects every column.
	t.Run("an empty mapping selects nothing", func(t *testing.T) {
		view := &FilteredView{Source: filteredSource(), ColumnMapping: []int{}}

		require.Empty(t, view.ColNames())
		require.Zero(t, view.NumCols())
		require.Nil(t, view.Cell(0, 0))
	})
}

// The row window and the column mapping have to compose, because
// paging a projected table applies both at once.
func TestFilteredViewRowsAndColumnsTogether(t *testing.T) {
	view := &FilteredView{
		Source:        filteredSource(),
		RowOffset:     1,
		RowLimit:      2,
		ColumnMapping: []int{2, 0},
	}

	require.Equal(t, []string{"C", "A"}, view.ColNames())
	require.Equal(t, 2, view.NumRows())
	require.Equal(t, "Source", view.Title(), "filtering does not change the title")

	require.Equal(t, "c1", view.Cell(0, 0))
	require.Equal(t, "a1", view.Cell(0, 1))
	require.Equal(t, "c2", view.Cell(1, 0))
	require.Equal(t, "a2", view.Cell(1, 1))
}

func TestFilteredViewOutOfRange(t *testing.T) {
	view := &FilteredView{Source: filteredSource(), RowOffset: 1, RowLimit: 2}

	require.Nil(t, view.Cell(-1, 0))
	require.Nil(t, view.Cell(0, -1))
	require.Nil(t, view.Cell(2, 0), "past the limit, even though the source has the row")
	require.Nil(t, view.Cell(0, 3), "past the last column")
}
