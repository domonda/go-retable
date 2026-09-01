package retable

import (
	"errors"
	"fmt"
	"reflect"
)

// Scanner is the interface for parsing string values into Go values using reflection.
// This is the inverse operation of formatting - it converts string representations
// back into typed Go values.
//
// Scanners work in conjunction with Parsers to handle the actual string-to-value conversion.
// The Scanner is responsible for orchestrating the conversion, while the Parser provides
// the primitive parsing operations (int, float, bool, time, etc.).
//
// The Scanner interface operates at a lower level than the table system, working directly
// with reflect.Value. This makes it reusable across different contexts beyond just table
// cell parsing.
//
// Design pattern:
// Scanners should check the dest type and call the appropriate Parser method to convert
// the string into the target type. For complex types, scanners may need to perform
// additional logic (e.g., splitting strings, handling null values, type conversions).
//
// Example usage:
//
//	scanner := ScannerFunc(func(dest reflect.Value, str string, parser Parser) error {
//	    if dest.Kind() == reflect.Int {
//	        i, err := parser.ParseInt(str)
//	        if err != nil {
//	            return err
//	        }
//	        dest.SetInt(i)
//	        return nil
//	    }
//	    return errors.ErrUnsupported
//	})
//
//	var result int
//	destValue := reflect.ValueOf(&result).Elem()
//	err := scanner.ScanString(destValue, "42", parser)
//	// result == 42
type Scanner interface {
	// ScanString parses a string value into the destination reflect.Value.
	//
	// The dest parameter must be settable (obtained from a pointer's Elem()).
	// The scanner should check dest's type and use the appropriate parser method
	// to convert the string.
	//
	// Returns errors.ErrUnsupported if the scanner doesn't support the dest type,
	// allowing scanner chains. Other errors indicate actual parsing failures.
	//
	// Parameters:
	//   - dest: The settable reflect.Value to write the parsed value into
	//   - str: The string to parse
	//   - parser: The Parser to use for primitive type conversions
	//
	// Returns:
	//   - error: Parsing error, or errors.ErrUnsupported if type not supported
	ScanString(dest reflect.Value, str string, parser Parser) error
}

// ScannerFunc is a function type that implements the Scanner interface,
// allowing plain functions to be used as Scanners.
//
// This adapter type follows the common Go pattern of defining a function type
// that implements an interface (similar to http.HandlerFunc), making it easy to
// create inline scanners without defining separate types.
//
// Example:
//
//	var scanner Scanner = ScannerFunc(func(dest reflect.Value, str string, parser Parser) error {
//	    if dest.Type() == reflect.TypeOf(time.Time{}) {
//	        t, err := parser.ParseTime(str)
//	        if err != nil {
//	            return err
//	        }
//	        dest.Set(reflect.ValueOf(t))
//	        return nil
//	    }
//	    return errors.ErrUnsupported
//	})
type ScannerFunc func(dest reflect.Value, str string, parser Parser) error

// ScanString implements the Scanner interface by calling the function itself.
func (f ScannerFunc) ScanString(dest reflect.Value, str string, parser Parser) error {
	return f(dest, str, parser)
}

// Scanners calls every Scanner in order until one of them handles the
// destination type, which is the chain the Scanner documentation refers
// to when it asks unsupported types to be reported as
// errors.ErrUnsupported.
//
// The first Scanner that does not report errors.ErrUnsupported decides
// the result, so an earlier Scanner overrides a later one, and a real
// parsing error stops the chain instead of falling through to the next
// Scanner. If no Scanner handles the type, errors.ErrUnsupported is
// returned so that SmartAssign continues with its own conversions.
//
// Example:
//
//	scanner := retable.Scanners{retable.StrictNilStrings, myScanner}
//	rows, err := retable.ViewToStructSlice[Row](view, nil, scanner, nil, nil, nil)
type Scanners []Scanner

// ScanString implements the Scanner interface.
func (s Scanners) ScanString(dest reflect.Value, str string, parser Parser) error {
	for _, scanner := range s {
		err := scanner.ScanString(dest, str, parser)
		if !errors.Is(err, errors.ErrUnsupported) {
			return err // nil or a real parsing error
		}
	}
	return errors.ErrUnsupported
}

// StrictNilStrings is a Scanner that reports an error for a source
// string that means no value, which the passed Parser classifies with
// its IsNil method, when it is assigned to a destination type that has
// no way to represent the absence of a value, which are the numeric
// types, bool, time.Time and time.Duration.
//
// Without it SmartAssign assigns the zero value to those, because an
// empty cell of a CSV file or a spreadsheet usually means "no value"
// and reading one must not fail a whole file. The cost is that the
// parsed data cannot tell an empty cell from a cell containing 0, and
// that a struct field wired to the wrong column keeps parsing every
// empty cell and only fails on the first non-empty one.
//
// Pass this Scanner to require that an optional column is declared as a
// pointer type instead. Pointer destinations are left to SmartAssign,
// which assigns nil for an empty string, so the absence stays visible
// in the parsed data and the type states which columns are optional.
//
// Combine it with an own Scanner using Scanners:
//
//	scanner := retable.Scanners{retable.StrictNilStrings, myScanner}
var StrictNilStrings ScannerFunc = func(dest reflect.Value, str string, parser Parser) error {
	if !parser.IsNil(str) || dest.Kind() == reflect.Pointer || !zeroValueForNilString(dest.Type()) {
		return errors.ErrUnsupported
	}
	return fmt.Errorf("cannot assign %q to %s, use a pointer type for an optional column", str, dest.Type())
}
