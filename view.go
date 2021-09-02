package retable

import (
	"reflect"
)

type Viewer interface {
	NewView(table interface{}) (View, error)
}

// View is an interface implemented by
// types with table like data
// to enable reading (viewing) the data.
type View interface {
	Columns() []string
	NumRows() int
	ReflectRow(index int) ([]reflect.Value, error)
}

type ViewCell struct {
	View
	Row int
	Col int
}
