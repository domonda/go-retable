package retable

import "reflect"

// DerefView returns a View that dereferences
// every value returned by the source View
// by calling reflect.Value.Elem()
// wich might panic if the contained value
// does not support calling the Elem method.
func DerefView(source View) ReflectCellView {
	return derefView{source: AsReflectCellView(source)}
}

type derefView struct {
	source ReflectCellView
}

func (v derefView) Title() string     { return v.source.Title() }
func (v derefView) Columns() []string { return v.source.Columns() }
func (v derefView) NumRows() int      { return v.source.NumRows() }

func (v derefView) Cell(row, col int) any {
	return v.source.ReflectCell(row, col).Elem().Interface()
}

func (v derefView) ReflectCell(row, col int) reflect.Value {
	return v.source.ReflectCell(row, col).Elem()
}
