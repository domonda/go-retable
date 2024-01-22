package retable

import (
	"context"
)

func FormatTableAsStrings(ctx context.Context, table any, formatter CellFormatter, options ...Option) (rows [][]string, err error) {
	viewer, err := SelectViewer(table)
	if err != nil {
		return nil, err
	}
	view, err := viewer.NewView("", table)
	if err != nil {
		return nil, err
	}
	return FormatViewAsStrings(ctx, view, formatter, options...)
}

func FormatViewAsStrings(ctx context.Context, view View, formatter CellFormatter, options ...Option) (rows [][]string, err error) {
	formatter = TryFormattersOrSprint(formatter)
	numRows := view.NumRows()
	numCols := len(view.Columns())

	if HasOption(options, OptionAddHeaderRow) {
		// view.Columns() would already returns a string slice,
		// but use formatter for any additional formatting of strings
		headerView := NewHeaderViewFrom(view)
		rowStrings := make([]string, numCols)
		for col := 0; col < numCols; col++ {
			rowStrings[col], _, err = formatter.FormatCell(ctx, headerView, 0, col)
			if err != nil {
				return nil, err
			}
		}
		rows = append(rows, rowStrings)
	}

	for row := 0; row < numRows; row++ {
		rowStrings := make([]string, numCols)
		for col := 0; col < numCols; col++ {
			rowStrings[col], _, err = formatter.FormatCell(ctx, view, row, col)
			if err != nil {
				return nil, err
			}
		}
		rows = append(rows, rowStrings)
	}

	return rows, nil
}
