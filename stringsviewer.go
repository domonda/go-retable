package retable

import (
	"fmt"
	"reflect"
)

var (
	_ View   = new(StringsView)
	_ Viewer = new(StringsViewer)
)

// NewStringsView returns a StringsView using either
// the optional cols arguments as column titles
// or the first row if not cols have been passed.
func NewStringsView(rows [][]string, cols ...string) *StringsView {
	if len(cols) == 0 && len(rows) > 0 {
		cols = rows[0]
		rows = rows[1:]
	}
	return &StringsView{Cols: cols, Rows: rows}
}

type StringsViewer struct {
	Cols []string
}

func (v StringsViewer) NewView(table interface{}) (View, error) {
	rows, ok := table.([][]string)
	if !ok {
		return nil, fmt.Errorf("expected table of type [][]string, but got %T", table)
	}
	return NewStringsView(rows, v.Cols...), nil
}

type StringsView struct {
	Cols []string
	Rows [][]string
}

func (view *StringsView) Columns() []string { return view.Cols }
func (view *StringsView) NumRows() int      { return len(view.Rows) }

func (view *StringsView) ReflectRow(index int) ([]reflect.Value, error) {
	if index < 0 || index >= len(view.Rows) {
		return nil, fmt.Errorf("row index %d out of bounds [0..%d)", index, len(view.Rows))
	}
	var (
		row = view.Rows[index]
		re  = make([]reflect.Value, len(view.Cols))
	)
	for i := range re {
		if i < len(row) {
			re[i] = reflect.ValueOf(row[i])
		} else {
			re[i] = reflect.ValueOf("")
		}
	}
	return re, nil
}
