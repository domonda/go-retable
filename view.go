package retable

import (
	"fmt"
	"reflect"
	"strings"
)

type View interface {
	Caption() string
	Columns() []string
	Rows() int
	ReflectRow(index int) ([]reflect.Value, error)
}

func NewView(rows interface{}, caption ...string) (View, error) {
	return NewViewFromColumnMapper(rows, DefaultReflectColumnTitles, caption...)
}

func NewViewFromColumnMapper(rows interface{}, columnMapper ColumnMapper, caption ...string) (View, error) {
	v := reflect.ValueOf(rows)
	for v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice || v.Kind() == reflect.Array {
		return nil, fmt.Errorf("rows must be slice or array kind, got %T", rows)
	}
	columns, rowReflector := columnMapper.ColumnTitlesAndRowReflector(v.Type().Elem())
	return &rowReflectorView{
		caption:      strings.Join(caption, " "),
		columns:      columns,
		rowReflector: rowReflector,
		rows:         v,
	}, nil
}

type rowReflectorView struct {
	caption      string
	columns      []string
	rowReflector RowReflector
	rows         reflect.Value
}

func (v *rowReflectorView) Caption() string   { return v.caption }
func (v *rowReflectorView) Columns() []string { return v.columns }
func (v *rowReflectorView) Rows() int         { return v.rows.Len() }

func (v *rowReflectorView) ReflectRow(index int) ([]reflect.Value, error) {
	if index < 0 || index >= v.rows.Len() {
		return nil, fmt.Errorf("row index %d out of bounds [0..%d)", index, v.rows.Len())
	}
	return v.rowReflector.ReflectRow(v.rows.Index(index)), nil
}
