package retable

import "reflect"

type Columns interface {
	NumCols() int
	Titles() []string
	ReflectRow(row reflect.Value) (cols []reflect.Value)
}
