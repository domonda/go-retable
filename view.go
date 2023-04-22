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
	Rows [][]any
}

func (view *MockView) Columns() []string { return view.Cols }
func (view *MockView) NumRows() int      { return len(view.Rows) }

func (view *MockView) ReflectRow(index int) ([]reflect.Value, error) {
	if index < 0 || index >= len(view.Rows) {
		return nil, fmt.Errorf("row index %d out of bounds [0..%d)", index, len(view.Rows))
	}
	row := make([]reflect.Value, len(view.Cols))
	for col := range row {
		row[col] = reflect.ValueOf(view.Rows[index][col])
	}
	return row, nil
}

type FilteredView struct {
	Source        View
	Offset        int   // Must be positive
	Limit         int   // Only used if > 0
	ColumnMapping []int // If not nil, then every element is a column index of the Source view
}

func (view *FilteredView) Columns() []string {
	sourceCols := view.Source.Columns()
	if view.ColumnMapping == nil {
		return sourceCols
	}
	mappedCols := make([]string, len(view.ColumnMapping))
	for i, iSource := range view.ColumnMapping {
		mappedCols[i] = sourceCols[iSource]
	}
	return mappedCols
}

func (view *FilteredView) NumRows() int {
	offset := view.Offset
	if offset < 0 {
		offset = 0
	}
	n := view.Source.NumRows() - offset
	if n < 0 {
		return 0
	}
	if view.Limit > 0 && n > view.Limit {
		return view.Limit
	}
	return n
}

func (view *FilteredView) ReflectRow(index int) ([]reflect.Value, error) {
	if index < 0 || index >= view.NumRows() {
		return nil, fmt.Errorf("row index %d out of bounds [0..%d)", index, view.NumRows())
	}
	offset := view.Offset
	if offset < 0 {
		offset = 0
	}
	sourceRow, err := view.Source.ReflectRow(index + offset)
	if err != nil {
		return nil, err
	}
	if view.ColumnMapping == nil {
		return sourceRow, nil
	}
	mappedRow := make([]reflect.Value, len(view.ColumnMapping))
	for i, iSource := range view.ColumnMapping {
		mappedRow[i] = sourceRow[iSource]
	}
	return mappedRow, nil
}
