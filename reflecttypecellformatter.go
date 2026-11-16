package retable

import (
	"context"
	"errors"
	"reflect"
)

// Ensure that ReflectTypeCellFormatter implements CellFormatter
var _ CellFormatter = new(ReflectTypeCellFormatter)

// ReflectTypeCellFormatter is a type-based routing formatter that selects
// the CellFormatter registered for the reflected type, an implemented interface,
// or the kind of a cell value.
//
// Matching order, see FormatCell for the details:
//  1. Exact type (WithTypeFormatter)
//  2. Implemented interfaces, in registration order (WithInterfaceTypeFormatter)
//  3. Kind (WithKindFormatter)
//  4. For a non-nil pointer: steps 1-3 for the pointed-to type
//  5. For a non-nil interface value: steps 1-4 for its dynamic type
//  6. Default formatter
//
// The formatter returns errors.ErrUnsupported if no matching formatter is found and no
// Default is configured, allowing it to be used in formatter chains.
//
// The lookup of steps 1-4 is a ReflectTypeDispatcher[CellFormatter],
// shared with the exceltable writer.
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
	// Default is the fallback formatter used when no type, interface, or kind matches.
	// If nil and no match is found, errors.ErrUnsupported is returned.
	Default CellFormatter

	// dispatcher holds the type, interface and kind registrations.
	dispatcher *ReflectTypeDispatcher[CellFormatter]
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
//  1. Formatter registered for the exact type
//  2. Formatters of implemented interfaces, in registration order
//  3. Formatter registered for the kind
//  4. If the cell is a non-nil pointer, steps 1-3 for the pointed-to type,
//     called with a DerefView so the formatter sees the pointed-to value
//  5. If the cell is a non-nil interface value (a struct field of interface type),
//     steps 1-4 for its dynamic type, called with a DerefView
//  6. Default formatter if configured, called with the original view
//  7. errors.ErrUnsupported
//
// At each step, if a formatter returns errors.ErrUnsupported, the algorithm continues
// to the next step. Any other error is returned immediately.
// A cell without a value (a nil interface) is reported as errors.ErrUnsupported
// before any formatter runs, so the caller can treat it as null.
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
	if !cellVal.IsValid() {
		// A cell of a nil interface has no type to dispatch on,
		// so let the caller handle it as a null value instead
		// of panicking on the invalid reflect.Value.
		return "", false, errors.ErrUnsupported
	}
	str, raw, err = f.formatCandidates(ctx, view, row, col, cellVal)
	if !errors.Is(err, errors.ErrUnsupported) {
		return str, raw, err
	}
	if cellVal.Kind() == reflect.Interface && !cellVal.IsNil() {
		// A struct field of interface type: nothing matched the
		// interface type itself, so try the dynamic type of the value.
		str, raw, err = f.formatCandidates(ctx, DerefView(view), row, col, cellVal.Elem())
		if !errors.Is(err, errors.ErrUnsupported) {
			return str, raw, err
		}
	}
	if f.Default != nil {
		return f.Default.FormatCell(ctx, view, row, col)
	}
	return "", false, errors.ErrUnsupported
}

// formatCandidates calls the formatters the dispatcher yields for the type
// of cellVal until one handles the cell. Candidates for the pointed-to type
// of a pointer get a DerefView and are skipped for a nil pointer.
func (f *ReflectTypeCellFormatter) formatCandidates(ctx context.Context, view View, row, col int, cellVal reflect.Value) (string, bool, error) {
	var derefView View
	for formatter, deref := range f.dispatcher.Candidates(cellVal.Type()) {
		formatView := view
		if deref {
			if cellVal.IsNil() {
				break
			}
			if derefView == nil {
				derefView = DerefView(view)
			}
			formatView = derefView
		}
		str, raw, err := formatter.FormatCell(ctx, formatView, row, col)
		if !errors.Is(err, errors.ErrUnsupported) {
			return str, raw, err
		}
	}
	return "", false, errors.ErrUnsupported
}

// WithTypeFormatter returns a new ReflectTypeCellFormatter with an exact type formatter added.
// This creates a copy of the formatter, leaving the original unchanged (immutable builder pattern).
//
// The type formatter has the highest priority in the matching hierarchy and will be tried
// before interface, kind, or default formatters.
// A nil fmt removes the formatter registered for typ.
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
	mod.dispatcher = mod.dispatcher.WithType(typ, fmt)
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
// Interfaces are tried in the order they were first registered; registering an
// interface again replaces its formatter and keeps its position.
// A nil fmt removes the formatter registered for typ.
//
// Panics if typ is nil or not an interface type. Like template.Must, this treats
// the mistake as a programming error at configuration time instead of letting
// reflect.Type.Implements panic while a table is written.
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
	mod.dispatcher = mod.dispatcher.WithInterfaceType(typ, fmt)
	return mod
}

// WithKindFormatter returns a new ReflectTypeCellFormatter with a kind formatter added.
// This creates a copy, leaving the original unchanged.
//
// Kind formatters provide broad category matching for types with the same reflect.Kind
// (int, string, struct, etc.). This is checked after exact type and interface matching,
// providing a coarse-grained fallback before the default formatter.
// A nil fmt removes the formatter registered for kind.
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
	mod.dispatcher = mod.dispatcher.WithKind(kind, fmt)
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
	// The dispatcher is immutable, so sharing it is safe.
	c := *f
	return &c
}
