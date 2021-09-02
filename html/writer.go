package html

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/domonda/go-retable"
)

type Writer struct {
	formatter retable.TypeFormatters
	nilValue  string
}

func NewWriter() *Writer {
	return &Writer{}
}

func (w *Writer) WithTypeFormatters(formatter retable.TypeFormatters) *Writer {
	w.formatter = formatter
	return w
}

func (w *Writer) WithTypeFormatter(typ reflect.Type, fmt retable.ValueFormatter) *Writer {
	w.formatter.SetTypeFormatter(typ, fmt)
	return w
}

func (w *Writer) WithTypeFormatterFunc(typ reflect.Type, fmt retable.ValueFormatterFunc) *Writer {
	w.formatter.SetTypeFormatter(typ, fmt)
	return w
}

func (w *Writer) WithInterfaceTypeFormatter(typ reflect.Type, fmt retable.ValueFormatter) *Writer {
	w.formatter.SetInterfaceTypeFormatter(typ, fmt)
	return w
}

func (w *Writer) WithInterfaceTypeFormatterFunc(typ reflect.Type, fmt retable.ValueFormatterFunc) *Writer {
	w.formatter.SetInterfaceTypeFormatter(typ, fmt)
	return w
}

func (w *Writer) WithKindFormatter(kind reflect.Kind, fmt retable.ValueFormatter) *Writer {
	w.formatter.SetKindFormatter(kind, fmt)
	return w
}

func (w *Writer) WithKindFormatterFunc(kind reflect.Kind, fmt retable.ValueFormatterFunc) *Writer {
	w.formatter.SetKindFormatter(kind, fmt)
	return w
}

func (w *Writer) WithNilValue(nilValue string) *Writer {
	w.nilValue = nilValue
	return w
}

func (w *Writer) NilValue() string {
	return w.nilValue
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
		rowBuf = bytes.NewBuffer(make([]byte, 0, 1024))
	)
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
	if ctx.Err() != nil {
		return ctx.Err()
	}
	cell := retable.ViewCell{
		View: view,
		Row:  row,
	}
	for col, val := range rowVals {
		cell.Col = col

		if formatter, ok := val.Interface().(RawFormatter); ok {
			raw, err := formatter.RawHTML(ctx, &cell)
			if err != nil {
				return err
			}
			rowBuf.WriteString(raw)
		} else {
			str, err := w.formatter.FormatValue(ctx, val, &cell)
			if err != nil {
				if !errors.Is(err, retable.ErrNotSupported) {
					return err
				}
				switch {
				case isNil(val):
					str = w.nilValue
				case val.Kind() == reflect.Ptr:
					str = fmt.Sprint(val.Elem().Interface())
				default:
					str = fmt.Sprint(val.Interface())
				}
			}
			rowBuf.WriteString(str)
		}
	}
	_, err = dest.Write(rowBuf.Bytes())
	rowBuf.Reset()
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
