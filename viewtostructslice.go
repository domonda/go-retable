package retable

import (
	"fmt"
	"reflect"
	"slices"
)

// ViewToStructSlice converts a View into a strongly-typed slice of structs.
// It maps each row in the View to a struct instance by matching column names
// to struct field names using the provided naming conventions.
//
// Type Parameter:
//   - T: The target struct type. Can be either a struct type (MyStruct) or a
//     pointer to struct (*MyStruct). The function will handle both cases
//     appropriately.
//
// Column to Field Mapping:
//
// The function uses StructFieldNaming to determine how View column names
// map to struct field names. For each column in the View, it attempts to
// find a corresponding struct field and assigns the value using SmartAssign,
// which performs intelligent type conversions.
//
// If a column has no corresponding struct field, it is silently skipped.
// This allows Views to contain extra columns that aren't needed in the struct.
//
// Required Columns:
//
// The requiredCols parameter specifies column names that MUST exist both in
// the View and as struct fields. If any required column is missing from either
// the View or the struct definition, an error is returned immediately before
// processing any rows.
//
// Type Conversion:
//
// SmartAssign is used for each field assignment, enabling automatic conversion
// between compatible types:
//   - String to numeric types (int, float, etc.)
//   - Numeric to string
//   - Boolean to numeric (true=1, false=0)
//   - Custom conversions via dstScanner and srcFormatter
//
// Validation:
//
// After each successful field assignment, if a validate function is provided,
// it is called with the struct field value. If validation fails, processing
// stops and the error is returned. This allows for field-level validation
// during the conversion process.
//
// Use CallValidateMethod as the validate function to automatically invoke
// Validate() or Valid() methods on struct field values that implement them.
//
// Parameters:
//   - view: The source View to convert. Must not be nil.
//   - naming: Defines how column names map to struct field names. Must not be nil.
//   - dstScanner: Optional Scanner for custom string-to-type conversions (can be nil).
//   - srcFormatter: Optional Formatter for custom type-to-string conversions (can be nil).
//   - validate: Optional function called after each field assignment for validation (can be nil).
//   - requiredCols: Column names that must exist in both View and struct.
//
// Returns:
//   - []T: A slice of structs with length equal to view.NumRows(). Each struct
//     represents one row from the View.
//   - error: An error if:
//   - T is not a struct or pointer to struct
//   - Any required column is missing from View or struct
//   - Type conversion fails for any field
//   - Validation fails for any field
//
// Example:
//
//	// Define a struct to hold row data
//	type Person struct {
//	    Name string `db:"name"`
//	    Age  int    `db:"age"`
//	}
//
//	// Create a View with data
//	view := NewStringsView("people",
//	    [][]string{
//	        {"Alice", "30"},
//	        {"Bob", "25"},
//	    },
//	    "name", "age")
//
//	// Convert to slice of structs
//	naming := &StructFieldNaming{Tag: "db"}
//	people, err := ViewToStructSlice[Person](view, naming, nil, nil, nil)
//	// people[0] == Person{Name: "Alice", Age: 30}
//	// people[1] == Person{Name: "Bob", Age: 25}
//
//	// With required columns and validation
//	people, err = ViewToStructSlice[Person](
//	    view,
//	    naming,
//	    nil, nil,
//	    CallValidateMethod, // Validate each field
//	    "name", "age",      // Both columns are required
//	)
//
//	// Using pointer type
//	people, err := ViewToStructSlice[*Person](view, naming, nil, nil, nil)
//	// people[0] == &Person{Name: "Alice", Age: 30}
func ViewToStructSlice[T any](view View, naming *StructFieldNaming, dstScanner Scanner, srcFormatter Formatter, validate func(reflect.Value) error, requiredCols ...string) ([]T, error) {
	rowType := reflect.TypeFor[T]()
	if rowType.Kind() != reflect.Struct && (rowType.Kind() != reflect.Pointer || rowType.Elem().Kind() != reflect.Struct) {
		return nil, fmt.Errorf("slice element type %s is not a struct or pointer to struct", rowType)
	}

	viewCols := view.Columns()
	reflectView := AsReflectCellView(view)

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
			src := reflectView.ReflectCell(rowIndex, colIndex)
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

// CallValidateMethod is a validation function that can be passed to ViewToStructSlice.
// It checks if the value implements either Validate() or Valid() methods and calls them.
//
// Validation Method Support:
//
//  1. Validate() error: If the value implements this method, it is called.
//     Any non-nil error returned is propagated as a validation failure.
//
//  2. Valid() bool: If the value implements this method, it is called.
//     If it returns false, an error is created describing the validation failure.
//
// The method checks are performed on v.Interface(), so they work with any type
// that implements these methods, including pointer receivers.
//
// Parameters:
//   - v: The reflect.Value to validate. Can be invalid or nil, in which case
//     the function returns nil without error.
//
// Returns:
//   - error: nil if validation passes, v is invalid, or v doesn't implement
//     validation methods. Non-nil error if validation fails.
//
// Example:
//
//	type Email string
//
//	func (e Email) Validate() error {
//	    if !strings.Contains(string(e), "@") {
//	        return fmt.Errorf("invalid email: %s", e)
//	    }
//	    return nil
//	}
//
//	type Person struct {
//	    Email Email `db:"email"`
//	    Age   int   `db:"age"`
//	}
//
//	// CallValidateMethod will call Email.Validate() for the Email field
//	people, err := ViewToStructSlice[Person](
//	    view,
//	    naming,
//	    nil, nil,
//	    CallValidateMethod,
//	)
//
//	// Example with Valid() bool method
//	type Age int
//
//	func (a Age) Valid() bool {
//	    return a >= 0 && a <= 150
//	}
//
//	type PersonWithAge struct {
//	    Age Age `db:"age"`
//	}
//
//	// CallValidateMethod will call Age.Valid() for the Age field
//	people, err := ViewToStructSlice[PersonWithAge](
//	    view,
//	    naming,
//	    nil, nil,
//	    CallValidateMethod,
//	)
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
