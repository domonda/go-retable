package csvtable

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
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

// UTF8BOM is the UTF-8 encoded Unicode byte order mark that
// Writer.WithUTF8BOM writes at the start of the output.
const UTF8BOM = string(bomUTF8)

// ErrUTF8BOMWithEncoder is returned by the write methods of a Writer
// configured with both WithUTF8BOM(true) and a non-nil WithEncoder,
// because the encoder converts the output to another encoding than UTF-8,
// which a UTF-8 byte order mark in front of it would misdeclare.
// Write the byte order mark of the target encoding to the destination
// before calling the writer instead.
var ErrUTF8BOMWithEncoder = errors.New("csvtable: UTF-8 byte order mark can't be combined with an encoder")

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
	utf8BOM          bool
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
// using the writer's viewer if not nil or else retable.SelectViewer.
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
//
// With WithUTF8BOM(true) the output starts with UTF8BOM,
// see WithUTF8BOM for the combination with an encoder.
func (w *Writer[T]) WriteView(ctx context.Context, dest io.Writer, view retable.View) (err error) {
	if w.utf8BOM {
		if w.encoder != nil {
			return ErrUTF8BOMWithEncoder
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// The mark goes out with the first row, so that an error before
		// it, like of a formatter, leaves dest untouched as without the mark,
		// for example an http.ResponseWriter that can still send an error
		bom := &utf8BOMWriter{dest: dest}
		dest = bom
		defer func() {
			if err == nil {
				// A table without rows still starts with the mark
				err = bom.writeBOM()
			}
		}()
	}

	if w.padding != NoPadding {
		return w.writeViewPadded(ctx, dest, view)
	}

	if w.headerRow {
		err := w.headerWriter().writeView(ctx, dest, retable.NewHeaderViewFrom(view))
		if err != nil {
			return err
		}
	}
	return w.writeView(ctx, dest, view)
}

// utf8BOMWriter writes UTF8BOM to dest in front of the first written bytes.
type utf8BOMWriter struct {
	dest    io.Writer
	written bool
}

func (b *utf8BOMWriter) Write(p []byte) (int, error) {
	err := b.writeBOM()
	if err != nil {
		return 0, err
	}
	return b.dest.Write(p)
}

// writeBOM writes UTF8BOM to dest unless it was already written.
func (b *utf8BOMWriter) writeBOM() error {
	if b.written {
		return nil
	}
	b.written = true
	_, err := io.WriteString(b.dest, UTF8BOM)
	return err
}

// headerWriter returns a clone for writing the header row.
// Column formatters format cell values, so they must not be
// applied to the column titles of the header row.
func (w *Writer[T]) headerWriter() *Writer[T] {
	mod := w.clone()
	mod.columnFormatters = nil
	return mod
}

func (w *Writer[T]) writeView(ctx context.Context, dest io.Writer, view retable.View) error {
	reflectView := retable.AsReflectCellView(view)
	rowBuf := bytes.NewBuffer(make([]byte, 0, 1024))
	for row, numRows := 0, view.NumRows(); row < numRows; row++ {
		err := w.writeRow(ctx, rowBuf, view, reflectView, row)
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

func (w *Writer[T]) writeRow(ctx context.Context, rowBuf *bytes.Buffer, view retable.View, reflectView retable.ReflectCellView, row int) error {
	for col := range view.ColumnNames() {
		if col > 0 {
			_, err := rowBuf.WriteRune(w.delimiter)
			if err != nil {
				return err
			}
		}
		str, err := w.cellString(ctx, view, reflectView, row, col)
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
	colRuneCount := retable.StringColumnWidths(rows, view.NumColumns())

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
		numRows     = view.NumRows()
		rows        = make([][]string, 0, numRows+1)
		reflectView = retable.AsReflectCellView(view)
	)
	if w.headerRow {
		// view.ColumnNames() already returns a string slice,
		// but use HeaderView for any potential formatting
		headerView := retable.NewHeaderViewFrom(view)
		rowStrs, err := w.headerWriter().rowStrings(ctx, headerView, retable.AsReflectCellView(headerView), 0)
		if err != nil {
			return nil, err
		}
		rows = append(rows, rowStrs)
	}
	for row := range numRows {
		rowStrs, err := w.rowStrings(ctx, view, reflectView, row)
		if err != nil {
			return nil, err
		}
		rows = append(rows, rowStrs)
	}
	return rows, nil
}

func (w *Writer[T]) rowStrings(ctx context.Context, view retable.View, reflectView retable.ReflectCellView, row int) ([]string, error) {
	columns := view.ColumnNames()
	rowStrs := make([]string, len(columns))
	for col := range columns {
		var err error
		rowStrs[col], err = w.cellString(ctx, view, reflectView, row, col)
		if err != nil {
			return nil, err
		}
	}
	return rowStrs, nil
}

func (w *Writer[T]) cellString(ctx context.Context, view retable.View, reflectView retable.ReflectCellView, row, col int) (string, error) {
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
	v := reflectView.ReflectCell(row, col)
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
	case w.quoteAllFields || strings.ContainsRune(str, w.delimiter) || strings.ContainsRune(str, '\n') || strings.ContainsRune(str, '"'):
		return `"` + strings.ReplaceAll(str, `"`, w.escapeQuotes) + `"`
	case w.quoteEmptyFields && str == "":
		return `""`
	}
	// Any string containing a quote is returned by the first switch case,
	// so there is nothing left to escape here.
	return str
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
	maps.Copy(mod.columnFormatters, w.columnFormatters)
	if isNilFormatter(formatter) {
		delete(mod.columnFormatters, columnIndex)
	} else {
		mod.columnFormatters[columnIndex] = formatter
	}
	return mod
}

// isNilFormatter reports whether formatter is nil or a nil func, like a nil
// retable.CellFormatterFunc, which is a non-nil interface that panics when called.
func isNilFormatter(formatter retable.CellFormatter) bool {
	if formatter == nil {
		return true
	}
	v := reflect.ValueOf(formatter)
	return v.Kind() == reflect.Func && v.IsNil()
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

// WithTypeFormatter returns a new writer with formatter registered for
// cells of the exact type typ and for pointers to it.
// A nil formatter removes the formatter registered for typ.
func (w *Writer[T]) WithTypeFormatter(typ reflect.Type, formatter retable.CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.formatters = w.formatters.WithTypeFormatter(typ, formatter)
	return mod
}

// WithTypeFormatterFunc is WithTypeFormatter for a function.
// A nil formatterFunc removes the formatter registered for typ.
func (w *Writer[T]) WithTypeFormatterFunc(typ reflect.Type, formatterFunc retable.CellFormatterFunc) *Writer[T] {
	return w.WithTypeFormatter(typ, formatterFunc)
}

// WithInterfaceTypeFormatter returns a new writer with formatter registered
// for cells whose type implements the interface type typ. Interfaces are
// tried in registration order, after exact types.
// A nil formatter removes the formatter registered for typ.
//
// Panics if typ is nil or not an interface type.
func (w *Writer[T]) WithInterfaceTypeFormatter(typ reflect.Type, formatter retable.CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.formatters = w.formatters.WithInterfaceTypeFormatter(typ, formatter)
	return mod
}

// WithInterfaceTypeFormatterFunc is WithInterfaceTypeFormatter for a function.
// A nil formatterFunc removes the formatter registered for typ.
//
// Panics if typ is nil or not an interface type.
func (w *Writer[T]) WithInterfaceTypeFormatterFunc(typ reflect.Type, formatterFunc retable.CellFormatterFunc) *Writer[T] {
	return w.WithInterfaceTypeFormatter(typ, formatterFunc)
}

func (w *Writer[T]) WithTypeFormatterReflectFunc(function any) *Writer[T] {
	formatter, typ, err := retable.ReflectCellFormatterFunc(function, false)
	if err != nil {
		panic(err)
	}
	return w.WithTypeFormatter(typ, formatter)
}

func (w *Writer[T]) WithTypeFormatterReflectRawFunc(function any) *Writer[T] {
	formatter, typ, err := retable.ReflectCellFormatterFunc(function, true)
	if err != nil {
		panic(err)
	}
	return w.WithTypeFormatter(typ, formatter)
}

// WithKindFormatter returns a new writer with formatter registered for
// cells of the reflect.Kind kind, tried after exact types and interfaces.
// A nil formatter removes the formatter registered for kind.
func (w *Writer[T]) WithKindFormatter(kind reflect.Kind, formatter retable.CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.formatters = w.formatters.WithKindFormatter(kind, formatter)
	return mod
}

// WithKindFormatterFunc is WithKindFormatter for a function.
// A nil formatterFunc removes the formatter registered for kind.
func (w *Writer[T]) WithKindFormatterFunc(kind reflect.Kind, formatterFunc retable.CellFormatterFunc) *Writer[T] {
	return w.WithKindFormatter(kind, formatterFunc)
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

// WithUTF8BOM returns a new writer that starts the output of Write,
// WriteWithViewer and WriteView with the UTF-8 byte order mark UTF8BOM
// when utf8BOM is true, before the header row, also with padding
// and also for a table without rows. Excel needs it to open a CSV file
// as UTF-8 instead of the legacy code page of the system.
// The default is false. ViewStrings returns cells and is not affected.
//
// The byte order mark is only valid for UTF-8 output, so writing returns
// ErrUTF8BOMWithEncoder without writing anything if an encoder is also
// set with WithEncoder, because it converts the output to another encoding.
//
// Example:
//
//	writer := csvtable.NewWriter[[]Invoice]().
//	    WithHeaderRow(true).
//	    WithUTF8BOM(true)
//	err := writer.Write(ctx, dest, invoices) // "\xEF\xBB\xBFNumber;Total\r\n..."
func (w *Writer[T]) WithUTF8BOM(utf8BOM bool) *Writer[T] {
	mod := w.clone()
	mod.utf8BOM = utf8BOM
	return mod
}

// UTF8BOM reports whether the writer starts its output
// with the UTF-8 byte order mark, see WithUTF8BOM.
func (w *Writer[T]) UTF8BOM() bool {
	return w.utf8BOM
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
