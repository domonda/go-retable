package retable

import (
	"context"
	"errors"
	"reflect"
)

// Ensure that TypeFormatters implements CellFormatter
var _ CellFormatter = new(ReflectTypeCellFormatter)

type ReflectTypeCellFormatter struct {
	Types          map[reflect.Type]CellFormatter
	InterfaceTypes map[reflect.Type]CellFormatter
	Kinds          map[reflect.Kind]CellFormatter
	Default        CellFormatter
}

func NewReflectTypeCellFormatter() *ReflectTypeCellFormatter {
	return new(ReflectTypeCellFormatter)
}

// FormatCell implements CellFormatter
func (f *ReflectTypeCellFormatter) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	if f == nil {
		return "", false, errors.ErrUnsupported
	}
	if err = ctx.Err(); err != nil {
		return "", false, err
	}
	cellVal := AsReflectCellView(view).ReflectCell(row, col)
	cellType := cellVal.Type()
	if typeFmt, ok := f.Types[cellType]; ok {
		str, raw, err := typeFmt.FormatCell(ctx, view, row, col)
		if !errors.Is(err, errors.ErrUnsupported) {
			return str, raw, err
		}
		// Continue after errors.ErrUnsupported
	}
	for interfaceType, interfaceFmt := range f.InterfaceTypes {
		if cellType.Implements(interfaceType) {
			str, raw, err := interfaceFmt.FormatCell(ctx, view, row, col)
			if !errors.Is(err, errors.ErrUnsupported) {
				return str, raw, err
			}
			// Continue after errors.ErrUnsupported
		}
	}
	if kindFmt, ok := f.Kinds[cellType.Kind()]; ok {
		str, raw, err := kindFmt.FormatCell(ctx, view, row, col)
		if !errors.Is(err, errors.ErrUnsupported) {
			return str, raw, err
		}
		// Continue after errors.ErrUnsupported
	}
	// If pointer type had no direct formatter
	// check if dereferenced value type has a formatter
	if cellType.Kind() == reflect.Pointer && !cellVal.IsNil() {
		derefCellType := cellType.Elem()
		if typeFmt, ok := f.Types[derefCellType]; ok {
			str, raw, err := typeFmt.FormatCell(ctx, DerefView(view), row, col)
			if !errors.Is(err, errors.ErrUnsupported) {
				return str, raw, err
			}
			// Continue after errors.ErrUnsupported
		}
		for interfaceType, interfaceFmt := range f.InterfaceTypes {
			if derefCellType.Implements(interfaceType) {
				str, raw, err := interfaceFmt.FormatCell(ctx, DerefView(view), row, col)
				if !errors.Is(err, errors.ErrUnsupported) {
					return str, raw, err
				}
				// Continue after errors.ErrUnsupported
			}
		}
		if kindFmt, ok := f.Kinds[derefCellType.Kind()]; ok {
			str, raw, err := kindFmt.FormatCell(ctx, DerefView(view), row, col)
			if !errors.Is(err, errors.ErrUnsupported) {
				return str, raw, err
			}
			// Continue after errors.ErrUnsupported
		}
	}
	if f.Default != nil {
		return f.Default.FormatCell(ctx, view, row, col)
	}
	return "", false, errors.ErrUnsupported
}

func (f *ReflectTypeCellFormatter) WithTypeFormatter(typ reflect.Type, fmt CellFormatter) *ReflectTypeCellFormatter {
	mod := f.cloneOrNew()
	if mod.Types == nil {
		mod.Types = make(map[reflect.Type]CellFormatter)
	}
	mod.Types[typ] = fmt
	return mod
}

// func (f *TypeFormatters) WithTypeFormatterReflectFunc(function any) *TypeFormatters {
// 	fmt, typ, err := ReflectCellFormatterFunc(function, false)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return f.WithTypeFormatter(typ, fmt)
// }

// func (f *TypeFormatters) WithTypeFormatterReflectRawFunc(function any) *TypeFormatters {
// 	fmt, typ, err := ReflectCellFormatterFunc(function, true)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return f.WithTypeFormatter(typ, fmt)
// }

func (f *ReflectTypeCellFormatter) WithInterfaceTypeFormatter(typ reflect.Type, fmt CellFormatter) *ReflectTypeCellFormatter {
	mod := f.cloneOrNew()
	if mod.InterfaceTypes == nil {
		mod.InterfaceTypes = make(map[reflect.Type]CellFormatter)
	}
	mod.InterfaceTypes[typ] = fmt
	return mod
}

func (f *ReflectTypeCellFormatter) WithKindFormatter(kind reflect.Kind, fmt CellFormatter) *ReflectTypeCellFormatter {
	mod := f.cloneOrNew()
	if mod.Kinds == nil {
		mod.Kinds = make(map[reflect.Kind]CellFormatter)
	}
	mod.Kinds[kind] = fmt
	return mod
}

func (f *ReflectTypeCellFormatter) WithDefaultFormatter(fmt CellFormatter) *ReflectTypeCellFormatter {
	mod := f.cloneOrNew()
	mod.Default = fmt
	return mod
}

// func (f *TypeFormatters) WithDefaultFormatterReflectFunc(function any) *TypeFormatters {
// 	fmt, _, err := ReflectCellFormatterFunc(function, false)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return f.WithDefaultFormatter(fmt)
// }

func (f *ReflectTypeCellFormatter) cloneOrNew() *ReflectTypeCellFormatter {
	if f == nil {
		return new(ReflectTypeCellFormatter)
	}
	c := &ReflectTypeCellFormatter{Default: f.Default}
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
