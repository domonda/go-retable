package retable

import (
	"context"
	"errors"
	"reflect"
)

type ValueFormatter interface {
	FormatValue(ctx context.Context, val reflect.Value, cell *ViewCell) (string, error)
}

type ValueFormatterFunc func(ctx context.Context, val reflect.Value, cell *ViewCell) (string, error)

func (f ValueFormatterFunc) FormatValue(ctx context.Context, val reflect.Value, cell *ViewCell) (string, error) {
	return f(ctx, val, cell)
}

type TypeFormatters struct {
	Types          map[reflect.Type]ValueFormatter
	InterfaceTypes map[reflect.Type]ValueFormatter
	Kinds          map[reflect.Kind]ValueFormatter
}

func (f *TypeFormatters) FormatValue(ctx context.Context, val reflect.Value, cell *ViewCell) (string, error) {
	if tw, ok := f.Types[val.Type()]; ok {
		str, err := tw.FormatValue(ctx, val, cell)
		if !errors.Is(err, ErrNotSupported) {
			return str, err
		}
	}
	for it, iw := range f.InterfaceTypes {
		if val.Type().Implements(it) {
			str, err := iw.FormatValue(ctx, val, cell)
			if !errors.Is(err, ErrNotSupported) {
				return str, err
			}
		}
	}
	if kw, ok := f.Kinds[val.Kind()]; ok {
		return kw.FormatValue(ctx, val, cell)
	}
	return "", ErrNotSupported
}

func (f *TypeFormatters) SetTypeFormatter(typ reflect.Type, fmt ValueFormatter) {
	if f.Types == nil {
		f.Types = make(map[reflect.Type]ValueFormatter)
	}
	f.Types[typ] = fmt
}

func (f *TypeFormatters) SetInterfaceTypeFormatter(typ reflect.Type, fmt ValueFormatter) {
	if f.InterfaceTypes == nil {
		f.InterfaceTypes = make(map[reflect.Type]ValueFormatter)
	}
	f.InterfaceTypes[typ] = fmt
}

func (f *TypeFormatters) SetKindFormatter(kind reflect.Kind, fmt ValueFormatter) {
	if f.Kinds == nil {
		f.Kinds = make(map[reflect.Kind]ValueFormatter)
	}
	f.Kinds[kind] = fmt
}
