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
	formatter        retable.TypeFormatter
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
