package exceltable

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/domonda/go-retable"
)

// testSheet describes one sheet of a workbook built by newTestWorkbook.
type testSheet struct {
	name string
	rows [][]string
}

// newTestWorkbook returns an in-memory xlsx workbook with the passed sheets.
// Empty cell strings are not written to the workbook at all, which is how Excel
// stores blank cells and what makes excelize.File.GetRows return short rows.
func newTestWorkbook(t *testing.T, sheets ...testSheet) *excelize.File {
	t.Helper()

	f := excelize.NewFile()
	t.Cleanup(func() {
		require.NoError(t, f.Close())
	})

	for i, sheet := range sheets {
		if i == 0 {
			require.NoError(t, f.SetSheetName(f.GetSheetName(0), sheet.name))
		} else {
			_, err := f.NewSheet(sheet.name)
			require.NoError(t, err)
		}
		for r, row := range sheet.rows {
			for c, cell := range row {
				if cell == "" {
					continue
				}
				axis, err := excelize.CoordinatesToCellName(c+1, r+1)
				require.NoError(t, err)
				require.NoError(t, f.SetCellStr(sheet.name, axis, cell))
			}
		}
	}
	return f
}

// writeTestWorkbook returns the bytes of an xlsx file with the passed sheets.
func writeTestWorkbook(t *testing.T, sheets ...testSheet) []byte {
	t.Helper()

	buf, err := newTestWorkbook(t, sheets...).WriteToBuffer()
	require.NoError(t, err)
	return buf.Bytes()
}

// writeTestWorkbookFile writes the workbook to a temporary file and returns its path.
func writeTestWorkbookFile(t *testing.T, sheets ...testSheet) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "workbook.xlsx")
	require.NoError(t, newTestWorkbook(t, sheets...).SaveAs(path))
	return path
}

// Excel leaves trailing cells of a row blank and excelize.File.GetRows omits
// them, so data rows are routinely shorter than the header row. Such a cell is
// still within the range of Columns() and must therefore behave like an empty
// cell instead of like an out-of-range coordinate: retable.View documents nil
// only for out-of-range access, and reflection based formatters call
// reflect.Value.Type on the result of ReflectCell, which panics for an
// invalid reflect.Value.
func TestReadFirstSheetSparseRows(t *testing.T) {
	data := writeTestWorkbook(t, testSheet{
		name: "Sheet1",
		rows: [][]string{
			{"Name", "Age"},
			{"Alice"}, // Age left blank
			{"Bob", "25"},
		},
	})

	view, err := ReadFirstSheet(bytes.NewReader(data), false)
	require.NoError(t, err)
	require.Equal(t, []string{"Name", "Age"}, view.Columns())
	require.Equal(t, 2, view.NumRows())

	require.Equal(t, "", view.Cell(0, 1), "blank trailing cell must be an empty string, not nil")
	require.Nil(t, view.Cell(0, 2), "column index out of range must be nil")
	require.Nil(t, view.Cell(2, 0), "row index out of range must be nil")

	cell := retable.AsReflectCellView(view).ReflectCell(0, 1)
	require.True(t, cell.IsValid(), "blank trailing cell must yield a valid reflect.Value")
	require.Equal(t, reflect.String, cell.Kind())

	// retable.ReflectTypeCellFormatter calls reflect.Value.Type on every cell,
	// which panics for an invalid reflect.Value.
	formatter := retable.NewReflectTypeCellFormatter().
		WithKindFormatter(reflect.String, retable.PrintfCellFormatter("%s"))
	str, _, err := formatter.FormatCell(context.Background(), view, 0, 1)
	require.NoError(t, err)
	require.Equal(t, "", str)

	// retable.ViewToStructSlice skips cells with an invalid reflect.Value,
	// which would silently bypass the validate func for blank cells.
	var validated []string
	_, err = retable.ViewToStructSlice[struct{ Name, Age string }](
		view, nil, nil, nil, nil,
		func(v reflect.Value) error {
			validated = append(validated, v.String())
			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, []string{"Alice", "", "Bob", "25"}, validated)
}

// Hand edited spreadsheets frequently contain padded header cells. Untrimmed
// column names break every lookup by column name, so they are trimmed like in
// all other retable View constructors.
func TestReadFirstSheetTrimsColumnNames(t *testing.T) {
	data := writeTestWorkbook(t, testSheet{
		name: "Sheet1",
		rows: [][]string{
			{"  Name  ", " Age "},
			{"Alice", "30"},
		},
	})

	view, err := ReadFirstSheet(bytes.NewReader(data), false)
	require.NoError(t, err)
	require.Equal(t, []string{"Name", "Age"}, view.Columns())
}

// Completely empty rows and columns are removed wherever they occur, not only
// at the edges of the data range, so a blank separator row or column does not
// survive and the remaining data is compacted.
func TestReadFirstSheetRemovesEmptyRowsAndColumns(t *testing.T) {
	data := writeTestWorkbook(t, testSheet{
		name: "Sheet1",
		rows: [][]string{
			{"Name", "", "Age"},
			{"Alice", "", "30"},
			nil, // Separator row
			{"Bob", "", "25"},
		},
	})

	view, err := ReadFirstSheet(bytes.NewReader(data), false)
	require.NoError(t, err)
	require.Equal(t, []string{"Name", "Age"}, view.Columns())
	require.Equal(t, 2, view.NumRows())
	require.Equal(t, "Alice", view.Cell(0, 0))
	require.Equal(t, "30", view.Cell(0, 1))
	require.Equal(t, "Bob", view.Cell(1, 0))
	require.Equal(t, "25", view.Cell(1, 1))
}

func TestReadFirstSheetEmptySheet(t *testing.T) {
	data := writeTestWorkbook(t, testSheet{name: "Blank"})

	_, err := ReadFirstSheet(bytes.NewReader(data), false)
	require.ErrorIs(t, err, ErrEmptySheet)
}

// rawCellStrings selects between the raw stored cell value and the value
// rendered with the number format of the cell.
func TestReadFirstSheetRawCellStrings(t *testing.T) {
	f := newTestWorkbook(t, testSheet{name: "Sheet1", rows: [][]string{{"Ratio"}}})
	require.NoError(t, f.SetCellValue("Sheet1", "A2", 0.5))
	style, err := f.NewStyle(&excelize.Style{NumFmt: 10}) // 0.00%
	require.NoError(t, err)
	require.NoError(t, f.SetCellStyle("Sheet1", "A2", "A2", style))
	buf, err := f.WriteToBuffer()
	require.NoError(t, err)

	formatted, err := ReadFirstSheet(bytes.NewReader(buf.Bytes()), false)
	require.NoError(t, err)
	require.Equal(t, "50.00%", formatted.Cell(0, 0))

	raw, err := ReadFirstSheet(bytes.NewReader(buf.Bytes()), true)
	require.NoError(t, err)
	require.Equal(t, "0.5", raw.Cell(0, 0))
}

func TestReadFirstSheetInvalidFile(t *testing.T) {
	_, err := ReadFirstSheet(strings.NewReader("not an Excel file"), false)
	require.Error(t, err)
}

// A workbook with a blank sheet is the Excel default, so an empty sheet must
// be skipped instead of failing the whole workbook.
func TestReadSkipsEmptySheets(t *testing.T) {
	data := writeTestWorkbook(t,
		testSheet{name: "Data", rows: [][]string{{"Name"}, {"Alice"}}},
		testSheet{name: "Blank"},
		testSheet{name: "More", rows: [][]string{{"Name"}, {"Bob"}}},
	)

	views, err := Read(bytes.NewReader(data), false)
	require.NoError(t, err)
	require.Len(t, views, 2)
	require.Equal(t, "Data", views[0].Title())
	require.Equal(t, "Alice", views[0].Cell(0, 0))
	require.Equal(t, "More", views[1].Title())
	require.Equal(t, "Bob", views[1].Cell(0, 0))
}

func TestReadOnlyEmptySheets(t *testing.T) {
	data := writeTestWorkbook(t, testSheet{name: "Blank1"}, testSheet{name: "Blank2"})

	views, err := Read(bytes.NewReader(data), false)
	require.NoError(t, err)
	require.Empty(t, views)
}

func TestReadInvalidFile(t *testing.T) {
	_, err := Read(strings.NewReader("not an Excel file"), false)
	require.Error(t, err)
}

func TestReadLocalFile(t *testing.T) {
	path := writeTestWorkbookFile(t,
		testSheet{name: "First", rows: [][]string{{"Name", "Age"}, {"Alice", "30"}}},
		testSheet{name: "Second", rows: [][]string{{"Name", "Age"}, {"Bob", "25"}}},
	)

	views, err := ReadLocalFile(path, false)
	require.NoError(t, err)
	require.Len(t, views, 2)
	require.Equal(t, "First", views[0].Title())
	require.Equal(t, []string{"Name", "Age"}, views[0].Columns())
	require.Equal(t, "Alice", views[0].Cell(0, 0))
	require.Equal(t, "Second", views[1].Title())
	require.Equal(t, "Bob", views[1].Cell(0, 0))
}

func TestReadLocalFileFirstSheet(t *testing.T) {
	path := writeTestWorkbookFile(t,
		testSheet{name: "First", rows: [][]string{{"Name"}, {"Alice"}}},
		testSheet{name: "Second", rows: [][]string{{"Name"}, {"Bob"}}},
	)

	view, err := ReadLocalFileFirstSheet(path, false)
	require.NoError(t, err)
	require.Equal(t, "First", view.Title())
	require.Equal(t, 1, view.NumRows())
	require.Equal(t, "Alice", view.Cell(0, 0))
}

func TestReadLocalFileNotExist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.xlsx")

	_, err := ReadLocalFile(missing, false)
	require.Error(t, err)

	_, err = ReadLocalFileFirstSheet(missing, false)
	require.Error(t, err)
}
