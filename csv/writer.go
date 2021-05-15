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
	valueWriters     retable.ValueWriters
	writeHeaderRow   bool
	quoteAllFields   bool
	quoteEmptyFields bool
	delimiter        string
	newLine          string
	charset          retable.Charset
}

func NewWriter() *Writer {
	return &Writer{
		delimiter: ";",
		newLine:   "\r\n",
	}
}

func (w *Writer) Write(ctx context.Context, dest io.Writer, view retable.View) error {
	var (
		rowBuf         = bytes.NewBuffer(make([]byte, 0, 1024))
		mustQuoteChars = "\"\n" + w.delimiter
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
			rowBuf.WriteString(w.delimiter)
		}
		if w.quoteAllFields {
			rowBuf.WriteByte('"')
		}
		start := rowBuf.Len()
		err := w.valueWriters.WriteValue(ctx, rowBuf, val, row, col, view)
		if errors.Is(err, retable.ErrNotSupported) {
			fmt.Fprint(rowBuf, val.Interface())
		} else if err != nil {
			return err
		}
		if w.quoteAllFields {
			rowBuf.WriteByte('"')
		} else if bytes.ContainsAny(rowBuf.Bytes()[start:], mustQuoteChars) {
			valStr := string(rowBuf.Bytes()[start:]) // string forces copy
			valStr = strings.ReplaceAll(valStr, `"`, `""`)
			rowBuf.Truncate(start)
			rowBuf.WriteByte('"')
			rowBuf.WriteString(valStr)
			rowBuf.WriteByte('"')
		} else if w.quoteEmptyFields && rowBuf.Len() == start {
			rowBuf.WriteString(`""`)
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
