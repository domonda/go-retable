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

// StructFieldTypes returns the exported fields of a struct type
// including the inlined fields of any anonymously embedded structs.
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

// StructFieldReflectValues returns the reflect.Value of exported struct fields
// including the inlined fields of any anonymously embedded structs.
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

// IndexedStructFieldReflectValues returns the reflect.Value of exported struct fields
// including the inlined fields of any anonymously embedded structs.
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

// StructFieldAnyValues returns the values of exported struct fields
// including the inlined fields of any anonymously embedded structs.
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

// IndexedStructFieldAnyValues returns the values of exported struct fields
// including the inlined fields of any anonymously embedded structs.
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

// StructFieldIndex returns the index of of the struct field
// pointed to by fieldPtr within the struct pointed to by structPtr.
// The returned index counts exported struct fields
// including the inlined fields of any anonymously embedded structs.
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

// MustStructFieldIndex returns the index of of the struct field
// pointed to by fieldPtr within the struct pointed to by structPtr.
// The returned index counts exported struct fields
// including the inlined fields of any anonymously embedded structs.
func MustStructFieldIndex(structPtr, fieldPtr any) int {
	index, err := StructFieldIndex(structPtr, fieldPtr)
	if err != nil {
		panic(err)
	}
	return index
}

// SpacePascalCase inserts spaces before upper case
// characters within PascalCase like names.
// It also replaces underscore '_' characters with spaces.
// Usable for ReflectColumnTitles.UntaggedTitle
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

// StringColumnWidths returns the column widths of the passed
// table as count of UTF-8 runes.
// maxCols limits the number of columns to consider,
// if maxCols is -1, then all columns are considered.
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

// UseTitle returns a function that
// always returns the passed columnTitle.
func UseTitle(columnTitle string) func(fieldName string) (columnTitle string) {
	return func(string) string { return columnTitle }
}

// IsNullLike return true if passed reflect.Value
// fulfills any of the following conditions:
//   - is not valid
//   - nil (of a type that can be nil),
//   - is of type struct{},
//   - implements the IsNull() bool method which returns true,
//   - implements the IsZero() bool method which returns true,
//   - implements the driver.Valuer interface which returns nil, nil.
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

// IsStringRowEmpty returns true if all cells in the row
// are empty strings or if the length of the row is zero.
func IsStringRowEmpty(row []string) bool {
	for _, cell := range row {
		if cell != "" {
			return false
		}
	}
	return true
}

func RemoveEmptyStringRows(rows [][]string) [][]string {
	for i := len(rows) - 1; i >= 0; i-- {
		if IsStringRowEmpty(rows[i]) {
			rows = append(rows[:i], rows[i+1:]...)
		}
	}
	return rows
}

// RemoveEmptyStringColumns removes all columns that only contain empty strings
// and returns the new number of columns.
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

func SprintlnView(w io.Writer, view View) (string, error) {
	var b strings.Builder
	err := FprintlnView(&b, view)
	return b.String(), err
}

func PrintlnView(view View) error {
	return FprintlnView(os.Stdout, view)
}

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

func SprintlnTable(w io.Writer, title string, table any) (string, error) {
	var b strings.Builder
	err := FprintlnTable(&b, title, table)
	return b.String(), err
}

func PrintlnTable(title string, table any) error {
	return FprintlnTable(os.Stdout, title, table)
}
