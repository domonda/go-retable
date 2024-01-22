package retable

import "reflect"

func ViewWithTitle(source View, title string) View {
	return viewWithTitle{source: source, title: title}
}

type viewWithTitle struct {
	source View
	title  string
}

func (v viewWithTitle) Title() string     { return v.title }
func (v viewWithTitle) Columns() []string { return v.source.Columns() }
func (v viewWithTitle) NumRows() int      { return v.source.NumRows() }

func (v viewWithTitle) AnyValue(row, col int) any {
	return v.source.ReflectValue(row, col).Elem().Interface()
}

func (v viewWithTitle) ReflectValue(row, col int) reflect.Value {
	return v.source.ReflectValue(row, col).Elem()
}
