package retable

import (
	"context"
	"errors"
	"reflect"
)

type ValueFormatter interface {
	FormatValue(ctx context.Context, val reflect.Value, row, col int, view View) (string, error)
}

type ValueFormatterFunc func(ctx context.Context, val reflect.Value, row, col int, view View) (string, error)

func (f ValueFormatterFunc) FormatValue(ctx context.Context, val reflect.Value, row, col int, view View) (string, error) {
	return f(ctx, val, row, col, view)
}

type TypeFormatter struct {
	Types          map[reflect.Type]ValueFormatter
	InterfaceTypes map[reflect.Type]ValueFormatter
	Kinds          map[reflect.Kind]ValueFormatter
}

func (f *TypeFormatter) FormatValue(ctx context.Context, val reflect.Value, row, col int, view View) (string, error) {
	if tw, ok := f.Types[val.Type()]; ok {
		str, err := tw.FormatValue(ctx, val, row, col, view)
		if !errors.Is(err, ErrNotSupported) {
			return str, err
		}
	}
	for it, iw := range f.InterfaceTypes {
		if val.Type().Implements(it) {
			str, err := iw.FormatValue(ctx, val, row, col, view)
			if !errors.Is(err, ErrNotSupported) {
				return str, err
			}
		}
	}
	if kw, ok := f.Kinds[val.Kind()]; ok {
		return kw.FormatValue(ctx, val, row, col, view)
	}
	return "", ErrNotSupported
}
