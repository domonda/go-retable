package retable

import (
	"fmt"
	"reflect"
)

type StructRowsView struct {
	title   string
	columns []string
	indices []int         // nil for 1:1 mapping of columns to struct fields
	rows    reflect.Value // slice of structs

	cachedRow           int
	cachedValues        []any
	cachedReflectValues []reflect.Value
}

func NewStructRowsView(title string, columns []string, indices []int, rows reflect.Value) View {
	if rows.Kind() != reflect.Slice && rows.Kind() != reflect.Array {
		panic(fmt.Errorf("rows must be a slice or array, got %s", rows.Type()))
	}
	if is1on1Mapping(columns, indices) {
		indices = nil
	} else if indices != nil {
		colMapped := make([]bool, len(columns))
		for _, index := range indices {
			if index < 0 {
				continue
			}
			if index >= len(columns) {
				panic(fmt.Errorf("index %d out of range for %d columns", index, len(columns)))
			}
			if colMapped[index] {
				panic(fmt.Errorf("index %d mapped to column %q more than once", index, columns[index]))
			}
			colMapped[index] = true
		}
		for col, mapped := range colMapped {
			if !mapped {
				panic(fmt.Errorf("column %q not mapped", columns[col]))
			}
		}
	}
	return &StructRowsView{
		title:     title,
		columns:   columns,
		indices:   indices,
		rows:      rows,
		cachedRow: -1,
	}
}

func is1on1Mapping(columns []string, indices []int) bool {
	if indices == nil {
		return true
	}
	if len(columns) != len(indices) {
		return false
	}
	for i, index := range indices {
		if index != i {
			return false
		}
	}
	return true
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
		if view.indices != nil {
			view.cachedValues = IndexedStructFieldAnyValues(view.rows.Index(row), len(view.columns), view.indices)
		} else {
			view.cachedValues = StructFieldAnyValues(view.rows.Index(row))
		}
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
		if view.indices != nil {
			view.cachedReflectValues = IndexedStructFieldReflectValues(view.rows.Index(row), len(view.columns), view.indices)
		} else {
			view.cachedReflectValues = StructFieldReflectValues(view.rows.Index(row))
		}
	}
	return view.cachedReflectValues[col]
}
