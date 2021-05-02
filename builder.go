package retable

import "reflect"

type Builder interface {
	Columns() Columns
	NewRow() []reflect.Value
}
