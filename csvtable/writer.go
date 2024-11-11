package csvtable

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

// Encoder is an interface to encode byte strings.
type Encoder interface {
	Bytes([]byte) ([]byte, error)
}

// EncoderFunc implements the Encoder interface for a function.
type EncoderFunc func([]byte) ([]byte, error)

func (f EncoderFunc) Bytes(data []byte) ([]byte, error) {
	return f(data)
}

// PassthroughEncoder returns an Encoder that returns the passed data unchanged.
func PassthroughEncoder() Encoder {
	return EncoderFunc(func(data []byte) ([]byte, error) {
		return data, nil
	})
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
	formatters       *retable.ReflectTypeCellFormatter
	padding          Padding
	headerRow        bool
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
		headerRow:        false,
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
func (w *Writer[T]) Write(ctx context.Context, dest io.Writer, table T) error {
	viewer := w.viewer
	if viewer == nil {
		var err error
		viewer, err = retable.SelectViewer(table)
		if err != nil {
			return err
		}
	}
	return w.WriteWithViewer(ctx, dest, viewer, table)
}

// WriteWithViewer calls WriteView with the result of viewer.NewView(table).
func (w *Writer[T]) WriteWithViewer(ctx context.Context, dest io.Writer, viewer retable.Viewer, table T) error {
	view, err := viewer.NewView("", table)
	if err != nil {
		return err
	}
	return w.WriteView(ctx, dest, view)
}

// WriteView writes the view to dest as formatted as CSV.
func (w *Writer[T]) WriteView(ctx context.Context, dest io.Writer, view retable.View) error {
	if w.padding != NoPadding {
		return w.writeViewPadded(ctx, dest, view)
	}

	if w.headerRow {
		err := w.writeView(ctx, dest, retable.NewHeaderViewFrom(view))
		if err != nil {
			return err
		}
	}
	return w.writeView(ctx, dest, view)
}

func (w *Writer[T]) writeView(ctx context.Context, dest io.Writer, view retable.View) error {
	rowBuf := bytes.NewBuffer(make([]byte, 0, 1024))
	for row, numRows := 0, view.NumRows(); row < numRows; row++ {
		err := w.writeRow(ctx, rowBuf, view, row)
		if err != nil {
			return err
		}
		_, err = dest.Write(rowBuf.Bytes())
		if err != nil {
			return err
		}
		rowBuf.Reset()
	}
	return nil
}

func (w *Writer[T]) writeRow(ctx context.Context, rowBuf *bytes.Buffer, view retable.View, row int) error {
	for col := range view.Columns() {
		if col > 0 {
			_, err := rowBuf.WriteRune(w.delimiter)
			if err != nil {
				return err
			}
		}
		str, err := w.cellString(ctx, view, row, col)
		if err != nil {
			return err
		}
		_, err = rowBuf.WriteString(str)
		if err != nil {
			return err
		}
	}
	_, err := rowBuf.WriteString(w.newLine)
	if err != nil {
		return err
	}

	if w.encoder == nil {
		return nil
	}

	// Read, encode, and write back the buffered row
	encoded, err := w.encoder.Bytes(rowBuf.Bytes())
	if err != nil {
		return err
	}
	rowBuf.Reset()
	_, err = rowBuf.Write(encoded)
	return err
}

func (w *Writer[T]) writeViewPadded(ctx context.Context, dest io.Writer, view retable.View) error {
	rows, err := w.ViewStrings(ctx, view)
	if err != nil {
		return err
	}

	// Collect column widths
	colRuneCount := retable.StringColumnWidths(rows, len(view.Columns()))

	rowBuf := bytes.NewBuffer(make([]byte, 0, 1024))
	for row := range rows {
		for col, str := range rows[row] {
			if col > 0 {
				_, err := rowBuf.WriteRune(w.delimiter)
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
				err := rowBuf.WriteByte(' ')
				if err != nil {
					return err
				}
			}
			_, err := rowBuf.WriteString(str)
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
		_, err := rowBuf.WriteString(w.newLine)
		if err != nil {
			return err
		}

		if w.encoder != nil {
			// Read, encode, and write back the buffered row
			encoded, err := w.encoder.Bytes(rowBuf.Bytes())
			if err != nil {
				return err
			}
			rowBuf.Reset()
			_, err = rowBuf.Write(encoded)
			if err != nil {
				return err
			}
		}

		_, err = dest.Write(rowBuf.Bytes())
		if err != nil {
			return err
		}
		rowBuf.Reset()
	}

	return nil
}

// ViewStrings returns the view formatted as a slice of string slices.
func (w *Writer[T]) ViewStrings(ctx context.Context, view retable.View) ([][]string, error) {
	var (
		numRows = view.NumRows()
		rows    = make([][]string, 0, numRows+1)
	)
	if w.headerRow {
		// view.Columns() already returns a string slice,
		// but use HeaderView for any potential formatting
		rowStrs, err := w.rowStrings(ctx, retable.NewHeaderViewFrom(view), 0)
		if err != nil {
			return nil, err
		}
		rows = append(rows, rowStrs)
	}
	for row := 0; row < numRows; row++ {
		rowStrs, err := w.rowStrings(ctx, view, row)
		if err != nil {
			return nil, err
		}
		rows = append(rows, rowStrs)
	}
	return rows, nil
}

func (w *Writer[T]) rowStrings(ctx context.Context, view retable.View, row int) ([]string, error) {
	columns := view.Columns()
	rowStrs := make([]string, len(columns))
	for col := range columns {
		var err error
		rowStrs[col], err = w.cellString(ctx, view, row, col)
		if err != nil {
			return nil, err
		}
	}
	return rowStrs, nil
}

func (w *Writer[T]) cellString(ctx context.Context, view retable.View, row, col int) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	if colFormatter, ok := w.columnFormatters[col]; ok {
		str, isRaw, err := colFormatter.FormatCell(ctx, view, row, col)
		if err == nil {
			return w.escapeString(str, isRaw), nil
		}
		if !errors.Is(err, errors.ErrUnsupported) {
			return "", err
		}
		// Continue after errors.ErrUnsupported
	}

	str, isRaw, err := w.formatters.FormatCell(ctx, view, row, col)
	if err == nil {
		return w.escapeString(str, isRaw), nil
	}
	if !errors.Is(err, errors.ErrUnsupported) {
		return "", err
	}
	// Continue after errors.ErrUnsupported

	// Use fallback methods for formatting
	v := retable.AsReflectCellView(view).ReflectCell(row, col)
	if retable.IsNullLike(v) {
		return w.escapeString(w.nilValue, false), nil
	}
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	return w.escapeString(fmt.Sprint(v.Interface()), false), nil
}

func (w *Writer[T]) escapeString(str string, isRaw bool) string {
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

func (w *Writer[T]) WithHeaderRow(headerRow bool) *Writer[T] {
	mod := w.clone()
	mod.headerRow = headerRow
	return mod
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

func (w *Writer[T]) WithTypeFormatters(formatter *retable.ReflectTypeCellFormatter) *Writer[T] {
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

func (w *Writer[T]) WithTypeFormatterReflectFunc(function any) *Writer[T] {
	fmt, typ, err := retable.ReflectCellFormatterFunc(function, false)
	if err != nil {
		panic(err)
	}
	return w.WithTypeFormatter(typ, fmt)
}

func (w *Writer[T]) WithTypeFormatterReflectRawFunc(function any) *Writer[T] {
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
