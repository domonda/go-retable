package retable

import (
	"fmt"
	"reflect"
)

var (
	_ View = new(CachedView)
	_ View = new(MockView)
)

// View is an interface implemented by
// types with table like data
// to enable reading (viewing) the data.
type View interface {
	// Columns returns the column titles
	Columns() []string
	// Numrows returns the number of rows
	NumRows() int
	// ReflectRow returns the reflected column values of a row
	ReflectRow(index int) ([]reflect.Value, error)
}

type CachedView struct {
	Cols []string
	Rows [][]reflect.Value
}

func NewCachedViewFrom(view View) (*CachedView, error) {
	cached := &CachedView{
		Cols: view.Columns(),
		Rows: make([][]reflect.Value, view.NumRows()),
	}
	for i := range cached.Rows {
		row, err := view.ReflectRow(i)
		if err != nil {
			return nil, err
		}
		cached.Rows[i] = row
	}
	return cached, nil
}

func (view *CachedView) Columns() []string { return view.Cols }
func (view *CachedView) NumRows() int      { return len(view.Rows) }

func (view *CachedView) ReflectRow(index int) ([]reflect.Value, error) {
	if index < 0 || index >= len(view.Rows) {
		return nil, fmt.Errorf("row index %d out of bounds [0..%d)", index, len(view.Rows))
	}
	return view.Rows[index], nil
}

type MockView struct {
	Cols []string
	Rows [][]interface{}
}

func (view *MockView) Columns() []string { return view.Cols }
func (view *MockView) NumRows() int      { return len(view.Rows) }

func (view *MockView) ReflectRow(index int) ([]reflect.Value, error) {
	if index < 0 || index >= len(view.Rows) {
		return nil, fmt.Errorf("row index %d out of bounds [0..%d)", index, len(view.Rows))
	}
	rowValues := make([]reflect.Value, len(view.Cols))
	for col := range rowValues {
		rowValues[col] = reflect.ValueOf(view.Rows[index][col])
	}
	return rowValues, nil
}
