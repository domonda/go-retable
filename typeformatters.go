package retable

import (
	"context"
	"errors"
	"reflect"
)

// Ensure that TypeFormatters implements ValueFormatter
var _ ValueFormatter = new(TypeFormatters)

type TypeFormatters struct {
	Types          map[reflect.Type]ValueFormatter
	InterfaceTypes map[reflect.Type]ValueFormatter
	Kinds          map[reflect.Kind]ValueFormatter
	Other          ValueFormatter
}

func (f *TypeFormatters) FormatValue(ctx context.Context, val reflect.Value, cell *ViewCell) (string, error) {
	if f == nil {
		return "", ErrNotSupported
	}
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
	if f.Other != nil {
		return f.Other.FormatValue(ctx, val, cell)
	}
	return "", ErrNotSupported
}

func (f *TypeFormatters) cloneOrNew() *TypeFormatters {
	if f == nil {
		return new(TypeFormatters)
	}
	c := &TypeFormatters{Other: f.Other}
	if len(f.Types) > 0 {
		c.Types = make(map[reflect.Type]ValueFormatter, len(f.Types))
		for key, val := range f.Types {
			c.Types[key] = val
		}
	}
	if len(f.InterfaceTypes) > 0 {
		c.InterfaceTypes = make(map[reflect.Type]ValueFormatter, len(f.InterfaceTypes))
		for key, val := range f.InterfaceTypes {
			c.InterfaceTypes[key] = val
		}
	}
	if len(f.Kinds) > 0 {
		c.Kinds = make(map[reflect.Kind]ValueFormatter, len(f.Kinds))
		for key, val := range f.Kinds {
			c.Kinds[key] = val
		}
	}
	return c
}

func (f *TypeFormatters) SetTypeFormatter(typ reflect.Type, fmt ValueFormatter) {
	if f.Types == nil {
		f.Types = make(map[reflect.Type]ValueFormatter)
	}
	f.Types[typ] = fmt
}

func (f *TypeFormatters) WithTypeFormatter(typ reflect.Type, fmt ValueFormatter) *TypeFormatters {
	mod := f.cloneOrNew()
	mod.SetTypeFormatter(typ, fmt)
	return mod
}

func (f *TypeFormatters) SetInterfaceTypeFormatter(typ reflect.Type, fmt ValueFormatter) {
	if f.InterfaceTypes == nil {
		f.InterfaceTypes = make(map[reflect.Type]ValueFormatter)
	}
	f.InterfaceTypes[typ] = fmt
}

func (f *TypeFormatters) WithInterfaceTypeFormatter(typ reflect.Type, fmt ValueFormatter) *TypeFormatters {
	mod := f.cloneOrNew()
	mod.SetInterfaceTypeFormatter(typ, fmt)
	return mod
}

func (f *TypeFormatters) SetKindFormatter(kind reflect.Kind, fmt ValueFormatter) {
	if f.Kinds == nil {
		f.Kinds = make(map[reflect.Kind]ValueFormatter)
	}
	f.Kinds[kind] = fmt
}

func (f *TypeFormatters) WithKindFormatter(kind reflect.Kind, fmt ValueFormatter) *TypeFormatters {
	mod := f.cloneOrNew()
	mod.SetKindFormatter(kind, fmt)
	return mod
}
