package retable

import (
	"reflect"
)

var _ View = ExtraRowView(nil)

type ExtraRowView []View

func (e ExtraRowView) Title() string {
	if len(e) == 0 {
		return ""
	}
	return e[0].Title()
}

func (e ExtraRowView) Columns() []string {
	if len(e) == 0 {
		return nil
	}
	return e[0].Columns()
}

func (e ExtraRowView) NumRows() int {
	numRows := 0
	for _, view := range e {
		numRows += view.NumRows()
	}
	return numRows
}

func (e ExtraRowView) AnyValue(row, col int) any {
	if row < 0 || col < 0 || col >= len(e.Columns()) {
		return nil
	}
	rowTop := 0
	for _, view := range e {
		numRows := view.NumRows()
		rowBottom := rowTop + numRows
		if row < rowBottom {
			return view.AnyValue(row-rowTop, col)
		}
		rowTop = rowBottom
	}
	return nil
}

func (e ExtraRowView) ReflectValue(row, col int) reflect.Value {
	if row < 0 || col < 0 || col >= len(e.Columns()) {
		return reflect.Value{}
	}
	rowTop := 0
	for _, view := range e {
		numRows := view.NumRows()
		rowBottom := rowTop + numRows
		if row < rowBottom {
			return view.ReflectValue(row-rowTop, col)
		}
		rowTop = rowBottom
	}
	return reflect.Value{}
}
