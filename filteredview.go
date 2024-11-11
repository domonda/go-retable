package retable

var _ View = new(FilteredView)

type FilteredView struct {
	Source View
	// Offset index of the first row from Source, must be positive.
	RowOffset int
	// Limits the number of rows, only used if > 0.
	RowLimit int
	// If not nil then the view has as many
	// columns as ColumnMapping has elements and
	// every element is a column index into the Source view.
	// If nil then the view has as many columns as the Source view.
	ColumnMapping []int
}

func (view *FilteredView) Title() string {
	return view.Source.Title()
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

func (view *FilteredView) NumCols() int {
	if view.ColumnMapping != nil {
		return len(view.ColumnMapping)
	}
	return len(view.Source.Columns())
}

func (view *FilteredView) NumRows() int {
	n := view.Source.NumRows() - max(view.RowOffset, 0)
	if n < 0 {
		return 0
	}
	if view.RowLimit > 0 && n > view.RowLimit {
		return view.RowLimit
	}
	return n
}

func (view *FilteredView) Cell(row, col int) any {
	numRows := view.NumRows()
	numCols := view.NumCols()
	if row < 0 || col < 0 || row >= numRows || col >= numCols {
		return nil
	}
	row += max(view.RowOffset, 0)
	if view.ColumnMapping != nil {
		col = view.ColumnMapping[col]
	}
	return view.Source.Cell(row, col)
}
