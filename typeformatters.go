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
	Default        CellFormatter
}

func (f *TypeFormatters) FormatCell(ctx context.Context, cell *Cell) (str string, raw bool, err error) {
	if f == nil {
		return "", false, ErrNotSupported
	}
	if typeFmt, ok := f.Types[cell.Value.Type()]; ok {
		str, raw, err := typeFmt.FormatCell(ctx, cell)
		if !errors.Is(err, ErrNotSupported) {
			return str, raw, err
		}
	}
	for interfaceType, interfaceFmt := range f.InterfaceTypes {
		if cell.Value.Type().Implements(interfaceType) {
			str, raw, err := interfaceFmt.FormatCell(ctx, cell)
			if !errors.Is(err, ErrNotSupported) {
				return str, raw, err
			}
		}
	}
	if kindFmt, ok := f.Kinds[cell.Value.Kind()]; ok {
		return kindFmt.FormatCell(ctx, cell)
	}
	if f.Default != nil {
		return f.Default.FormatCell(ctx, cell)
	}
	return "", false, ErrNotSupported
}

func (f *TypeFormatters) WithTypeFormatter(typ reflect.Type, fmt CellFormatter) *TypeFormatters {
	mod := f.cloneOrNew()
	if mod.Types == nil {
		mod.Types = make(map[reflect.Type]CellFormatter)
	}
	mod.Types[typ] = fmt
	return mod
}

func (f *TypeFormatters) WithTypeFormatterReflectFunc(function interface{}) *TypeFormatters {
	fmt, typ, err := ReflectCellFormatterFunc(function, false)
	if err != nil {
		panic(err)
	}
	return f.WithTypeFormatter(typ, fmt)
}

func (f *TypeFormatters) WithTypeFormatterReflectRawFunc(function interface{}) *TypeFormatters {
	fmt, typ, err := ReflectCellFormatterFunc(function, true)
	if err != nil {
		panic(err)
	}
	return f.WithTypeFormatter(typ, fmt)
}

func (f *TypeFormatters) WithInterfaceTypeFormatter(typ reflect.Type, fmt CellFormatter) *TypeFormatters {
	mod := f.cloneOrNew()
	if mod.InterfaceTypes == nil {
		mod.InterfaceTypes = make(map[reflect.Type]CellFormatter)
	}
	mod.InterfaceTypes[typ] = fmt
	return mod
}

func (f *TypeFormatters) WithKindFormatter(kind reflect.Kind, fmt CellFormatter) *TypeFormatters {
	mod := f.cloneOrNew()
	if mod.Kinds == nil {
		mod.Kinds = make(map[reflect.Kind]CellFormatter)
	}
	mod.Kinds[kind] = fmt
	return mod
}

func (f *TypeFormatters) WithDefaultFormatter(fmt CellFormatter) *TypeFormatters {
	mod := f.cloneOrNew()
	mod.Default = fmt
	return mod
}

func (f *TypeFormatters) cloneOrNew() *TypeFormatters {
	if f == nil {
		return new(TypeFormatters)
	}
	c := &TypeFormatters{Default: f.Default}
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
