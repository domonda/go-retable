package retable

import (
	"context"
)

// FormatTableAsStrings converts any table data into a 2D string slice.
//
// This function automatically selects an appropriate Viewer for the table type,
// creates a View, and then formats it as strings using the provided formatter and options.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control during formatting
//   - table: The table data to format (e.g., [][]string, []Person, etc.)
//   - formatter: Optional CellFormatter to customize cell rendering (can be nil for default formatting)
//   - options: Optional formatting options (e.g., OptionAddHeaderRow)
//
// Returns a 2D string slice where each inner slice represents a row,
// and an error if viewer selection, view creation, or formatting fails.
//
// Example:
//
//	type Person struct {
//	    Name string
//	    Age  int
//	}
//	people := []Person{{Name: "John", Age: 30}, {Name: "Alice", Age: 25}}
//	rows, err := FormatTableAsStrings(ctx, people, nil, OptionAddHeaderRow)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// rows[0] = ["Name", "Age"]       // header row
//	// rows[1] = ["John", "30"]        // data row
//	// rows[2] = ["Alice", "25"]       // data row
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

// FormatViewAsStrings converts a View into a 2D string slice.
//
// This function iterates through all rows and columns in the view,
// formatting each cell using the provided CellFormatter. If no formatter is provided,
// TryFormattersOrSprint is used as the default.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control during formatting
//   - view: The View to format, providing access to rows, columns, and cell data
//   - formatter: Optional CellFormatter to customize cell rendering (can be nil for default formatting)
//   - options: Optional formatting options (e.g., OptionAddHeaderRow to include column titles)
//
// When OptionAddHeaderRow is set, the column titles from view.Columns() are added
// as the first row, also passed through the formatter for consistent formatting.
//
// Returns a 2D string slice where each inner slice represents a row,
// and an error if any cell formatting fails.
//
// Example:
//
//	rows, err := FormatViewAsStrings(ctx, myView, nil, OptionAddHeaderRow)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Process the formatted rows
//	for _, row := range rows {
//	    fmt.Println(strings.Join(row, " | "))
//	}
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
