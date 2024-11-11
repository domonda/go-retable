package retable

import (
	"errors"
	"reflect"
)

var _ ReflectCellView = new(ReflectValuesView)

// ReflectValuesView is a View implementation
// that holds its rows as slices of reflect.Value.
type ReflectValuesView struct {
	Tit  string
	Cols []string
	Rows [][]reflect.Value
}

// NewReflectValuesViewFrom reads and caches all cells
// as reflect.Value from the source View as ReflectValuesView.
func NewReflectValuesViewFrom(source View) (*ReflectValuesView, error) {
	if source == nil {
		return nil, errors.New("view is nil")
	}
	view := &ReflectValuesView{
		Tit:  source.Title(),
		Cols: source.Columns(),
		Rows: make([][]reflect.Value, source.NumRows()),
	}
	reflectSource := AsReflectCellView(source)
	for row := 0; row < source.NumRows(); row++ {
		view.Rows[row] = make([]reflect.Value, len(source.Columns()))
		for col := range view.Rows[row] {
			view.Rows[row][col] = reflectSource.ReflectCell(row, col)
		}
	}
	return view, nil
}

func (view *ReflectValuesView) Title() string     { return view.Tit }
func (view *ReflectValuesView) Columns() []string { return view.Cols }
func (view *ReflectValuesView) NumRows() int      { return len(view.Rows) }

func (view *ReflectValuesView) Cell(row, col int) any {
	if row < 0 || col < 0 || row >= len(view.Rows) || col >= len(view.Rows[row]) {
		return nil
	}
	return view.Rows[row][col].Interface()
}

func (view *ReflectValuesView) ReflectCell(row, col int) reflect.Value {
	if row < 0 || col < 0 || row >= len(view.Rows) || col >= len(view.Rows[row]) {
		return reflect.Value{}
	}
	return view.Rows[row][col]
}

var _ ReflectCellView = new(ReflectValuesView)

// SingleReflectValueView is a View implementation
// that holds its rows as slices of reflect.Value.
type SingleReflectValueView struct {
	Tit string
	Col string
	Val reflect.Value
}

// NewSingleReflectValueView reads the cell at row/col
// from the source View and wraps it as SingleReflectValueView.
func NewSingleReflectValueView(source View, row, col int) *SingleReflectValueView {
	if source == nil || row < 0 || col < 0 || row >= source.NumRows() || col >= len(source.Columns()) {
		return &SingleReflectValueView{Tit: source.Title()}
	}
	return &SingleReflectValueView{
		Tit: source.Title(),
		Col: source.Columns()[col],
		Val: reflect.ValueOf(source.Cell(row, col)),
	}
}

func (view *SingleReflectValueView) Title() string     { return view.Tit }
func (view *SingleReflectValueView) Columns() []string { return []string{view.Col} }
func (view *SingleReflectValueView) NumRows() int      { return 1 }

func (view *SingleReflectValueView) Cell(row, col int) any {
	if row != 0 || col != 0 {
		return nil
	}
	return view.Val.Interface()
}

func (view *SingleReflectValueView) ReflectCell(row, col int) reflect.Value {
	if row != 0 || col != 0 {
		return reflect.Value{}
	}
	return view.Val
}
