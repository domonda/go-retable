package retable

import (
	"fmt"
	"reflect"
)

// NewViewWithExtraColumns returns a new View that expands the base View
// with additional columns titled by extraCols and with row values
// returned by reflectRowExtraCols.
// The number of values returnd by reflectRowExtraCols must be equal to len(extraCols).
func NewViewWithExtraColumns(base View, extraCols []string, reflectRowExtraCols func(index int) ([]reflect.Value, error)) View {
	return &extraColsView{
		base:            base,
		extraCols:       extraCols,
		reflectRowExtra: reflectRowExtraCols,
	}
}

type extraColsView struct {
	base            View
	extraCols       []string
	reflectRowExtra func(index int) ([]reflect.Value, error)
}

func (e *extraColsView) Columns() []string {
	return append(e.base.Columns(), e.extraCols...)
}

func (e *extraColsView) NumRows() int {
	return e.base.NumRows()
}

func (e *extraColsView) ReflectRow(index int) ([]reflect.Value, error) {
	rowVals, err := e.base.ReflectRow(index)
	if err != nil {
		return nil, err
	}
	extraVals, err := e.reflectRowExtra(index)
	if err != nil {
		return nil, err
	}
	if len(extraVals) != len(e.extraCols) {
		return nil, fmt.Errorf("reflected %d extra values for row %d, but expected %d", len(extraVals), index, len(e.extraCols))
	}
	return append(rowVals, extraVals...), nil
}
