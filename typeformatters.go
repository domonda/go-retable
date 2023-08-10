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

func NewTypeFormatters() *TypeFormatters { return new(TypeFormatters) }

func (f *TypeFormatters) FormatCell(ctx context.Context, cell *Cell) (str string, raw bool, err error) {
	if f == nil {
		return "", false, errors.ErrUnsupported
	}
	cellType := cell.Value.Type()
	if typeFmt, ok := f.Types[cellType]; ok {
		str, raw, err := typeFmt.FormatCell(ctx, cell)
		if !errors.Is(err, errors.ErrUnsupported) {
			return str, raw, err
		}
	}
	for interfaceType, interfaceFmt := range f.InterfaceTypes {
		if cellType.Implements(interfaceType) {
			str, raw, err := interfaceFmt.FormatCell(ctx, cell)
			if !errors.Is(err, errors.ErrUnsupported) {
				return str, raw, err
			}
		}
	}
	if kindFmt, ok := f.Kinds[cellType.Kind()]; ok {
		return kindFmt.FormatCell(ctx, cell)
	}
	// If pointer type had no direct formatter
	// check if dereferenced value type has a formatter
	if cellType.Kind() == reflect.Ptr && !cell.Value.IsNil() {
		derefCellType := cellType.Elem()
		if typeFmt, ok := f.Types[derefCellType]; ok {
			str, raw, err := typeFmt.FormatCell(ctx, cell.DerefValue())
			if !errors.Is(err, errors.ErrUnsupported) {
				return str, raw, err
			}
		}
		for interfaceType, interfaceFmt := range f.InterfaceTypes {
			if derefCellType.Implements(interfaceType) {
				str, raw, err := interfaceFmt.FormatCell(ctx, cell.DerefValue())
				if !errors.Is(err, errors.ErrUnsupported) {
					return str, raw, err
				}
			}
		}
		if kindFmt, ok := f.Kinds[derefCellType.Kind()]; ok {
			return kindFmt.FormatCell(ctx, cell.DerefValue())
		}
	}
	if f.Default != nil {
		return f.Default.FormatCell(ctx, cell)
	}
	return "", false, errors.ErrUnsupported
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

func (f *TypeFormatters) WithDefaultFormatterReflectFunc(function interface{}) *TypeFormatters {
	fmt, _, err := ReflectCellFormatterFunc(function, false)
	if err != nil {
		panic(err)
	}
	return f.WithDefaultFormatter(fmt)
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
