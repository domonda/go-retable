package retable

import (
	"fmt"
	"reflect"
)

type structRowsView struct {
	columns []string
	indices []int
	rows    reflect.Value
}

func (view *structRowsView) Columns() []string { return view.columns }
func (view *structRowsView) NumRows() int      { return view.rows.Len() }

func (view *structRowsView) ReflectRow(index int) ([]reflect.Value, error) {
	if index < 0 || index >= view.rows.Len() {
		return nil, fmt.Errorf("row index %d out of bounds [0..%d)", index, view.rows.Len())
	}
	columnValues := make([]reflect.Value, len(view.columns))
	structFields := StructFieldValues(view.rows.Index(index))
	for i, index := range view.indices {
		if index >= 0 && index < len(view.columns) {
			columnValues[index] = structFields[i]
		}
	}
	return columnValues, nil
}
