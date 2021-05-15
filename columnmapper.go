package retable

import (
	"fmt"
	"go/token"
	"reflect"
	"strings"
	"unicode"
)

// DefaultReflectColumnTitles provides the default ReflectColumnTitles
// using "col" as Tag and the SpacePascalCase function for UntaggedFieldTitle.
// Implements ColumnMapper.
var DefaultReflectColumnTitles = &ReflectColumnTitles{
	Tag:                "col",
	IgnoreTitle:        "-",
	UntaggedFieldTitle: SpacePascalCase,
}

// RowReflector is used to reflect column values from the fields of a struct
// representing a table row.
type RowReflector interface {
	// ReflectRow returns reflection values for struct fields
	// of structValue representing a table row.
	ReflectRow(structValue reflect.Value) (columnValues []reflect.Value)
}

// RowReflectorFunc implements RowReflector with a function
type RowReflectorFunc func(structValue reflect.Value) (columnValues []reflect.Value)

func (f RowReflectorFunc) ReflectRow(structValue reflect.Value) (columnValues []reflect.Value) {
	return f(structValue)
}

// ColumnMapper is used to map struct type fields to column names
type ColumnMapper interface {
	// ColumnTitlesAndRowReflector returns the column titles and indices for structFields.
	// The length of the titles and indices slices must be identical to the length of structFields.
	// The indices start at zero, the special index -1 filters removes the column
	// for the corresponding struct field.
	ColumnTitlesAndRowReflector(structType reflect.Type) (titles []string, rowReflector RowReflector)
}

// ColumnMapperFunc implements the ColumnMapper interface with a function
type ColumnMapperFunc func(structType reflect.Type) (titles []string, rowReflector RowReflector)

func (f ColumnMapperFunc) ColumnTitlesAndRowReflector(structType reflect.Type) (titles []string, rowReflector RowReflector) {
	return f(structType)
}

// ColumnTitles implements ColumnMapper by returning the underlying string slice as column titles
// and the StructFieldValues function of this package as RowReflector.
// It does not check if the number of column titles and the reflected row values are identical
// and re-mapping or ignoring of columns is not possible.
type ColumnTitles []string

func (t ColumnTitles) ColumnTitlesAndRowReflector(structType reflect.Type) (titles []string, rowReflector RowReflector) {
	return t, RowReflectorFunc(StructFieldValues)
}

// NoColumnTitles returns a ColumnMapper that returns nil as column titles
// and the StructFieldValues function of this package as RowReflector.
func NoColumnTitles() ColumnMapper {
	return noColumnTitles{}
}

// noColumnTitles implements ColumnMapper by returning nil as column titles
// and the StructFieldValues function of this package as RowReflector.
type noColumnTitles struct{}

func (noColumnTitles) ColumnTitlesAndRowReflector(structType reflect.Type) (titles []string, rowReflector RowReflector) {
	return nil, RowReflectorFunc(StructFieldValues)
}

// ReflectColumnTitles implements ColumnMapper with a struct field Tag
// to be used for naming and a UntaggedFieldTitle in case the Tag is not set.
type ReflectColumnTitles struct {
	// Tag is the struct field tag to be used as column name
	Tag string
	// IgnoreTitle will result in a column index of -1
	IgnoreTitle string
	// UntaggedFieldTitle will be called with the struct field name to
	// return a column name in case the struct field has no tag named Tag.
	// If UntaggedFieldTitle is nil, then the struct field name with be used unchanged.
	UntaggedFieldTitle func(fieldName string) (columnTitle string)
	// MapIndices is a map from the index of a field in struct
	// to the column index returned by ColumnTitlesAndRowReflector.
	// If MapIndices is nil, then no mapping will be performed.
	// Map to the index -1 to not create a column for a struct field.
	MapIndices map[int]int
}

func (n *ReflectColumnTitles) WithTag(tag string) *ReflectColumnTitles {
	mod := *n
	mod.Tag = tag
	return &mod
}

func (n *ReflectColumnTitles) WithIgnoreTitle(ignoreTitle string) *ReflectColumnTitles {
	mod := *n
	mod.IgnoreTitle = ignoreTitle
	return &mod
}

func (n *ReflectColumnTitles) WithMapIndex(fieldIndex, columnIndex int) *ReflectColumnTitles {
	mod := *n
	if mod.MapIndices == nil {
		mod.MapIndices = make(map[int]int)
	}
	mod.MapIndices[fieldIndex] = columnIndex
	return &mod
}

func (n *ReflectColumnTitles) WithIgnoreIndex(fieldIndex int) *ReflectColumnTitles {
	mod := *n
	if mod.MapIndices == nil {
		mod.MapIndices = make(map[int]int)
	}
	mod.MapIndices[fieldIndex] = -1
	return &mod
}

func (n *ReflectColumnTitles) WithMapIndices(mapIndices map[int]int) *ReflectColumnTitles {
	mod := *n
	mod.MapIndices = mapIndices
	return &mod
}

func (n *ReflectColumnTitles) ColumnTitlesAndRowReflector(structType reflect.Type) (titles []string, rowReflector RowReflector) {
	structFields := StructFieldTypes(structType)
	indices := make([]int, len(structFields))

	columnIndexUsed := make(map[int]bool)
	getNextFreeColumnIndex := func() int {
		for i := range structFields {
			if !columnIndexUsed[i] {
				return i
			}
		}
		panic("getNextFreeColumnIndex should always find a free column index")
	}

	for i, structField := range structFields {
		title := n.titleFromStructField(structField)
		if title == n.IgnoreTitle {
			indices[i] = -1
			continue
		}

		index := getNextFreeColumnIndex()
		if n.MapIndices != nil {
			mappedIndex, ok := n.MapIndices[i]
			if ok && !columnIndexUsed[mappedIndex] {
				index = mappedIndex
			}
		}
		if index < 0 || index >= len(structFields) {
			indices[i] = -1
			continue
		}

		indices[i] = index
		columnIndexUsed[index] = true

		titles = append(titles, title)
	}

	rowReflector = RowReflectorFunc(func(structValue reflect.Value) []reflect.Value {
		columnValues := make([]reflect.Value, len(titles))
		structFields := StructFieldValues(structValue)
		for i, index := range indices {
			if index >= 0 && index < len(titles) {
				columnValues[index] = structFields[i]
			}
		}
		return columnValues
	})

	return titles, rowReflector
}

func (n *ReflectColumnTitles) titleFromStructField(structField reflect.StructField) string {
	if tag, ok := structField.Tag.Lookup(n.Tag); ok {
		if i := strings.IndexByte(tag, ','); i != -1 {
			tag = tag[:i]
		}
		if tag != "" {
			return tag
		}
	}
	if n.UntaggedFieldTitle == nil {
		return structField.Name
	}
	return n.UntaggedFieldTitle(structField.Name)
}

func (n *ReflectColumnTitles) String() string {
	return fmt.Sprintf("Tag: %q, Ignore: %q", n.Tag, n.IgnoreTitle)
}

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
// Usable for ReflectColumnTitles.UntaggedFieldTitle
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
