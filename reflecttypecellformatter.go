package retable

import (
	"context"
	"errors"
	"reflect"
)

// Ensure that ReflectTypeCellFormatter implements CellFormatter
var _ CellFormatter = new(ReflectTypeCellFormatter)

// ReflectTypeCellFormatter is a sophisticated type-based routing formatter that selects
// the appropriate CellFormatter based on the reflected type, interface, or kind of a cell value.
//
// This formatter implements a hierarchical matching strategy:
//  1. Exact type match (Types map) - highest priority
//  2. Interface type match (InterfaceTypes map) - checks if type implements interface
//  3. Kind match (Kinds map) - matches reflect.Kind (int, string, struct, etc.)
//  4. Pointer dereferencing - if pointer type has no match, checks dereferenced type
//  5. Default formatter - fallback for unmatched types
//
// The formatter returns errors.ErrUnsupported if no matching formatter is found and no
// Default is configured, allowing it to be used in formatter chains.
//
// Design pattern:
// This type uses an immutable builder pattern where all With* methods return a new
// instance, allowing for safe concurrent usage and compositional configuration.
//
// Example usage:
//
//	formatter := NewReflectTypeCellFormatter().
//	    WithTypeFormatter(reflect.TypeOf(time.Time{}), LayoutFormatter("2006-01-02")).
//	    WithKindFormatter(reflect.Int, PrintfCellFormatter("%d")).
//	    WithDefaultFormatter(SprintCellFormatter(false))
//
//	str, raw, err := formatter.FormatCell(ctx, view, 0, 0)
//	// Automatically routes to the correct formatter based on cell type
type ReflectTypeCellFormatter struct {
	// Types maps exact reflect.Type to their CellFormatters.
	// This has the highest priority in the matching hierarchy.
	// Example: reflect.TypeOf(time.Time{}) -> LayoutFormatter("2006-01-02")
	Types map[reflect.Type]CellFormatter

	// InterfaceTypes maps interface types to their CellFormatters.
	// Cell types are checked if they implement these interfaces.
	// Example: reflect.TypeOf((*fmt.Stringer)(nil)).Elem() -> custom formatter
	InterfaceTypes map[reflect.Type]CellFormatter

	// Kinds maps reflect.Kind to their CellFormatters.
	// This provides fallback formatting for broad categories of types.
	// Example: reflect.Int -> PrintfCellFormatter("%d")
	Kinds map[reflect.Kind]CellFormatter

	// Default is the fallback formatter used when no type, interface, or kind matches.
	// If nil and no match is found, errors.ErrUnsupported is returned.
	Default CellFormatter
}

// NewReflectTypeCellFormatter creates a new empty ReflectTypeCellFormatter.
// Use the With* methods to configure type mappings.
//
// Example:
//
//	formatter := NewReflectTypeCellFormatter().
//	    WithTypeFormatter(reflect.TypeOf(time.Time{}), LayoutFormatter("2006-01-02")).
//	    WithKindFormatter(reflect.Float64, PrintfCellFormatter("%.2f"))
func NewReflectTypeCellFormatter() *ReflectTypeCellFormatter {
	return new(ReflectTypeCellFormatter)
}

// FormatCell implements CellFormatter by routing to the appropriate formatter based on
// the cell value's reflected type.
//
// Matching algorithm:
//  1. Check Types map for exact type match
//  2. Check InterfaceTypes map for interface implementations
//  3. Check Kinds map for reflect.Kind match
//  4. If pointer type and not matched, dereference and try steps 1-3 on dereferenced type
//  5. Use Default formatter if configured
//  6. Return errors.ErrUnsupported if no match found
//
// At each step, if a formatter returns errors.ErrUnsupported, the algorithm continues
// to the next step. Any other error is returned immediately.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - view: The table view containing the cell
//   - row: Zero-based row index
//   - col: Zero-based column index
//
// Returns:
//   - str: The formatted string from the matched formatter
//   - raw: Whether the output is raw (from the matched formatter)
//   - err: Error from formatter, or errors.ErrUnsupported if no match
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

// WithTypeFormatter returns a new ReflectTypeCellFormatter with an exact type formatter added.
// This creates a copy of the formatter, leaving the original unchanged (immutable builder pattern).
//
// The type formatter has the highest priority in the matching hierarchy and will be tried
// before interface, kind, or default formatters.
//
// Parameters:
//   - typ: The exact reflect.Type to match
//   - fmt: The CellFormatter to use for this type
//
// Returns:
//   - A new ReflectTypeCellFormatter with the type formatter added
//
// Example:
//
//	// Format time.Time values with custom layout
//	formatter := base.WithTypeFormatter(
//	    reflect.TypeOf(time.Time{}),
//	    LayoutFormatter("2006-01-02"),
//	)
//
//	// Format custom type with specific formatter
//	formatter = formatter.WithTypeFormatter(
//	    reflect.TypeOf(MyCustomType{}),
//	    PrintfCellFormatter("custom: %v"),
//	)
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

// WithInterfaceTypeFormatter returns a new ReflectTypeCellFormatter with an interface
// type formatter added. This creates a copy, leaving the original unchanged.
//
// Interface type formatters match any cell value whose type implements the specified interface.
// This is checked after exact type matching but before kind matching.
//
// Parameters:
//   - typ: The interface type to match (use reflect.TypeOf((*InterfaceName)(nil)).Elem())
//   - fmt: The CellFormatter to use for types implementing this interface
//
// Returns:
//   - A new ReflectTypeCellFormatter with the interface formatter added
//
// Example:
//
//	// Format any type implementing fmt.Stringer with its String() method
//	stringerType := reflect.TypeOf((*fmt.Stringer)(nil)).Elem()
//	formatter := base.WithInterfaceTypeFormatter(
//	    stringerType,
//	    CellFormatterFunc(func(ctx context.Context, view View, row, col int) (string, bool, error) {
//	        return view.Cell(row, col).(fmt.Stringer).String(), false, nil
//	    }),
//	)
//
//	// Format any type implementing a custom interface
//	formatterInterface := reflect.TypeOf((*CustomFormatter)(nil)).Elem()
//	formatter = formatter.WithInterfaceTypeFormatter(formatterInterface, myFormatter)
func (f *ReflectTypeCellFormatter) WithInterfaceTypeFormatter(typ reflect.Type, fmt CellFormatter) *ReflectTypeCellFormatter {
	mod := f.cloneOrNew()
	if mod.InterfaceTypes == nil {
		mod.InterfaceTypes = make(map[reflect.Type]CellFormatter)
	}
	mod.InterfaceTypes[typ] = fmt
	return mod
}

// WithKindFormatter returns a new ReflectTypeCellFormatter with a kind formatter added.
// This creates a copy, leaving the original unchanged.
//
// Kind formatters provide broad category matching for types with the same reflect.Kind
// (int, string, struct, etc.). This is checked after exact type and interface matching,
// providing a coarse-grained fallback before the default formatter.
//
// Parameters:
//   - kind: The reflect.Kind to match (e.g., reflect.Int, reflect.String)
//   - fmt: The CellFormatter to use for this kind
//
// Returns:
//   - A new ReflectTypeCellFormatter with the kind formatter added
//
// Example:
//
//	// Format all integer types with zero-padding
//	formatter := base.WithKindFormatter(
//	    reflect.Int,
//	    PrintfCellFormatter("%05d"),
//	)
//
//	// Format all float types with 2 decimal places
//	formatter = formatter.WithKindFormatter(
//	    reflect.Float64,
//	    PrintfCellFormatter("%.2f"),
//	)
//
//	// Format all struct types with JSON
//	formatter = formatter.WithKindFormatter(
//	    reflect.Struct,
//	    customJSONFormatter,
//	)
func (f *ReflectTypeCellFormatter) WithKindFormatter(kind reflect.Kind, fmt CellFormatter) *ReflectTypeCellFormatter {
	mod := f.cloneOrNew()
	if mod.Kinds == nil {
		mod.Kinds = make(map[reflect.Kind]CellFormatter)
	}
	mod.Kinds[kind] = fmt
	return mod
}

// WithDefaultFormatter returns a new ReflectTypeCellFormatter with a default formatter set.
// This creates a copy, leaving the original unchanged.
//
// The default formatter is used as the ultimate fallback when no type, interface, or kind
// formatters match. If no default is set and no formatters match, errors.ErrUnsupported
// is returned, allowing the formatter to be used in chains.
//
// Parameters:
//   - fmt: The CellFormatter to use as default
//
// Returns:
//   - A new ReflectTypeCellFormatter with the default formatter set
//
// Example:
//
//	// Use fmt.Sprint as fallback for unmatched types
//	formatter := base.WithDefaultFormatter(SprintCellFormatter(false))
//
//	// Use a custom fallback that logs unmatched types
//	formatter = base.WithDefaultFormatter(
//	    CellFormatterFunc(func(ctx context.Context, view View, row, col int) (string, bool, error) {
//	        val := view.Cell(row, col)
//	        log.Printf("unmatched type: %T", val)
//	        return fmt.Sprint(val), false, nil
//	    }),
//	)
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

// cloneOrNew creates a deep copy of the ReflectTypeCellFormatter.
// If f is nil, returns a new empty formatter.
//
// This method implements the immutable builder pattern by creating a copy before
// modifications, ensuring that the original formatter remains unchanged when
// With* methods are called.
//
// Returns:
//   - A new ReflectTypeCellFormatter with copied maps and default formatter
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
