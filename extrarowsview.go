package retable

var _ View = ExtraRowView(nil)

type ExtraRowView []View

func (e ExtraRowView) Title() string {
	if len(e) == 0 {
		return ""
	}
	return e[0].Title()
}

func (e ExtraRowView) Columns() []string {
	if len(e) == 0 {
		return nil
	}
	return e[0].Columns()
}

func (e ExtraRowView) NumRows() int {
	numRows := 0
	for _, view := range e {
		numRows += view.NumRows()
	}
	return numRows
}

func (e ExtraRowView) Cell(row, col int) any {
	if row < 0 || col < 0 || col >= len(e.Columns()) {
		return nil
	}
	rowTop := 0
	for _, view := range e {
		numRows := view.NumRows()
		rowBottom := rowTop + numRows
		if row < rowBottom {
			return view.Cell(row-rowTop, col)
		}
		rowTop = rowBottom
	}
	return nil
}
