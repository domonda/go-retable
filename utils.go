package retable

import (
	"go/token"
	"reflect"
	"strings"
	"unicode"
)

// StructFieldTypes returns the exported fields of a struct type
// including the inlined fields of any anonymously embedded structs.
func StructFieldTypes(structType reflect.Type) (fields []reflect.StructField) {
	if structType.Kind() == reflect.Ptr {
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

// StructFieldValues returns the reflect.Value of exported struct fields
// including the inlined fields of any anonymously embedded structs.
func StructFieldValues(structValue reflect.Value) (values []reflect.Value) {
	if structValue.Kind() == reflect.Ptr {
		structValue = structValue.Elem()
	}
	structType := structValue.Type()
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		switch {
		case field.Anonymous:
			values = append(values, StructFieldValues(structValue.Field(i))...)
		case token.IsExported(field.Name):
			values = append(values, structValue.Field(i))
		}
	}
	return values
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

// UseTitle returns a function that
// always returns the passed columnTitle.
func UseTitle(columnTitle string) func(fieldName string) (columnTitle string) {
	return func(string) string { return columnTitle }
}

// ValueIsNil return true if passed reflect.Value
// is not valid, nil (of a type that can be nil),
// or is of type struct{}
func ValueIsNil(val reflect.Value) bool {
	if !val.IsValid() {
		return true
	}
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map,
		reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return val.IsNil()
	case reflect.Struct:
		if t := val.Type(); t.NumField() == 0 && t.NumMethod() == 0 {
			// Treat a value of type struct{} like nil
			return true
		}
	}
	return false
}
