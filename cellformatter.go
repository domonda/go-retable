package retable

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// CellFormatter is the primary interface for formatting table cell values as strings.
// This is the higher-level interface compared to Formatter, as it operates on cells
// within a table View context and supports format-specific "raw" output.
//
// The CellFormatter design supports a sophisticated formatting pipeline where multiple
// formatters can be tried in sequence until one successfully formats a value. Formatters
// signal their inability to handle a value by returning errors.ErrUnsupported, which
// allows the caller to try alternative formatters.
//
// The "raw" result concept is crucial for output format optimization: when raw is true,
// the string can be used directly in the output format without escaping or sanitization
// (e.g., HTML tags in HTML output, CSV-formatted values in CSV output). When raw is false,
// the string should be sanitized according to the output format (e.g., HTML-escaped).
//
// Design pattern:
//
//	// Try formatters in priority order
//	str, raw, err := formatter1.FormatCell(ctx, view, row, col)
//	if errors.Is(err, errors.ErrUnsupported) {
//	    str, raw, err = formatter2.FormatCell(ctx, view, row, col)
//	}
//	if err != nil {
//	    // handle error or use fallback
//	}
//
// Example usage:
//
//	formatter := PrintfCellFormatter("%.2f")
//	str, raw, err := formatter.FormatCell(ctx, view, 0, 0)
//	if err != nil {
//	    // handle error
//	}
//	if !raw {
//	    str = html.EscapeString(str) // sanitize for HTML output
//	}
type CellFormatter interface {
	// FormatCell formats the view cell at a row/col position as string.
	//
	// Returns errors.ErrUnsupported if this formatter doesn't support the cell's type,
	// signaling that alternative formatters should be tried. Other errors indicate
	// actual formatting failures.
	//
	// The raw result indicates whether the returned string is in the raw format of
	// the target table format and can be used as-is (true), or if it needs to be
	// sanitized according to the output format (false).
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//   - view: The table view containing the cell to format
	//   - row: Zero-based row index
	//   - col: Zero-based column index
	//
	// Returns:
	//   - str: The formatted string representation
	//   - raw: Whether the string is format-specific raw output
	//   - err: Error if formatting failed, or errors.ErrUnsupported if unsupported
	FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error)
}

// CellFormatterFunc is a function type that implements the CellFormatter interface,
// allowing plain functions to be used as CellFormatters.
//
// This adapter type follows the common Go pattern of defining a function type
// that implements an interface (similar to http.HandlerFunc), making it easy to
// create inline formatters without defining separate types.
//
// Example:
//
//	formatter := CellFormatterFunc(func(ctx context.Context, view View, row, col int) (string, bool, error) {
//	    val := view.Cell(row, col)
//	    if num, ok := val.(int); ok {
//	        return fmt.Sprintf("#%05d", num), false, nil
//	    }
//	    return "", false, errors.ErrUnsupported
//	})
type CellFormatterFunc func(ctx context.Context, view View, row, col int) (str string, raw bool, err error)

// FormatCell implements the CellFormatter interface by calling the function itself.
func (f CellFormatterFunc) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	return f(ctx, view, row, col)
}

// PrintfCellFormatter implements CellFormatter by applying a Printf-style format string
// to cell values. The string value of this type is used as the format string for fmt.Sprintf.
//
// This formatter never returns errors.ErrUnsupported - it will format any value type
// that fmt.Sprintf can handle, which includes all basic Go types. The formatted result
// is marked as non-raw (raw=false), meaning it should be sanitized for the output format.
//
// Example usage:
//
//	// Format numbers with 2 decimal places
//	formatter := PrintfCellFormatter("%.2f")
//	str, raw, _ := formatter.FormatCell(ctx, view, 0, 0)
//	// For value 3.14159, str == "3.14", raw == false
//
//	// Format as percentage
//	percentFormatter := PrintfCellFormatter("%.0f%%")
//	str, _, _ := percentFormatter.FormatCell(ctx, view, 0, 0)
//	// For value 95, str == "95%"
type PrintfCellFormatter string

// FormatCell implements CellFormatter using fmt.Sprintf with the format string.
// The formatted result is always marked as non-raw (requiring sanitization).
// Never returns an error.
func (format PrintfCellFormatter) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	return fmt.Sprintf(string(format), view.Cell(row, col)), false, nil
}

// PrintfRawCellFormatter implements CellFormatter by applying a Printf-style format string
// to cell values, similar to PrintfCellFormatter, but marks the result as raw output.
//
// The key difference from PrintfCellFormatter is that this formatter marks its output
// as "raw" (raw=true), indicating the formatted string is already in the correct format
// for the target output and doesn't need sanitization. This is useful when the format
// string produces output that's already escaped or contains format-specific markup.
//
// Example usage:
//
//	// Format as HTML with embedded tags (already properly escaped)
//	formatter := PrintfRawCellFormatter("<b>%s</b>")
//	str, raw, _ := formatter.FormatCell(ctx, view, 0, 0)
//	// For value "Title", str == "<b>Title</b>", raw == true
//	// The raw=true signals that no further HTML escaping is needed
type PrintfRawCellFormatter string

// FormatCell implements CellFormatter using fmt.Sprintf with the format string.
// The formatted result is always marked as raw (not requiring sanitization).
// Never returns an error.
func (format PrintfRawCellFormatter) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	return fmt.Sprintf(string(format), view.Cell(row, col)), true, nil
}

// SprintCellFormatter returns a universal CellFormatter that formats any cell value
// using fmt.Sprint, which uses the value's String() method if available, or falls back
// to Go's default formatting.
//
// This formatter never returns errors.ErrUnsupported and accepts all value types,
// making it an ideal fallback formatter at the end of a formatting chain.
//
// The rawResult parameter determines whether the formatted output should be marked
// as raw (true) or requiring sanitization (false). This allows the caller to control
// the raw flag based on the output format requirements.
//
// Parameters:
//   - rawResult: Whether formatted strings should be marked as raw output
//
// Returns:
//   - A CellFormatter that uses fmt.Sprint for all values
//
// Example usage:
//
//	// Non-raw formatter (strings need sanitization)
//	formatter := SprintCellFormatter(false)
//	str, raw, _ := formatter.FormatCell(ctx, view, 0, 0)
//	// str contains fmt.Sprint output, raw == false
//
//	// Raw formatter (strings are pre-formatted)
//	rawFormatter := SprintCellFormatter(true)
//	str, raw, _ := rawFormatter.FormatCell(ctx, view, 0, 0)
//	// str contains fmt.Sprint output, raw == true
func SprintCellFormatter(rawResult bool) CellFormatter {
	return CellFormatterFunc(func(ctx context.Context, view View, row, col int) (string, bool, error) {
		return fmt.Sprint(view.Cell(row, col)), rawResult, nil
	})
}

// UnsupportedCellFormatter is a CellFormatter that always returns errors.ErrUnsupported.
// This is useful as a placeholder or for explicitly marking certain cell types as
// unsupported in a formatting configuration.
//
// In a formatter chain, this can be used to force trying the next formatter, or
// to document that a particular cell type intentionally has no formatter.
//
// Example usage:
//
//	// Explicitly mark a type as unsupported
//	var formatter CellFormatter = UnsupportedCellFormatter{}
//	_, _, err := formatter.FormatCell(ctx, view, 0, 0)
//	// err == errors.ErrUnsupported
type UnsupportedCellFormatter struct{}

// FormatCell implements CellFormatter by always returning errors.ErrUnsupported.
func (UnsupportedCellFormatter) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	return "", false, errors.ErrUnsupported
}

// TryFormattersOrSprint creates a composite CellFormatter that tries multiple formatters
// in sequence until one succeeds, with fmt.Sprint as the ultimate fallback.
//
// This is the primary function for building flexible formatting chains. It implements
// a "chain of responsibility" pattern where each formatter is tried in order until one
// successfully formats the value (returns no error or a non-ErrUnsupported error).
//
// Behavior:
//   - Tries each formatter in the order provided
//   - Continues to next formatter only if current returns errors.ErrUnsupported
//   - Returns immediately on success or non-ErrUnsupported errors
//   - Falls back to fmt.Sprint if all formatters return ErrUnsupported
//   - Returns empty string for nil values
//   - Ignores nil formatters in the list
//   - Fallback results are always marked as non-raw (raw=false)
//
// This function is essential for type-safe formatting where you want to try specialized
// formatters first and fall back to generic formatting if the value type doesn't match.
//
// Parameters:
//   - formatters: Variable number of CellFormatters to try in order
//
// Returns:
//   - A composite CellFormatter that implements the fallback chain
//
// Example usage:
//
//	formatter := TryFormattersOrSprint(
//	    PrintfCellFormatter("%.2f"),      // Try formatting as float with 2 decimals
//	    LayoutFormatter("2006-01-02"),    // Try formatting as date
//	    // Falls back to fmt.Sprint for other types
//	)
//	str, raw, err := formatter.FormatCell(ctx, view, 0, 0)
//	// Uses first matching formatter, or fmt.Sprint if none match
func TryFormattersOrSprint(formatters ...CellFormatter) CellFormatter {
	return CellFormatterFunc(func(ctx context.Context, view View, row, col int) (string, bool, error) {
		for _, f := range formatters {
			if f == nil {
				continue
			}
			str, raw, err := f.FormatCell(ctx, view, row, col)
			if !errors.Is(err, errors.ErrUnsupported) {
				return str, raw, err
			}
		}

		// Fallback for no formatters passed or when
		// all formatters returned errors.ErrUnsupported
		v := AsReflectCellView(view).ReflectCell(row, col)
		if IsNullLike(v) {
			return "", false, nil
		}
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		return fmt.Sprint(v.Interface()), false, nil
	})
}

// RawCellString is a constant string CellFormatter that ignores the cell value entirely
// and always returns its own string value, marked as raw output.
//
// This formatter is useful for injecting fixed content (like HTML snippets, icons, or
// format-specific markup) into table cells regardless of the actual cell value. Since
// the output is marked as raw, it won't be sanitized by the output format.
//
// Example usage:
//
//	// Always show an HTML checkbox, regardless of cell value
//	checkboxFormatter := RawCellString(`<input type="checkbox" checked>`)
//	str, raw, _ := checkboxFormatter.FormatCell(ctx, view, 0, 0)
//	// str == `<input type="checkbox" checked>`, raw == true
//
//	// Use as a constant marker
//	marker := RawCellString("✓")
type RawCellString string

// FormatCell implements CellFormatter by returning the constant string as raw output.
// The cell value is ignored. Never returns an error.
func (rawStr RawCellString) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	return string(rawStr), true, nil
}

// LayoutFormatter formats values that implement the Format(string) string method,
// such as time.Time, by calling their Format method with the layout string.
//
// This formatter is specifically designed for types like time.Time that use layout
// strings for formatting. The string value of this type becomes the layout parameter
// passed to the value's Format method.
//
// If the cell value doesn't implement the required interface, an error is returned
// (not errors.ErrUnsupported, but a descriptive error about the type mismatch).
//
// Example usage:
//
//	// Format time.Time values as ISO dates
//	formatter := LayoutFormatter("2006-01-02")
//	str, raw, err := formatter.FormatCell(ctx, view, 0, 0)
//	// For time.Time value of "2024-03-15 14:30:00"
//	// str == "2024-03-15", raw == false
//
//	// Format as 12-hour time
//	timeFormatter := LayoutFormatter("3:04 PM")
//	// For time.Time value of "2024-03-15 14:30:00"
//	// str == "2:30 PM"
type LayoutFormatter string

// FormatCell implements CellFormatter by calling the cell value's Format method
// with the layout string. Returns an error if the value doesn't implement
// interface{ Format(string) string }.
func (f LayoutFormatter) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	formatter, ok := view.Cell(row, col).(interface{ Format(string) string })
	if !ok {
		return "", false, fmt.Errorf("%T does not implement interface{ Format(string) string }", view.Cell(row, col))
	}
	return formatter.Format(string(f)), false, nil
}

// StringIfTrue formats boolean cells by returning a specified string for true values
// and an empty string for false values. The output is marked as non-raw.
//
// This formatter is useful for creating checkmarks, labels, or indicators that only
// appear when a condition is true. It panics if the cell value is not a bool.
//
// Example usage:
//
//	// Show checkmark for true, nothing for false
//	formatter := StringIfTrue("✓")
//	str, raw, _ := formatter.FormatCell(ctx, view, 0, 0)
//	// If cell value is true: str == "✓", raw == false
//	// If cell value is false: str == "", raw == false
//
//	// Show "Active" for true, nothing for false
//	statusFormatter := StringIfTrue("Active")
type StringIfTrue string

// FormatCell implements CellFormatter for boolean values.
// Returns the string value for true, empty string for false.
// Panics if the cell value is not a bool.
// Never returns an error.
func (f StringIfTrue) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	if view.Cell(row, col).(bool) {
		return string(f), false, nil
	}
	return "", false, nil
}

// RawStringIfTrue formats boolean cells by returning a specified string for true values
// and an empty string for false values, with the output marked as raw.
//
// This is similar to StringIfTrue, but marks the output as raw, indicating it doesn't
// need sanitization. This is useful when the string contains format-specific markup
// like HTML tags or pre-escaped content. It panics if the cell value is not a bool.
//
// Example usage:
//
//	// Show HTML icon for true, nothing for false
//	formatter := RawStringIfTrue(`<span class="icon-check"></span>`)
//	str, raw, _ := formatter.FormatCell(ctx, view, 0, 0)
//	// If cell value is true: str == `<span class="icon-check"></span>`, raw == true
//	// If cell value is false: str == "", raw == true
type RawStringIfTrue string

// FormatCell implements CellFormatter for boolean values with raw output.
// Returns the string value marked as raw for true, empty string for false.
// Panics if the cell value is not a bool.
// Never returns an error.
func (f RawStringIfTrue) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	if view.Cell(row, col).(bool) {
		return string(f), true, nil
	}
	return "", true, nil
}

// ReflectCellFormatterFunc converts an arbitrary function into a CellFormatterFunc using
// reflection, enabling type-safe cell formatting with minimal boilerplate.
//
// This powerful function allows you to write formatters as simple, strongly-typed functions
// that work with specific value types, and automatically adapts them to work with the
// generic CellFormatter interface. This provides both type safety and ergonomic API.
//
// Function signature requirements:
//
// Arguments (0-2 parameters):
//   - Optional first parameter: context.Context (if present, must be first)
//   - Optional value parameter: Any type (becomes the valType return value)
//
// Results (1-2 return values):
//   - First result: Must be string (the formatted output)
//   - Optional second result: Must be error (formatting error)
//
// The rawResult parameter determines the raw flag returned by the generated formatter.
//
// Parameters:
//   - function: The function to convert (validated via reflection)
//   - rawResult: Whether the formatter should mark output as raw
//
// Returns:
//   - formatter: The generated CellFormatterFunc
//   - valType: The reflect.Type of the value parameter (for type registration)
//   - err: Error if function signature is invalid
//
// Example usage:
//
//	// Simple type-safe formatter
//	formatter, typ, err := ReflectCellFormatterFunc(
//	    func(t time.Time) string {
//	        return t.Format("2006-01-02")
//	    },
//	    false,
//	)
//	// typ == reflect.TypeOf(time.Time{})
//	// formatter is a CellFormatterFunc that only works with time.Time
//
//	// With context and error handling
//	formatter, typ, err := ReflectCellFormatterFunc(
//	    func(ctx context.Context, val CustomType) (string, error) {
//	        if err := ctx.Err(); err != nil {
//	            return "", err
//	        }
//	        return val.Format(), nil
//	    },
//	    false,
//	)
//
//	// No arguments (formats any value the same way)
//	formatter, _, err := ReflectCellFormatterFunc(
//	    func() string { return "constant" },
//	    true,
//	)
func ReflectCellFormatterFunc(function any, rawResult bool) (formatter CellFormatterFunc, valType reflect.Type, err error) {
	// Check if function is really a function
	fv := reflect.ValueOf(function)
	if !fv.IsValid() {
		return nil, nil, errors.New("nil function")
	}
	ft := fv.Type()
	if ft.Kind() != reflect.Func {
		return nil, nil, fmt.Errorf("expected function but got %T", function)
	}

	// Check results of function
	if ft.NumOut() == 0 {
		return nil, nil, errors.New("function needs result")
	}
	if ft.NumOut() > 2 {
		return nil, nil, errors.New("function must not have more than 2 results")
	}
	if ft.Out(0).Kind() != reflect.String {
		return nil, nil, fmt.Errorf("function result must be a string kind, but is %s", ft.Out(0))
	}
	errIndex := -1
	if ft.NumOut() == 2 {
		if ft.Out(1) != typeOfError {
			return nil, nil, fmt.Errorf("second function result must be error, but is %s", ft.Out(1))
		}
		errIndex = 1
	}

	// Check arguments of function
	var (
		ctxIndex = -1
		valIndex = -1
	)
	for i := range ft.NumIn() {
		switch ft.In(i) {
		case typeOfContext:
			if ctxIndex != -1 {
				return nil, nil, errors.New("second context.Context argument not allowed")
			}
			ctxIndex = i
		default:
			if valIndex != -1 {
				return nil, nil, errors.New("too many arguments")
			}
			valIndex = i
			valType = ft.In(i)
		}
	}

	formatter = func(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
		if err = ctx.Err(); err != nil {
			return "", false, err
		}
		args := make([]reflect.Value, ft.NumIn())
		if ctxIndex != -1 {
			args[ctxIndex] = reflect.ValueOf(ctx)
		}
		if valIndex != -1 {
			args[valIndex] = AsReflectCellView(view).ReflectCell(row, col)
		}
		res := fv.Call(args)
		if errIndex != -1 && !res[errIndex].IsNil() {
			return "", false, res[errIndex].Interface().(error)
		}
		return res[0].String(), rawResult, nil
	}

	return formatter, valType, nil
}
