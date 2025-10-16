// Package exceltable provides functionality for reading Excel files (.xlsx, .xlsm, .xltm, .xltx)
// and converting them into retable.View interfaces for tabular data manipulation.
//
// The package uses the excelize library (github.com/xuri/excelize/v2) under the hood
// to parse Excel files and extract sheet data as string-based tables.
//
// Key features:
//   - Read single or multiple sheets from Excel files
//   - Support for both file paths and io.Reader sources
//   - Automatic cleaning of empty rows and columns
//   - Configurable raw cell value extraction
//   - Sheet name preservation as table titles
//
// Example usage:
//
//	// Read all sheets from a file
//	views, err := exceltable.ReadLocalFile("data.xlsx", false)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, view := range views {
//	    fmt.Printf("Sheet: %s, Rows: %d\n", view.Title(), view.NumRows())
//	}
//
//	// Read first sheet from an io.Reader
//	file, _ := os.Open("data.xlsx")
//	defer file.Close()
//	view, err := exceltable.ReadFirstSheet(file, false)
//	if err != nil {
//	    log.Fatal(err)
//	}
package exceltable

import (
	"errors"
	"io"
	"reflect"

	"github.com/xuri/excelize/v2"

	"github.com/domonda/go-retable"
)

// ReadFirstSheet reads the first sheet from an Excel file provided via io.Reader
// and returns it as a retable.View.
//
// The first row of the sheet is used as column headers, and subsequent rows
// contain the data. Empty rows and columns are automatically removed from the
// edges of the data range.
//
// Parameters:
//   - reader: An io.Reader containing Excel file data (.xlsx, .xlsm, .xltm, .xltx)
//   - rawCellStrings: If true, cell values are returned as raw strings without
//     formatting applied. If false, Excel's display formatting is used (e.g.,
//     dates and numbers are formatted according to the cell's number format).
//
// Returns:
//   - sheetView: A retable.View representing the first sheet's data
//   - err: Error if the file cannot be read, parsed, or if the sheet doesn't exist
//
// Errors:
//   - Returns ErrSheetNotExist if the file contains no sheets (unlikely)
//   - Returns ErrEmptySheet if the first sheet has no data after cleanup
//   - Returns excelize parsing errors for malformed Excel files
//
// Example:
//
//	file, err := os.Open("report.xlsx")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer file.Close()
//
//	view, err := exceltable.ReadFirstSheet(file, false)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Columns: %v\n", view.Columns())
//	fmt.Printf("Rows: %d\n", view.NumRows())
func ReadFirstSheet(reader io.Reader, rawCellStrings bool) (sheetView retable.View, err error) {
	f, e := excelize.OpenReader(reader)
	if e != nil {
		return nil, e
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()
	sheet := f.GetSheetName(0)
	if sheet == "" {
		return nil, ErrSheetNotExist{SheetName: "<FirstSheet>"} // Should never happen (?)
	}
	return readSheet(f, sheet, rawCellStrings)
}

// Read reads all sheets from an Excel file provided via io.Reader and returns
// them as a slice of retable.View, one for each non-empty sheet.
//
// Each sheet's first row is used as column headers, with subsequent rows
// containing the data. Empty rows and columns are automatically removed from
// the edges of each sheet's data range.
//
// Parameters:
//   - reader: An io.Reader containing Excel file data (.xlsx, .xlsm, .xltm, .xltx)
//   - rawCellStrings: If true, cell values are returned as raw strings without
//     formatting applied. If false, Excel's display formatting is used (e.g.,
//     dates and numbers are formatted according to the cell's number format).
//
// Returns:
//   - sheetViews: A slice of retable.View, one for each sheet in the file.
//     The Title() method of each view returns the sheet name.
//   - err: Error if the file cannot be read, parsed, or if any sheet
//     processing fails
//
// Errors:
//   - Returns excelize parsing errors for malformed Excel files
//   - Returns errors from individual sheet processing (except ErrEmptySheet)
//   - If a sheet is empty, it is skipped rather than returning an error
//
// Example:
//
//	file, err := os.Open("workbook.xlsx")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer file.Close()
//
//	views, err := exceltable.Read(file, false)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, view := range views {
//	    fmt.Printf("Sheet: %s\n", view.Title())
//	    fmt.Printf("  Columns: %d\n", len(view.Columns()))
//	    fmt.Printf("  Rows: %d\n", view.NumRows())
//	}
func Read(reader io.Reader, rawCellStrings bool) (sheetViews []retable.View, err error) {
	f, e := excelize.OpenReader(reader)
	if e != nil {
		return nil, e
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()
	for _, sheet := range f.GetSheetList() {
		view, err := readSheet(f, sheet, rawCellStrings)
		if err != nil {
			return nil, err
		}
		if view != nil {
			sheetViews = append(sheetViews, view)
		}
	}
	return sheetViews, nil
}

// ReadLocalFile reads all sheets from an Excel file at the specified file path
// and returns them as a slice of retable.View, one for each non-empty sheet.
//
// This function is a convenience wrapper that opens a local file and processes
// all sheets. Empty sheets are automatically skipped without generating errors.
//
// Each sheet's first row is used as column headers, with subsequent rows
// containing the data. Empty rows and columns are automatically removed from
// the edges of each sheet's data range.
//
// Parameters:
//   - filename: Path to the Excel file (.xlsx, .xlsm, .xltm, .xltx)
//   - rawCellStrings: If true, cell values are returned as raw strings without
//     formatting applied. If false, Excel's display formatting is used (e.g.,
//     dates and numbers are formatted according to the cell's number format).
//
// Returns:
//   - sheetViews: A slice of retable.View, one for each non-empty sheet.
//     The Title() method of each view returns the sheet name.
//   - err: Error if the file cannot be opened, read, or parsed
//
// Errors:
//   - Returns file system errors if the file cannot be opened
//   - Returns excelize parsing errors for malformed Excel files
//   - Empty sheets are silently skipped (ErrEmptySheet is not returned)
//
// Example:
//
//	views, err := exceltable.ReadLocalFile("/path/to/data.xlsx", false)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, view := range views {
//	    fmt.Printf("Sheet: %s has %d rows\n", view.Title(), view.NumRows())
//	    for row := 0; row < view.NumRows(); row++ {
//	        for col := 0; col < len(view.Columns()); col++ {
//	            fmt.Printf("%v ", view.Cell(row, col))
//	        }
//	        fmt.Println()
//	    }
//	}
func ReadLocalFile(filename string, rawCellStrings bool) (sheetViews []retable.View, err error) {
	f, e := excelize.OpenFile(filename)
	if e != nil {
		return nil, e
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()
	for _, sheet := range f.GetSheetList() {
		view, err := readSheet(f, sheet, rawCellStrings)
		if err != nil {
			if errors.Is(err, ErrEmptySheet) {
				continue
			}
			return nil, err
		}
		sheetViews = append(sheetViews, view)
	}
	return sheetViews, nil
}

// ReadLocalFileFirstSheet reads only the first sheet from an Excel file at the
// specified file path and returns it as a retable.View.
//
// This function is a convenience wrapper for reading just the first sheet from
// a local file without processing all sheets.
//
// The first row of the sheet is used as column headers, and subsequent rows
// contain the data. Empty rows and columns are automatically removed from the
// edges of the data range.
//
// Parameters:
//   - filename: Path to the Excel file (.xlsx, .xlsm, .xltm, .xltx)
//   - rawCellStrings: If true, cell values are returned as raw strings without
//     formatting applied. If false, Excel's display formatting is used (e.g.,
//     dates and numbers are formatted according to the cell's number format).
//
// Returns:
//   - sheetView: A retable.View representing the first sheet's data
//   - err: Error if the file cannot be opened, read, or parsed
//
// Errors:
//   - Returns file system errors if the file cannot be opened
//   - Returns ErrSheetNotExist if the file contains no sheets (unlikely)
//   - Returns ErrEmptySheet if the first sheet has no data after cleanup
//   - Returns excelize parsing errors for malformed Excel files
//
// Example:
//
//	view, err := exceltable.ReadLocalFileFirstSheet("report.xlsx", true)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access column names
//	columns := view.Columns()
//	fmt.Printf("Columns: %v\n", columns)
//
//	// Iterate through rows
//	for row := 0; row < view.NumRows(); row++ {
//	    for col, colName := range columns {
//	        cellValue := view.Cell(row, col)
//	        fmt.Printf("%s: %v, ", colName, cellValue)
//	    }
//	    fmt.Println()
//	}
func ReadLocalFileFirstSheet(filename string, rawCellStrings bool) (sheetView retable.View, err error) {
	f, e := excelize.OpenFile(filename)
	if e != nil {
		return nil, e
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()
	sheet := f.GetSheetName(0)
	if sheet == "" {
		return nil, ErrSheetNotExist{SheetName: "<FirstSheet>"} // Should never happen (?)
	}
	return readSheet(f, sheet, rawCellStrings)
}

// readSheet is an internal helper function that extracts data from a specific
// sheet in an opened Excel file and converts it to a retable.View.
//
// The function performs the following operations:
//  1. Extracts all rows from the specified sheet using excelize
//  2. Removes empty rows from the top and bottom edges
//  3. Removes empty columns from the left and right edges
//  4. Uses the first row as column headers
//  5. Ensures column names array matches the actual number of columns
//
// Parameters:
//   - f: An opened excelize.File instance
//   - sheet: The name of the sheet to read
//   - rawCellStrings: If true, returns raw cell values; if false, uses formatted values
//
// Returns:
//   - retable.View: A view of the sheet's data, or nil if the sheet is empty
//   - error: ErrEmptySheet if no data remains after cleaning, or excelize errors
func readSheet(f *excelize.File, sheet string, rawCellStrings bool) (retable.View, error) {
	rows, err := f.GetRows(sheet, excelize.Options{RawCellValue: rawCellStrings})
	if err != nil {
		return nil, err
	}
	rows = retable.RemoveEmptyStringRows(rows)
	numCols := retable.RemoveEmptyStringColumns(rows)
	if len(rows) == 0 || numCols == 0 {
		return nil, ErrEmptySheet
	}
	columns := rows[0]
	rows = rows[1:]
	if len(columns) < numCols {
		// Append empty strings to columns to match numCols
		columns = append(columns, make([]string, numCols-len(columns))...)
	}
	return &sheetStringsView{
		sheet:   sheet,
		columns: columns,
		rows:    rows,
	}, nil
}

// sheetStringsView is an internal implementation of retable.View that holds
// Excel sheet data as a string-based table in memory.
//
// All cell values are stored as strings, regardless of their original Excel
// data type. This provides a simple, uniform interface for accessing tabular
// data from Excel files.
//
// The view is immutable once created and safe for concurrent read access.
type sheetStringsView struct {
	sheet   string     // Name of the Excel sheet
	columns []string   // Column headers from the first row
	rows    [][]string // Data rows (excluding the header row)
}

// Title returns the name of the Excel sheet this view represents.
// This implements the retable.View interface.
func (view *sheetStringsView) Title() string { return view.sheet }

// Columns returns the column headers extracted from the first row of the sheet.
// The returned slice should not be modified by callers.
// This implements the retable.View interface.
func (view *sheetStringsView) Columns() []string { return view.columns }

// NumRows returns the number of data rows in this view, excluding the header row.
// This implements the retable.View interface.
func (view *sheetStringsView) NumRows() int { return len(view.rows) }

// Cell returns the value at the specified row and column as a string.
// Returns nil if the row or column index is out of bounds, or if the
// specific cell is empty (beyond the row's column count).
//
// Parameters:
//   - row: Zero-based row index (0 is the first data row, after headers)
//   - col: Zero-based column index
//
// Returns:
//   - any: The cell value as a string, or nil if out of bounds
//
// This implements the retable.View interface.
func (view *sheetStringsView) Cell(row, col int) any {
	if row < 0 || col < 0 || row >= len(view.rows) || col >= len(view.rows[row]) {
		return nil
	}
	return view.rows[row][col]
}

// ReflectCell returns the reflect.Value of the cell at the specified row and column.
// Returns an invalid reflect.Value if the row or column index is out of bounds,
// or if the specific cell is empty (beyond the row's column count).
//
// This method is useful for generic reflection-based processing of table data.
//
// Parameters:
//   - row: Zero-based row index (0 is the first data row, after headers)
//   - col: Zero-based column index
//
// Returns:
//   - reflect.Value: A reflection value wrapping the cell's string value,
//     or an invalid reflect.Value if out of bounds
//
// This implements the retable.View interface.
func (view *sheetStringsView) ReflectCell(row, col int) reflect.Value {
	if row < 0 || col < 0 || row >= len(view.rows) || col >= len(view.rows[row]) {
		return reflect.Value{}
	}
	return reflect.ValueOf(view.rows[row][col])
}

// type sheetView struct {
// 	file    *excelize.File
// 	sheet   string
// 	columns []string
// 	numRows int
// }

// func (view *sheetView) Title() string     { return view.sheet }
// func (view *sheetView) Columns() []string { return view.columns }
// func (view *sheetView) NumRows() int      { return view.numRows }

// func (view *sheetView) Cell(row, col int) any {
// 	if row < 0 || col < 0 || row >= view.numRows || col >= len(view.columns) {
// 		return nil
// 	}
// 	panic("TODO")
// }

// func (view *sheetView) ReflectCell(row, col int) reflect.Value {
// 	if row < 0 || col < 0 || row >= view.numRows || col >= len(view.columns) {
// 		return reflect.Value{}
// 	}
// 	panic("TODO")
// }
