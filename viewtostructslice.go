package retable

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
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
// Column names are compared with StructFieldNaming.TitlesMatch, exactly by
// default. Set StructFieldNaming.MatchTitle to TrimEqual to ignore
// surrounding whitespace, or to TrimEqualFold to also ignore case.
//
// If several View columns match the same struct field, the field is
// assigned once per row, from one of them:
//   - The closest match wins, whatever the order of the columns: a column
//     name equal to the field title, before one that is equal without
//     surrounding whitespace, before one that also differs in case,
//     before one that only MatchTitle accepts.
//   - Of several equally close columns, like two of the same name, the
//     cell with content wins over an empty one: a null-like value,
//     see IsNullLike, or a string of only whitespace. If more than one
//     of them has content the last one wins, and if none has, the last
//     one is assigned. With StructFieldNaming.PreferFirstColumn
//     it is the first one in both cases.
//
// A column that matches several struct fields is assigned to the closest
// match in the same order, and to the first field of equally close ones.
// With StructFieldNaming.DisallowAmbiguousTitles both cases are an error
// wrapping ErrAmbiguousColumnTitle instead, returned before any row
// is processed.
//
// With a MatchTitle, two struct fields whose titles match each other are
// an error wrapping ErrDuplicateColumnTitle, returned before any row is
// processed, see StructFieldNaming.CheckUniqueColumns: every column
// matching both titles would be assigned to the first of the fields.
// Without a MatchTitle equal field titles are accepted as before
// and the first field is assigned.
//
// If a column has no corresponding struct field, it is silently skipped.
// This allows Views to contain extra columns that aren't needed in the struct.
//
// Required Columns:
//
// The requiredCols parameter specifies column names that MUST exist both in
// the View and as struct fields, compared with StructFieldNaming.TitlesMatch.
// If any required column is missing from either the View or the struct
// definition, an error is returned immediately before processing any rows.
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
//   - naming: Defines how column names map to struct field names. Can be nil,
//     which maps a column to the struct field of the same name.
//   - dstScanner: Optional Scanner for custom string-to-type conversions,
//     asked for a string source before the built-in conversions and skipped
//     on errors.ErrUnsupported (can be nil).
//   - parser: Parser used by SmartAssign for every string conversion of a
//     cell, which are the number, bool, time and duration ones and the
//     classification of a cell as nil, and passed on to dstScanner.ScanString.
//     Can be nil, in which case a default StringParser is used. Pass one
//     explicitly to configure boolean strings, time formats or number locales.
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
//	people, err := ViewToStructSlice[Person](view, naming, nil, nil, nil, nil)
//	// people[0] == Person{Name: "Alice", Age: 30}
//	// people[1] == Person{Name: "Bob", Age: 25}
//
//	// With required columns and validation
//	people, err = ViewToStructSlice[Person](
//	    view,
//	    naming,
//	    nil, nil, nil,
//	    CallValidateMethod, // Validate each field
//	    "name", "age",      // Both columns are required
//	)
//
//	// Using pointer type
//	people, err := ViewToStructSlice[*Person](view, naming, nil, nil, nil, nil)
//	// people[0] == &Person{Name: "Alice", Age: 30}
func ViewToStructSlice[T any](view View, naming *StructFieldNaming, dstScanner Scanner, parser Parser, srcFormatter Formatter, validate func(reflect.Value) error, requiredCols ...string) ([]T, error) {
	rowType := reflect.TypeFor[T]()
	if rowType.Kind() != reflect.Struct && (rowType.Kind() != reflect.Pointer || rowType.Elem().Kind() != reflect.Struct) {
		return nil, fmt.Errorf("slice element type %s is not a struct or pointer to struct", rowType)
	}

	if naming != nil && naming.MatchTitle != nil {
		// A loose MatchTitle can make two fields match the same column,
		// of which only the first would ever be assigned
		var row T
		err := naming.CheckUniqueColumns(row)
		if err != nil {
			return nil, err
		}
	}

	viewCols := view.ColumnNames()
	reflectView := AsReflectCellView(view)

	// Allocate a parser for dstScanner once per view instead of
	// using the shared DefaultParser, so that a Scanner which
	// reconfigures it cannot affect a concurrent conversion.
	if parser == nil && dstScanner != nil {
		parser = NewStringParser()
	}

	if len(requiredCols) > 0 {
		var v reflect.Value
		if rowType.Kind() == reflect.Pointer {
			v = reflect.New(rowType.Elem()).Elem()
		} else {
			v = reflect.New(rowType).Elem()
		}
		for _, requiredCol := range requiredCols {
			viewColIndex := slices.IndexFunc(viewCols, func(viewCol string) bool { return naming.TitlesMatch(viewCol, requiredCol) })
			if viewColIndex == -1 {
				return nil, fmt.Errorf("required column %q not found in View columns", requiredCol)
			}
			// Look up the field with the view column, like the rows below,
			// so that MatchTitle gets the view title as its first argument
			if !naming.ColumnStructFieldValue(v, viewCols[viewColIndex]).IsValid() {
				return nil, fmt.Errorf("required column %q not found as struct field", requiredCol)
			}
		}
	}

	structType := rowType
	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}
	// The columns of every field are the same for all rows
	fields, err := naming.fieldSourceColumns(structType, viewCols)
	if err != nil {
		return nil, err
	}

	rows := make([]T, view.NumRows())
	for rowIndex := range rows {
		rowStruct := reflect.ValueOf(&rows[rowIndex]).Elem()
		if rowType.Kind() == reflect.Pointer {
			rowStruct.Set(reflect.New(rowType.Elem())) // Set allocated struct pointer for row
			rowStruct = rowStruct.Elem()               // Continue with struct value instead of pointer
		}
		for _, field := range fields {
			src := firstCellWithContent(reflectView, rowIndex, field.columns)
			if !src.IsValid() {
				continue
			}
			dst := rowStruct.FieldByIndex(field.fieldIndex)
			err := SmartAssign(dst, src, dstScanner, parser, srcFormatter)
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

// fieldSourceColumns are the columns of a view
// that a struct field is assigned from.
type fieldSourceColumns struct {
	fieldIndex []int          // of the field, for reflect.Value.FieldByIndex
	rank       titleMatchRank // how closely the names of all columns match the field title
	columns    []int          // indices of the view columns, the preferred one first
}

// fieldSourceColumns returns for every field of structType that a column
// of viewCols matches, the columns to assign it from: the ones that match
// its title the closest, see titleMatchRank, ordered by preference:
// the last column first, or the first one with PreferFirstColumn.
// A column matching several fields counts for the closest match,
// the first field of equally close ones.
// The fields are in the order of their first matching column.
//
// With DisallowAmbiguousTitles it returns an error wrapping
// ErrAmbiguousColumnTitle if a column matches more than one field
// or more than one column matches a field.
func (n *StructFieldNaming) fieldSourceColumns(structType reflect.Type, viewCols []string) ([]fieldSourceColumns, error) {
	var (
		fields  []fieldSourceColumns
		titles  []string // titles[i] is the title of fields[i]
		matches []fieldTitleMatch
	)
	for colIndex, viewCol := range viewCols {
		if n.IsIgnored(viewCol) {
			continue
		}
		matches = n.appendFieldTitleMatches(matches[:0], structType, nil, viewCol)
		match, ok := closestFieldTitleMatch(matches)
		if !ok {
			continue
		}
		if len(matches) > 1 && n.disallowAmbiguousTitles() {
			return nil, fmt.Errorf("%w: column %q matches both %q and %q of %s", ErrAmbiguousColumnTitle, viewCol, matches[0].title, matches[1].title, structType)
		}
		i := slices.IndexFunc(fields, func(field fieldSourceColumns) bool {
			return slices.Equal(field.fieldIndex, match.index)
		})
		switch {
		case i == -1:
			fields = append(fields, fieldSourceColumns{fieldIndex: match.index, rank: match.rank, columns: []int{colIndex}})
			titles = append(titles, match.title)
		case n.disallowAmbiguousTitles():
			return nil, fmt.Errorf("%w: columns %q and %q both match %q of %s", ErrAmbiguousColumnTitle, viewCols[fields[i].columns[0]], viewCol, titles[i], structType)
		case match.rank < fields[i].rank:
			// A closer match replaces all looser ones
			fields[i].rank = match.rank
			fields[i].columns = []int{colIndex}
		case match.rank == fields[i].rank && n.preferFirstColumn():
			fields[i].columns = append(fields[i].columns, colIndex)
		case match.rank == fields[i].rank:
			fields[i].columns = slices.Insert(fields[i].columns, 0, colIndex)
		}
	}
	return fields, nil
}

// firstCellWithContent returns the cell of the first of columns in the row
// of view that has content, see cellHasContent. Without such a cell it
// returns the first valid one, or an invalid value if there is none.
func firstCellWithContent(view ReflectCellView, row int, columns []int) reflect.Value {
	var firstValid reflect.Value
	for _, col := range columns {
		cell := view.ReflectCell(row, col)
		if cellHasContent(cell) {
			return cell
		}
		if cell.IsValid() && !firstValid.IsValid() {
			firstValid = cell
		}
	}
	return firstValid
}

// cellHasContent reports whether the cell of a view holds a value:
// it is valid, not null-like, see IsNullLike, and not a string,
// or pointer to one, that is empty or only whitespace.
func cellHasContent(cell reflect.Value) bool {
	if !cell.IsValid() || IsNullLike(cell) {
		return false
	}
	// IsNullLike has ruled out nil pointers up to maxPointerDepth
	for depth := 0; depth <= maxPointerDepth && (cell.Kind() == reflect.Pointer || cell.Kind() == reflect.Interface); depth++ {
		cell = cell.Elem()
	}
	return cell.Kind() != reflect.String || strings.TrimSpace(cell.String()) != ""
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
//	    nil, nil, nil,
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
//	    nil, nil, nil,
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
