package retable

import "fmt"

var _ Viewer = new(StringsViewer)

// StringsViewer creates views for tables of type [][]string
// using Cols as optional column names.
type StringsViewer struct {
	Cols []string
}

// NewView creates a View with the passed title
// for the passed table which must be of type [][]string.
func (v StringsViewer) NewView(title string, table any) (View, error) {
	rows, ok := table.([][]string)
	if !ok {
		return nil, fmt.Errorf("expected table of type [][]string, but got %T", table)
	}
	return NewStringsView(title, rows, v.Cols...), nil
}
