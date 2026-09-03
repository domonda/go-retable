package retable

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStructRowsViewer_NewView_ColumnNames covers how a StructRowsViewer turns
// the fields of a struct type into the columns of a View. That mapping is the
// contract every struct-backed table depends on: it decides which fields are
// exported to the table at all, and under which header they appear. Getting it
// wrong silently produces a table with missing or mislabelled columns rather
// than an error.
func TestStructRowsViewer_NewView_ColumnNames(t *testing.T) {
	type Tagged struct {
		SKU        string `col:"Product Code"`
		Name       string
		Ignored    int `col:"-"`
		unexported int //nolint:unused // present to assert it is not a column
	}
	type Embedded struct {
		Tagged
		Extra string
	}

	tests := []struct {
		name   string
		viewer *StructRowsViewer
		table  any
		want   []string
	}{
		{
			// A struct with no fields is a table with no columns, not an error.
			name:   "struct without fields has no columns",
			viewer: DefaultStructRowsViewer(),
			table:  []struct{}{},
			want:   []string{},
		},
		{
			// Untagged field names are split into words, so Go identifiers read
			// as human headers. This is the default because most structs are
			// untagged.
			name:   "untagged field name is space separated",
			viewer: DefaultStructRowsViewer(),
			table:  []struct{ OneTitle int }{{}},
			want:   []string{"One Title"},
		},
		{
			// The col tag wins over the derived name, "-" drops the field, and
			// unexported fields are never columns.
			name:   "col tag overrides, dash ignores, unexported excluded",
			viewer: DefaultStructRowsViewer(),
			table:  []Tagged{{}},
			want:   []string{"Product Code", "Name"},
		},
		{
			// Without a tag name configured, no tag is read at all. That also
			// means col:"-" is not seen, so Ignored becomes a column here. This
			// is the documented difference from DefaultStructRowsViewer.
			name:   "no tags viewer uses raw field names and reads no tags",
			viewer: NoTagsStructRowsViewer(),
			table:  []Tagged{{}},
			want:   []string{"SKU", "Name", "Ignored"},
		},
		{
			// Embedded struct fields are flattened into the outer table rather
			// than becoming a single nested column.
			name:   "embedded struct fields are inlined",
			viewer: DefaultStructRowsViewer(),
			table:  []Embedded{{}},
			want:   []string{"Product Code", "Name", "Extra"},
		},
		{
			// A slice of pointers describes the same table as a slice of values.
			name:   "pointer element type behaves like value type",
			viewer: DefaultStructRowsViewer(),
			table:  []*Tagged{{}},
			want:   []string{"Product Code", "Name"},
		},
		{
			// Opting into tags-only makes column membership explicit, so adding
			// a field to the struct cannot silently widen the table.
			name:   "ignore untagged includes only tagged fields",
			viewer: &StructRowsViewer{StructFieldNaming: DefaultStructFieldNamingIgnoreUntagged},
			table:  []Tagged{{}},
			want:   []string{"Product Code"},
		},
		{
			// An empty slice still describes its columns, because they come from
			// the element type and not from the data.
			name:   "columns come from the type not the data",
			viewer: DefaultStructRowsViewer(),
			table:  []Tagged{},
			want:   []string{"Product Code", "Name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view, err := tt.viewer.NewView("Title", tt.table)
			require.NoError(t, err)
			require.Equal(t, tt.want, view.ColumnNames())
			require.Equal(t, len(tt.want), view.NumColumns(), "NumColumns must agree with ColumnNames")
			require.Equal(t, "Title", view.Title())
		})
	}
}

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

		require.Equal(t, []string{"B", "C", "A"}, view.ColumnNames())
		require.Equal(t, "b", view.Cell(0, 0))
		require.Equal(t, "c", view.Cell(0, 1))
		require.Equal(t, "a", view.Cell(0, 2))
	})

	t.Run("full reversal", func(t *testing.T) {
		view, err := DefaultStructRowsViewer().
			WithMapIndices(map[int]int{0: 2, 1: 1, 2: 0}).
			NewView("T", rows)
		require.NoError(t, err)

		require.Equal(t, []string{"C", "B", "A"}, view.ColumnNames())
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

		require.Equal(t, plain.ColumnNames(), mapped.ColumnNames())
		for col := range plain.ColumnNames() {
			require.Equal(t, plain.Cell(0, col), mapped.Cell(0, col))
		}
	})

	t.Run("every column name is paired with its own field", func(t *testing.T) {
		// The invariant behind all of the above: whatever the mapping, the
		// value under a header is the field that header names.
		view, err := DefaultStructRowsViewer().WithMapIndex(1, 0).NewView("T", rows)
		require.NoError(t, err)

		byName := map[string]any{}
		for col, name := range view.ColumnNames() {
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

	require.Equal(t, []string{"Product Code", "Name"}, view.ColumnNames())
	require.Equal(t, 2, view.NumRows())

	require.Equal(t, "A1", view.Cell(0, 0))
	require.Equal(t, "Widget", view.Cell(0, 1))
	require.Equal(t, "B2", view.Cell(1, 0))
	require.Equal(t, "Gadget", view.Cell(1, 1))

	// The ignored field must not be reachable as a third column.
	require.Nil(t, view.Cell(0, 2))
}

// TestStructRowsViewer_NewView_Errors covers the inputs that cannot describe a
// table. These return an error rather than panicking, because the table value
// usually comes from a caller's data rather than from a literal.
func TestStructRowsViewer_NewView_Errors(t *testing.T) {
	tests := []struct {
		name  string
		table any
	}{
		{name: "not a slice or array", table: 42},
		{name: "slice of non struct", table: []int{1, 2, 3}},
		{name: "slice of strings", table: []string{"a"}},
		{name: "nil", table: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view, err := DefaultStructRowsViewer().NewView("Title", tt.table)
			require.Error(t, err)
			require.Nil(t, view)
		})
	}
}
