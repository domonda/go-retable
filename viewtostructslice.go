package retable

import (
	"fmt"
	"reflect"
)

// CallValidateMethod calls the Validate() method on v.Interface()
// and returns its error if v is not nil and
// if it implements interface{ Validate() error }
func CallValidateMethod(v reflect.Value) error {
	if !v.IsValid() {
		return nil
	}
	if validator, ok := v.Interface().(interface{ Validate() error }); ok {
		return validator.Validate()
	}
	return nil
}

// ViewToStructSlice converts a View to a slice of structs
// mapping the View's columns to the struct fields using
// the passed StructFieldNaming.
//
// SmartAssign is used to assign the View's values to the struct fields.
//
// After a successful value assignment to a struct field,
// validate is called with the struct field value
// and its error is returned if not nil.
//
// The arguments dstScanner, srcFormatter, and validate can be nil.
func ViewToStructSlice[T any](view View, naming *StructFieldNaming, dstScanner Scanner, srcFormatter Formatter, validate func(reflect.Value) error) ([]T, error) {
	rowType := reflect.TypeOf(*new(T))
	if rowType.Kind() != reflect.Struct && (rowType.Kind() != reflect.Pointer || rowType.Elem().Kind() != reflect.Struct) {
		return nil, fmt.Errorf("slice element type %s is not a struct or pointer to struct", rowType)
	}

	rows := make([]T, view.NumRows())
	for row := range rows {
		rowStruct := reflect.ValueOf(&rows[row]).Elem()
		if rowType.Kind() == reflect.Pointer {
			rowStruct.Set(reflect.New(rowType.Elem())) // Set allocated struct pointer for row
			rowStruct = rowStruct.Elem()               // Continue with struct value instead of pointer
		}
		for col, column := range view.Columns() {
			dst := naming.ColumnStructFieldValue(rowStruct, column)
			if !dst.IsValid() {
				continue // Struct field for column not found
			}
			src := view.ReflectValue(row, col)
			if !src.IsValid() {
				continue // No value for column in row
			}
			err := SmartAssign(
				dst,
				src,
				dstScanner,
				srcFormatter,
			)
			if err != nil {
				return nil, err
			}
			if validate != nil {
				err = validate(dst)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return rows, nil
}
