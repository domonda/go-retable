package retable

import (
	"encoding"
	"errors"
	"fmt"
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
//
//  3. Nil pointer handling: If src is a nil pointer, dst is set to its zero value.
//
//  4. Custom formatting: If dst is a string type and srcFormatter is provided,
//     srcFormatter.Format is used to convert src to string.
//
//  5. TextMarshaler: If src implements encoding.TextMarshaler, its MarshalText
//     method is used to get a text representation for further conversion.
//
//  6. Stringer: If src implements fmt.Stringer, its String method is used
//     to get a string representation for further conversion.
//
//  7. Time parsing: If src is a string and dst is time.Time or *time.Time,
//     ParseTime is used to convert the string to a time value.
//
//  8. Pointer dereferencing: If src is a non-nil pointer, SmartAssign is
//     recursively called with the dereferenced value.
//
//  9. Empty struct handling: If src is an empty struct (struct{}), dst is
//     set to its zero value.
//
//  10. Boolean conversions:
//     - bool to numeric types: true becomes 1, false becomes 0
//     - bool to string: "true" or "false"
//     - numeric types to bool: non-zero becomes true, zero becomes false
//     - string to bool: parsed using strconv.ParseBool
//
//  11. String to numeric conversions:
//     - String to int/uint: parsed using strconv.ParseInt/ParseUint
//     - String to float: parsed using strconv.ParseFloat
//
//  12. Fallback string conversion: Any type can be converted to string
//     using fmt.Sprint as a last resort.
//
//  13. Pointer allocation: If dst is a pointer type and previous strategies
//     failed, a new instance is created and SmartAssign is recursively
//     called to assign to the dereferenced pointer.
//
// Parameters:
//   - dst: The destination reflect.Value to assign to. Must be valid and settable.
//   - src: The source reflect.Value to assign from. Must be valid.
//   - dstScanner: Optional Scanner for custom string-to-type conversions (can be nil).
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
//	err := SmartAssign(dst, src, nil, nil)
//	// result == 42
//
//	// Convert bool to string
//	var str string
//	dst = reflect.ValueOf(&str).Elem()
//	src = reflect.ValueOf(true)
//	err = SmartAssign(dst, src, nil, nil)
//	// str == "true"
//
//	// Convert with custom formatter
//	formatter := FormatterFunc(func(v reflect.Value) (string, error) {
//	    return fmt.Sprintf("#%v", v.Interface()), nil
//	})
//	var output string
//	dst = reflect.ValueOf(&output).Elem()
//	src = reflect.ValueOf(42)
//	err = SmartAssign(dst, src, nil, formatter)
//	// output == "#42"
func SmartAssign(dst, src reflect.Value, dstScanner Scanner, srcFormatter Formatter) (err error) {
	if !dst.IsValid() {
		return fmt.Errorf("dst value is invalid")
	}
	if !dst.CanSet() {
		return fmt.Errorf("cannot set dst value")
	}
	if !src.IsValid() {
		return fmt.Errorf("src value is invalid")
	}
	var (
		srcType = src.Type()
		srcKind = srcType.Kind()
		dstType = dst.Type()
		dstKind = dstType.Kind()
	)

	// Package reflect might panic in some edge cases
	// like converting a slice to an array with non matching length.
	// Recover and return as error instead to make code more robust.
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		err = errors.Join(err, fmt.Errorf("%+v", r))
	// 	}
	// }()

	// Assign zero value in case of IsNull.
	// Conversions further down might assign something
	// different than the zero value dependent on the
	// underlying type.
	if nullable, ok := src.Interface().(interface{ IsNull() bool }); ok && nullable.IsNull() {
		dst.Set(reflect.Zero(dstType))
		return nil
	}

	// Convert assigns directly if possible
	if srcType.ConvertibleTo(dstType) {
		// Check because conversion can panic
		if srcKind == reflect.Slice && dstKind == reflect.Pointer && dstType.Elem().Kind() == reflect.Array && dst.Elem().Len() > src.Len() {
			return fmt.Errorf("cannot convert slice of length %d to array pointer with length %d", src.Len(), dst.Elem().Len())
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

	// Try assigning string from MarshalText method
	if m, ok := src.Interface().(encoding.TextMarshaler); ok {
		txt, err := m.MarshalText()
		if err != nil {
			return err
		}
		err = SmartAssign(dst, reflect.ValueOf(string(txt)), dstScanner, srcFormatter)
		if !errors.Is(err, errors.ErrUnsupported) {
			return err // nil or other than errors.ErrUnsupported
		}
		// Continue after errors.ErrUnsupported
	}

	// Try assigning string from String method
	if m, ok := src.Interface().(fmt.Stringer); ok {
		err = SmartAssign(dst, reflect.ValueOf(m.String()), dstScanner, srcFormatter)
		if !errors.Is(err, errors.ErrUnsupported) {
			return err // nil or other than errors.ErrUnsupported
		}
		// Continue after errors.ErrUnsupported
	}

	// Try converting string to time.Time
	if srcKind == reflect.String && (dstType == typeOfTime || dstKind == reflect.Pointer && dstType.Elem() == typeOfTime) {
		if t, _, err := ParseTime(src.String()); err == nil {
			if dstType == typeOfTime {
				dst.Set(reflect.ValueOf(t))
			} else {
				dst.Set(reflect.ValueOf(&t))
			}
			return nil
		}
	}

	// Try assigning the dereferenced value
	if srcKind == reflect.Pointer && !src.IsNil() {
		err := SmartAssign(dst, src.Elem(), dstScanner, srcFormatter)
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
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			if src.Bool() {
				dst.SetUint(1)
			} else {
				dst.SetUint(0)
			}
		case reflect.Float32, reflect.Float64:
			if src.Bool() {
				dst.SetFloat(1)
			} else {
				dst.SetFloat(0)
			}
		case reflect.String:
			dst.SetString(strconv.FormatBool(src.Bool()))
		}
	}

	switch dstKind {

	// Convert 0 / 1 numbers, or "true" / "false" strings to bool
	case reflect.Bool:
		switch srcKind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			dst.SetBool(src.Int() != 0)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			dst.SetBool(src.Uint() != 0)
		case reflect.Float32, reflect.Float64:
			dst.SetBool(src.Float() != 0)
		case reflect.String:
			b, err := strconv.ParseBool(src.String())
			if err == nil {
				dst.SetBool(b)
				return nil
			}
		}

	// Convert string to integers
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if srcKind == reflect.String {
			if i, e := strconv.ParseInt(src.String(), 10, 64); e == nil {
				dst.SetInt(i)
				return nil
			}
		}

	// Convert string to unsigned integers
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if srcKind == reflect.String {
			if i, e := strconv.ParseUint(src.String(), 10, 64); e == nil {
				dst.SetUint(i)
				return nil
			}
		}

	case reflect.Float32, reflect.Float64:
		if srcKind == reflect.String {
			if f, e := strconv.ParseFloat(src.String(), 64); e == nil {
				dst.SetFloat(f)
				return nil
			}
		}

	// Convert any type to string with fmt.Sprint
	case reflect.String:
		dst.SetString(fmt.Sprint(src.Interface()))
		return nil

	// If all other failed and dest is a pointer,
	// try to create a new instance and assign to that
	// then assign the pointer to the new instance.
	case reflect.Pointer:
		newDest := reflect.New(dstType.Elem())
		err = SmartAssign(newDest.Elem(), src, dstScanner, srcFormatter)
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
