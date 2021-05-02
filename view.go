package retable

import (
	"fmt"
	"reflect"
)

type View interface {
	Title() string
	Columns() Columns
	NumRows() int
	ReflectRow(index int) ([]reflect.Value, error)
}

func NewView(rows interface{}) (View, error) {
	return NewViewWithTitle(rows, "")
}

func NewViewWithTitle(rows interface{}, title string) (View, error) {
	panic("TODO")
}

type sliceView struct {
	title string
	cols  Columns
	rows  reflect.Value
}

func (r *sliceView) Title() string    { return r.title }
func (r *sliceView) Columns() Columns { return r.cols }
func (r *sliceView) NumRows() int     { return r.rows.Len() }

func (r *sliceView) ReflectRow(index int) ([]reflect.Value, error) {
	if index < 0 || index >= r.NumRows() {
		return nil, fmt.Errorf("row index %d out of bounds [0..%d)", index, r.NumRows())
	}

	panic("TODO")
}
