package exceltable

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/xuri/excelize/v2"

	"github.com/domonda/go-retable"
)

// ContentType is the MIME type of an .xlsx file.
const ContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

const (
	defaultTimeFormat     = "dd.mm.yyyy hh:mm:ss"
	defaultDurationFormat = "[h]:mm:ss"

	// maxDerefDepth bounds how far the built-in steps unwrap pointers
	// and interfaces, because a self referential value, like an any
	// holding a pointer to itself, would make them loop forever.
	maxDerefDepth = 32
)

var (
	typeOfTime     = reflect.TypeFor[time.Time]()
	typeOfDuration = reflect.TypeFor[time.Duration]()
)

// Workbook is an .xlsx workbook that Writer.WriteSheet and
// Writer.WriteViewSheet add sheets to, one after another.
// Sheets of different row types go into one workbook by using
// one Writer per row type.
//
// A Workbook keeps the first error of a sheet that failed after it was
// added, for example a CellFormatter error or a cancelled context.
// The sheet then holds only part of its rows, so every later WriteSheet,
// WriteViewSheet and WriteTo returns that error and a truncated file can
// never be written. Errors found before a sheet is added, such as
// ErrDuplicateSheetName, leave the workbook usable.
//
// A Workbook is not safe for concurrent use.
// Call Close to remove the temporary files of large sheets.
type Workbook struct {
	file   *excelize.File
	sheets []string
	styles map[styleKey]int
	err    error
	closed bool
}

// styleKey identifies a cell style of a workbook.
type styleKey struct {
	numFmt     string
	header     bool
	fontFamily string
	fontSize   float64
}

// NewWorkbook returns an empty workbook.
func NewWorkbook() *Workbook {
	return &Workbook{
		file:   excelize.NewFile(),
		styles: make(map[styleKey]int),
	}
}

// WriteTo writes the workbook as .xlsx file to dest.
// It can be called more than once and sheets can be added in between.
//
// Returns an error if the workbook has no sheets
// or if a sheet failed while it was written.
func (wb *Workbook) WriteTo(dest io.Writer) (int64, error) {
	if wb.closed {
		return 0, errWorkbookClosed
	}
	if wb.err != nil {
		return 0, wb.poisonedError()
	}
	if len(wb.sheets) == 0 {
		return 0, errors.New("exceltable: workbook has no sheets")
	}
	return wb.file.WriteTo(dest)
}

// Close releases the resources of the workbook,
// including temporary files of large sheets.
// After Close, WriteTo and adding sheets return an error,
// because the released sheet data would make a broken file.
func (wb *Workbook) Close() error {
	wb.closed = true
	return wb.file.Close()
}

// errWorkbookClosed is returned when a closed Workbook is used.
var errWorkbookClosed = errors.New("exceltable: workbook is closed")

func (wb *Workbook) poisonedError() error {
	return fmt.Errorf("exceltable: workbook unusable after a failed sheet: %w", wb.err)
}

// addSheet sanitizes name, checks it against the existing sheet names
// and adds the sheet. The first sheet replaces the default sheet
// that excelize creates.
func (wb *Workbook) addSheet(name string) (string, error) {
	name = sanitizeSheetName(name)
	for _, existing := range wb.sheets {
		// Excel compares sheet names case-insensitively
		if strings.EqualFold(existing, name) {
			return "", fmt.Errorf("%w: %q (existing sheet %q)", ErrDuplicateSheetName, name, existing)
		}
	}
	var err error
	if len(wb.sheets) == 0 {
		err = wb.file.SetSheetName(wb.file.GetSheetName(0), name)
	} else {
		_, err = wb.file.NewSheet(name)
	}
	if err != nil {
		return "", err
	}
	wb.sheets = append(wb.sheets, name)
	return name, nil
}

// style returns the ID of the cell style for key, creating it once per workbook.
func (wb *Workbook) style(key styleKey) (int, error) {
	if id, ok := wb.styles[key]; ok {
		return id, nil
	}
	style := new(excelize.Style)
	if key.numFmt != "" {
		// A copy, so that key does not escape to the heap on cache hits
		numFmt := key.numFmt
		style.CustomNumFmt = &numFmt
	}
	if key.header {
		style.Font = &excelize.Font{
			Bold:   true,
			Family: key.fontFamily,
			Size:   key.fontSize,
		}
	}
	id, err := wb.file.NewStyle(style)
	if err != nil {
		return 0, err
	}
	wb.styles[key] = id
	return id, nil
}

// sanitizeSheetName turns name into a valid Excel sheet name:
// surrounding whitespace is trimmed, an empty name becomes "UNNAMED",
// the characters \ / ? * [ ] : and an apostrophe at the start or end
// are replaced with an underscore, and a name longer than 31 UTF-16
// code units is cut to whole runes within 30 units followed by "…".
func sanitizeSheetName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "UNNAMED"
	}
	name = strings.Map(func(r rune) rune {
		switch r {
		case '\\', '/', '?', '*', '[', ']', ':':
			return '_'
		}
		return r
	}, name)
	if strings.HasPrefix(name, "'") {
		name = "_" + name[1:]
	}
	if strings.HasSuffix(name, "'") {
		name = name[:len(name)-1] + "_"
	}
	// Excel and excelize count the limit in UTF-16 code units
	if utf16Len(name) <= excelize.MaxSheetNameLength {
		return name
	}
	var (
		b     strings.Builder
		units int
	)
	for _, r := range name {
		n := utf16.RuneLen(r)
		if units+n > excelize.MaxSheetNameLength-1 {
			break
		}
		b.WriteRune(r)
		units += n
	}
	b.WriteRune('…')
	return b.String()
}

// utf16Len returns the number of UTF-16 code units of s.
func utf16Len(s string) (n int) {
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// Writer writes tables as sheets of an .xlsx workbook with typed cells:
// numbers stay numbers and dates stay Excel dates, so the sheet can be
// sorted, filtered and summed. It has the shape of csvtable.Writer and
// htmltable.Writer: an immutable builder whose With* methods return a
// modified copy, generic over the table type T.
//
// Each data cell is converted to a Cell by the first step that handles it:
//
//	cell(row, col)
//	  ├─ column formatter (WithColumnFormatter)
//	  ├─ formatters by type: exact type, interfaces in registration order, kind,
//	  │    then the same for the pointed-to type of a non-nil pointer,
//	  │    then for the dynamic type of a non-nil interface value
//	  ├─ time.Time: date serial with the time format; zero time → empty cell;
//	  │    before 1900 or after 9999 → text, see DateTimeCell
//	  ├─ time.Duration: fraction of a day with the duration format;
//	  │    negative → text -H:MM:SS[.fraction], Excel has no negative times
//	  ├─ retable.IsNullLike → the nil value text, empty cell by default
//	  ├─ bool, string, integer and float kinds as Excel bool, string, number
//	  ├─ fmt.Stringer of the value, of its address, of the dereferenced value
//	  ├─ []byte as string
//	  └─ fmt.Sprint of the dereferenced value
//
// A formatter returning errors.ErrUnsupported passes the cell on to the
// next step; any other error aborts the sheet and poisons the workbook.
// The built-in steps from time.Time on look through pointers and
// interfaces, so a *time.Time is written as a date.
// The kinds come before fmt.Stringer, so that a number type with a
// String method, like an amount, stays a number Excel can sum.
// csvtable and htmltable write the String result instead, so an enum
// like time.Month is "March" there and 3 here; register a type or
// kind formatter to write its text.
//
// The header row, written when enabled with WithHeaderRow, holds the
// column names of the view, for a struct slice the titles of
// retable.DefaultStructFieldNaming, as bold text.
//
// Round trip: a sheet written with a header row and read back with
// exceltable.Read and retable.ViewToStructSlice gives the same structs,
// except that times lose their time zone (Excel dates have none),
// and times and durations lose any fraction of a second that their
// format does not show: the default duration format "[h]:mm:ss" rounds
// 1.5s to 2s. Use WithDurationFormat("[h]:mm:ss.000") to keep milliseconds.
// Without a header row, a column whose cells are all empty is dropped
// by the reader and the following columns shift.
//
// A Writer is safe for concurrent use.
type Writer[T any] struct {
	viewer           retable.Viewer
	columnFormatters map[int]CellFormatter
	formatters       *retable.ReflectTypeDispatcher[CellFormatter]
	headerRow        bool
	headerFontFamily string
	headerFontSize   float64
	timeFormat       string
	durationFormat   string
	nilValue         string
}

// NewWriter returns a Writer for tables of type T with the default
// configuration: no header row, time format "dd.mm.yyyy hh:mm:ss",
// duration format "[h]:mm:ss", nil values as empty cells,
// and no formatters.
func NewWriter[T any]() *Writer[T] {
	return &Writer[T]{
		timeFormat:     defaultTimeFormat,
		durationFormat: defaultDurationFormat,
	}
}

// Write writes table as the only sheet of a new workbook to dest.
// The sheet name is sanitized, see WriteSheet.
func (w *Writer[T]) Write(ctx context.Context, dest io.Writer, table T, sheetName string) error {
	view, err := w.newView(table, sheetName)
	if err != nil {
		return err
	}
	return w.WriteView(ctx, dest, view)
}

// WriteView writes view as the only sheet of a new workbook to dest,
// named after the view's title.
func (w *Writer[T]) WriteView(ctx context.Context, dest io.Writer, view retable.View) (err error) {
	wb := NewWorkbook()
	defer func() {
		err = errors.Join(err, wb.Close())
	}()
	err = w.WriteViewSheet(ctx, wb, view)
	if err != nil {
		return err
	}
	_, err = wb.WriteTo(dest)
	return err
}

// WriteSheet adds table as a sheet named sheetName to wb.
//
// The name is sanitized for Excel: surrounding whitespace is trimmed,
// an empty name becomes "UNNAMED", the characters \ / ? * [ ] : and an
// apostrophe at the start or end are replaced with an underscore, and
// a name longer than 31 UTF-16 code units is cut to 30 units plus "…".
// If the sanitized name equals an existing sheet name, ignoring case,
// ErrDuplicateSheetName is returned and wb is left unchanged.
func (w *Writer[T]) WriteSheet(ctx context.Context, wb *Workbook, table T, sheetName string) error {
	view, err := w.newView(table, sheetName)
	if err != nil {
		return err
	}
	return w.WriteViewSheet(ctx, wb, view)
}

// WriteViewSheet adds view as a sheet to wb, named after the view's title.
// See WriteSheet for how the name is sanitized.
func (w *Writer[T]) WriteViewSheet(ctx context.Context, wb *Workbook, view retable.View) error {
	if wb.closed {
		return errWorkbookClosed
	}
	if wb.err != nil {
		return wb.poisonedError()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Everything that can fail before the sheet exists happens first,
	// so that such errors leave no empty sheet behind.
	var headerStyle int
	if w.headerRow {
		var err error
		headerStyle, err = wb.style(styleKey{
			header:     true,
			fontFamily: w.headerFontFamily,
			fontSize:   w.headerFontSize,
		})
		if err != nil {
			return err
		}
	}
	name, err := wb.addSheet(view.Title())
	if err != nil {
		return err
	}
	err = w.writeSheetRows(ctx, wb, name, view, headerStyle)
	if err != nil {
		wb.err = fmt.Errorf("sheet %q: %w", name, err)
		return wb.err
	}
	return nil
}

// writeSheetRows streams the header and data rows of view into the sheet.
func (w *Writer[T]) writeSheetRows(ctx context.Context, wb *Workbook, sheet string, view retable.View, headerStyle int) error {
	stream, err := wb.file.NewStreamWriter(sheet)
	if err != nil {
		return err
	}
	var (
		columns     = view.ColumnNames()
		reflectView = retable.AsReflectCellView(view)
		values      = make([]any, len(columns))
		rowNum      = 1
	)
	if w.headerRow {
		for col, name := range columns {
			values[col] = excelize.Cell{StyleID: headerStyle, Value: name}
		}
		err = setStreamRow(stream, rowNum, values)
		if err != nil {
			return err
		}
		rowNum++
	}
	for row := range view.NumRows() {
		if err := ctx.Err(); err != nil {
			return err
		}
		for col := range columns {
			cell, err := w.cell(ctx, view, reflectView, row, col)
			if err != nil {
				return fmt.Errorf("row %d, column %q: %w", row, columns[col], err)
			}
			values[col], err = w.excelizeValue(wb, cell)
			if err != nil {
				return err
			}
		}
		err = setStreamRow(stream, rowNum, values)
		if err != nil {
			return err
		}
		rowNum++
	}
	return stream.Flush()
}

func setStreamRow(stream *excelize.StreamWriter, rowNum int, values []any) error {
	ref, err := excelize.CoordinatesToCellName(1, rowNum)
	if err != nil {
		return err
	}
	return stream.SetRow(ref, values)
}

// excelizeValue converts cell into a value for excelize.StreamWriter.SetRow.
func (w *Writer[T]) excelizeValue(wb *Workbook, cell Cell) (any, error) {
	value := cell.value
	if f, ok := value.(float64); ok {
		rounded := roundToExcelPrecision(f)
		// Rounding a date serial up must not carry it into the next day,
		// for example 23:59:59.999999 into the following date. Keep the
		// serial, which excelize cuts to 15 digits within the same day.
		if cell.kind != cellDateTime || math.Floor(rounded) == math.Floor(f) {
			value = rounded
		}
	}
	// Only number and date/time cells have a format;
	// an empty number format is Excel's General format
	format := cell.format
	if format == "" && cell.kind == cellDateTime {
		format = w.timeFormat
	}
	if format == "" {
		return value, nil
	}
	style, err := wb.style(styleKey{numFmt: format})
	if err != nil {
		return nil, err
	}
	return excelize.Cell{StyleID: style, Value: value}, nil
}

// cell converts the view cell at row/col into a Cell,
// following the steps documented at Writer.
func (w *Writer[T]) cell(ctx context.Context, view retable.View, reflectView retable.ReflectCellView, row, col int) (Cell, error) {
	if formatter, ok := w.columnFormatters[col]; ok {
		cell, err := formatter.FormatCell(ctx, view, row, col)
		if !errors.Is(err, errors.ErrUnsupported) {
			return cell, err
		}
	}
	val := reflectView.ReflectCell(row, col)
	for formatter, formatView := range w.formatters.CellCandidates(view, val) {
		cell, err := formatter.FormatCell(ctx, formatView, row, col)
		if !errors.Is(err, errors.ErrUnsupported) {
			return cell, err
		}
	}
	return w.builtinCell(val), nil
}

// builtinCell converts val with the built-in steps documented at Writer.
func (w *Writer[T]) builtinCell(val reflect.Value) Cell {
	deref := val
	for range maxDerefDepth {
		if !deref.IsValid() || deref.Kind() != reflect.Pointer && deref.Kind() != reflect.Interface || deref.IsNil() {
			break
		}
		deref = deref.Elem()
	}
	if deref.IsValid() && deref.CanInterface() {
		switch deref.Type() {
		case typeOfTime:
			// DateTimeCell returns an empty cell for the zero time,
			// which is not null in the sense of the nil value.
			// excelizeValue applies the writer's time format.
			return DateTimeCell(deref.Interface().(time.Time), "")
		case typeOfDuration:
			return durationCell(time.Duration(deref.Int()), w.durationFormat)
		}
	}
	if retable.IsNullLike(val) {
		if w.nilValue == "" {
			return Cell{}
		}
		return StringCell(w.nilValue)
	}
	switch deref.Kind() {
	case reflect.Bool:
		return BoolCell(deref.Bool())
	case reflect.String:
		return StringCell(deref.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intCell(deref.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return uintCell(deref.Uint())
	case reflect.Float32:
		// The shortest decimal of the float32, so that 19.99 is not
		// widened to 19.989999771118164 by the conversion to float64
		f, _ := strconv.ParseFloat(strconv.FormatFloat(deref.Float(), 'g', -1, 32), 64)
		return NumberCell(f, "")
	case reflect.Float64:
		return NumberCell(deref.Float(), "")
	}
	if !deref.CanInterface() {
		// Value of an unexported struct field
		return StringCell(fmt.Sprint(deref))
	}
	if val.CanInterface() {
		if s, ok := val.Interface().(fmt.Stringer); ok {
			return StringCell(s.String())
		}
	}
	if val.CanAddr() {
		if s, ok := val.Addr().Interface().(fmt.Stringer); ok {
			return StringCell(s.String())
		}
	}
	x := deref.Interface()
	if s, ok := x.(fmt.Stringer); ok {
		return StringCell(s.String())
	}
	if deref.Kind() == reflect.Slice && deref.Type().Elem().Kind() == reflect.Uint8 {
		return StringCell(string(deref.Bytes()))
	}
	return StringCell(fmt.Sprint(x))
}

// newView creates the view of table with the configured viewer
// or the one retable.SelectViewer selects.
func (w *Writer[T]) newView(table T, title string) (retable.View, error) {
	viewer := w.viewer
	if viewer == nil {
		var err error
		viewer, err = retable.SelectViewer(table)
		if err != nil {
			return nil, err
		}
	}
	return viewer.NewView(title, table)
}

func (w *Writer[T]) clone() *Writer[T] {
	c := *w
	return &c
}

// WithHeaderRow returns a new writer that writes the column names
// as a bold header row when headerRow is true. The default is false.
func (w *Writer[T]) WithHeaderRow(headerRow bool) *Writer[T] {
	mod := w.clone()
	mod.headerRow = headerRow
	return mod
}

// WithHeaderFont returns a new writer that writes the header row in the
// passed font family and size in points, always bold.
// An empty family or a size of 0 keeps the workbook's default.
//
// Example:
//
//	writer := exceltable.NewWriter[[]Row]().
//	    WithHeaderRow(true).
//	    WithHeaderFont("Liberation Sans", 10)
func (w *Writer[T]) WithHeaderFont(family string, size float64) *Writer[T] {
	mod := w.clone()
	mod.headerFontFamily = family
	mod.headerFontSize = size
	return mod
}

// WithTableViewer returns a new writer that uses viewer to create the
// view of a table. If not set, retable.SelectViewer chooses one.
func (w *Writer[T]) WithTableViewer(viewer retable.Viewer) *Writer[T] {
	mod := w.clone()
	mod.viewer = viewer
	return mod
}

// WithColumnFormatter returns a new writer with formatter registered
// for the column with the zero-based columnIndex.
// Column formatters run before all other formatters.
// A nil formatter removes the formatter of the column.
func (w *Writer[T]) WithColumnFormatter(columnIndex int, formatter CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.columnFormatters = maps.Clone(w.columnFormatters)
	// A nil CellFormatterFunc converted to CellFormatter is not a nil
	// interface, but calling it would panic, so it removes too
	if f, isFunc := formatter.(CellFormatterFunc); formatter == nil || isFunc && f == nil {
		delete(mod.columnFormatters, columnIndex)
		return mod
	}
	if mod.columnFormatters == nil {
		mod.columnFormatters = make(map[int]CellFormatter)
	}
	mod.columnFormatters[columnIndex] = formatter
	return mod
}

// WithTypeFormatter returns a new writer with formatter registered for
// cells of the exact type typ and for pointers to it.
// A nil formatter removes the formatter of typ.
//
// Example:
//
//	writer := exceltable.NewWriter[[]Invoice]().
//	    WithTypeFormatter(reflect.TypeFor[money.Amount](), amountCell)
func (w *Writer[T]) WithTypeFormatter(typ reflect.Type, formatter CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.formatters = w.formatters.WithType(typ, formatter)
	return mod
}

// WithInterfaceTypeFormatter returns a new writer with formatter
// registered for cells whose type implements the interface type typ.
// Interfaces are tried in registration order, after exact types.
// A nil formatter removes the formatter of typ.
//
// Panics if typ is nil or not an interface type.
func (w *Writer[T]) WithInterfaceTypeFormatter(typ reflect.Type, formatter CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.formatters = w.formatters.WithInterfaceType(typ, formatter)
	return mod
}

// WithKindFormatter returns a new writer with formatter registered for
// cells of the reflect.Kind kind, tried after exact types and interfaces.
// A nil formatter removes the formatter of kind.
func (w *Writer[T]) WithKindFormatter(kind reflect.Kind, formatter CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.formatters = w.formatters.WithKind(kind, formatter)
	return mod
}

// WithTimeFormat returns a new writer that displays time.Time cells and
// DateTimeCell values without their own format with the passed Excel
// number format code. The default is "dd.mm.yyyy hh:mm:ss".
func (w *Writer[T]) WithTimeFormat(format string) *Writer[T] {
	mod := w.clone()
	mod.timeFormat = format
	return mod
}

// WithDurationFormat returns a new writer that displays time.Duration
// cells, stored as a fraction of a day, with the passed Excel number
// format code. The default "[h]:mm:ss" shows 30 hours as 30:00:00.
func (w *Writer[T]) WithDurationFormat(format string) *Writer[T] {
	mod := w.clone()
	mod.durationFormat = format
	return mod
}

// WithNilValue returns a new writer that writes nilValue as text for
// null-like cells, see retable.IsNullLike. The default "" writes an
// empty cell. A zero time.Time is always an empty cell.
func (w *Writer[T]) WithNilValue(nilValue string) *Writer[T] {
	mod := w.clone()
	mod.nilValue = nilValue
	return mod
}
