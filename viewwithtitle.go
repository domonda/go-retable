package retable

import "reflect"

func ViewWithTitle(source View, title string) View {
	return viewWithTitle{source: AsReflectCellView(source), title: title}
}

type viewWithTitle struct {
	source ReflectCellView
	title  string
}

func (v viewWithTitle) Title() string     { return v.title }
func (v viewWithTitle) Columns() []string { return v.source.Columns() }
func (v viewWithTitle) NumRows() int      { return v.source.NumRows() }

func (v viewWithTitle) Cell(row, col int) any {
	return v.source.Cell(row, col)
}

func (v viewWithTitle) ReflectCell(row, col int) reflect.Value {
	return v.source.ReflectCell(row, col).Elem()
}
