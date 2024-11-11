package retable

import (
	"reflect"
)

func ExtraColsAnyValueFuncView(left View, columns []string, anyValue func(row, col int) any) ReflectCellView {
	return &extraColsFuncView{
		left:     AsReflectCellView(left),
		columns:  columns,
		anyValue: anyValue,
		reflectValue: func(row, col int) reflect.Value {
			return reflect.ValueOf(anyValue(row, col))
		},
	}
}

func ExtraColsReflectValueFuncView(left View, columns []string, reflectValue func(row, col int) reflect.Value) ReflectCellView {
	return &extraColsFuncView{
		left:    AsReflectCellView(left),
		columns: columns,
		anyValue: func(row, col int) any {
			v := reflectValue(row, col)
			if !v.IsValid() {
				return nil
			}
			return v.Interface()
		},
		reflectValue: reflectValue,
	}
}

type extraColsFuncView struct {
	left         ReflectCellView
	columns      []string
	anyValue     func(row, col int) any
	reflectValue func(row, col int) reflect.Value
}

func (e *extraColsFuncView) Title() string {
	return e.left.Title()
}

func (e *extraColsFuncView) Columns() []string {
	return append(e.left.Columns(), e.columns...)
}

func (e *extraColsFuncView) NumRows() int {
	return e.left.NumRows()
}

func (e *extraColsFuncView) Cell(row, col int) any {
	numLeftCols := len(e.left.Columns())
	if col < numLeftCols {
		return e.left.Cell(row, col)
	}
	return e.anyValue(row, col-numLeftCols)
}

func (e *extraColsFuncView) ReflectCell(row, col int) reflect.Value {
	numLeftCols := len(e.left.Columns())
	if col < numLeftCols {
		return e.left.ReflectCell(row, col)
	}
	return e.reflectValue(row, col-numLeftCols)
}
