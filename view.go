package retable

import (
	"reflect"
)

// View is an interface implemented by
// types with table like data
// to enable reading (viewing) the data
// in a uniform table like way.
//
// The design of this package assumes that
// the contents of a View are first read
// into memory and then wrapped as View,
// so the View methods don't need a
// context parameter and error result.
type View interface {
	// Title of the View
	Title() string
	// Columns returns the column names
	// which can be empty strings.
	Columns() []string
	// Numrows returns the number of rows
	NumRows() int
	// Cell returns the empty interface value of the cell at the given row and column.
	// If row and col are out of bounds then nil is returned.
	Cell(row, col int) any
}

// ReflectCellView expands the View interface
// with a method to return the reflect.Value
// of the cell at the given row and column.
type ReflectCellView interface {
	View

	// ReflectCell returns the reflect.Value of the cell at the given row and column.
	// If row and col are out of bounds then the zero value is returned.
	ReflectCell(row, col int) reflect.Value
}

// AsReflectCellView returns the passed view as
// a ReflectCellView if it implements the interface,
// otherwise it wraps the view with a helper type
// to create a ReflectCellView.
func AsReflectCellView(view View) ReflectCellView {
	if v, ok := view.(ReflectCellView); ok {
		return v
	}
	return wrapAsReflectCellView{view}
}

type wrapAsReflectCellView struct {
	View
}

func (w wrapAsReflectCellView) ReflectCell(row, col int) reflect.Value {
	return reflect.ValueOf(w.View.Cell(row, col))
}
