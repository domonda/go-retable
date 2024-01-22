package retable

import (
	"reflect"
)

var _ View = new(AnyValuesView)

// AnyValuesView is a View implementation
// that holds its rows as slices of value with any type.
type AnyValuesView struct {
	Tit  string
	Cols []string
	Rows [][]any
}

// NewAnyValuesViewFrom reads and caches all cells
// from the source View as ValuesView.
func NewAnyValuesViewFrom(source View) *AnyValuesView {
	view := &AnyValuesView{
		Tit:  source.Title(),
		Cols: source.Columns(),
		Rows: make([][]any, source.NumRows()),
	}
	for row := 0; row < source.NumRows(); row++ {
		view.Rows[row] = make([]any, len(source.Columns()))
		for col := range view.Rows[row] {
			view.Rows[row][col] = source.AnyValue(row, col)
		}
	}
	return view
}

func (view *AnyValuesView) Title() string     { return view.Tit }
func (view *AnyValuesView) Columns() []string { return view.Cols }
func (view *AnyValuesView) NumRows() int      { return len(view.Rows) }

func (view *AnyValuesView) AnyValue(row, col int) any {
	if row < 0 || col < 0 || row >= len(view.Rows) || col >= len(view.Rows[row]) {
		return nil
	}
	return view.Rows[row][col]
}

func (view *AnyValuesView) ReflectValue(row, col int) reflect.Value {
	if row < 0 || col < 0 || row >= len(view.Rows) || col >= len(view.Rows[row]) {
		return reflect.Value{}
	}
	return reflect.ValueOf(view.Rows[row][col])
}
