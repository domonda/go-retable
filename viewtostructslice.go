package retable

import (
	"fmt"
	"reflect"
	"slices"
)

// ViewToStructSlice converts a View to a slice of structs
// mapping the View's columns to the struct fields using
// the passed StructFieldNaming.
//
// requiredCols must be present in the View and as named struct fields
// else an error is returned.
//
// SmartAssign is used to assign the View's values to the struct fields
// using the passed dstScanner and srcFormatter
// to convert types from and to strings.
//
// After a successful value assignment to a struct field,
// a non nil validate function is called with the struct field value
// as argument. If the validate function returns an error,
// it is returned immediately.
//
// CallValidateMethod can be passed as validate function
// to call Validate() methods on the struct field values.
//
// The arguments dstScanner, srcFormatter, and validate can be nil.
func ViewToStructSlice[T any](view View, naming *StructFieldNaming, requiredCols []string, dstScanner Scanner, srcFormatter Formatter, validate func(reflect.Value) error) ([]T, error) {
	rowType := reflect.TypeFor[T]()
	if rowType.Kind() != reflect.Struct && (rowType.Kind() != reflect.Pointer || rowType.Elem().Kind() != reflect.Struct) {
		return nil, fmt.Errorf("slice element type %s is not a struct or pointer to struct", rowType)
	}

	viewCols := view.Columns()

	if len(requiredCols) > 0 {
		var v reflect.Value
		if rowType.Kind() == reflect.Pointer {
			v = reflect.New(rowType.Elem()).Elem()
		} else {
			v = reflect.New(rowType).Elem()
		}
		for _, requiredCol := range requiredCols {
			if !slices.Contains(viewCols, requiredCol) {
				return nil, fmt.Errorf("required column %q not found in View columns", requiredCol)
			}
			if !naming.ColumnStructFieldValue(v, requiredCol).IsValid() {
				return nil, fmt.Errorf("required column %q not found as struct field", requiredCol)
			}
		}
	}

	rows := make([]T, view.NumRows())
	for rowIndex := range rows {
		rowStruct := reflect.ValueOf(&rows[rowIndex]).Elem()
		if rowType.Kind() == reflect.Pointer {
			rowStruct.Set(reflect.New(rowType.Elem())) // Set allocated struct pointer for row
			rowStruct = rowStruct.Elem()               // Continue with struct value instead of pointer
		}
		for colIndex, colName := range viewCols {
			dst := naming.ColumnStructFieldValue(rowStruct, colName)
			if !dst.IsValid() {
				continue
			}
			src := view.ReflectValue(rowIndex, colIndex)
			if !src.IsValid() {
				continue
			}
			err := SmartAssign(dst, src, dstScanner, srcFormatter)
			if err == nil && validate != nil {
				err = validate(dst)
			}
			if err != nil {
				return nil, err
			}
		}
	}
	return rows, nil
}

// CallValidateMethod calls the `Validate() error` or `Valid() bool`
// method on v.Interface() if available and v is not nil.
func CallValidateMethod(v reflect.Value) error {
	if !v.IsValid() {
		return nil
	}
	switch x := v.Interface().(type) {
	case interface{ Validate() error }:
		return x.Validate()
	case interface{ Valid() bool }:
		if !x.Valid() {
			return fmt.Errorf("value %[1]#v of type %[1]T is not valid", v.Interface())
		}
	}
	return nil
}
