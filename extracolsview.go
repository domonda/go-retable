package retable

var _ View = ExtraColsView(nil)

type ExtraColsView []View

func (e ExtraColsView) Title() string {
	if len(e) == 0 {
		return ""
	}
	return e[0].Title()
}

func (e ExtraColsView) Columns() []string {
	var columns []string
	for _, view := range e {
		columns = append(columns, view.Columns()...)
	}
	return columns
}

func (e ExtraColsView) NumRows() int {
	maxNumRows := 0
	for _, view := range e {
		maxNumRows = max(maxNumRows, view.NumRows())
	}
	return maxNumRows
}

func (e ExtraColsView) Cell(row, col int) any {
	if row < 0 || col < 0 {
		return nil
	}
	colLeft := 0
	for _, view := range e {
		numCols := len(view.Columns())
		colRight := colLeft + numCols
		if col < colRight {
			return view.Cell(row, col-colLeft)
		}
		colLeft = colRight
	}
	return nil
}
