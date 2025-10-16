package exceltable

import (
	"errors"

	"github.com/xuri/excelize/v2"
)

var (
	// ErrEmptySheet indicates that an Excel sheet contains no data after
	// removing empty rows and columns. This error is returned when a sheet
	// exists but has no meaningful content to process.
	//
	// Empty sheets are automatically skipped when reading multiple sheets
	// from a file using ReadLocalFile.
	ErrEmptySheet = errors.New("empty sheet")
)

// ErrSheetNotExist is re-exported from excelize and indicates that a requested
// sheet name does not exist in the Excel file.
//
// This error type contains additional information about the missing sheet:
//   - SheetName: The name of the sheet that was not found
//
// Example:
//
//	var sheetErr exceltable.ErrSheetNotExist
//	if errors.As(err, &sheetErr) {
//	    fmt.Printf("Sheet not found: %s\n", sheetErr.SheetName)
//	}
type ErrSheetNotExist = excelize.ErrSheetNotExist
