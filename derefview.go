package retable

import "reflect"

// DerefView returns a View that dereferences
// every value returned by the source View
// by calling reflect.Value.Elem()
// wich might panic if the contained value
// does not support calling the Elem method.
func DerefView(source View) View {
	return derefView{source: source}
}

type derefView struct {
	source View
}

func (v derefView) Title() string     { return v.source.Title() }
func (v derefView) Columns() []string { return v.source.Columns() }
func (v derefView) NumRows() int      { return v.source.NumRows() }

func (v derefView) AnyValue(row, col int) any {
	return v.source.ReflectValue(row, col).Elem().Interface()
}

func (v derefView) ReflectValue(row, col int) reflect.Value {
	return v.source.ReflectValue(row, col).Elem()
}
