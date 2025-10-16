package retable

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"go/token"
	"io"
	"os"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

// StructFieldTypes returns the exported fields of a struct type,
// including the inlined fields of any anonymously embedded structs.
// If structType is a pointer, it automatically dereferences to the underlying struct type.
//
// The function recursively processes embedded structs, flattening their fields into the result.
// Only exported fields (those starting with an uppercase letter) are included.
//
// Example:
//
//	type Address struct {
//	    Street string
//	    City   string
//	}
//	type Person struct {
//	    Name string
//	    Address // embedded struct
//	    age  int // unexported, will be excluded
//	}
//	fields := StructFieldTypes(reflect.TypeOf(Person{}))
//	// Returns: [Name, Street, City]
func StructFieldTypes(structType reflect.Type) (fields []reflect.StructField) {
	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		switch {
		case field.Anonymous:
			fields = append(fields, StructFieldTypes(field.Type)...)
		case token.IsExported(field.Name):
			fields = append(fields, field)
		}
	}
	return fields
}

// StructFieldReflectValues returns the reflect.Value of exported struct fields,
// including the inlined fields of any anonymously embedded structs.
// If structValue is a pointer, it automatically dereferences to the underlying struct value.
//
// The function recursively processes embedded structs, flattening their field values into the result.
// Only exported fields (those starting with an uppercase letter) are included.
// The order of fields matches the order returned by StructFieldTypes.
//
// Example:
//
//	type Address struct {
//	    Street string
//	    City   string
//	}
//	type Person struct {
//	    Name string
//	    Address
//	}
//	p := Person{Name: "John", Address: Address{Street: "Main St", City: "NYC"}}
//	values := StructFieldReflectValues(reflect.ValueOf(p))
//	// Returns reflect.Values for: ["John", "Main St", "NYC"]
func StructFieldReflectValues(structValue reflect.Value) []reflect.Value {
	if structValue.Kind() == reflect.Pointer {
		structValue = structValue.Elem()
	}
	structType := structValue.Type()
	values := make([]reflect.Value, 0, structType.NumField())
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		switch {
		case field.Anonymous:
			values = append(values, StructFieldReflectValues(structValue.Field(i))...)
		case token.IsExported(field.Name):
			values = append(values, structValue.Field(i))
		}
	}
	return values
}

// IndexedStructFieldReflectValues returns a reordered subset of reflect.Value for exported struct fields,
// including the inlined fields of any anonymously embedded structs.
//
// The indices parameter maps from the flattened field list to the desired output positions.
// A negative index value (-1) indicates that the field should be skipped.
// The numVals parameter specifies the size of the output slice.
//
// Panics if the length of indices does not match the number of fields in the struct.
//
// Example:
//
//	type Person struct {
//	    Name string
//	    Age  int
//	    City string
//	}
//	p := Person{Name: "John", Age: 30, City: "NYC"}
//	// Reorder to [City, Name] and skip Age
//	values := IndexedStructFieldReflectValues(reflect.ValueOf(p), 2, []int{1, -1, 0})
//	// Returns reflect.Values at positions: [0]=City, [1]=Name
func IndexedStructFieldReflectValues(structValue reflect.Value, numVals int, indices []int) []reflect.Value {
	// TODO optimized algorithm that does not allocate a slice for all values but only numVals
	allVals := StructFieldReflectValues(structValue)
	if len(allVals) != len(indices) {
		panic(fmt.Errorf("got %d indices for struct with %d fields", len(indices), len(allVals)))
	}
	vals := make([]reflect.Value, numVals)
	for i, index := range indices {
		if index < 0 {
			continue
		}
		vals[index] = allVals[i]
	}
	return vals
}

// StructFieldAnyValues returns the values of exported struct fields as []any,
// including the inlined fields of any anonymously embedded structs.
// If structValue is a pointer, it automatically dereferences to the underlying struct value.
//
// This is similar to StructFieldReflectValues but returns the values as any interfaces
// instead of reflect.Value, making it easier to work with in non-reflection contexts.
//
// The function recursively processes embedded structs, flattening their field values into the result.
// Only exported fields are included. The order matches StructFieldTypes.
//
// Example:
//
//	type Person struct {
//	    Name string
//	    Age  int
//	}
//	p := Person{Name: "John", Age: 30}
//	values := StructFieldAnyValues(reflect.ValueOf(p))
//	// Returns: []any{"John", 30}
func StructFieldAnyValues(structValue reflect.Value) []any {
	if structValue.Kind() == reflect.Pointer {
		structValue = structValue.Elem()
	}
	structType := structValue.Type()
	values := make([]any, 0, structType.NumField())
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		switch {
		case field.Anonymous:
			values = append(values, StructFieldAnyValues(structValue.Field(i))...)
		case token.IsExported(field.Name):
			values = append(values, structValue.Field(i).Interface())
		}
	}
	return values
}

// IndexedStructFieldAnyValues returns a reordered subset of exported struct field values as []any,
// including the inlined fields of any anonymously embedded structs.
//
// The indices parameter maps from the flattened field list to the desired output positions.
// A negative index value (-1) indicates that the field should be skipped.
// The numVals parameter specifies the size of the output slice.
//
// This is similar to IndexedStructFieldReflectValues but returns values as any interfaces.
//
// Panics if the length of indices does not match the number of fields in the struct.
//
// Example:
//
//	type Person struct {
//	    Name string
//	    Age  int
//	    City string
//	}
//	p := Person{Name: "John", Age: 30, City: "NYC"}
//	// Reorder to [City, Name] and skip Age
//	values := IndexedStructFieldAnyValues(reflect.ValueOf(p), 2, []int{1, -1, 0})
//	// Returns: []any{NYC, John}
func IndexedStructFieldAnyValues(structValue reflect.Value, numVals int, indices []int) []any {
	// TODO optimized algorithm that does not allocate a slice for all values but only numVals
	allVals := StructFieldAnyValues(structValue)
	if len(allVals) != len(indices) {
		panic(fmt.Errorf("got %d indices for struct with %d fields", len(indices), len(allVals)))
	}
	vals := make([]any, numVals)
	for i, index := range indices {
		if index < 0 {
			continue
		}
		vals[index] = allVals[i]
	}
	return vals
}

// StructFieldIndex returns the index of the struct field pointed to by fieldPtr
// within the struct pointed to by structPtr.
//
// The returned index counts exported struct fields including the inlined fields
// of any anonymously embedded structs, using the same ordering as StructFieldTypes.
//
// Both structPtr and fieldPtr must be non-nil pointers.
// Returns an error if either parameter is nil, not a pointer, or if the field
// cannot be found within the struct.
//
// Example:
//
//	type Person struct {
//	    Name string
//	    Age  int
//	}
//	p := Person{Name: "John", Age: 30}
//	index, err := StructFieldIndex(&p, &p.Age)
//	// Returns: index=1, err=nil
func StructFieldIndex(structPtr, fieldPtr any) (int, error) {
	if structPtr == nil {
		return 0, errors.New("expected struct pointer, got <nil>")
	}
	structVal := reflect.ValueOf(structPtr)
	if structVal.Kind() != reflect.Pointer {
		return 0, fmt.Errorf("expected struct pointer, got %T", structPtr)
	}
	if structVal.IsNil() {
		return 0, errors.New("expected struct pointer, got <nil>")
	}
	structVal = structVal.Elem()

	if fieldPtr == nil {
		return 0, errors.New("expected struct field pointer, got <nil>")
	}
	fieldVal := reflect.ValueOf(fieldPtr)
	if fieldVal.Kind() != reflect.Pointer {
		return 0, fmt.Errorf("expected struct field pointer, got %T", fieldPtr)
	}
	if fieldVal.IsNil() {
		return 0, errors.New("expected struct field pointer, got <nil>")
	}
	fieldVal = fieldVal.Elem()

	for i, v := range StructFieldReflectValues(structVal) {
		if v == fieldVal {
			return i, nil
		}
	}
	return 0, fmt.Errorf("struct field not found in %s", structVal.Type())
}

// MustStructFieldIndex returns the index of the struct field pointed to by fieldPtr
// within the struct pointed to by structPtr.
//
// This is the panic-on-error version of StructFieldIndex.
// The returned index counts exported struct fields including the inlined fields
// of any anonymously embedded structs, using the same ordering as StructFieldTypes.
//
// Panics if either parameter is invalid or if the field cannot be found.
//
// Example:
//
//	type Person struct {
//	    Name string
//	    Age  int
//	}
//	p := Person{Name: "John", Age: 30}
//	index := MustStructFieldIndex(&p, &p.Age)
//	// Returns: 1
func MustStructFieldIndex(structPtr, fieldPtr any) int {
	index, err := StructFieldIndex(structPtr, fieldPtr)
	if err != nil {
		panic(err)
	}
	return index
}

// SpacePascalCase inserts spaces before upper case characters within PascalCase names.
//
// The function processes each character in the input string and:
//   - Inserts a space before each uppercase letter that follows a lowercase letter or non-space
//   - Replaces underscore '_' characters with spaces
//   - Avoids duplicate spaces
//   - Trims leading and trailing spaces from the result
//
// This is particularly useful for converting struct field names to human-readable column titles.
// Can be used as the UntaggedTitle function in StructFieldNaming.
//
// Example:
//
//	SpacePascalCase("FirstName")      // Returns: "First Name"
//	SpacePascalCase("HTTPServer")     // Returns: "H T T P Server"
//	SpacePascalCase("user_name")      // Returns: "user name"
//	SpacePascalCase("API_Key")        // Returns: "A P I Key"
func SpacePascalCase(name string) string {
	var b strings.Builder
	b.Grow(len(name) + 4)
	lastWasUpper := true
	lastWasSpace := true
	for _, r := range name {
		if r == '_' {
			if !lastWasSpace {
				b.WriteByte(' ')
			}
			lastWasUpper = false
			lastWasSpace = true
			continue
		}
		isUpper := unicode.IsUpper(r)
		if isUpper && !lastWasUpper && !lastWasSpace {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		lastWasUpper = isUpper
		lastWasSpace = unicode.IsSpace(r)
	}
	return strings.TrimSpace(b.String())
}

// SpaceGoCase inserts spaces before upper case characters within Go-style names,
// with special handling for acronyms and abbreviations.
//
// The function processes each character and:
//   - Inserts a space before uppercase letters following lowercase letters ("CamelCase" -> "Camel Case")
//   - Detects acronyms by recognizing consecutive uppercase letters
//   - Inserts a space before the last uppercase letter in an acronym when followed by lowercase ("HTTPServer" -> "HTTP Server")
//   - Replaces underscore '_' characters with spaces
//   - Avoids duplicate spaces
//   - Trims leading and trailing spaces from the result
//
// This is more sophisticated than SpacePascalCase as it properly handles Go naming conventions
// where acronyms like HTTP, API, URL are kept in uppercase.
// Can be used as the UntaggedTitle function in StructFieldNaming.
//
// Example:
//
//	SpaceGoCase("FirstName")      // Returns: "First Name"
//	SpaceGoCase("HTTPServer")     // Returns: "HTTP Server"
//	SpaceGoCase("APIKey")         // Returns: "API Key"
//	SpaceGoCase("URLPath")        // Returns: "URL Path"
//	SpaceGoCase("user_name")      // Returns: "user name"
func SpaceGoCase(name string) string {
	var b strings.Builder
	b.Grow(len(name) + 4)
	beforeLastWasUpper := false
	lastWasUpper := true
	lastWasSpace := true
	for _, r := range name {
		if r == '_' {
			if !lastWasSpace {
				b.WriteByte(' ')
			}
			lastWasUpper = false
			lastWasSpace = true
			continue
		}
		isUpper := unicode.IsUpper(r)
		switch {
		case isUpper && !lastWasUpper && !lastWasSpace:
			// First upper case rune after lower case non-space rune
			// "CamelCase" -> "Camel Case"
			b.WriteByte(' ')
		case !isUpper && lastWasUpper && beforeLastWasUpper:
			// First lower case rune after two upper case runes assumes that
			// the upper case part before the last upper case rune is an all upper case acronym
			// "HTTPServer" -> "HTTP Server"
			s := b.String()
			lastR, lenLastR := utf8.DecodeLastRuneInString(s)
			if lastR != utf8.RuneError {
				b.Reset()
				b.WriteString(s[:len(s)-lenLastR])
				b.WriteByte(' ')
				b.WriteRune(lastR)
			}
		}
		b.WriteRune(r)
		beforeLastWasUpper = lastWasUpper
		lastWasUpper = isUpper
		lastWasSpace = unicode.IsSpace(r)
	}
	return strings.TrimSpace(b.String())
}

// StringColumnWidths returns the maximum width of each column in a string table,
// measured as the count of UTF-8 runes (not bytes).
//
// The maxCols parameter limits the number of columns to consider:
//   - If maxCols is -1, all columns across all rows are considered
//   - If maxCols is positive, only the first maxCols columns are processed
//
// This is useful for formatting tables with aligned columns, ensuring each column
// is wide enough to accommodate its widest value.
//
// Returns a slice where each element represents the maximum width of that column.
// Returns nil if maxCols is 0.
//
// Example:
//
//	rows := [][]string{
//	    {"Name", "Age", "City"},
//	    {"John", "30", "New York"},
//	    {"Alice", "25", "SF"},
//	}
//	widths := StringColumnWidths(rows, -1)
//	// Returns: [5, 3, 8] (lengths of "Alice", "Age", "New York")
func StringColumnWidths(rows [][]string, maxCols int) []int {
	if maxCols < 0 {
		for _, r := range rows {
			maxCols = max(maxCols, len(r))
		}
	}
	if maxCols == 0 {
		return nil
	}
	colWidths := make([]int, maxCols)
	for row := range rows {
		for col := 0; col < maxCols && col < len(rows[row]); col++ {
			numRunes := utf8.RuneCountInString(rows[row][col])
			colWidths[col] = max(colWidths[col], numRunes)
		}
	}
	return colWidths
}

// UseTitle returns a function that always returns the same fixed columnTitle,
// ignoring the input fieldName parameter.
//
// This is useful when you want to provide a constant column title regardless of the field name,
// or as a way to ignore certain fields by returning a special marker like "-".
// Can be used as the UntaggedTitle function in StructFieldNaming.
//
// Example:
//
//	// Create a function that always returns "Fixed Title"
//	titleFunc := UseTitle("Fixed Title")
//	title := titleFunc("AnyFieldName") // Returns: "Fixed Title"
//
//	// Common pattern to ignore untagged fields
//	naming := StructFieldNaming{
//	    Tag:      "col",
//	    Ignore:   "-",
//	    Untagged: UseTitle("-"), // Ignore all untagged fields
//	}
func UseTitle(columnTitle string) func(fieldName string) (columnTitle string) {
	return func(string) string { return columnTitle }
}

// IsNullLike returns true if the passed reflect.Value should be treated as null-like.
//
// A value is considered null-like if it fulfills any of the following conditions:
//   - Is not valid (zero value of reflect.Value)
//   - Is nil (for pointer, interface, slice, map, channel, func, or unsafe pointer types)
//   - Is of type struct{} (empty struct)
//   - Implements IsNull() bool method which returns true
//   - Implements IsZero() bool method which returns true
//   - Implements driver.Valuer interface which returns (nil, nil)
//
// This function is useful for determining whether a value should be rendered as NULL
// or empty in table output, supporting various nullable type conventions used in Go.
//
// Example:
//
//	var ptr *string
//	IsNullLike(reflect.ValueOf(ptr))  // true (nil pointer)
//
//	type Nullable struct{}
//	func (n Nullable) IsNull() bool { return true }
//	IsNullLike(reflect.ValueOf(Nullable{}))  // true
//
//	IsNullLike(reflect.ValueOf(42))  // false
func IsNullLike(val reflect.Value) bool {
	// Treat zero value of reflect.Value as nil
	if !val.IsValid() {
		return true
	}
	switch val.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map,
		reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return val.IsNil()
	}
	// Treat struct{}{} as nil
	if val.Type() == typeOfEmptyStruct {
		return true
	}
	if nullable, ok := val.Interface().(interface{ IsNull() bool }); ok && nullable.IsNull() {
		return true
	}
	if zeroable, ok := val.Interface().(interface{ IsZero() bool }); ok && zeroable.IsZero() {
		return true
	}
	if valuer, ok := val.Interface().(driver.Valuer); ok {
		if v, e := valuer.Value(); v == nil && e == nil {
			return true
		}
	}
	return false
}

// IsStringRowEmpty returns true if all cells in the row are empty strings
// or if the length of the row is zero.
//
// This is useful for filtering out empty rows from table data before rendering or processing.
//
// Example:
//
//	IsStringRowEmpty([]string{})              // true
//	IsStringRowEmpty([]string{"", "", ""})    // true
//	IsStringRowEmpty([]string{"", "data", ""}) // false
func IsStringRowEmpty(row []string) bool {
	for _, cell := range row {
		if cell != "" {
			return false
		}
	}
	return true
}

// RemoveEmptyStringRows removes all rows that contain only empty strings
// from the input slice and returns the modified slice.
//
// The function modifies the input slice in-place by removing empty rows.
// Rows are checked using IsStringRowEmpty and removed in reverse order
// to maintain correct indexing during removal.
//
// Example:
//
//	rows := [][]string{
//	    {"Name", "Age"},
//	    {"", ""},        // Will be removed
//	    {"John", "30"},
//	    {"", ""},        // Will be removed
//	}
//	result := RemoveEmptyStringRows(rows)
//	// Returns: [["Name", "Age"], ["John", "30"]]
func RemoveEmptyStringRows(rows [][]string) [][]string {
	for i := len(rows) - 1; i >= 0; i-- {
		if IsStringRowEmpty(rows[i]) {
			rows = append(rows[:i], rows[i+1:]...)
		}
	}
	return rows
}

// RemoveEmptyStringColumns removes all columns that contain only empty strings
// from the input rows and returns the new number of columns.
//
// The function modifies the input rows slice in-place by removing columns
// that are empty across all rows. It processes columns from right to left
// to maintain correct indexing during removal.
//
// Returns the number of columns remaining after removal.
//
// Example:
//
//	rows := [][]string{
//	    {"Name", "",   "Age", ""},
//	    {"John", "",   "30",  ""},
//	    {"Alice", "",  "25",  ""},
//	}
//	numCols := RemoveEmptyStringColumns(rows)
//	// Modifies rows to: [["Name", "Age"], ["John", "30"], ["Alice", "25"]]
//	// Returns: 2
func RemoveEmptyStringColumns(rows [][]string) (numCols int) {
	for _, row := range rows {
		numCols = max(numCols, len(row))
	}
	for c := numCols - 1; c >= 0; c-- {
		empty := true
		for _, row := range rows {
			if c < len(row) && row[c] != "" {
				empty = false
				break
			}
		}
		if empty {
			for r, row := range rows {
				if c < len(row) {
					rows[r] = append(row[:c], row[c+1:]...)
				}
			}
			numCols--
		}
	}
	return numCols
}

// FprintlnView formats and writes a View as a human-readable table to the provided io.Writer.
//
// The table is formatted with pipe-separated columns, aligned based on the maximum width
// of each column. If the view has a title, it is printed first on a separate line.
// A header row is automatically added using the view's column names.
//
// The output format looks like:
//
//	Title:
//	| Column1 | Column2 | Column3 |
//	| value1  | value2  | value3  |
//
// Returns an error if formatting fails or if writing to the io.Writer fails.
//
// Example:
//
//	var buf bytes.Buffer
//	err := FprintlnView(&buf, myView)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Print(buf.String())
func FprintlnView(w io.Writer, view View) error {
	rows, err := FormatViewAsStrings(context.Background(), view, nil, OptionAddHeaderRow)
	if err != nil {
		return err
	}
	if view.Title() != "" {
		_, err = fmt.Fprintf(w, "%s:\n", view.Title())
		if err != nil {
			return err
		}
	}
	colWidths := StringColumnWidths(rows, -1)
	for _, rowStrs := range rows {
		for col, colWidth := range colWidths {
			switch {
			case col == 0:
				_, err = w.Write([]byte("| "))
			case col < len(colWidths):
				_, err = w.Write([]byte(" | "))
			}
			if err != nil {
				return err
			}
			str := ""
			if col < len(rowStrs) {
				str = rowStrs[col]
			}
			_, err = io.WriteString(w, str)
			if err != nil {
				return err
			}
			strLen := utf8.RuneCountInString(str)
			for i := strLen; i < colWidth; i++ {
				_, err = w.Write([]byte{' '})
				if err != nil {
					return err
				}
			}
		}
		_, err = w.Write([]byte(" |\n"))
		if err != nil {
			return err
		}
	}
	return nil
}

// SprintlnView formats a View as a human-readable table and returns it as a string.
//
// This is a convenience wrapper around FprintlnView that writes to a strings.Builder
// and returns the resulting string. See FprintlnView for format details.
//
// Returns the formatted table string and any error that occurred during formatting.
//
// Example:
//
//	str, err := SprintlnView(myView)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Print(str)
func SprintlnView(w io.Writer, view View) (string, error) {
	var b strings.Builder
	err := FprintlnView(&b, view)
	return b.String(), err
}

// PrintlnView formats and prints a View as a human-readable table to standard output.
//
// This is a convenience wrapper around FprintlnView that writes to os.Stdout.
// See FprintlnView for format details.
//
// Returns any error that occurred during formatting or writing.
//
// Example:
//
//	err := PrintlnView(myView)
//	if err != nil {
//	    log.Fatal(err)
//	}
func PrintlnView(view View) error {
	return FprintlnView(os.Stdout, view)
}

// FprintlnTable formats and writes any table data as a human-readable table to the provided io.Writer.
//
// The function automatically selects an appropriate Viewer for the table type,
// creates a View with the given title, and then formats it using FprintlnView.
//
// Supported table types include [][]string, struct slices, and any type with a registered Viewer.
//
// Parameters:
//   - w: The io.Writer to write the formatted table to
//   - title: The title to display above the table (can be empty)
//   - table: The table data to format (e.g., [][]string, []Person, etc.)
//
// Returns an error if the table type is not supported, if view creation fails,
// or if writing fails.
//
// Example:
//
//	people := []Person{
//	    {Name: "John", Age: 30},
//	    {Name: "Alice", Age: 25},
//	}
//	err := FprintlnTable(os.Stdout, "People", people)
func FprintlnTable(w io.Writer, title string, table any) error {
	viewer, err := SelectViewer(table)
	if err != nil {
		return err
	}
	view, err := viewer.NewView(title, table)
	if err != nil {
		return err
	}
	return FprintlnView(w, view)
}

// SprintlnTable formats any table data as a human-readable table and returns it as a string.
//
// This is a convenience wrapper around FprintlnTable that writes to a strings.Builder
// and returns the resulting string. See FprintlnTable for details on supported table types.
//
// Parameters:
//   - w: Ignored parameter (kept for signature compatibility)
//   - title: The title to display above the table (can be empty)
//   - table: The table data to format
//
// Returns the formatted table string and any error that occurred.
//
// Example:
//
//	people := []Person{{Name: "John", Age: 30}}
//	str, err := SprintlnTable(nil, "People", people)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Print(str)
func SprintlnTable(w io.Writer, title string, table any) (string, error) {
	var b strings.Builder
	err := FprintlnTable(&b, title, table)
	return b.String(), err
}

// PrintlnTable formats and prints any table data as a human-readable table to standard output.
//
// This is a convenience wrapper around FprintlnTable that writes to os.Stdout.
// See FprintlnTable for details on supported table types.
//
// Parameters:
//   - title: The title to display above the table (can be empty)
//   - table: The table data to format
//
// Returns any error that occurred during formatting or writing.
//
// Example:
//
//	people := []Person{
//	    {Name: "John", Age: 30},
//	    {Name: "Alice", Age: 25},
//	}
//	err := PrintlnTable("People", people)
func PrintlnTable(title string, table any) error {
	return FprintlnTable(os.Stdout, title, table)
}
