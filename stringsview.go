package retable

import (
	"reflect"
)

// StringsView is a View that uses strings as values.
// Cols defines the column names and number of columns.
//
// StringsView is a sparse table int the sense that
// a row within Rows can have fewer slice elements
// than Cols in which case empty strings are used as value.
type StringsView struct {
	Tit  string
	Cols []string
	Rows [][]string
}

var _ View = new(StringsView)

// NewStringsView returns a StringsView using either
// the optional cols arguments as column names
// or the first row if no cols have been passed.
func NewStringsView(title string, rows [][]string, cols ...string) *StringsView {
	if len(cols) == 0 && len(rows) > 0 {
		cols = rows[0]
		rows = rows[1:]
	}
	return &StringsView{Tit: title, Cols: cols, Rows: rows}
}

func (view *StringsView) Title() string     { return view.Tit }
func (view *StringsView) Columns() []string { return view.Cols }
func (view *StringsView) NumRows() int      { return len(view.Rows) }

func (view *StringsView) AnyValue(row, col int) any {
	if row < 0 || col < 0 || row >= len(view.Rows) || col >= len(view.Cols) {
		return nil
	}
	if col >= len(view.Rows[row]) {
		return ""
	}
	return view.Rows[row][col]
}

func (view *StringsView) ReflectValue(row, col int) reflect.Value {
	if row < 0 || col < 0 || row >= len(view.Rows) || col >= len(view.Cols) {
		return reflect.Value{}
	}
	if col >= len(view.Rows[row]) {
		return reflect.ValueOf("")
	}
	return reflect.ValueOf(view.Rows[row][col])
}

// NewHeaderView returns a View using
// the passed cols as column names and also as first row.
func NewHeaderView(cols ...string) *HeaderView {
	return &HeaderView{Cols: cols}
}

// NewHeaderViewFrom returns a View using
// the column names from the source View
// also as first row.
func NewHeaderViewFrom(source View) *HeaderView {
	return &HeaderView{Tit: source.Title(), Cols: source.Columns()}
}

// HeaderView is a View that uses
// the Cols field as column names and also as first row.
type HeaderView struct {
	Tit  string
	Cols []string
}

func (view *HeaderView) Title() string     { return view.Tit }
func (view *HeaderView) Columns() []string { return view.Cols }
func (view *HeaderView) NumRows() int      { return 1 }

func (view *HeaderView) AnyValue(row, col int) any {
	if row != 0 || col < 0 || col >= len(view.Cols) {
		return nil
	}
	return view.Cols[col]
}

func (view *HeaderView) ReflectValue(row, col int) reflect.Value {
	if row != 0 || col < 0 || col >= len(view.Cols) {
		return reflect.Value{}
	}
	return reflect.ValueOf(view.Cols[col])
}
