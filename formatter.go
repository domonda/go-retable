package retable

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

type Formatter interface {
	Format(reflect.Value) (string, error)
}

type FormatterFunc func(reflect.Value) (string, error)

func (f FormatterFunc) Format(v reflect.Value) (string, error) {
	return f(v)
}

// SprintFormatter is a Formatter that uses fmt.Sprint to format any value.
type SprintFormatter struct{}

func (SprintFormatter) Format(v reflect.Value) (string, error) {
	return fmt.Sprint(v.Interface()), nil
}

// UnsupportedFormatter is a Formatter that always returns errors.ErrUnsupported.
type UnsupportedFormatter struct{}

func (UnsupportedFormatter) Format(v reflect.Value) (string, error) {
	return "", errors.ErrUnsupported
}

func CellFormatterFromFormatter(f Formatter, rawResult bool) CellFormatter {
	return CellFormatterFunc(func(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
		str, err = f.Format(view.ReflectValue(row, col))
		return str, rawResult, err
	})
}

func FormatterFromCellFormatter(f CellFormatter) Formatter {
	return FormatterFunc(func(v reflect.Value) (string, error) {
		str, _, err := f.FormatCell(context.Background(), &SingleReflectValueView{Val: v}, 0, 0)
		return str, err
	})
}
