package retable

import (
	"reflect"
)

type StructRowsView struct {
	title   string
	columns []string
	indices []int
	rows    reflect.Value // slice of structs

	cachedRow           int
	cachedValues        []any
	cachedReflectValues []reflect.Value
}

func NewStructRowsView(title string, columns []string, indices []int, rows reflect.Value) View {
	if rows.Kind() != reflect.Slice && rows.Kind() != reflect.Array {
		panic("rows must be a slice or array")
	}
	return &StructRowsView{
		title:     title,
		columns:   columns,
		indices:   indices,
		rows:      rows,
		cachedRow: -1,
	}
}

func (view *StructRowsView) Title() string     { return view.title }
func (view *StructRowsView) Columns() []string { return view.columns }
func (view *StructRowsView) NumRows() int      { return view.rows.Len() }

func (view *StructRowsView) AnyValue(row, col int) any {
	if row < 0 || col < 0 || row >= view.rows.Len() || col >= len(view.columns) {
		return nil
	}
	if row != view.cachedRow {
		view.cachedRow = row
		view.cachedValues = nil
		view.cachedReflectValues = nil
	}
	if view.cachedValues == nil {
		view.cachedValues = StructFieldValues(view.rows.Index(row))
	}
	return view.cachedValues[col]
}

func (view *StructRowsView) ReflectValue(row, col int) reflect.Value {
	if row < 0 || col < 0 || row >= view.rows.Len() || col >= len(view.columns) {
		return reflect.Value{}
	}
	if row != view.cachedRow {
		view.cachedRow = row
		view.cachedValues = nil
		view.cachedReflectValues = nil
	}
	if view.cachedReflectValues == nil {
		view.cachedReflectValues = StructFieldReflectValues(view.rows.Index(row))
	}
	return view.cachedReflectValues[col]
}
