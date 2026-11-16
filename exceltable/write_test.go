package exceltable

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/domonda/go-retable"
)

// The tests in this file read the written files back with excelize
// directly instead of with exceltable.Read, because the reader returns
// strings and the point of the writer is typed cells: they assert the
// cell type, the raw stored value and the number format code.

// writtenCell is what a test sees of one written cell.
type writtenCell struct {
	Type   excelize.CellType
	Raw    string
	NumFmt string // custom number format code, "" for General
}

var (
	emptyCell = writtenCell{Type: excelize.CellTypeUnset}
	textCell  = func(s string) writtenCell { return writtenCell{Type: excelize.CellTypeInlineString, Raw: s} }
	numCell   = func(raw, numFmt string) writtenCell {
		return writtenCell{Type: excelize.CellTypeUnset, Raw: raw, NumFmt: numFmt}
	}
	boolCell = func(b bool) writtenCell {
		if b {
			return writtenCell{Type: excelize.CellTypeBool, Raw: "1"}
		}
		return writtenCell{Type: excelize.CellTypeBool, Raw: "0"}
	}
)

// openWritten parses the .xlsx bytes of data.
func openWritten(t *testing.T, data []byte) *excelize.File {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// readCell returns the cell of sheet at the 1-based column and row.
// An empty sheet name reads the first sheet.
func readCell(t *testing.T, f *excelize.File, sheet string, col, row int) writtenCell {
	t.Helper()
	if sheet == "" {
		sheet = f.GetSheetName(0)
	}
	ref, err := excelize.CoordinatesToCellName(col, row)
	require.NoError(t, err)
	cellType, err := f.GetCellType(sheet, ref)
	require.NoError(t, err)
	raw, err := f.GetCellValue(sheet, ref, excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	styleID, err := f.GetCellStyle(sheet, ref)
	require.NoError(t, err)
	style, err := f.GetStyle(styleID)
	require.NoError(t, err)
	numFmt := ""
	if style.CustomNumFmt != nil {
		numFmt = *style.CustomNumFmt
	} else {
		require.Zero(t, style.NumFmt, "cell %s uses built-in number format %d instead of a custom code", ref, style.NumFmt)
	}
	return writtenCell{Type: cellType, Raw: raw, NumFmt: numFmt}
}

// writeOneSheet writes table with writer as a single sheet "S" and opens the result.
func writeOneSheet[T any](t *testing.T, writer *Writer[T], table T) *excelize.File {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, writer.Write(context.Background(), &buf, table, "S"))
	return openWritten(t, buf.Bytes())
}

func TestContentType(t *testing.T) {
	require.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ContentType)
}

func TestSanitizeSheetName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Overview", "Overview"},
		{"  Overview \t", "Overview"},
		{"", "UNNAMED"},
		{"   ", "UNNAMED"},
		{`a\b`, "a_b"},
		{"a/b", "a_b"},
		{"a?b", "a_b"},
		{"a*b", "a_b"},
		{"a[b]", "a_b_"},
		{"KST: Vertrieb", "KST_ Vertrieb"},
		{"'Q1'", "_Q1_"},
		{"'", "_"},
		{"O'Brien", "O'Brien"},
		{strings.Repeat("a", 30), strings.Repeat("a", 30)},
		{strings.Repeat("a", 31), strings.Repeat("a", 31)},
		{strings.Repeat("a", 32), strings.Repeat("a", 30) + "…"},
		// 31-character Maltese IBAN stays whole
		{"MT84MALT011000012345MTLCAST001S", "MT84MALT011000012345MTLCAST001S"},
		// Multi-byte runes inside the Basic Multilingual Plane count as one unit
		{strings.Repeat("ä", 31), strings.Repeat("ä", 31)},
		{strings.Repeat("ä", 32), strings.Repeat("ä", 30) + "…"},
		// An emoji counts two UTF-16 units; 16 of them are 32 units
		{strings.Repeat("😀", 15) + "a", strings.Repeat("😀", 15) + "a"},
		{strings.Repeat("😀", 16), strings.Repeat("😀", 15) + "…"},
		// The emoji does not fit into the last unit before "…"
		{strings.Repeat("a", 29) + "😀b", strings.Repeat("a", 29) + "…"},
		// Replacement happens before truncation
		{strings.Repeat("/", 40), strings.Repeat("_", 30) + "…"},
		// A trailing apostrophe cut off by truncation is no problem
		{strings.Repeat("a", 30) + "'b", strings.Repeat("a", 30) + "…"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeSheetName(tc.input)
			require.Equal(t, tc.want, got)
			// Every sanitized name must be accepted by excelize
			wb := NewWorkbook()
			defer wb.Close()
			_, err := wb.addSheet(tc.input)
			require.NoError(t, err)
			require.Equal(t, []string{got}, wb.file.GetSheetList())
		})
	}
}

type overviewRow struct {
	Name  string `col:"Name"`
	Count int    `col:"Count"`
}

type detailRow struct {
	ID     string  `col:"ID"`
	Amount float64 `col:"Amount"`
}

// TestWriter_MultipleSheets writes sheets of different row types into one
// workbook, like the cost center export: an overview, then one sheet per
// cost center, in call order.
func TestWriter_MultipleSheets(t *testing.T) {
	ctx := context.Background()
	wb := NewWorkbook()
	defer wb.Close()

	require.NoError(t, NewWriter[[]overviewRow]().WithHeaderRow(true).
		WriteSheet(ctx, wb, []overviewRow{{"KST 1", 2}, {"KST 2", 1}}, "Overview"))
	details := NewWriter[[]*detailRow]().WithHeaderRow(true)
	require.NoError(t, details.WriteSheet(ctx, wb, []*detailRow{{"a", 1.5}, {"b", 2}}, "KST 1"))
	require.NoError(t, details.WriteSheet(ctx, wb, []*detailRow{{"c", 3}}, "KST 2"))

	var buf bytes.Buffer
	_, err := wb.WriteTo(&buf)
	require.NoError(t, err)
	f := openWritten(t, buf.Bytes())

	require.Equal(t, []string{"Overview", "KST 1", "KST 2"}, f.GetSheetList())
	require.Equal(t, textCell("Name"), readCell(t, f, "Overview", 1, 1))
	require.Equal(t, textCell("KST 2"), readCell(t, f, "Overview", 1, 3))
	require.Equal(t, numCell("1", ""), readCell(t, f, "Overview", 2, 3))
	require.Equal(t, textCell("Amount"), readCell(t, f, "KST 1", 2, 1))
	require.Equal(t, numCell("1.5", ""), readCell(t, f, "KST 1", 2, 2))
	require.Equal(t, textCell("c"), readCell(t, f, "KST 2", 1, 2))
	require.Equal(t, numCell("3", ""), readCell(t, f, "KST 2", 2, 2))
}

func TestWriter_DuplicateSheetName(t *testing.T) {
	ctx := context.Background()
	writer := NewWriter[[]overviewRow]()
	rows := []overviewRow{{"x", 1}}
	cases := []struct {
		name         string
		first, again string
	}{
		{"exact", "Overview", "Overview"},
		{"case-insensitive", "Overview", "OVERVIEW"},
		{"after trimming", "Overview", " Overview "},
		{"after replacing", "a/b", "a?b"},
		{"after truncation", strings.Repeat("a", 31) + "1", strings.Repeat("a", 31) + "2"},
		{"umlaut case", "Übersicht", "übersicht"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wb := NewWorkbook()
			defer wb.Close()
			require.NoError(t, writer.WriteSheet(ctx, wb, rows, tc.first))
			err := writer.WriteSheet(ctx, wb, rows, tc.again)
			require.ErrorIs(t, err, ErrDuplicateSheetName)
			require.ErrorContains(t, err, strconv.Quote(sanitizeSheetName(tc.again)))

			// The workbook is unchanged and still usable
			require.Len(t, wb.file.GetSheetList(), 1)
			require.NoError(t, writer.WriteSheet(ctx, wb, rows, "Other"))
			var buf bytes.Buffer
			_, err = wb.WriteTo(&buf)
			require.NoError(t, err)
			require.Equal(t, []string{sanitizeSheetName(tc.first), "Other"}, openWritten(t, buf.Bytes()).GetSheetList())
		})
	}
}

func TestWriter_HeaderRow(t *testing.T) {
	rows := []overviewRow{{"KST 1", 2}}

	t.Run("off by default", func(t *testing.T) {
		f := writeOneSheet(t, NewWriter[[]overviewRow](), rows)
		require.Equal(t, textCell("KST 1"), readCell(t, f, "S", 1, 1))
		require.Equal(t, numCell("2", ""), readCell(t, f, "S", 2, 1))
	})

	t.Run("bold column titles with default font", func(t *testing.T) {
		f := writeOneSheet(t, NewWriter[[]overviewRow]().WithHeaderRow(true), rows)
		require.Equal(t, textCell("Name"), readCell(t, f, "S", 1, 1))
		require.Equal(t, textCell("Count"), readCell(t, f, "S", 2, 1))
		require.Equal(t, textCell("KST 1"), readCell(t, f, "S", 1, 2))
		styleID, err := f.GetCellStyle("S", "A1")
		require.NoError(t, err)
		style, err := f.GetStyle(styleID)
		require.NoError(t, err)
		require.NotNil(t, style.Font)
		require.True(t, style.Font.Bold)
		require.Empty(t, style.Font.Family)
		// Data cells have no style of their own
		styleID, err = f.GetCellStyle("S", "A2")
		require.NoError(t, err)
		require.Zero(t, styleID)
	})

	t.Run("configured font", func(t *testing.T) {
		writer := NewWriter[[]overviewRow]().WithHeaderRow(true).WithHeaderFont("Liberation Sans", 10)
		f := writeOneSheet(t, writer, rows)
		styleID, err := f.GetCellStyle("S", "B1")
		require.NoError(t, err)
		style, err := f.GetStyle(styleID)
		require.NoError(t, err)
		require.Equal(t, &excelize.Font{Bold: true, Family: "Liberation Sans", Size: 10}, style.Font)
	})

	t.Run("titles from untagged fields", func(t *testing.T) {
		type row struct{ CostCenterNumber string }
		f := writeOneSheet(t, NewWriter[[]row]().WithHeaderRow(true), []row{{"1"}})
		require.Equal(t, textCell("Cost Center Number"), readCell(t, f, "S", 1, 1))
	})
}

// Types for TestWriter_BuiltinCells
type (
	builtinStringer  struct{ s string }
	builtinPtrString struct{ s string }
	builtinEnum      string
	builtinIntEnum   int
	builtinNullable  struct{ s string }
	builtinPlain     struct {
		A int
		B string
	}
)

func (b builtinStringer) String() string   { return "stringer:" + b.s }
func (b *builtinPtrString) String() string { return "ptr-stringer:" + b.s }
func (e builtinEnum) String() string       { return "enum:" + string(e) }
func (e builtinIntEnum) String() string    { return "int-enum" }
func (n builtinNullable) IsNull() bool     { return n.s == "" }

// TestWriter_BuiltinCells covers each built-in conversion step of the
// resolution order with no formatter registered.
func TestWriter_BuiltinCells(t *testing.T) {
	vienna := mustLoadLocation(t, "Europe/Vienna")
	someTime := time.Date(2026, 11, 16, 19, 1, 35, 0, vienna)
	// timeSerial is 46342.792766203704, stored with Excel's 15 significant digits
	someSerial := "46342.7927662037"
	var (
		nilTimePtr *time.Time
		nilIntPtr  *int
		zeroTime   time.Time
		one        = 1
	)
	longString := strings.Repeat("x", 32767+10)
	// A self referential value is legal Go and has no end to unwrap
	selfAny := new(any)
	*selfAny = selfAny
	cases := []struct {
		name string
		cell any
		want writtenCell
		// wantNull is the expected cell with WithNilValue("NULL")
		// when it differs from want
		wantNull *writtenCell
	}{
		{name: "string", cell: "text", want: textCell("text")},
		{name: "empty string is a text cell", cell: "", want: textCell("")},
		{name: "formula-like string stays text", cell: "=1+1", want: textCell("=1+1")},
		{name: "long string truncated to 32767", cell: longString, want: textCell(longString[:32767])},
		{name: "int", cell: -42, want: numCell("-42", "")},
		{name: "int64 max exact", cell: int64(math.MaxInt64), want: numCell("9223372036854775807", "")},
		{name: "uint64 max exact", cell: uint64(math.MaxUint64), want: numCell("18446744073709551615", "")},
		{name: "uint8", cell: uint8(200), want: numCell("200", "")},
		{name: "float64", cell: 1234.5, want: numCell("1234.5", "")},
		{name: "float32", cell: float32(0.5), want: numCell("0.5", "")},
		{name: "NaN as text", cell: math.NaN(), want: textCell("NaN")},
		{name: "+Inf as text", cell: math.Inf(1), want: textCell("+Inf")},
		{name: "bool true", cell: true, want: boolCell(true)},
		{name: "bool false", cell: false, want: boolCell(false)},
		{name: "time in its own location", cell: someTime, want: numCell(someSerial, "dd.mm.yyyy hh:mm:ss")},
		{name: "pointer to time", cell: &someTime, want: numCell(someSerial, "dd.mm.yyyy hh:mm:ss")},
		{name: "zero time is empty, not null", cell: zeroTime, want: emptyCell},
		{name: "pointer to zero time is empty, not null", cell: &zeroTime, want: emptyCell},
		{name: "nil time pointer is null", cell: nilTimePtr, want: emptyCell, wantNull: &writtenCell{Type: excelize.CellTypeInlineString, Raw: "NULL"}},
		{name: "duration over 24h", cell: 30 * time.Hour, want: numCell("1.25", "[h]:mm:ss")},
		{name: "pointer to duration", cell: new(30 * time.Hour), want: numCell("1.25", "[h]:mm:ss")},
		{name: "negative duration as text", cell: -90 * time.Minute, want: textCell("-1:30:00")},
		{name: "time before 1900 as text", cell: time.Date(1899, 1, 2, 3, 4, 5, 0, time.UTC), want: textCell("1899-01-02T03:04:05")},
		{name: "1900-01-01 is serial 1", cell: time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC), want: numCell("1", "dd.mm.yyyy hh:mm:ss")},
		{name: "nil pointer is null", cell: nilIntPtr, want: emptyCell, wantNull: &writtenCell{Type: excelize.CellTypeInlineString, Raw: "NULL"}},
		{name: "nil interface is null", cell: nil, want: emptyCell, wantNull: &writtenCell{Type: excelize.CellTypeInlineString, Raw: "NULL"}},
		{name: "pointer to int", cell: &one, want: numCell("1", "")},
		{name: "IsNull true is null", cell: builtinNullable{}, want: emptyCell, wantNull: &writtenCell{Type: excelize.CellTypeInlineString, Raw: "NULL"}},
		{name: "sql.NullString is null", cell: sql.NullString{}, want: emptyCell, wantNull: &writtenCell{Type: excelize.CellTypeInlineString, Raw: "NULL"}},
		{name: "named string kind wins over Stringer", cell: builtinEnum("a"), want: textCell("a")},
		{name: "named int kind wins over Stringer", cell: builtinIntEnum(7), want: numCell("7", "")},
		{name: "Stringer of the value", cell: builtinStringer{"v"}, want: textCell("stringer:v")},
		{name: "Stringer of a pointer", cell: &builtinPtrString{"p"}, want: textCell("ptr-stringer:p")},
		{name: "Stringer of the dereferenced value", cell: &builtinStringer{"d"}, want: textCell("stringer:d")},
		{name: "bytes as string", cell: []byte("bytes"), want: textCell("bytes")},
		{name: "Sprint fallback", cell: builtinPlain{A: 1, B: "b"}, want: textCell("{1 b}")},
		{name: "Sprint of dereferenced value", cell: &builtinPlain{A: 2, B: "c"}, want: textCell("{2 c}")},
		{name: "self referential value does not hang", cell: selfAny, want: textCell(fmt.Sprint(selfAny))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			view := &retable.AnyValuesView{Cols: []string{"A"}, Rows: [][]any{{tc.cell}}}

			var buf bytes.Buffer
			require.NoError(t, NewWriter[any]().WriteView(context.Background(), &buf, view))
			require.Equal(t, tc.want, readCell(t, openWritten(t, buf.Bytes()), "", 1, 1))

			wantNull := tc.want
			if tc.wantNull != nil {
				wantNull = *tc.wantNull
			}
			buf.Reset()
			require.NoError(t, NewWriter[any]().WithNilValue("NULL").WriteView(context.Background(), &buf, view))
			require.Equal(t, wantNull, readCell(t, openWritten(t, buf.Bytes()), "", 1, 1))
		})
	}
}

// TestWriter_AddressableStringer: a struct field whose type has String()
// on the pointer receiver is formatted through the field's address.
func TestWriter_AddressableStringer(t *testing.T) {
	type row struct{ Value builtinPtrString }
	f := writeOneSheet(t, NewWriter[[]row](), []row{{builtinPtrString{"field"}}})
	require.Equal(t, textCell("ptr-stringer:field"), readCell(t, f, "S", 1, 1))
}

func TestWriter_TimeAndDurationFormats(t *testing.T) {
	someTime := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	view := &retable.AnyValuesView{
		Cols: []string{"Time", "Duration", "DateTimeCell"},
		Rows: [][]any{{someTime, time.Hour, someTime}},
	}
	writer := NewWriter[any]().
		WithTimeFormat("yyyy-mm-dd").
		WithDurationFormat("[mm]:ss").
		WithColumnFormatter(2, CellFormatterFunc(func(ctx context.Context, view retable.View, row, col int) (Cell, error) {
			// An empty format falls back to the writer's time format
			return DateTimeCell(view.Cell(row, col).(time.Time), ""), nil
		}))
	var buf bytes.Buffer
	require.NoError(t, writer.WriteView(context.Background(), &buf, view))
	f := openWritten(t, buf.Bytes())
	serial := "46342" // midnight UTC
	require.Equal(t, numCell(serial, "yyyy-mm-dd"), readCell(t, f, "", 1, 1))
	// One hour is 1/24 = 0.041666666666666664, rounded to 15 significant
	// digits, not cut off to 0.0416666666666666 which is less than an hour
	require.Equal(t, numCell("0.0416666666666667", "[mm]:ss"), readCell(t, f, "", 2, 1))
	require.Equal(t, numCell(serial, "yyyy-mm-dd"), readCell(t, f, "", 3, 1))
}

// TestWriter_FormatCodesVerbatim: format codes are stored as given, also
// the ones Excel has built-in IDs for, which Excel would display per the
// reader's locale.
func TestWriter_FormatCodesVerbatim(t *testing.T) {
	codes := []string{
		"dd.mm.yyyy",
		"dd.mm.yyyy hh:mm:ss",
		"[h]:mm:ss",
		"#,##0.00",
		"#,##0.00 [$EUR];-#,##0.00 [$EUR]",
		"0.00%",
	}
	row := make([]any, len(codes))
	for i := range codes {
		row[i] = i
	}
	view := &retable.AnyValuesView{Cols: codes, Rows: [][]any{row}}
	writer := NewWriter[any]()
	for i, code := range codes {
		writer = writer.WithColumnFormatter(i, CellFormatterFunc(func(context.Context, retable.View, int, int) (Cell, error) {
			return NumberCell(1234.5, code), nil
		}))
	}
	var buf bytes.Buffer
	require.NoError(t, writer.WriteView(context.Background(), &buf, view))
	f := openWritten(t, buf.Bytes())
	for i, code := range codes {
		require.Equal(t, numCell("1234.5", code), readCell(t, f, "", i+1, 1), code)
	}
}

// resolutionValue implements both test interfaces.
type resolutionValue struct{}

func (resolutionValue) String() string { return "resolutionValue" }
func (resolutionValue) Error() string  { return "resolutionValue" }

// TestWriter_ResolutionOrder registers a formatter at every step for the
// same cell and lets them return errors.ErrUnsupported from the front,
// so each case shows which step handles the cell next.
func TestWriter_ResolutionOrder(t *testing.T) {
	steps := []string{"column", "type", "error interface", "stringer interface", "kind"}
	for unsupported := 0; unsupported <= len(steps); unsupported++ {
		name := "builtin"
		if unsupported < len(steps) {
			name = steps[unsupported]
		}
		t.Run(name, func(t *testing.T) {
			var calls []string
			step := func(index int) CellFormatter {
				return CellFormatterFunc(func(context.Context, retable.View, int, int) (Cell, error) {
					calls = append(calls, steps[index])
					if index < unsupported {
						return Cell{}, errors.ErrUnsupported
					}
					return StringCell(steps[index]), nil
				})
			}
			writer := NewWriter[any]().
				WithColumnFormatter(0, step(0)).
				WithTypeFormatter(reflect.TypeFor[resolutionValue](), step(1)).
				WithInterfaceTypeFormatter(reflect.TypeFor[error](), step(2)).
				WithInterfaceTypeFormatter(reflect.TypeFor[fmt.Stringer](), step(3)).
				WithKindFormatter(reflect.Struct, step(4))
			view := &retable.AnyValuesView{Cols: []string{"A"}, Rows: [][]any{{resolutionValue{}}}}
			var buf bytes.Buffer
			require.NoError(t, writer.WriteView(context.Background(), &buf, view))
			got := readCell(t, openWritten(t, buf.Bytes()), "", 1, 1)
			if unsupported < len(steps) {
				require.Equal(t, textCell(steps[unsupported]), got)
				require.Equal(t, steps[:unsupported+1], calls)
			} else {
				// All formatters declined: the built-in Stringer step
				require.Equal(t, textCell("resolutionValue"), got)
				require.Equal(t, steps, calls)
			}
		})
	}
}

// TestWriter_FormatterForPointedToType: a formatter registered for T is
// used for *T cells with a dereferenced view, and skipped for a nil *T.
func TestWriter_FormatterForPointedToType(t *testing.T) {
	type amount float64
	type row struct {
		Value *amount
		Nil   *amount
	}
	value := amount(12.5)
	amountCell := CellFormatterFunc(func(ctx context.Context, view retable.View, row, col int) (Cell, error) {
		return NumberCell(float64(view.Cell(row, col).(amount)), "#,##0.00"), nil
	})
	writer := NewWriter[[]row]().WithTypeFormatter(reflect.TypeFor[amount](), amountCell)
	f := writeOneSheet(t, writer, []row{{Value: &value}})
	require.Equal(t, numCell("12.5", "#,##0.00"), readCell(t, f, "S", 1, 1))
	require.Equal(t, emptyCell, readCell(t, f, "S", 2, 1))
}

// TestWriter_InterfaceField: a value in a struct field of type any reaches
// the formatter registered for its dynamic type.
func TestWriter_InterfaceField(t *testing.T) {
	type amount float64
	type row struct{ Value any }
	amountCell := CellFormatterFunc(func(ctx context.Context, view retable.View, row, col int) (Cell, error) {
		return NumberCell(float64(view.Cell(row, col).(amount)), "#,##0.00"), nil
	})
	value := amount(3)
	writer := NewWriter[[]row]().WithTypeFormatter(reflect.TypeFor[amount](), amountCell)
	f := writeOneSheet(t, writer, []row{{amount(2.5)}, {&value}, {nil}, {"text"}})
	require.Equal(t, numCell("2.5", "#,##0.00"), readCell(t, f, "S", 1, 1))
	require.Equal(t, numCell("3", "#,##0.00"), readCell(t, f, "S", 1, 2))
	require.Equal(t, emptyCell, readCell(t, f, "S", 1, 3))
	require.Equal(t, textCell("text"), readCell(t, f, "S", 1, 4))
}

func TestWriter_BuilderIsImmutable(t *testing.T) {
	view := &retable.AnyValuesView{Cols: []string{"A"}, Rows: [][]any{{1}}}
	read := func(w *Writer[any]) writtenCell {
		t.Helper()
		var buf bytes.Buffer
		require.NoError(t, w.WriteView(context.Background(), &buf, view))
		return readCell(t, openWritten(t, buf.Bytes()), "", 1, 1)
	}
	constant := func(s string) CellFormatter {
		return CellFormatterFunc(func(context.Context, retable.View, int, int) (Cell, error) {
			return StringCell(s), nil
		})
	}
	base := NewWriter[any]().WithColumnFormatter(0, constant("base"))
	require.Equal(t, textCell("base"), read(base))

	_ = base.WithColumnFormatter(0, constant("changed"))
	_ = base.WithColumnFormatter(1, constant("added"))
	_ = base.WithColumnFormatter(0, nil)
	_ = base.WithKindFormatter(reflect.Int, constant("kind"))
	_ = base.WithNilValue("NULL").WithHeaderRow(true).WithTimeFormat("x").WithDurationFormat("y").WithHeaderFont("f", 1)
	require.Equal(t, textCell("base"), read(base))

	// A nil formatter removes the registration
	require.Equal(t, numCell("1", ""), read(base.WithColumnFormatter(0, nil)))
	withKind := NewWriter[any]().WithKindFormatter(reflect.Int, constant("kind"))
	require.Equal(t, textCell("kind"), read(withKind))
	require.Equal(t, numCell("1", ""), read(withKind.WithKindFormatter(reflect.Int, nil)))
	withType := NewWriter[any]().WithTypeFormatter(reflect.TypeFor[int](), constant("type"))
	require.Equal(t, numCell("1", ""), read(withType.WithTypeFormatter(reflect.TypeFor[int](), nil)))
	withIface := NewWriter[any]().WithInterfaceTypeFormatter(reflect.TypeFor[fmt.Stringer](), constant("iface"))
	require.Equal(t, textCell("iface"), read(withIface.WithColumnFormatter(0, nil).WithTypeFormatter(reflect.TypeFor[int](), nil).WithInterfaceTypeFormatter(reflect.TypeFor[fmt.Stringer](), nil).WithColumnFormatter(0, constant("iface"))))
}

func TestWriter_WithInterfaceTypeFormatterPanics(t *testing.T) {
	require.PanicsWithValue(t, "retable: int is not an interface type", func() {
		NewWriter[any]().WithInterfaceTypeFormatter(reflect.TypeFor[int](), nil)
	})
	require.PanicsWithValue(t, "retable: <nil> is not an interface type", func() {
		NewWriter[any]().WithInterfaceTypeFormatter(nil, nil)
	})
}

// TestWriter_StringTables covers raw string tables: a [][]string whose
// first row is the header, and a retable.View of strings like the SQL
// query results domonda writes.
func TestWriter_StringTables(t *testing.T) {
	rows := [][]string{
		{"Name", "Amount"},
		{"a", "1.5"},
		{"b", ""},
	}
	f := writeOneSheet(t, NewWriter[[][]string]().WithHeaderRow(true), rows)
	require.Equal(t, textCell("Name"), readCell(t, f, "S", 1, 1))
	require.Equal(t, textCell("1.5"), readCell(t, f, "S", 2, 2))
	require.Equal(t, textCell(""), readCell(t, f, "S", 2, 3))

	view := retable.NewStringsView("Query Result", rows[1:], rows[0]...)
	var buf bytes.Buffer
	require.NoError(t, NewWriter[retable.View]().WithHeaderRow(true).WriteView(context.Background(), &buf, view))
	f = openWritten(t, buf.Bytes())
	require.Equal(t, []string{"Query Result"}, f.GetSheetList())
	require.Equal(t, textCell("Amount"), readCell(t, f, "Query Result", 2, 1))
	require.Equal(t, textCell("b"), readCell(t, f, "Query Result", 1, 3))
}

func TestWorkbook_WriteTo(t *testing.T) {
	t.Run("no sheets", func(t *testing.T) {
		wb := NewWorkbook()
		defer wb.Close()
		_, err := wb.WriteTo(new(bytes.Buffer))
		require.EqualError(t, err, "exceltable: workbook has no sheets")
	})

	t.Run("twice, with a sheet added in between", func(t *testing.T) {
		ctx := context.Background()
		wb := NewWorkbook()
		defer wb.Close()
		writer := NewWriter[[]overviewRow]()
		require.NoError(t, writer.WriteSheet(ctx, wb, []overviewRow{{"a", 1}}, "One"))

		var first, second bytes.Buffer
		n, err := wb.WriteTo(&first)
		require.NoError(t, err)
		require.Equal(t, int64(first.Len()), n)
		_, err = wb.WriteTo(&second)
		require.NoError(t, err)
		require.Equal(t, []string{"One"}, openWritten(t, first.Bytes()).GetSheetList())
		require.Equal(t, []string{"One"}, openWritten(t, second.Bytes()).GetSheetList())

		require.NoError(t, writer.WriteSheet(ctx, wb, []overviewRow{{"b", 2}}, "Two"))
		var third bytes.Buffer
		_, err = wb.WriteTo(&third)
		require.NoError(t, err)
		f := openWritten(t, third.Bytes())
		require.Equal(t, []string{"One", "Two"}, f.GetSheetList())
		require.Equal(t, textCell("b"), readCell(t, f, "Two", 1, 1))
	})
}

// TestWorkbook_Poisoned: once a sheet failed after it was added, the
// workbook refuses every further write, so a sheet with only part of its
// rows can never reach a file. Errors before the sheet exists do not
// poison the workbook.
func TestWorkbook_Poisoned(t *testing.T) {
	rows := []overviewRow{{"a", 1}, {"b", 2}, {"c", 3}}
	errFormatter := errors.New("formatter failed")

	t.Run("formatter error in row 2", func(t *testing.T) {
		wb := NewWorkbook()
		defer wb.Close()
		failing := NewWriter[[]overviewRow]().WithColumnFormatter(0, CellFormatterFunc(
			func(ctx context.Context, view retable.View, row, col int) (Cell, error) {
				if row == 1 {
					return Cell{}, errFormatter
				}
				return Cell{}, errors.ErrUnsupported
			},
		))
		err := failing.WriteSheet(context.Background(), wb, rows, "Failing")
		require.ErrorIs(t, err, errFormatter)
		require.ErrorContains(t, err, `sheet "Failing": row 1, column "Name"`)

		_, err = wb.WriteTo(new(bytes.Buffer))
		require.ErrorIs(t, err, errFormatter)
		err = NewWriter[[]overviewRow]().WriteSheet(context.Background(), wb, rows, "Next")
		require.ErrorIs(t, err, errFormatter)
	})

	t.Run("context cancelled while writing", func(t *testing.T) {
		wb := NewWorkbook()
		defer wb.Close()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cancelling := NewWriter[[]overviewRow]().WithColumnFormatter(0, CellFormatterFunc(
			func(context.Context, retable.View, int, int) (Cell, error) {
				cancel()
				return Cell{}, errors.ErrUnsupported
			},
		))
		err := cancelling.WriteSheet(ctx, wb, rows, "Cancelled")
		require.ErrorIs(t, err, context.Canceled)
		_, err = wb.WriteTo(new(bytes.Buffer))
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("context cancelled before the sheet is added", func(t *testing.T) {
		wb := NewWorkbook()
		defer wb.Close()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := NewWriter[[]overviewRow]().WriteSheet(ctx, wb, rows, "Never")
		require.ErrorIs(t, err, context.Canceled)
		require.NoError(t, NewWriter[[]overviewRow]().WriteSheet(context.Background(), wb, rows, "Later"))
		var buf bytes.Buffer
		_, err = wb.WriteTo(&buf)
		require.NoError(t, err)
		require.Equal(t, []string{"Later"}, openWritten(t, buf.Bytes()).GetSheetList())
	})

	t.Run("write after a failed sheet in Write", func(t *testing.T) {
		failing := NewWriter[[]overviewRow]().WithKindFormatter(reflect.String, CellFormatterFunc(
			func(context.Context, retable.View, int, int) (Cell, error) {
				return Cell{}, errFormatter
			},
		))
		var buf bytes.Buffer
		err := failing.Write(context.Background(), &buf, rows, "S")
		require.ErrorIs(t, err, errFormatter)
		require.Zero(t, buf.Len(), "nothing is written to dest")
	})
}

func TestWriter_WriteViewerError(t *testing.T) {
	type row struct{ A int }
	// The default struct viewer rejects a non-slice table
	err := NewWriter[row]().Write(context.Background(), new(bytes.Buffer), row{1}, "S")
	require.Error(t, err)
	err = NewWriter[row]().WriteSheet(context.Background(), NewWorkbook(), row{1}, "S")
	require.Error(t, err)

	// A configured viewer is used instead of retable.SelectViewer
	viewer := retable.StringsViewer{Cols: []string{"X"}}
	f := writeOneSheet(t, NewWriter[[][]string]().WithTableViewer(viewer).WithHeaderRow(true), [][]string{{"1"}})
	require.Equal(t, textCell("X"), readCell(t, f, "S", 1, 1))
	require.Equal(t, textCell("1"), readCell(t, f, "S", 1, 2))
}

// roundTripRow has every supported field kind in value and pointer form.
type roundTripRow struct {
	String      string         `col:"String"`
	Int         int            `col:"Int"`
	Uint        uint16         `col:"Uint"`
	Float       float64        `col:"Float"`
	Bool        bool           `col:"Bool"`
	Time        time.Time      `col:"Time"`
	Duration    time.Duration  `col:"Duration"`
	StringPtr   *string        `col:"String Ptr"`
	IntPtr      *int           `col:"Int Ptr"`
	TimePtr     *time.Time     `col:"Time Ptr"`
	DurationPtr *time.Duration `col:"Duration Ptr"`
	NullPtr     *int           `col:"Null Ptr"`
}

// TestWriter_RoundTrip pins the round trip principle of the module for
// exceltable: structs written with a header row and read back with
// exceltable.Read and retable.ViewToStructSlice are equal. Times are UTC
// with whole seconds, because Excel dates have no time zone and the
// default time format shows no fraction of a second. Durations read back
// through the H:MM:SS form of StringParser.ParseDuration.
func TestWriter_RoundTrip(t *testing.T) {
	s := "pointer"
	i := -7
	tm := time.Date(2026, 11, 16, 19, 1, 35, 0, time.UTC)
	d := 30*time.Hour + 2*time.Second
	rows := []roundTripRow{
		{
			String: "text", Int: -42, Uint: 65535, Float: 1234.5, Bool: true,
			Time: tm, Duration: 90*time.Minute + 2*time.Second,
			StringPtr: &s, IntPtr: &i, TimePtr: &tm, DurationPtr: &d,
		},
		{
			// Excel cannot display a negative duration or a date before
			// 1900, so both are written as text that reads back.
			// Before 1900-03-01, Excel's day zero is 1899-12-31.
			String: "edge values", Duration: -(90*time.Minute + 2500*time.Millisecond),
			Time: time.Date(1899, 12, 31, 12, 0, 0, 0, time.UTC), TimePtr: new(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)),
			DurationPtr: new(time.Duration(0)), StringPtr: new(""),
		},
		{
			String: "zero values",
			// A nil *string would be written as an empty cell and read back
			// as a pointer to "", because the reader assigns "" to string
			// destinations verbatim. That is SmartAssign behavior shared
			// with csvtable, not specific to this writer.
			StringPtr: new(""),
		},
	}
	var buf bytes.Buffer
	require.NoError(t, NewWriter[[]roundTripRow]().WithHeaderRow(true).Write(context.Background(), &buf, rows, "Round Trip"))

	view, err := ReadFirstSheet(bytes.NewReader(buf.Bytes()), false)
	require.NoError(t, err)
	got, err := retable.ViewToStructSlice[roundTripRow](view, &retable.DefaultStructFieldNaming, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, rows, got)
}

// TestWriter_DurationPrecision: the default duration format rounds to
// whole seconds, a format with milliseconds keeps them.
func TestWriter_DurationPrecision(t *testing.T) {
	type row struct{ Duration time.Duration }
	rows := []row{{1500 * time.Millisecond}, {999 * time.Millisecond}}
	readBack := func(writer *Writer[[]row]) []row {
		t.Helper()
		var buf bytes.Buffer
		require.NoError(t, writer.WithHeaderRow(true).Write(context.Background(), &buf, rows, "D"))
		view, err := ReadFirstSheet(bytes.NewReader(buf.Bytes()), false)
		require.NoError(t, err)
		got, err := retable.ViewToStructSlice[row](view, &retable.DefaultStructFieldNaming, nil, nil, nil, nil)
		require.NoError(t, err)
		return got
	}
	require.Equal(t, []row{{2 * time.Second}, {time.Second}}, readBack(NewWriter[[]row]()))
	require.Equal(t, rows, readBack(NewWriter[[]row]().WithDurationFormat("[h]:mm:ss.000")))
}

// TestWriter_FloatPrecision: numbers are stored rounded to Excel's
// 15 significant digits. excelize cuts off the digits beyond 15, so
// without rounding first a value could be stored just below where it
// belongs.
func TestWriter_FloatPrecision(t *testing.T) {
	cases := []struct {
		value float64
		want  string
	}{
		{1.0 / 3, "0.333333333333333"},
		{2.0 / 3, "0.666666666666667"},             // cut off: 0.666666666666666
		{0.1 + 0.2, "0.3"},                         // 0.30000000000000004
		{123456789012345678, "123456789012346000"}, // cut off: 123456789012345000
		{-1.0 / 24, "-0.0416666666666667"},
		{1234.5, "1234.5"},
		{math.MaxFloat64, strconv.FormatFloat(math.MaxFloat64, 'f', -1, 64)},
	}
	row := make([]any, len(cases))
	cols := make([]string, len(cases))
	for i, tc := range cases {
		row[i] = tc.value
		cols[i] = strconv.Itoa(i)
	}
	var buf bytes.Buffer
	view := &retable.AnyValuesView{Cols: cols, Rows: [][]any{row}}
	require.NoError(t, NewWriter[any]().WriteView(context.Background(), &buf, view))
	f := openWritten(t, buf.Bytes())
	for i, tc := range cases {
		require.Equal(t, numCell(tc.want, ""), readCell(t, f, "", i+1, 1), "value %v", tc.value)
	}
}

// TestWriter_NilRowAndArray: a nil element of a slice of struct pointers is
// a row of null cells, and an array of structs is a table like a slice.
// Both used to fail in retable's struct viewer.
func TestWriter_NilRowAndArray(t *testing.T) {
	t.Run("nil row", func(t *testing.T) {
		rows := []*overviewRow{{"a", 1}, nil, {"c", 3}}
		f := writeOneSheet(t, NewWriter[[]*overviewRow]().WithNilValue("NULL"), rows)
		require.Equal(t, textCell("a"), readCell(t, f, "S", 1, 1))
		require.Equal(t, textCell("NULL"), readCell(t, f, "S", 1, 2))
		require.Equal(t, textCell("NULL"), readCell(t, f, "S", 2, 2))
		require.Equal(t, numCell("3", ""), readCell(t, f, "S", 2, 3))
	})

	t.Run("array", func(t *testing.T) {
		rows := [2]overviewRow{{"a", 1}, {"b", 2}}
		f := writeOneSheet(t, NewWriter[[2]overviewRow]().WithHeaderRow(true), rows)
		require.Equal(t, textCell("Name"), readCell(t, f, "S", 1, 1))
		require.Equal(t, textCell("b"), readCell(t, f, "S", 1, 3))
		require.Equal(t, numCell("2", ""), readCell(t, f, "S", 2, 3))
	})
}
