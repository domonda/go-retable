package retable

import (
	"cmp"
	"errors"
	"fmt"
	"reflect"
)

// Scanner assigns a string to a destination of any type, dispatching on the
// type of that destination. It is the extension point of SmartAssign for a
// string source and the inverse of Formatter, which converts a reflect.Value
// into a string.
//
// A Scanner does not do the conversions of Go's basic types itself: it decides
// what the string means for the destination it was handed and calls the Parser
// it receives for the number, bool, time and duration conversions. That
// division is what separates the two interfaces. A Parser has a fixed method
// per type and never sees a destination, so it can only answer what a string
// is as an int64 or a time.Time, while a Scanner sees the destination type and
// decides what the string means for it, including that it is an error, which
// is what StrictNilStrings does.
//
// SmartAssign asks the Scanner after its direct conversion, so a destination
// that the source string is convertible to, which are string, a type defined
// as string, []byte and []rune, is assigned the string itself and never
// reaches the Scanner.
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
	// ScanString assigns str to dest, converted to the type of dest.
	//
	// The dest parameter must be settable (obtained from a pointer's Elem()).
	// The scanner should check dest's type and use the appropriate parser method
	// to convert the string.
	//
	// Returns errors.ErrUnsupported if the scanner doesn't support the dest type,
	// allowing scanner chains, see MultiScanner. Any other error is a parsing
	// failure that stops the chain and SmartAssign instead of falling through
	// to the built-in conversions.
	//
	// Parameters:
	//   - dest: The settable reflect.Value to write the parsed value into
	//   - str: The string to parse
	//   - parser: The Parser to use for the conversions of Go's basic types.
	//     SmartAssign defaults it to the shared DefaultParser, which must not
	//     be modified, so a Scanner that reconfigures the Parser it receives
	//     needs the caller to pass one explicitly. ViewToStructSlice allocates
	//     a Parser per view when it has a Scanner but was passed none.
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

// MultiScanner returns a Scanner that calls every passed Scanner in
// order until one of them handles the destination type, which is the
// chain the Scanner documentation refers to when it asks unsupported
// types to be reported as errors.ErrUnsupported.
//
// The first Scanner that does not report errors.ErrUnsupported decides
// the result, so an earlier Scanner overrides a later one, and a real
// parsing error stops the chain instead of falling through to the next
// Scanner. If no Scanner handles the type, errors.ErrUnsupported is
// returned so that SmartAssign continues with its own conversions.
//
// Nil Scanners are ignored, like TryFormattersOrSprint ignores nil
// Formatters, because the Scanner arguments of this package are nil by
// default and are meant to be composed as they are. Passing none at all
// returns a nil Scanner, which those arguments accept as "no Scanner".
//
// Example:
//
//	scanner := retable.MultiScanner(retable.StrictNilStrings, myScanner)
//	rows, err := retable.ViewToStructSlice[Row](view, nil, scanner, nil, nil, nil)
func MultiScanner(scanners ...Scanner) Scanner {
	if len(scanners) == 0 {
		return nil
	}
	return multiScanner(scanners)
}

// multiScanner is the Scanner chain returned by MultiScanner.
type multiScanner []Scanner

// ScanString implements the Scanner interface.
func (s multiScanner) ScanString(dest reflect.Value, str string, parser Parser) error {
	for _, scanner := range s {
		if scanner == nil {
			continue
		}
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
// Combine it with an own Scanner using MultiScanner:
//
//	scanner := retable.MultiScanner(retable.StrictNilStrings, myScanner)
var StrictNilStrings ScannerFunc = func(dest reflect.Value, str string, parser Parser) error {
	// A Scanner is documented as usable on its own, and only SmartAssign
	// substitutes DefaultParser for a nil one, so a direct call has to
	// resolve it here too instead of dereferencing nil.
	parser = cmp.Or(parser, DefaultParser)
	if !parser.IsNil(str) || dest.Kind() == reflect.Pointer || !zeroValueForNilString(dest.Type()) {
		return errors.ErrUnsupported
	}
	return fmt.Errorf("cannot assign %q to %s, use a pointer type for an optional column", str, dest.Type())
}
