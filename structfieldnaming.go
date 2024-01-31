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
func (n *StructFieldNaming) String() string {
	if n == nil {
		return `StructFieldNaming{Tag: "", Ignore: ""}`
	}
	return fmt.Sprintf("StructFieldNaming{Tag: %#v, Ignore: %#v}", n.Tag, n.Ignore)
}

// StructFieldColumn returns the column title for a struct field.
func (n *StructFieldNaming) StructFieldColumn(structField reflect.StructField) string {
	if n == nil {
		return structField.Name
	}
	if n.Tag != "" {
		if tag, ok := structField.Tag.Lookup(n.Tag); ok {
			if i := strings.IndexByte(tag, ','); i != -1 {
				tag = tag[:i]
			}
			if tag != "" {
				return tag
			}
		}
	}
	if n.Untagged == nil {
		return structField.Name
	}
	return n.Untagged(structField.Name)
}

func (n *StructFieldNaming) ColumnStructFieldValue(strct reflect.Value, column string) reflect.Value {
	strctType := strct.Type()
	for i := 0; i < strctType.NumField(); i++ {
		if n.StructFieldColumn(strctType.Field(i)) == column {
			return strct.Field(i)
		}
	}
	return reflect.Value{}
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
