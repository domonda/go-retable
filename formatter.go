package retable

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// Formatter is a reflection-based value formatter that converts a reflect.Value to a string.
// This is the simpler, lower-level interface compared to CellFormatter, as it operates
// directly on reflect.Value without requiring a View context or cell coordinates.
//
// Formatter is useful when you need to format individual values that aren't necessarily
// part of a table structure, or when building custom CellFormatter implementations.
//
// Example usage:
//
//	formatter := FormatterFunc(func(v reflect.Value) (string, error) {
//	    if v.Kind() == reflect.Int {
//	        return fmt.Sprintf("#%d", v.Int()), nil
//	    }
//	    return "", errors.ErrUnsupported
//	})
//	str, err := formatter.Format(reflect.ValueOf(42))
//	// str == "#42"
type Formatter interface {
	// Format converts a reflect.Value to its string representation.
	// Returns errors.ErrUnsupported if the formatter doesn't support the value's type.
	Format(reflect.Value) (string, error)
}

// FormatterFunc is a function type that implements the Formatter interface,
// allowing plain functions to be used as Formatters.
//
// This adapter type follows the common Go pattern of defining a function type
// that implements an interface (similar to http.HandlerFunc).
//
// Example:
//
//	var formatter Formatter = FormatterFunc(func(v reflect.Value) (string, error) {
//	    return fmt.Sprintf("value: %v", v.Interface()), nil
//	})
type FormatterFunc func(reflect.Value) (string, error)

// Format implements the Formatter interface by calling the function itself.
func (f FormatterFunc) Format(v reflect.Value) (string, error) {
	return f(v)
}

// SprintFormatter is a universal Formatter that uses fmt.Sprint to format any value.
// This formatter never returns an error and accepts all value types.
//
// It extracts the actual Go value using reflect.Value.Interface() and formats it
// using fmt.Sprint, which uses the value's String() method if available, or falls
// back to the default Go formatting.
//
// Example:
//
//	var f Formatter = SprintFormatter{}
//	str, _ := f.Format(reflect.ValueOf(time.Now()))
//	// str contains the time formatted by time.Time's String() method
type SprintFormatter struct{}

// Format implements Formatter by using fmt.Sprint on the underlying Go value.
// Never returns an error.
func (SprintFormatter) Format(v reflect.Value) (string, error) {
	return fmt.Sprint(v.Interface()), nil
}

// UnsupportedFormatter is a Formatter that always returns errors.ErrUnsupported.
// This is useful as a placeholder or when you want to explicitly mark certain
// types as unsupported in a formatting chain.
//
// Example:
//
//	// Mark a type as unsupported, forcing fallback to another formatter
//	var f Formatter = UnsupportedFormatter{}
//	_, err := f.Format(reflect.ValueOf("anything"))
//	// err == errors.ErrUnsupported
type UnsupportedFormatter struct{}

// Format implements Formatter by always returning errors.ErrUnsupported.
func (UnsupportedFormatter) Format(v reflect.Value) (string, error) {
	return "", errors.ErrUnsupported
}

// CellFormatterFromFormatter adapts a simple Formatter to work as a CellFormatter.
// This bridge function allows reflection-based value formatters to be used in the
// table cell formatting system.
//
// The rawResult parameter determines whether the formatted strings should be marked
// as "raw" (true) or requiring sanitization (false) in the table output format.
//
// This adapter extracts the reflect.Value from the specified cell position and
// passes it to the underlying Formatter.
//
// Parameters:
//   - f: The Formatter to adapt
//   - rawResult: Whether the formatted output is considered raw (doesn't need escaping)
//
// Returns:
//   - A CellFormatter that delegates to the provided Formatter
//
// Example:
//
//	formatter := SprintFormatter{}
//	cellFormatter := CellFormatterFromFormatter(formatter, false)
//	// cellFormatter can now be used in table rendering
func CellFormatterFromFormatter(f Formatter, rawResult bool) CellFormatter {
	return CellFormatterFunc(func(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
		str, err = f.Format(AsReflectCellView(view).ReflectCell(row, col))
		return str, rawResult, err
	})
}

// FormatterFromCellFormatter adapts a CellFormatter to work as a simple Formatter.
// This is the reverse bridge of CellFormatterFromFormatter, allowing table cell
// formatters to be used for formatting individual reflect.Values.
//
// The adapter creates a minimal single-value View containing just the provided
// reflect.Value at position (0, 0) and calls the CellFormatter on it.
// The context used is context.Background() since there's no external context available.
//
// The "raw" result from the CellFormatter is discarded, as the simpler Formatter
// interface doesn't have this concept.
//
// Parameters:
//   - f: The CellFormatter to adapt
//
// Returns:
//   - A Formatter that delegates to the provided CellFormatter
//
// Example:
//
//	cellFormatter := PrintfCellFormatter("%d%%")
//	formatter := FormatterFromCellFormatter(cellFormatter)
//	str, _ := formatter.Format(reflect.ValueOf(95))
//	// str == "95%"
func FormatterFromCellFormatter(f CellFormatter) Formatter {
	return FormatterFunc(func(v reflect.Value) (string, error) {
		str, _, err := f.FormatCell(context.Background(), &SingleReflectValueView{Val: v}, 0, 0)
		return str, err
	})
}
