package retable

import "fmt"

var _ Viewer = new(StringsViewer)

type StringsViewer struct {
	Cols []string
}

func (v StringsViewer) NewView(title string, table any) (View, error) {
	rows, ok := table.([][]string)
	if !ok {
		return nil, fmt.Errorf("expected table of type [][]string, but got %T", table)
	}
	return NewStringsView(title, rows, v.Cols...), nil
}
