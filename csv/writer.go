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
	formatter        retable.TypeFormatters
	writeHeaderRow   bool
	quoteAllFields   bool
	quoteEmptyFields bool
	delimiter        rune
	newLine          string
	charset          retable.Charset
}

func NewWriter() *Writer {
	return &Writer{
		delimiter: ';',
		newLine:   "\r\n",
	}
}

func (w *Writer) WithTypeFormatters(formatter retable.TypeFormatters) *Writer {
	w.formatter = formatter
	return w
}

func (w *Writer) SetTypeFormatter(typ reflect.Type, fmt retable.ValueFormatter) *Writer {
	w.formatter.SetTypeFormatter(typ, fmt)
	return w
}

func (w *Writer) SetInterfaceTypeFormatter(typ reflect.Type, fmt retable.ValueFormatter) *Writer {
	w.formatter.SetInterfaceTypeFormatter(typ, fmt)
	return w
}

func (w *Writer) SetKindFormatter(kind reflect.Kind, fmt retable.ValueFormatter) *Writer {
	w.formatter.SetKindFormatter(kind, fmt)
	return w
}

func (w *Writer) WithWriteHeaderRow(writeHeaderRow bool) *Writer {
	w.writeHeaderRow = writeHeaderRow
	return w
}

func (w *Writer) WithQuoteAllFields(quoteAllFields bool) *Writer {
	w.quoteAllFields = quoteAllFields
	return w
}

func (w *Writer) WithQuoteEmptyFields(quoteEmptyFields bool) *Writer {
	w.quoteEmptyFields = quoteEmptyFields
	return w
}

func (w *Writer) WithDelimiter(delimiter rune) *Writer {
	w.delimiter = delimiter
	return w
}

func (w *Writer) WithNewLine(newLine string) *Writer {
	w.newLine = newLine
	return w
}

func (w *Writer) WithCharset(charset retable.Charset) *Writer {
	w.charset = charset
	return w
}

func (w *Writer) WriteHeaderRow() bool {
	return w.writeHeaderRow
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

func (w *Writer) NewLine() string {
	return w.newLine
}

func (w *Writer) Charset() retable.Charset {
	return w.charset
}

func (w *Writer) Write(ctx context.Context, dest io.Writer, view retable.View) error {
	var (
		rowBuf         = bytes.NewBuffer(make([]byte, 0, 1024))
		mustQuoteChars = "\n\"" + string(w.delimiter)
	)
	if w.writeHeaderRow {
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
	for row := 0; row < view.Rows(); row++ {
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
	for col, val := range rowVals {
		if col > 0 {
			rowBuf.WriteRune(w.delimiter)
		}
		var str string
		if formatter, ok := val.Interface().(Formatter); ok {
			str, err = formatter.FormatCSV(ctx, val, row, col, view)
			if err != nil {
				return err
			}
		} else {
			str, err = w.formatter.FormatValue(ctx, val, row, col, view)
			if err != nil {
				if !errors.Is(err, retable.ErrNotSupported) {
					return err
				}
				str = fmt.Sprint(val.Interface())
			}
		}
		switch {
		case w.quoteAllFields || strings.ContainsAny(str, mustQuoteChars):
			rowBuf.WriteByte('"')
			rowBuf.WriteString(strings.ReplaceAll(str, `"`, `""`))
			rowBuf.WriteByte('"')
		case w.quoteEmptyFields && str == "":
			rowBuf.WriteString(`""`)
		default:
			rowBuf.WriteString(strings.ReplaceAll(str, `"`, `""`))
		}
	}
	rowBuf.WriteString(w.newLine)
	rowBytes := rowBuf.Bytes()
	rowBuf.Reset()
	if w.charset != nil {
		rowBytes, err = w.charset.Encode(rowBytes)
		if err != nil {
			return err
		}
	}
	_, err = dest.Write(rowBytes)
	return err
}
