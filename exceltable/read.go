// Package exceltable provides functionality for reading Excel files (.xlsx, .xlsm, .xltm, .xltx)
// and converting them into retable.View interfaces for tabular data manipulation.
//
// The package uses the excelize library (github.com/xuri/excelize/v2) under the hood
// to parse Excel files and extract sheet data as string-based tables.
//
// Key features:
//   - Read single or multiple sheets from Excel files
//   - Support for both file paths and io.Reader sources
//   - Automatic removal of every completely empty row and column, including
//     empty rows and columns between non-empty ones. The remaining data is
//     compacted, so row indices do not correspond to the original Excel row
//     numbers.
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

	"github.com/xuri/excelize/v2"

	"github.com/domonda/go-retable"
)

// ReadFirstSheet reads the first sheet from an Excel file provided via io.Reader
// and returns it as a retable.View.
//
// The first row of the sheet is used as column headers, and subsequent rows
// contain the data. Empty rows and columns are removed as described in the
// package documentation.
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
//	fmt.Printf("Columns: %v\n", view.ColumnNames())
//	fmt.Printf("Rows: %d\n", view.NumRows())
func ReadFirstSheet(reader io.Reader, rawCellStrings bool) (sheetView retable.View, err error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, err
	}
	return readFirstSheet(f, rawCellStrings)
}

// Read reads all sheets from an Excel file provided via io.Reader and returns
// them as a slice of retable.View, one for each non-empty sheet.
//
// Each sheet's first row is used as column headers, with subsequent rows
// containing the data. Empty rows and columns are removed as described in the
// package documentation.
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
//	    fmt.Printf("  Columns: %d\n", view.NumColumns())
//	    fmt.Printf("  Rows: %d\n", view.NumRows())
//	}
func Read(reader io.Reader, rawCellStrings bool) (sheetViews []retable.View, err error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, err
	}
	return readAllSheets(f, rawCellStrings)
}

// ReadLocalFile reads all sheets from an Excel file at the specified file path
// and returns them as a slice of retable.View, one for each non-empty sheet.
//
// This function is a convenience wrapper that opens a local file and processes
// all sheets. Empty sheets are automatically skipped without generating errors.
//
// Each sheet's first row is used as column headers, with subsequent rows
// containing the data. Empty rows and columns are removed as described in the
// package documentation.
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
//	        for col := 0; col < view.NumColumns(); col++ {
//	            fmt.Printf("%v ", view.Cell(row, col))
//	        }
//	        fmt.Println()
//	    }
//	}
func ReadLocalFile(filename string, rawCellStrings bool) (sheetViews []retable.View, err error) {
	f, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, err
	}
	return readAllSheets(f, rawCellStrings)
}

// ReadLocalFileFirstSheet reads only the first sheet from an Excel file at the
// specified file path and returns it as a retable.View.
//
// This function is a convenience wrapper for reading just the first sheet from
// a local file without processing all sheets.
//
// The first row of the sheet is used as column headers, and subsequent rows
// contain the data. Empty rows and columns are removed as described in the
// package documentation.
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
//	columns := view.ColumnNames()
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
	f, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, err
	}
	return readFirstSheet(f, rawCellStrings)
}

// readFirstSheet returns the View of the first sheet of f and closes f.
func readFirstSheet(f *excelize.File, rawCellStrings bool) (sheetView retable.View, err error) {
	defer func() {
		err = errors.Join(err, f.Close())
	}()
	sheet := f.GetSheetName(0)
	if sheet == "" {
		return nil, ErrSheetNotExist{SheetName: "<FirstSheet>"} // Should never happen (?)
	}
	return readSheet(f, sheet, rawCellStrings)
}

// readAllSheets returns a View for every non-empty sheet of f and closes f.
// Sheets without data are skipped instead of returning ErrEmptySheet.
func readAllSheets(f *excelize.File, rawCellStrings bool) (sheetViews []retable.View, err error) {
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

// readSheet extracts the data of a sheet from an opened Excel file as a
// retable.StringsView that uses the first row as column names.
//
// Empty rows and columns are removed with retable.RemoveEmptyStringRows and
// retable.RemoveEmptyStringColumns before retable.NewStringsView splits off
// the first row as column names and trims their surrounding whitespace.
//
// excelize.File.GetRows omits trailing empty cells, so rows can be shorter
// than the column count. retable.NewStringsView widens a header row that is
// shorter than the widest data row, and retable.StringsView handles the sparse
// data rows natively by reporting "" for the missing trailing cells.
//
// Parameters:
//   - f: An opened excelize.File instance
//   - sheet: The name of the sheet to read
//   - rawCellStrings: If true, returns raw cell values; if false, uses formatted values
//
// Returns:
//   - retable.View: A retable.StringsView of the sheet's data
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
	return retable.NewStringsView(sheet, rows), nil
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
