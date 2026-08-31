package retable

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStructRowsViewer_NewView_MapIndices covers reordering columns away from
// struct field order. The column name and the cell under it have to move
// together: a mapping that permutes the values but not the headers produces a
// table that looks right and is silently wrong, which is what this used to do.
func TestStructRowsViewer_NewView_MapIndices(t *testing.T) {
	type ABC struct {
		A string
		B string
		C string
	}
	rows := []ABC{{A: "a", B: "b", C: "c"}}

	t.Run("field moved to a later column", func(t *testing.T) {
		// Field A moves to column 2; B and C shift down to fill 0 and 1.
		view, err := DefaultStructRowsViewer().WithMapIndex(0, 2).NewView("T", rows)
		require.NoError(t, err)

		require.Equal(t, []string{"B", "C", "A"}, view.Columns())
		require.Equal(t, "b", view.Cell(0, 0))
		require.Equal(t, "c", view.Cell(0, 1))
		require.Equal(t, "a", view.Cell(0, 2))
	})

	t.Run("full reversal", func(t *testing.T) {
		view, err := DefaultStructRowsViewer().
			WithMapIndices(map[int]int{0: 2, 1: 1, 2: 0}).
			NewView("T", rows)
		require.NoError(t, err)

		require.Equal(t, []string{"C", "B", "A"}, view.Columns())
		require.Equal(t, "c", view.Cell(0, 0))
		require.Equal(t, "b", view.Cell(0, 1))
		require.Equal(t, "a", view.Cell(0, 2))
	})

	t.Run("identity mapping matches no mapping", func(t *testing.T) {
		mapped, err := DefaultStructRowsViewer().
			WithMapIndices(map[int]int{0: 0, 1: 1, 2: 2}).
			NewView("T", rows)
		require.NoError(t, err)
		plain, err := DefaultStructRowsViewer().NewView("T", rows)
		require.NoError(t, err)

		require.Equal(t, plain.Columns(), mapped.Columns())
		for col := range plain.Columns() {
			require.Equal(t, plain.Cell(0, col), mapped.Cell(0, col))
		}
	})

	t.Run("every column name is paired with its own field", func(t *testing.T) {
		// The invariant behind all of the above: whatever the mapping, the
		// value under a header is the field that header names.
		view, err := DefaultStructRowsViewer().WithMapIndex(1, 0).NewView("T", rows)
		require.NoError(t, err)

		byName := map[string]any{}
		for col, name := range view.Columns() {
			byName[name] = view.Cell(0, col)
		}
		require.Equal(t, map[string]any{"A": "a", "B": "b", "C": "c"}, byName)
	})
}

// TestStructRowsViewer_NewView_CellsMatchColumns pins the alignment between a
// column and the field value underneath it for the unmapped case, so that the
// fix above cannot regress the default path.
func TestStructRowsViewer_NewView_CellsMatchColumns(t *testing.T) {
	type Product struct {
		SKU     string `col:"Product Code"`
		Name    string
		Ignored int `col:"-"`
	}
	rows := []Product{
		{SKU: "A1", Name: "Widget", Ignored: 1},
		{SKU: "B2", Name: "Gadget", Ignored: 2},
	}

	view, err := DefaultStructRowsViewer().NewView("Products", rows)
	require.NoError(t, err)

	require.Equal(t, []string{"Product Code", "Name"}, view.Columns())
	require.Equal(t, 2, view.NumRows())

	require.Equal(t, "A1", view.Cell(0, 0))
	require.Equal(t, "Widget", view.Cell(0, 1))
	require.Equal(t, "B2", view.Cell(1, 0))
	require.Equal(t, "Gadget", view.Cell(1, 1))

	// The ignored field must not be reachable as a third column.
	require.Nil(t, view.Cell(0, 2))
}
