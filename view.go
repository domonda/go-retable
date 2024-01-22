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
	// AnyValue returns the empty interface value of the cell at the given row and column.
	// If row and col are out of bounds then nil is returned.
	AnyValue(row, col int) any
	// Value returns the reflect.Value of the cell at the given row and column.
	// If row and col are out of bounds then the zero value is returned.
	ReflectValue(row, col int) reflect.Value
}
