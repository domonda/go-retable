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

type Writer struct {
	formatter        *retable.TypeFormatters
	quoteAllFields   bool
	quoteEmptyFields bool
	escapeQuotes     string
	nilValue         string
	delimiter        rune
	newLine          string
	encoder          TextTransformer
}

func NewWriter() *Writer {
	return &Writer{
		formatter:        nil, // OK to use nil retable.TypeFormatters
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

func (w *Writer) WithTypeFormatters(formatter *retable.TypeFormatters) *Writer {
	mod := w.clone()
	mod.formatter = formatter
	return mod
}

func (w *Writer) WithTypeFormatter(typ reflect.Type, fmt retable.ValueFormatter) *Writer {
	mod := w.clone()
	mod.formatter = w.formatter.WithTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithTypeFormatterFunc(typ reflect.Type, fmt retable.ValueFormatterFunc) *Writer {
	mod := w.clone()
	mod.formatter = w.formatter.WithTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithInterfaceTypeFormatter(typ reflect.Type, fmt retable.ValueFormatter) *Writer {
	mod := w.clone()
	mod.formatter = w.formatter.WithInterfaceTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithInterfaceTypeFormatterFunc(typ reflect.Type, fmt retable.ValueFormatterFunc) *Writer {
	mod := w.clone()
	mod.formatter = w.formatter.WithInterfaceTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithKindFormatter(kind reflect.Kind, fmt retable.ValueFormatter) *Writer {
	mod := w.clone()
	mod.formatter = w.formatter.WithKindFormatter(kind, fmt)
	return mod
}

func (w *Writer) WithKindFormatterFunc(kind reflect.Kind, fmt retable.ValueFormatterFunc) *Writer {
	mod := w.clone()
	mod.formatter = w.formatter.WithKindFormatter(kind, fmt)
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

func (w *Writer) WithEncoder(encoder TextTransformer) *Writer {
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

func (w *Writer) Encoder() TextTransformer {
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
	var (
		rowBuf         = bytes.NewBuffer(make([]byte, 0, 1024))
		mustQuoteChars = "\n" + string(w.delimiter)
	)
	if writeHeaderRow {
		colTitles := view.Columns()
		rowVals := make([]reflect.Value, len(colTitles))
		for col, title := range colTitles {
			rowVals[col] = reflect.ValueOf(title)
		}
		err := w.writeRow(ctx, dest, rowBuf, rowVals, -1, view, mustQuoteChars)
		if err != nil {
			return err
		}
	}
	for row := 0; row < view.NumRows(); row++ {
		rowVals, err := view.ReflectRow(row)
		if err != nil {
			return err
		}
		err = w.writeRow(ctx, dest, rowBuf, rowVals, row, view, mustQuoteChars)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) writeRow(ctx context.Context, dest io.Writer, rowBuf *bytes.Buffer, rowVals []reflect.Value, row int, view retable.View, mustQuoteChars string) (err error) {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	rowBuf.Reset()
	// cell will be reused for every column of the row
	// with cell.Col set to the column index
	cell := retable.ViewCell{
		View: view,
		Row:  row,
	}
	for col, val := range rowVals {
		cell.Col = col
		if col > 0 {
			rowBuf.WriteRune(w.delimiter)
		}

		// Use RawFormatter if implemented by val
		rawFormatter, _ := val.Interface().(RawFormatter)
		if rawFormatter == nil && val.CanAddr() {
			rawFormatter, _ = val.Addr().Interface().(RawFormatter)
		}
		if rawFormatter != nil {
			raw, err := rawFormatter.RawCSV(ctx, &cell)
			if err != nil {
				return err
			}
			rowBuf.WriteString(raw)
			continue
		}

		// No RawFormatter, try retable.TypeFormatters
		str, err := w.formatter.FormatValue(ctx, val, &cell)
		if err != nil {
			if !errors.Is(err, retable.ErrNotSupported) {
				return err
			}
			// In case of retable.ErrNotSupported
			// fall back on nilValue or fmt.Sprint
			switch {
			case isNil(val):
				str = w.nilValue
			case val.Kind() == reflect.Ptr:
				str = fmt.Sprint(val.Elem().Interface())
			default:
				str = fmt.Sprint(val.Interface())
			}
		}
		// Just in case remove all \r,
		// \n alone is valid within quotes
		str = strings.ReplaceAll(str, "\r", "")
		switch {
		case w.quoteAllFields || strings.ContainsAny(str, mustQuoteChars):
			rowBuf.WriteByte('"')
			rowBuf.WriteString(strings.ReplaceAll(str, `"`, w.escapeQuotes))
			rowBuf.WriteByte('"')
		case w.quoteEmptyFields && str == "":
			rowBuf.WriteString(`""`)
		default:
			rowBuf.WriteString(strings.ReplaceAll(str, `"`, w.escapeQuotes))
		}
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
	return err
}

func isNil(val reflect.Value) bool {
	switch val.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return val.IsNil()
	default:
		return false
	}
}
