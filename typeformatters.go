package retable

import (
	"context"
	"errors"
	"reflect"
)

// Ensure that TypeFormatters implements ValueFormatter
var _ CellFormatter = new(TypeFormatters)

type TypeFormatters struct {
	Types          map[reflect.Type]CellFormatter
	InterfaceTypes map[reflect.Type]CellFormatter
	Kinds          map[reflect.Kind]CellFormatter
	Other          CellFormatter
}

func (f *TypeFormatters) FormatCell(ctx context.Context, cell *Cell) (str string, raw bool, err error) {
	if f == nil {
		return "", false, ErrNotSupported
	}
	if tw, ok := f.Types[cell.Value.Type()]; ok {
		str, raw, err := tw.FormatCell(ctx, cell)
		if !errors.Is(err, ErrNotSupported) {
			return str, raw, err
		}
	}
	for it, iw := range f.InterfaceTypes {
		if cell.Value.Type().Implements(it) {
			str, raw, err := iw.FormatCell(ctx, cell)
			if !errors.Is(err, ErrNotSupported) {
				return str, raw, err
			}
		}
	}
	if kw, ok := f.Kinds[cell.Value.Kind()]; ok {
		return kw.FormatCell(ctx, cell)
	}
	if f.Other != nil {
		return f.Other.FormatCell(ctx, cell)
	}
	return "", false, ErrNotSupported
}

func (f *TypeFormatters) cloneOrNew() *TypeFormatters {
	if f == nil {
		return new(TypeFormatters)
	}
	c := &TypeFormatters{Other: f.Other}
	if len(f.Types) > 0 {
		c.Types = make(map[reflect.Type]CellFormatter, len(f.Types))
		for key, val := range f.Types {
			c.Types[key] = val
		}
	}
	if len(f.InterfaceTypes) > 0 {
		c.InterfaceTypes = make(map[reflect.Type]CellFormatter, len(f.InterfaceTypes))
		for key, val := range f.InterfaceTypes {
			c.InterfaceTypes[key] = val
		}
	}
	if len(f.Kinds) > 0 {
		c.Kinds = make(map[reflect.Kind]CellFormatter, len(f.Kinds))
		for key, val := range f.Kinds {
			c.Kinds[key] = val
		}
	}
	return c
}

func (f *TypeFormatters) setTypeFormatter(typ reflect.Type, fmt CellFormatter) {
	if f.Types == nil {
		f.Types = make(map[reflect.Type]CellFormatter)
	}
	f.Types[typ] = fmt
}

func (f *TypeFormatters) WithTypeFormatter(typ reflect.Type, fmt CellFormatter) *TypeFormatters {
	mod := f.cloneOrNew()
	mod.setTypeFormatter(typ, fmt)
	return mod
}

func (f *TypeFormatters) setInterfaceTypeFormatter(typ reflect.Type, fmt CellFormatter) {
	if f.InterfaceTypes == nil {
		f.InterfaceTypes = make(map[reflect.Type]CellFormatter)
	}
	f.InterfaceTypes[typ] = fmt
}

func (f *TypeFormatters) WithInterfaceTypeFormatter(typ reflect.Type, fmt CellFormatter) *TypeFormatters {
	mod := f.cloneOrNew()
	mod.setInterfaceTypeFormatter(typ, fmt)
	return mod
}

func (f *TypeFormatters) setKindFormatter(kind reflect.Kind, fmt CellFormatter) {
	if f.Kinds == nil {
		f.Kinds = make(map[reflect.Kind]CellFormatter)
	}
	f.Kinds[kind] = fmt
}

func (f *TypeFormatters) WithKindFormatter(kind reflect.Kind, fmt CellFormatter) *TypeFormatters {
	mod := f.cloneOrNew()
	mod.setKindFormatter(kind, fmt)
	return mod
}
