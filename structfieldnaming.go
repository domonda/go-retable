package retable

import (
	"fmt"
	"reflect"
	"strings"
)

var _ Viewer = new(StructFieldNaming)

// StructFieldNaming defines how struct fields
// are mapped to column titles as used by View.
//
// nil is a valid value for *StructFieldNaming
// and is equal to the zero value
// which will use all exported struct fields
// with their field name as column title.
//
// StructFieldNaming implements the Viewer interface.
type StructFieldNaming struct {
	// Tag is the struct field tag to be used as column title.
	// If Tag is empty, then every struct field will be treated as untagged.
	Tag string
	// Ignore will result in a column index of -1
	// for columns with that title
	Ignore string
	// Untagged will be called with the struct field name to
	// return a title in case the struct field has no tag named Tag.
	// If Untagged is nil, then the struct field name will be used.
	Untagged func(fieldName string) (column string)
}

// String implements the fmt.Stringer interface for StructFieldNaming.
//
// Valid to call with nil receiver.
func (n *StructFieldNaming) String() string {
	if n == nil {
		return `StructFieldNaming{Tag: "", Ignore: ""}`
	}
	return fmt.Sprintf("StructFieldNaming{Tag: %#v, Ignore: %#v}", n.Tag, n.Ignore)
}

// StructFieldColumn returns the column title for a struct field.
//
// Valid to call with nil receiver.
func (n *StructFieldNaming) StructFieldColumn(field reflect.StructField) string {
	if !field.IsExported() || field.Anonymous {
		return ""
	}
	if n == nil {
		return field.Name
	}
	if n.Tag != "" {
		if tag, ok := field.Tag.Lookup(n.Tag); ok {
			if i := strings.IndexByte(tag, ','); i != -1 {
				tag = tag[:i]
			}
			if tag != "" {
				return tag
			}
		}
	}
	if n.Untagged == nil {
		return field.Name
	}
	return n.Untagged(field.Name)
}

func (n *StructFieldNaming) IsIgnored(column string) bool {
	return column == "" || (n != nil && column == n.Ignore)
}

// ColumnStructFieldValue returns the reflect.Value of the struct field
// that is mapped to the column title.
//
// Valid to call with nil receiver.
func (n *StructFieldNaming) ColumnStructFieldValue(structVal reflect.Value, column string) reflect.Value {
	if n.IsIgnored(column) {
		return reflect.Value{}
	}
	if structVal.Kind() == reflect.Pointer {
		structVal = structVal.Elem()
	}
	structType := structVal.Type()
	if structType.Kind() != reflect.Struct {
		panic("expected struct or pointer to struct instead of " + structVal.Type().String())
	}
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.Anonymous {
			// Recurse into anonymous embedded structs
			if v := n.ColumnStructFieldValue(structVal.Field(i), column); v.IsValid() {
				return v
			}
			continue
		}
		if n.StructFieldColumn(field) == column {
			return structVal.Field(i)
		}
	}
	return reflect.Value{}
}

// Columns returns the column titles for a struct
// or a pointer to a struct.
//
// It panics for non struct or struct pointer types.
//
// Valid to call with nil receiver.
func (n *StructFieldNaming) Columns(strct any) []string {
	return n.columns(reflect.TypeOf(strct))
}

func (n *StructFieldNaming) columns(t reflect.Type) []string {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic("expected struct or pointer to struct instead of " + t.String())
	}
	columns := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		field := t.Field(i)
		if field.Anonymous {
			// Recurse into anonymous embedded structs
			columns = append(columns, n.columns(field.Type)...)
			continue
		}
		column := n.StructFieldColumn(field)
		if !n.IsIgnored(column) {
			columns = append(columns, column)
		}
	}
	return columns
}

// NewView returns a View for a table made up of
// a slice or array of structs.
// NewView implements the Viewer interface for StructFieldNaming.
func (n *StructFieldNaming) NewView(title string, table any) (View, error) {
	viewer := StructRowsViewer{StructFieldNaming: *n}
	return viewer.NewView(title, table)
}

func (n *StructFieldNaming) WithTag(tag string) *StructFieldNaming {
	mod := *n
	mod.Tag = tag
	return &mod
}

func (n *StructFieldNaming) WithIgnore(ignore string) *StructFieldNaming {
	mod := *n
	mod.Ignore = ignore
	return &mod
}
