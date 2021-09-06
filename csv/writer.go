package csv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/domonda/go-retable"
)

type Encoder interface {
	Bytes([]byte) ([]byte, error)
}

type Writer struct {
	columnFormatters map[int]retable.CellFormatter
	formatters       *retable.TypeFormatters
	fieldPadding     bool
	quoteAllFields   bool
	quoteEmptyFields bool
	escapeQuotes     string
	nilValue         string
	delimiter        rune
	newLine          string
	encoder          Encoder
}

func NewWriter() *Writer {
	return &Writer{
		columnFormatters: make(map[int]retable.CellFormatter),
		formatters:       nil, // OK to use nil retable.TypeFormatters
		fieldPadding:     false,
		quoteAllFields:   false,
		quoteEmptyFields: false,
		escapeQuotes:     `""`,
		nilValue:         "",
		delimiter:        ';',
		newLine:          "\r\n",
		encoder:          nil,
	}
}

func (w *Writer) clone() *Writer {
	c := new(Writer)
	*c = *w
	return c
}

func (w *Writer) WithColumnFormatter(columnIndex int, formatter retable.CellFormatter) *Writer {
	mod := w.clone()
	mod.columnFormatters = make(map[int]retable.CellFormatter)
	for key, val := range w.columnFormatters {
		mod.columnFormatters[key] = val
	}
	mod.columnFormatters[columnIndex] = formatter
	return mod
}

func (w *Writer) WithTypeFormatters(formatter *retable.TypeFormatters) *Writer {
	mod := w.clone()
	mod.formatters = formatter
	return mod
}

func (w *Writer) WithTypeFormatter(typ reflect.Type, fmt retable.CellFormatter) *Writer {
	mod := w.clone()
	mod.formatters = w.formatters.WithTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithTypeFormatterFunc(typ reflect.Type, fmt retable.CellFormatterFunc) *Writer {
	mod := w.clone()
	mod.formatters = w.formatters.WithTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithInterfaceTypeFormatter(typ reflect.Type, fmt retable.CellFormatter) *Writer {
	mod := w.clone()
	mod.formatters = w.formatters.WithInterfaceTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithInterfaceTypeFormatterFunc(typ reflect.Type, fmt retable.CellFormatterFunc) *Writer {
	mod := w.clone()
	mod.formatters = w.formatters.WithInterfaceTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithKindFormatter(kind reflect.Kind, fmt retable.CellFormatter) *Writer {
	mod := w.clone()
	mod.formatters = w.formatters.WithKindFormatter(kind, fmt)
	return mod
}

func (w *Writer) WithKindFormatterFunc(kind reflect.Kind, fmt retable.CellFormatterFunc) *Writer {
	mod := w.clone()
	mod.formatters = w.formatters.WithKindFormatter(kind, fmt)
	return mod
}

func (w *Writer) WithFieldPadding(fieldPadding bool) *Writer {
	mod := w.clone()
	mod.fieldPadding = fieldPadding
	return mod
}

func (w *Writer) WithQuoteAllFields(quoteAllFields bool) *Writer {
	mod := w.clone()
	mod.quoteAllFields = quoteAllFields
	return mod
}

func (w *Writer) WithQuoteEmptyFields(quoteEmptyFields bool) *Writer {
	mod := w.clone()
	mod.quoteEmptyFields = quoteEmptyFields
	return mod
}

func (w *Writer) WithNilValue(nilValue string) *Writer {
	mod := w.clone()
	mod.nilValue = nilValue
	return mod
}

func (w *Writer) WithEscapeQuotes(escapeQuotes string) *Writer {
	mod := w.clone()
	mod.escapeQuotes = escapeQuotes
	return mod
}

func (w *Writer) WithDelimiter(delimiter rune) *Writer {
	mod := w.clone()
	mod.delimiter = delimiter
	return mod
}

func (w *Writer) WithNewLine(newLine string) *Writer {
	mod := w.clone()
	mod.newLine = newLine
	return mod
}

func (w *Writer) WithEncoder(encoder Encoder) *Writer {
	mod := w.clone()
	mod.encoder = encoder
	return mod
}

func (w *Writer) QuoteAllFields() bool {
	return w.quoteAllFields
}

func (w *Writer) QuoteEmptyFields() bool {
	return w.quoteEmptyFields
}

func (w *Writer) Delimiter() rune {
	return w.delimiter
}

func (w *Writer) EscapeQuotes() string {
	return w.escapeQuotes
}

func (w *Writer) NilValue() string {
	return w.nilValue
}

func (w *Writer) NewLine() string {
	return w.newLine
}

func (w *Writer) Encoder() Encoder {
	return w.encoder
}

// Write calls WriteView with the result of retable.DefaultViewer.NewView(table)
func (w *Writer) Write(ctx context.Context, dest io.Writer, table interface{}, writeHeaderRow bool) error {
	view, err := retable.DefaultViewer.NewView(table)
	if err != nil {
		return err
	}
	return w.WriteView(ctx, dest, view, writeHeaderRow)
}

func (w *Writer) WriteView(ctx context.Context, dest io.Writer, view retable.View, writeHeaderRow bool) error {
	rowBuf := bytes.NewBuffer(make([]byte, 0, 1024))
	if writeHeaderRow {
		colTitles := view.Columns()
		rowVals := make([]reflect.Value, len(colTitles))
		for col, title := range colTitles {
			rowVals[col] = reflect.ValueOf(title)
		}
		err := w.writeRow(ctx, dest, rowBuf, rowVals, -1, view)
		if err != nil {
			return err
		}
	}
	for row := 0; row < view.NumRows(); row++ {
		rowVals, err := view.ReflectRow(row)
		if err != nil {
			return err
		}
		err = w.writeRow(ctx, dest, rowBuf, rowVals, row, view)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) writeRow(ctx context.Context, dest io.Writer, rowBuf *bytes.Buffer, rowVals []reflect.Value, row int, view retable.View) (err error) {
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
			return err
		}

		if col > 0 {
			rowBuf.WriteRune(w.delimiter)
		}
		rowBuf.WriteString(str)
	}
	rowBuf.WriteString(w.newLine)

	// Write buffered row with optional encoding
	rowBytes := rowBuf.Bytes()
	if w.encoder != nil {
		rowBytes, err = w.encoder.Bytes(rowBytes)
		if err != nil {
			return err
		}
	}
	_, err = dest.Write(rowBytes)
	rowBuf.Reset()
	return err
}

func (w *Writer) cellString(ctx context.Context, cell *retable.Cell) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	if colFormatter, ok := w.columnFormatters[cell.Col]; ok {
		str, isRaw, err := colFormatter.FormatCell(ctx, cell)
		if err == nil {
			return w.escapeStr(str, isRaw), nil
		}
		if !errors.Is(err, retable.ErrNotSupported) {
			return "", err
		}
		// Continue after retable.ErrNotSupported from colFormatter
	}

	str, isRaw, err := w.formatters.FormatCell(ctx, cell)
	if err == nil {
		return w.escapeStr(str, isRaw), nil
	}
	if !errors.Is(err, retable.ErrNotSupported) {
		return "", err
	}

	// In case of retable.ErrNotSupported from w.formatters
	// use fallback methods for formatting
	if isNil(cell.Value) {
		return w.escapeStr(w.nilValue, false), nil
	}
	v := cell.Value
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return w.escapeStr(fmt.Sprint(v.Interface()), false), nil
}

func (w *Writer) escapeStr(str string, isRaw bool) string {
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

func isNil(val reflect.Value) bool {
	if !val.IsValid() {
		return true
	}
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map,
		reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return val.IsNil()
	}
	return false
}
