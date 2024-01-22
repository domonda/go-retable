package retable

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

// SmartAssign assigns src to dst by converting src to dst type
// as smart as possible using the passed dstScanner and srcFormatter.
// dstScanner can be nil
// srcFormatter can be nil
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

	return fmt.Errorf("%w: assigning %s to %s", errors.ErrUnsupported, srcType, dstType)
}
