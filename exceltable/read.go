package exceltable

import (
	"errors"
	"io"
	"reflect"

	"github.com/xuri/excelize/v2"

	"github.com/domonda/go-retable"
)

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

type sheetStringsView struct {
	sheet   string
	columns []string
	rows    [][]string
}

func (view *sheetStringsView) Title() string     { return view.sheet }
func (view *sheetStringsView) Columns() []string { return view.columns }
func (view *sheetStringsView) NumRows() int      { return len(view.rows) }

func (view *sheetStringsView) AnyValue(row, col int) any {
	if row < 0 || col < 0 || row >= len(view.rows) || col >= len(view.rows[row]) {
		return nil
	}
	return view.rows[row][col]
}

func (view *sheetStringsView) ReflectValue(row, col int) reflect.Value {
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

// func (view *sheetView) AnyValue(row, col int) any {
// 	if row < 0 || col < 0 || row >= view.numRows || col >= len(view.columns) {
// 		return nil
// 	}
// 	panic("TODO")
// }

// func (view *sheetView) ReflectValue(row, col int) reflect.Value {
// 	if row < 0 || col < 0 || row >= view.numRows || col >= len(view.columns) {
// 		return reflect.Value{}
// 	}
// 	panic("TODO")
// }
