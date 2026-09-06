package retable

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// DerefView dereferences every cell, which is what turns a view over
// pointer values into one a formatter can dispatch on by concrete type.
func TestDerefView(t *testing.T) {
	a, b := "Erik", "Ann"
	source := &AnyValuesView{
		TableTitle: "People",
		Cols:       []string{"Name"},
		Rows:       [][]any{{&a}, {&b}},
	}
	view := DerefView(source)

	require.Equal(t, "People", view.Title(), "the decorator passes the title through")
	require.Equal(t, []string{"Name"}, view.ColumnNames())
	require.Equal(t, 2, view.NumRows())

	require.Equal(t, "Erik", view.Cell(0, 0), "the pointed-to value, not the pointer")
	require.Equal(t, "Ann", view.Cell(1, 0))

	require.Equal(t, reflect.TypeFor[string](), view.ReflectCell(0, 0).Type(),
		"the dereferenced type is what a formatter dispatches on")
}

// An interface cell is dereferenced to the value inside it, which is
// the second indirection reflect.Value.Elem removes.
func TestDerefViewOfInterfaceCells(t *testing.T) {
	source := &ReflectValuesView{
		Cols: []string{"V"},
		Rows: [][]reflect.Value{
			{reflect.ValueOf(&[]any{"inside"}[0]).Elem()},
		},
	}
	require.Equal(t, reflect.Interface, source.ReflectCell(0, 0).Kind(),
		"the fixture needs an interface cell")

	view := DerefView(source)
	require.Equal(t, "inside", view.Cell(0, 0))
	require.Equal(t, reflect.TypeFor[string](), view.ReflectCell(0, 0).Type())
}

// The panics are the documented contract of this decorator, not an
// oversight: its doc lists them under "# Panics" and tells the caller
// to use it only for views whose cells really are indirect. Pinning
// them so that turning any of them into a nil cell is a deliberate
// change with a changelog entry, not a silent one.
func TestDerefViewDocumentedPanics(t *testing.T) {
	t.Run("a non pointer cell cannot be dereferenced", func(t *testing.T) {
		view := DerefView(NewStringsView("", [][]string{{"A"}, {"plain"}}))
		require.Panics(t, func() { view.Cell(0, 0) })
	})

	t.Run("a nil pointer has nothing to read", func(t *testing.T) {
		var nilPtr *string
		source := &AnyValuesView{Cols: []string{"A"}, Rows: [][]any{{nilPtr}}}
		view := DerefView(source)

		// Elem of a nil pointer is the zero Value, so the reflect view
		// reports no value rather than panicking
		require.False(t, view.ReflectCell(0, 0).IsValid())
		// but reading it as any has no value to unwrap
		require.Panics(t, func() { view.Cell(0, 0) })
	})
}

// A position outside the source is not one of the documented panics: it
// has no cell to dereference, and View.Cell promises nil for it, which a
// writer iterating a wrapped view relies on the same way it does for the
// undecorated one.
func TestDerefViewOutOfRange(t *testing.T) {
	view := DerefView(NewStringsView("", [][]string{{"A"}, {"plain"}}))

	for _, pos := range [][2]int{{-1, 0}, {0, -1}, {5, 0}, {0, 5}} {
		row, col := pos[0], pos[1]
		require.Nilf(t, view.Cell(row, col), "Cell(%d, %d)", row, col)
		require.Falsef(t, view.ReflectCell(row, col).IsValid(), "ReflectCell(%d, %d)", row, col)
	}
}

// A cell holding a nil interface has no value either, so it is reported
// like an out of range position rather than dereferenced. This is the
// same case as out of range for the decorator, because both reach it as
// an invalid reflect.Value from the source.
func TestDerefViewOfNilInterfaceCell(t *testing.T) {
	source := &AnyValuesView{Cols: []string{"A"}, Rows: [][]any{{nil}}}
	require.False(t, AsReflectCellView(source).ReflectCell(0, 0).IsValid(),
		"the fixture needs a cell without a value")

	view := DerefView(source)
	require.Nil(t, view.Cell(0, 0))
	require.False(t, view.ReflectCell(0, 0).IsValid())
}
