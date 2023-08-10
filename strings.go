package retable

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"unicode/utf8"
)

func Strings(ctx context.Context, table any, addHeaderRow bool, formatters *TypeFormatters) (rows [][]string, err error) {
	viewer, err := SelectViewer(table)
	if err != nil {
		return nil, err
	}
	view, err := viewer.NewView(table)
	if err != nil {
		return nil, err
	}
	return ViewStrings(ctx, view, addHeaderRow, formatters)
}

func ViewStrings(ctx context.Context, view View, addHeaderRow bool, formatters *TypeFormatters) (rows [][]string, err error) {
	numCols := len(view.Columns())

	if addHeaderRow {
		// view.Columns() already returns a string slice,
		// but use rowStrings() for any potential formatting
		rowVals := make([]reflect.Value, numCols)
		for col, title := range view.Columns() {
			rowVals[col] = reflect.ValueOf(title)
		}
		rowStrs, err := rowStrings(ctx, rowVals, -1, view, formatters)
		if err != nil {
			return nil, err
		}
		rows = append(rows, rowStrs)
	}

	for row := 0; row < view.NumRows(); row++ {
		rowVals, err := view.ReflectRow(row)
		if err != nil {
			return nil, err
		}
		rowStrs, err := rowStrings(ctx, rowVals, row, view, formatters)
		if err != nil {
			return nil, err
		}
		rows = append(rows, rowStrs)
	}

	return rows, nil
}

func rowStrings(ctx context.Context, rowVals []reflect.Value, row int, view View, formatters *TypeFormatters) (rowStrs []string, err error) {
	rowStrs = make([]string, len(rowVals))

	// cell will be reused for every column of the row
	cell := Cell{
		View: view,
		Row:  row,
	}
	for col, val := range rowVals {
		cell.Col = col
		cell.Value = val

		str, err := cellString(ctx, &cell, formatters)
		if err != nil {
			return nil, err
		}

		rowStrs[col] = str
	}

	return rowStrs, nil
}

func cellString(ctx context.Context, cell *Cell, formatters *TypeFormatters) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	str, _, err := formatters.FormatCell(ctx, cell)
	if err == nil {
		return str, nil
	}
	if !errors.Is(err, errors.ErrUnsupported) {
		return "", err
	}

	// In case of errors.ErrUnsupported from w.formatters
	// use fallback methods for formatting
	if ValueIsNil(cell.Value) {
		return "", nil
	}
	v := cell.Value
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return fmt.Sprint(v.Interface()), nil
}

// StringColumnWidths returns the column widths of the passed
// table as count of UTF-8 runes.
func StringColumnWidths(rows [][]string, numCols int) []int {
	if numCols < 0 {
		for _, row := range rows {
			if rowCols := len(row); rowCols > numCols {
				numCols = rowCols
			}
		}
		if numCols <= 0 {
			return nil
		}
	}
	colWidths := make([]int, numCols)
	for row := range rows {
		for col := 0; col < numCols; col++ {
			numRunes := utf8.RuneCountInString(rows[row][col])
			if numRunes > colWidths[col] {
				colWidths[col] = numRunes
			}
		}
	}
	return colWidths
}
