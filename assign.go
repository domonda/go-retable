package retable

import (
	"cmp"
	"encoding"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
)

// SmartAssign performs intelligent type conversion when assigning src to dst.
// It attempts multiple conversion strategies in order of preference, making it
// suitable for converting between different types in data mapping scenarios.
//
// Type Conversion Strategies (in order):
//
//  1. Null handling: If src implements IsNull() bool and returns true,
//     dst is set to its zero value.
//
//  2. Direct conversion: If src type is convertible to dst type using
//     reflect.Value.Convert, the conversion is performed directly.
//     Integer sources are excluded for string destinations because
//     reflect.Value.Convert applies Go's string(rune) conversion, which
//     would yield the character with that code point instead of digits.
//
//  3. Nil pointer handling: If src is a nil pointer, dst is set to its zero value.
//
//  4. Custom formatting: If dst is a string type and srcFormatter is provided,
//     srcFormatter.Format is used to convert src to string.
//
//  5. Custom scanning: If src is a string type and dstScanner is provided,
//     dstScanner.ScanString is used to parse src into dst, using the
//     passed parser for the primitive conversions.
//
//  6. Nil string handling: If parser.IsNil reports the source string to
//     represent no value, which the default StringParser does for the
//     empty string of an empty cell and for "NULL", "null", "nil" and
//     "<nil>", dst is set to its zero value, so a pointer destination
//     becomes nil. This only applies to the destinations that the
//     strategies below would parse the string into, which are the numeric
//     kinds, bool, time.Time, types defined as one of those, and pointers
//     to any of them. Destinations that can hold the string itself have
//     already been assigned by the direct conversion above, so a *string
//     keeps a pointer to the string rather than becoming nil. Any other
//     destination cannot hold a string of any content, so a nil one stays
//     a type mismatch and is reported as an error instead of being
//     silently turned into a zero value.
//     Pass the StrictNilStrings Scanner to report an error instead of
//     assigning the zero value to a type that cannot represent absence.
//
//  7. TextMarshaler: If src implements encoding.TextMarshaler, its MarshalText
//     method is used to get a text representation for further conversion.
//
//  8. Stringer: If src implements fmt.Stringer, its String method is used
//     to get a string representation for further conversion.
//
//  9. Time parsing: If src is a string and dst is time.Time or *time.Time,
//     parser.ParseTime is used to convert the string to a time value.
//     If dst is time.Duration or *time.Duration, parser.ParseDuration is used,
//     falling back to the integer parsing below for a plain number
//     of nanoseconds without a unit.
//     Types defined as time.Time, like "type Date time.Time", are
//     included, and so is every defined int64 type for the duration
//     case, because reflection cannot tell one defined as time.Duration
//     apart from any other, see timeType and durationType.
//
//  10. Pointer dereferencing: If src is a non-nil pointer, SmartAssign is
//     recursively called with the dereferenced value.
//
//  11. Empty struct handling: If src is an empty struct (struct{}), dst is
//     set to its zero value.
//
//  12. Boolean conversions:
//     - bool to numeric types: true becomes 1, false becomes 0
//     - bool to string: "true" or "false"
//     - numeric types to bool: non-zero becomes true, zero becomes false
//     - string to bool: parsed using parser.ParseBool
//
//  13. String to numeric conversions:
//     - String to int/uint: parsed using parser.ParseInt/ParseUint
//     - String to float: parsed using parser.ParseFloat
//
//  14. Fallback string conversion: Any type can be converted to string
//     using fmt.Sprint as a last resort.
//
//  15. Pointer allocation: If dst is a pointer type and previous strategies
//     failed, a new instance is created and SmartAssign is recursively
//     called to assign to the dereferenced pointer.
//
// Parameters:
//   - dst: The destination reflect.Value to assign to. Must be valid and settable.
//   - src: The source reflect.Value to assign from. Must be valid.
//   - dstScanner: Optional Scanner for custom string-to-type conversions (can be nil).
//   - parser: Parser used for all string conversions and passed to
//     dstScanner.ScanString. Can be nil, in which case the shared
//     DefaultParser is used. Pass one explicitly to configure parsing,
//     or when a Scanner reconfigures the Parser it receives, because
//     DefaultParser must not be modified.
//   - srcFormatter: Optional Formatter for custom type-to-string conversions (can be nil).
//
// Returns:
//   - error: nil on success, or an error describing why the assignment failed.
//     Returns errors.ErrUnsupported if no conversion strategy could handle
//     the type combination.
//
// Example:
//
//	// Convert string to int
//	var result int
//	dst := reflect.ValueOf(&result).Elem()
//	src := reflect.ValueOf("42")
//	err := SmartAssign(dst, src, nil, nil, nil)
//	// result == 42
//
//	// Convert bool to string
//	var str string
//	dst = reflect.ValueOf(&str).Elem()
//	src = reflect.ValueOf(true)
//	err = SmartAssign(dst, src, nil, nil, nil)
//	// str == "true"
//
//	// Convert with custom formatter
//	formatter := FormatterFunc(func(v reflect.Value) (string, error) {
//	    return fmt.Sprintf("#%v", v.Interface()), nil
//	})
//	var output string
//	dst = reflect.ValueOf(&output).Elem()
//	src = reflect.ValueOf(42)
//	err = SmartAssign(dst, src, nil, nil, formatter)
//	// output == "#42"
func SmartAssign(dst, src reflect.Value, dstScanner Scanner, parser Parser, srcFormatter Formatter) (err error) {
	if !dst.IsValid() {
		return errors.New("dst value is invalid")
	}
	if !dst.CanSet() {
		return errors.New("cannot set dst value")
	}
	if !src.IsValid() {
		return errors.New("src value is invalid")
	}
	parser = cmp.Or(parser, DefaultParser)
	var (
		srcType = src.Type()
		srcKind = srcType.Kind()
		dstType = dst.Type()
		dstKind = dstType.Kind()
	)

	// Package reflect might panic in some edge cases
	// like converting a slice to an array with non matching length.
	// Recover and return as error instead to make code more robust.
	defer func() {
		if r := recover(); r != nil {
			err = errors.Join(err, fmt.Errorf("%+v", r))
		}
	}()

	// Interface allocates for an addressable src, so call it
	// once here instead of again for every type assertion below.
	// It stays after the deferred recover because it panics
	// for a value obtained from an unexported field.
	srcAny := src.Interface()

	// Assign zero value in case of IsNull.
	// Conversions further down might assign something
	// different than the zero value dependent on the
	// underlying type.
	if nullable, ok := srcAny.(interface{ IsNull() bool }); ok && nullable.IsNull() {
		dst.Set(reflect.Zero(dstType))
		return nil
	}

	// Convert assigns directly if possible.
	// Integer sources are excluded for string destinations because
	// reflect.Value.Convert applies Go's string(rune) conversion,
	// which yields the character with that code point ("*" for 42)
	// instead of the decimal digits. Those fall through to
	// srcFormatter, Stringer, or the fmt.Sprint fallback below.
	if srcType.ConvertibleTo(dstType) && !(dstKind == reflect.String && integerKind(srcKind)) {
		// Check because converting a slice to a longer array panics.
		// The destination can be the array itself or a pointer to it.
		if srcKind == reflect.Slice {
			arrayType := dstType
			if dstKind == reflect.Pointer {
				arrayType = dstType.Elem()
			}
			if arrayType.Kind() == reflect.Array && arrayType.Len() > src.Len() {
				return fmt.Errorf("cannot convert slice of length %d to array of length %d", src.Len(), arrayType.Len())
			}
		}
		dst.Set(src.Convert(dstType))
		return nil
	}

	// Assign zero value in case of a nil pointer
	if srcKind == reflect.Pointer && src.IsNil() {
		dst.Set(reflect.Zero(dstType))
		return nil
	}

	// Try formatStr if dst is a string type
	if dstKind == reflect.String && srcFormatter != nil {
		str, err := srcFormatter.Format(src)
		if err == nil {
			dst.SetString(str)
			return nil
		}
		if !errors.Is(err, errors.ErrUnsupported) {
			return err
		}
		// Continue after errors.ErrUnsupported
	}

	// Try dstScanner if src is a string type.
	// Mirrors the srcFormatter case above: srcFormatter
	// formats a value into a string destination, dstScanner
	// parses a string source into any other destination.
	if srcKind == reflect.String && dstScanner != nil {
		err := dstScanner.ScanString(dst, src.String(), parser)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errors.ErrUnsupported) {
			return err
		}
		// Continue after errors.ErrUnsupported
	}

	// Assign the zero value for a source string that means no value.
	// An empty cell of a CSV file or a spreadsheet means "no value"
	// and must not be an error for a numeric, boolean or time
	// destination, the same way a null source assigns the zero value
	// above. Which strings mean that is configured on the parser,
	// because only the source format knows whether a cell reading
	// "NULL" is a null value or that text.
	// Without this the conversions further down would fail to parse
	// the string and the value would fall through to the unsupported
	// operation error.
	// Destinations that can hold the string itself have already been
	// assigned by the direct conversion above, and dstScanner has been
	// asked first so that a Scanner can give it a different meaning,
	// which is what StrictNilStrings does.
	if srcKind == reflect.String && parser.IsNil(src.String()) && zeroValueForNilString(dstType) {
		dst.Set(reflect.Zero(dstType))
		return nil
	}

	// Try assigning string from MarshalText method
	if m, ok := srcAny.(encoding.TextMarshaler); ok {
		txt, err := m.MarshalText()
		if err != nil {
			return err
		}
		err = SmartAssign(dst, reflect.ValueOf(string(txt)), dstScanner, parser, srcFormatter)
		if !errors.Is(err, errors.ErrUnsupported) {
			return err // nil or other than errors.ErrUnsupported
		}
		// Continue after errors.ErrUnsupported
	}

	// Try assigning string from String method
	if m, ok := srcAny.(fmt.Stringer); ok {
		err = SmartAssign(dst, reflect.ValueOf(m.String()), dstScanner, parser, srcFormatter)
		if !errors.Is(err, errors.ErrUnsupported) {
			return err // nil or other than errors.ErrUnsupported
		}
		// Continue after errors.ErrUnsupported
	}

	// Try converting string to time.Time
	if srcKind == reflect.String && (timeType(dstType) || dstKind == reflect.Pointer && timeType(dstType.Elem())) {
		if t, err := parser.ParseTime(src.String()); err == nil {
			setParsed(dst, reflect.ValueOf(t))
			return nil
		}
	}

	// Try converting string to time.Duration.
	// Without this the underlying int64 kind of time.Duration
	// would only accept a plain number of nanoseconds.
	if srcKind == reflect.String && (durationType(dstType) || dstKind == reflect.Pointer && durationType(dstType.Elem())) {
		if d, err := parser.ParseDuration(src.String()); err == nil {
			setParsed(dst, reflect.ValueOf(d))
			return nil
		}
		// Continue to the integer parsing further down
		// for a plain number without a unit
	}

	// Try assigning the dereferenced value
	if srcKind == reflect.Pointer && !src.IsNil() {
		err := SmartAssign(dst, src.Elem(), dstScanner, parser, srcFormatter)
		if !errors.Is(err, errors.ErrUnsupported) {
			return err // nil or other than errors.ErrUnsupported
		}
		// Continue after errors.ErrUnsupported
	}

	// A pure empty struct represents the zero value
	if srcType == typeOfEmptyStruct {
		dst.Set(reflect.Zero(dstType))
		return nil
	}

	// Convert bool to 0 / 1 numbers, or "true" / "false" strings
	if srcKind == reflect.Bool {
		switch dstKind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if src.Bool() {
				dst.SetInt(1)
			} else {
				dst.SetInt(0)
			}
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			if src.Bool() {
				dst.SetUint(1)
			} else {
				dst.SetUint(0)
			}
			return nil
		case reflect.Float32, reflect.Float64:
			if src.Bool() {
				dst.SetFloat(1)
			} else {
				dst.SetFloat(0)
			}
			return nil
		case reflect.String:
			dst.SetString(strconv.FormatBool(src.Bool()))
			return nil
		}
	}

	switch dstKind {

	// Convert 0 / 1 numbers, or "true" / "false" strings to bool
	case reflect.Bool:
		switch srcKind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			dst.SetBool(src.Int() != 0)
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			dst.SetBool(src.Uint() != 0)
			return nil
		case reflect.Float32, reflect.Float64:
			dst.SetBool(src.Float() != 0)
			return nil
		case reflect.String:
			b, err := parser.ParseBool(src.String())
			if err == nil {
				dst.SetBool(b)
				return nil
			}
		}

	// Convert string to integers.
	// The Parser always parses 64 bits, while reflect.Value.SetInt
	// silently truncates to the width of the destination, so a value
	// that does not fit has to be reported instead of assigned as a
	// different number: "300" into an int8 would be 44.
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if srcKind == reflect.String {
			if i, e := parser.ParseInt(src.String()); e == nil {
				if dst.OverflowInt(i) {
					return fmt.Errorf("%d overflows %s", i, dstType)
				}
				dst.SetInt(i)
				return nil
			}
		}

	// Convert string to unsigned integers, see the overflow note above
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if srcKind == reflect.String {
			if i, e := parser.ParseUint(src.String()); e == nil {
				if dst.OverflowUint(i) {
					return fmt.Errorf("%d overflows %s", i, dstType)
				}
				dst.SetUint(i)
				return nil
			}
		}

	case reflect.Float32, reflect.Float64:
		if srcKind == reflect.String {
			if f, e := parser.ParseFloat(src.String()); e == nil {
				// A finite value that does not fit becomes an infinity,
				// which is a different number, so it is reported.
				// An infinity that was parsed as such is assigned,
				// because it represents exactly what the source said.
				if !math.IsInf(f, 0) && dst.OverflowFloat(f) {
					return fmt.Errorf("%v overflows %s", f, dstType)
				}
				dst.SetFloat(f)
				return nil
			}
		}

	// Convert any type to string with fmt.Sprint
	case reflect.String:
		dst.SetString(fmt.Sprint(srcAny))
		return nil

	// If all other failed and dest is a pointer,
	// try to create a new instance and assign to that
	// then assign the pointer to the new instance.
	case reflect.Pointer:
		if _, ok := derefPointerType(dstType); !ok {
			// A self referential pointer type never reaches a type to
			// allocate, so it is reported as unsupported below instead
			// of recursing until the stack overflows.
			break
		}
		newDest := reflect.New(dstType.Elem())
		err = SmartAssign(newDest.Elem(), src, dstScanner, parser, srcFormatter)
		if err != nil && !errors.Is(err, errors.ErrUnsupported) {
			return err
		}
		if err == nil {
			dst.Set(newDest)
			return nil
		}
		// Continue after errors.ErrUnsupported

	}

	return fmt.Errorf("%w: assigning %s %#v to %s", errors.ErrUnsupported, srcType, src, dstType)
}

// maxPointerDepth bounds how far a pointer type is followed to the
// type it finally points to.
//
// A self referential pointer type is legal Go and its element type is
// itself, so "type SelfPtr *SelfPtr" makes an unbounded walk spin
// forever and makes the recursion of the pointer case in SmartAssign
// overflow the stack, which is a fatal error that no deferred recover
// can catch. No real destination nests pointers this deep.
const maxPointerDepth = 32

// derefPointerType follows pointer types to the first type that is not
// a pointer. ok is false when the chain did not end within
// maxPointerDepth, which only a self referential type can do.
func derefPointerType(t reflect.Type) (elem reflect.Type, ok bool) {
	for range maxPointerDepth {
		if t.Kind() != reflect.Pointer {
			return t, true
		}
		t = t.Elem()
	}
	return t, false
}

// timeType reports whether t is time.Time or a type defined as it,
// like "type Date time.Time", which parses from the same strings
// because it has the same underlying type.
//
// Only such a type is convertible to time.Time, because a conversion
// between struct types needs identical underlying types and the
// unexported fields of time.Time can only be named by package time.
// The kind is checked first because time.Time is also convertible to
// every interface type it implements, like fmt.Stringer.
func timeType(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && typeOfTime.ConvertibleTo(t)
}

// durationType reports whether t is time.Duration or a type defined
// as it, like "type Timeout time.Duration", which parses from the
// same strings because it has the same underlying type.
//
// A defined type only keeps the underlying type of time.Duration,
// which is int64, so reflection cannot tell "type Timeout
// time.Duration" from "type Bytes int64". Every defined int64 type is
// therefore accepted, which costs a "5m" cell of a Bytes column being
// read as 5 minutes in nanoseconds instead of being reported as an
// error. The predeclared int64 has no package path and is excluded,
// so a plain int64 destination keeps rejecting duration strings.
// Both types still parse a plain number of nanoseconds, because
// ParseDuration rejects a number without a unit and the integer
// parsing of SmartAssign handles it.
func durationType(t reflect.Type) bool {
	return t.Kind() == typeOfDuration.Kind() && t.PkgPath() != ""
}

// setParsed assigns the parsed value val to dst, converting it to the
// destination type, which is val's type, a type defined as it, or a
// pointer to one of those. A pointer destination is allocated here
// instead of by the pointer strategy of SmartAssign, which would
// re-parse the string for the allocated value.
func setParsed(dst, val reflect.Value) {
	dstType := dst.Type()
	if dstType.Kind() != reflect.Pointer {
		dst.Set(val.Convert(dstType))
		return
	}
	ptr := reflect.New(dstType.Elem())
	ptr.Elem().Set(val.Convert(dstType.Elem()))
	dst.Set(ptr)
}

// zeroValueForNilString reports whether a source string that means
// no value assigns the zero value to a destination of type dstType.
//
// Those are the destinations that the parsing strategies of
// SmartAssign would parse a value string into, which all fail for a
// nil one, so an empty cell has to mean "no value" for them.
//
// Every other destination cannot hold a string of any content,
// so a nil one is a type mismatch that stays an error instead
// of being silently turned into a zero value. Reporting it keeps
// a struct field wired to the wrong column type failing on the
// first row instead of only on the first row with a value.
//
// Pointers are followed all the way to the pointed-to type because
// the pointer allocation strategy of SmartAssign allocates every
// level, so *string and **string must be treated alike.
func zeroValueForNilString(dstType reflect.Type) bool {
	dstType, ok := derefPointerType(dstType)
	if !ok {
		return false
	}
	// time.Time has a struct kind, time.Duration an int64 kind
	// that is covered by the integer kinds below.
	if timeType(dstType) {
		return true
	}
	switch dstType.Kind() {
	case reflect.Bool, reflect.Float32, reflect.Float64:
		return true
	}
	return integerKind(dstType.Kind())
}

// integerKind reports whether kind is one of the
// signed or unsigned integer kinds.
func integerKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	}
	return false
}
