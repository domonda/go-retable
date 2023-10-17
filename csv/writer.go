package csv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/domonda/go-retable"
)

type Encoder interface {
	Bytes([]byte) ([]byte, error)
}

type Padding int

const (
	NoPadding Padding = iota
	AlignLeft
	AlignRight
	AlignCenter
)

type Writer[T any] struct {
	viewer           retable.Viewer
	columnFormatters map[int]retable.CellFormatter
	formatters       *retable.TypeFormatters
	padding          Padding
	quoteAllFields   bool
	quoteEmptyFields bool
	escapeQuotes     string
	nilValue         string
	delimiter        rune
	newLine          string
	encoder          Encoder
}

func NewWriter[T any]() *Writer[T] {
	return &Writer[T]{
		columnFormatters: make(map[int]retable.CellFormatter),
		formatters:       nil, // OK to use nil retable.TypeFormatters
		padding:          NoPadding,
		quoteAllFields:   false,
		quoteEmptyFields: false,
		escapeQuotes:     `""`,
		nilValue:         "",
		delimiter:        ';',
		newLine:          "\r\n",
		encoder:          nil,
	}
}

func (w *Writer[T]) clone() *Writer[T] {
	c := new(Writer[T])
	*c = *w
	return c
}

// Write calls WriteView with the result of Viewer.NewView(table)
// using the writer's viewer if not nil or else retable.DefaultViewer.
func (w *Writer[T]) Write(ctx context.Context, dest io.Writer, table T, writeHeaderRow bool) error {
	viewer := w.viewer
	if viewer == nil {
		var err error
		viewer, err = retable.SelectViewer(table)
		if err != nil {
			return err
		}
	}
	return w.WriteWithViewer(ctx, dest, viewer, table, writeHeaderRow)
}

func (w *Writer[T]) WriteWithViewer(ctx context.Context, dest io.Writer, viewer retable.Viewer, table T, writeHeaderRow bool) error {
	view, err := viewer.NewView(table)
	if err != nil {
		return err
	}
	return w.WriteView(ctx, dest, view, writeHeaderRow)
}

func (w *Writer[T]) WriteView(ctx context.Context, dest io.Writer, view retable.View, writeHeaderRow bool) error {
	if w.padding != NoPadding {
		return w.writeViewPadded(ctx, dest, view, writeHeaderRow)
	}

	rowBuf := bytes.NewBuffer(make([]byte, 0, 1024))
	if writeHeaderRow {
		colTitles := view.Columns()
		rowVals := make([]reflect.Value, len(colTitles))
		for col, title := range colTitles {
			rowVals[col] = reflect.ValueOf(title)
		}
		err := w.writeRow(ctx, rowBuf, rowVals, -1, view)
		if err != nil {
			return err
		}
		err = w.writeAndResetBuffer(dest, rowBuf)
		if err != nil {
			return err
		}
	}
	for row := 0; row < view.NumRows(); row++ {
		rowVals, err := view.ReflectRow(row)
		if err != nil {
			return err
		}
		err = w.writeRow(ctx, rowBuf, rowVals, row, view)
		if err != nil {
			return err
		}
		err = w.writeAndResetBuffer(dest, rowBuf)
		if err != nil {
			return err
		}
	}
	return nil
}

// writeAndResetBuffer writes a buffered row with optional encoding
func (w *Writer[T]) writeAndResetBuffer(dest io.Writer, buf *bytes.Buffer) (err error) {
	data := buf.Bytes()
	buf.Reset()

	if w.encoder != nil {
		data, err = w.encoder.Bytes(data)
		if err != nil {
			return err
		}
	}

	_, err = dest.Write(data)
	return err
}

func (w *Writer[T]) writeViewPadded(ctx context.Context, dest io.Writer, view retable.View, writeHeaderRow bool) (err error) {
	var (
		rows    [][]string
		numCols = len(view.Columns())
	)

	if writeHeaderRow {
		// view.Columns() already returns a string slice,
		// but use w.rowStrings() for any potential formatting
		rowVals := make([]reflect.Value, numCols)
		for col, title := range view.Columns() {
			rowVals[col] = reflect.ValueOf(title)
		}
		rowStrs, err := w.rowStrings(ctx, rowVals, -1, view)
		if err != nil {
			return err
		}
		rows = append(rows, rowStrs)
	}

	for row := 0; row < view.NumRows(); row++ {
		rowVals, err := view.ReflectRow(row)
		if err != nil {
			return err
		}
		rowStrs, err := w.rowStrings(ctx, rowVals, row, view)
		if err != nil {
			return err
		}
		rows = append(rows, rowStrs)
	}

	// Collect column widths
	colRuneCount := retable.StringColumnWidths(rows, numCols)

	rowBuf := bytes.NewBuffer(make([]byte, 0, 1024))
	for row := range rows {
		for col, str := range rows[row] {
			if col > 0 {
				_, err = rowBuf.WriteRune(w.delimiter)
				if err != nil {
					return err
				}
			}
			var (
				padTotal = colRuneCount[col] - utf8.RuneCountInString(str)
				padLeft  = 0
				padRight = 0
			)
			switch w.padding {
			case AlignLeft:
				padRight = padTotal
			case AlignRight:
				padLeft = padTotal
			case AlignCenter:
				padLeft = padTotal / 2
				padRight = (padTotal + 1) / 2
			}
			for i := 0; i < padLeft; i++ {
				err = rowBuf.WriteByte(' ')
				if err != nil {
					return err
				}
			}
			_, err = rowBuf.WriteString(str)
			if err != nil {
				return err
			}
			for i := 0; i < padRight; i++ {
				err = rowBuf.WriteByte(' ')
				if err != nil {
					return err
				}
			}
		}
		_, err = rowBuf.WriteString(w.newLine)
		if err != nil {
			return err
		}

		err = w.writeAndResetBuffer(dest, rowBuf)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *Writer[T]) writeRow(ctx context.Context, dest *bytes.Buffer, rowVals []reflect.Value, row int, view retable.View) (err error) {
	// cell will be reused for every column of the row
	cell := retable.Cell{
		View: view,
		Row:  row,
	}
	for col, val := range rowVals {
		cell.Col = col
		cell.Value = val

		if col > 0 {
			_, err = dest.WriteRune(w.delimiter)
			if err != nil {
				return err
			}
		}
		str, err := w.cellString(ctx, &cell)
		if err != nil {
			return err
		}
		_, err = dest.WriteString(str)
		if err != nil {
			return err
		}
	}
	_, err = dest.WriteString(w.newLine)
	return err
}

func (w *Writer[T]) rowStrings(ctx context.Context, rowVals []reflect.Value, row int, view retable.View) (rowStrs []string, err error) {
	rowStrs = make([]string, len(rowVals))

	// cell will be reused for every column of the row
	cell := retable.Cell{
		View: view,
		Row:  row,
	}
	for col, val := range rowVals {
		cell.Col = col
		cell.Value = val

		str, err := w.cellString(ctx, &cell)
		if err != nil {
			return nil, err
		}

		rowStrs[col] = str
	}

	return rowStrs, nil
}

func (w *Writer[T]) cellString(ctx context.Context, cell *retable.Cell) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	if colFormatter, ok := w.columnFormatters[cell.Col]; ok {
		str, isRaw, err := colFormatter.FormatCell(ctx, cell)
		if err == nil {
			return w.escapeStr(str, isRaw), nil
		}
		if !errors.Is(err, errors.ErrUnsupported) {
			return "", err
		}
		// Continue after errors.ErrUnsupported from colFormatter
	}

	str, isRaw, err := w.formatters.FormatCell(ctx, cell)
	if err == nil {
		return w.escapeStr(str, isRaw), nil
	}
	if !errors.Is(err, errors.ErrUnsupported) {
		return "", err
	}

	// In case of errors.ErrUnsupported from w.formatters
	// use fallback methods for formatting
	if retable.ValueIsNil(cell.Value) {
		return w.escapeStr(w.nilValue, false), nil
	}
	v := cell.Value
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	return w.escapeStr(fmt.Sprint(v.Interface()), false), nil
}

func (w *Writer[T]) escapeStr(str string, isRaw bool) string {
	if isRaw {
		return str
	}
	// Just in case remove all \r,
	// \n alone is valid within quotes
	str = strings.ReplaceAll(str, "\r", "")
	switch {
	case w.quoteAllFields || strings.ContainsRune(str, w.delimiter) || strings.ContainsRune(str, '\n'):
		return `"` + strings.ReplaceAll(str, `"`, w.escapeQuotes) + `"`
	case w.quoteEmptyFields && str == "":
		return `""`
	}
	return strings.ReplaceAll(str, `"`, w.escapeQuotes)
}

func (w *Writer[T]) WithTableViewer(viewer retable.Viewer) *Writer[T] {
	mod := w.clone()
	mod.viewer = viewer
	return mod
}

// WithColumnFormatter returns a new writer with the passed formatter registered for columnIndex.
// If nil is passed as formatter, then a previous registered column formatter is removed.
func (w *Writer[T]) WithColumnFormatter(columnIndex int, formatter retable.CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.columnFormatters = make(map[int]retable.CellFormatter)
	for key, val := range w.columnFormatters {
		mod.columnFormatters[key] = val
	}
	if formatter != nil {
		mod.columnFormatters[columnIndex] = formatter
	} else {
		delete(mod.columnFormatters, columnIndex)
	}
	return mod
}

// WithColumnFormatterFunc returns a new writer with the passed formatterFunc registered for columnIndex.
// If nil is passed as formatterFunc, then a previous registered column formatter is removed.
func (w *Writer[T]) WithColumnFormatterFunc(columnIndex int, formatterFunc retable.CellFormatterFunc) *Writer[T] {
	return w.WithColumnFormatter(columnIndex, formatterFunc)
}

func (w *Writer[T]) WithTypeFormatters(formatter *retable.TypeFormatters) *Writer[T] {
	mod := w.clone()
	mod.formatters = formatter
	return mod
}

func (w *Writer[T]) WithTypeFormatter(typ reflect.Type, fmt retable.CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.formatters = w.formatters.WithTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer[T]) WithTypeFormatterFunc(typ reflect.Type, fmt retable.CellFormatterFunc) *Writer[T] {
	mod := w.clone()
	mod.formatters = w.formatters.WithTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer[T]) WithInterfaceTypeFormatter(typ reflect.Type, fmt retable.CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.formatters = w.formatters.WithInterfaceTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer[T]) WithInterfaceTypeFormatterFunc(typ reflect.Type, fmt retable.CellFormatterFunc) *Writer[T] {
	mod := w.clone()
	mod.formatters = w.formatters.WithInterfaceTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer[T]) WithTypeFormatterReflectFunc(function interface{}) *Writer[T] {
	fmt, typ, err := retable.ReflectCellFormatterFunc(function, false)
	if err != nil {
		panic(err)
	}
	return w.WithTypeFormatter(typ, fmt)
}

func (w *Writer[T]) WithTypeFormatterReflectRawFunc(function interface{}) *Writer[T] {
	fmt, typ, err := retable.ReflectCellFormatterFunc(function, true)
	if err != nil {
		panic(err)
	}
	return w.WithTypeFormatter(typ, fmt)
}

func (w *Writer[T]) WithKindFormatter(kind reflect.Kind, fmt retable.CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.formatters = w.formatters.WithKindFormatter(kind, fmt)
	return mod
}

func (w *Writer[T]) WithKindFormatterFunc(kind reflect.Kind, fmt retable.CellFormatterFunc) *Writer[T] {
	mod := w.clone()
	mod.formatters = w.formatters.WithKindFormatter(kind, fmt)
	return mod
}

func (w *Writer[T]) WithPadding(padding Padding) *Writer[T] {
	mod := w.clone()
	mod.padding = padding
	return mod
}

func (w *Writer[T]) WithQuoteAllFields(quoteAllFields bool) *Writer[T] {
	mod := w.clone()
	mod.quoteAllFields = quoteAllFields
	return mod
}

func (w *Writer[T]) WithQuoteEmptyFields(quoteEmptyFields bool) *Writer[T] {
	mod := w.clone()
	mod.quoteEmptyFields = quoteEmptyFields
	return mod
}

func (w *Writer[T]) WithNilValue(nilValue string) *Writer[T] {
	mod := w.clone()
	mod.nilValue = nilValue
	return mod
}

func (w *Writer[T]) WithEscapeQuotes(escapeQuotes string) *Writer[T] {
	mod := w.clone()
	mod.escapeQuotes = escapeQuotes
	return mod
}

func (w *Writer[T]) WithDelimiter(delimiter rune) *Writer[T] {
	mod := w.clone()
	mod.delimiter = delimiter
	return mod
}

func (w *Writer[T]) WithNewLine(newLine string) *Writer[T] {
	mod := w.clone()
	mod.newLine = newLine
	return mod
}

func (w *Writer[T]) WithEncoder(encoder Encoder) *Writer[T] {
	mod := w.clone()
	mod.encoder = encoder
	return mod
}

func (w *Writer[T]) QuoteAllFields() bool {
	return w.quoteAllFields
}

func (w *Writer[T]) QuoteEmptyFields() bool {
	return w.quoteEmptyFields
}

func (w *Writer[T]) Delimiter() rune {
	return w.delimiter
}

func (w *Writer[T]) EscapeQuotes() string {
	return w.escapeQuotes
}

func (w *Writer[T]) NilValue() string {
	return w.nilValue
}

func (w *Writer[T]) NewLine() string {
	return w.newLine
}

func (w *Writer[T]) Encoder() Encoder {
	return w.encoder
}
