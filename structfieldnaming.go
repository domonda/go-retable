package retable

import (
	"fmt"
	"reflect"
	"strings"
)

var _ Viewer = new(StructFieldNaming)

// StructFieldNaming defines how struct fields are mapped to column titles
// as used by View. It provides flexible control over which struct fields
// become columns and what their column titles are through struct tags
// and custom naming functions.
//
// The mapping process follows these rules:
//   - Only exported (public) struct fields are considered for column mapping
//   - Anonymous embedded struct fields are recursively processed, with their fields inlined
//   - If a Tag is specified, the struct tag value is used as the column title
//   - For fields without tags (or if Tag is empty), the Untagged function determines the column title
//   - Fields matching the Ignore value are excluded from the table
//
// Example usage with struct tags:
//
//	type Person struct {
//	    FirstName string `csv:"first_name"`
//	    LastName  string `csv:"last_name"`
//	    Age       int    `csv:"age"`
//	    Internal  string `csv:"-"` // ignored when Ignore is set to "-"
//	}
//
//	naming := &StructFieldNaming{
//	    Tag:    "csv",
//	    Ignore: "-",
//	}
//	columns := naming.Columns(&Person{}) // ["first_name", "last_name", "age"]
//
// Example with custom naming function:
//
//	naming := &StructFieldNaming{
//	    Untagged: SpacePascalCase, // "FirstName" becomes "First Name"
//	}
//
// A nil *StructFieldNaming is a valid value and is equivalent to the zero value,
// which will use all exported struct fields with their field name as column title.
//
// StructFieldNaming implements the Viewer interface, allowing it to create
// Views from slices or arrays of structs.
type StructFieldNaming struct {
	// Tag is the struct field tag to be used as column title.
	// When non-empty, struct fields are checked for this tag name,
	// and the tag value is used as the column title.
	//
	// Tag values support comma-separated options, where only the first
	// part before the comma is used as the column title:
	//   `json:"name,omitempty"` results in column title "name"
	//
	// If Tag is empty, then every struct field will be treated as untagged.
	Tag string

	// Ignore specifies a column title value that marks fields to be excluded.
	// Any field whose column title matches this value will be ignored and
	// not appear in the resulting table view.
	//
	// Common patterns:
	//   - Use "-" to match the convention `json:"-"` for ignored fields
	//   - Use "" (empty string) to ignore all unexported or empty-titled fields
	Ignore string

	// Untagged is called with the struct field name to generate a column title
	// when the field has no tag matching Tag (or Tag is empty).
	//
	// If Untagged is nil, the raw struct field name is used as the column title.
	//
	// Common functions to use:
	//   - SpacePascalCase: "FirstName" -> "First Name"
	//   - SpaceGoCase: "HTTPServer" -> "HTTP Server"
	//   - Custom function for domain-specific naming conventions
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

// StructFieldColumn returns the column title for a struct field using
// the naming rules defined in the StructFieldNaming configuration.
//
// The function applies the following logic:
//   - Returns empty string for unexported or anonymous fields
//   - If Tag is set and the field has that tag, uses the tag value (before any comma)
//   - Otherwise calls Untagged function if set, or uses the field name
//
// This method is safe to call with a nil receiver, in which case it returns
// the field name for exported fields and empty string for unexported fields.
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

// IsIgnored returns true if the given column title should be ignored
// and excluded from the table view.
//
// A column is considered ignored if:
//   - The column title is an empty string
//   - The column title matches the Ignore field value (when n is not nil)
//
// This method is safe to call with a nil receiver.
func (n *StructFieldNaming) IsIgnored(column string) bool {
	return column == "" || (n != nil && column == n.Ignore)
}

// ColumnStructFieldValue returns the reflect.Value of the struct field
// that is mapped to the given column title.
//
// The function searches through all exported struct fields (including
// those from anonymously embedded structs) to find the field whose
// column title matches the given column parameter.
//
// Anonymous embedded structs are recursively searched, allowing fields
// from nested structs to be accessed by their column title.
//
// Returns an invalid reflect.Value if:
//   - The column is ignored (empty or matches Ignore)
//   - No struct field maps to the given column title
//   - structVal is not a struct or pointer to struct
//
// Panics if structVal is not a struct or pointer to struct type.
//
// This method is safe to call with a nil receiver.
//
// Example:
//
//	type Person struct {
//	    Name string `csv:"full_name"`
//	    Age  int    `csv:"age"`
//	}
//	naming := &StructFieldNaming{Tag: "csv"}
//	person := Person{Name: "John", Age: 30}
//	val := naming.ColumnStructFieldValue(reflect.ValueOf(person), "full_name")
//	// val.String() == "John"
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

// Columns returns the column titles for all non-ignored exported fields
// of a struct or pointer to a struct.
//
// The returned slice contains column titles in the order they appear in
// the struct definition, with fields from anonymously embedded structs
// inlined in their position.
//
// Fields are processed according to the StructFieldNaming rules:
//   - Tag determines which struct tag to read for column titles
//   - Untagged function transforms field names when no tag is present
//   - Ignore value filters out unwanted columns
//
// Panics if strct is not a struct or pointer to struct type.
//
// This method is safe to call with a nil receiver, in which case it
// returns all exported field names without transformation.
//
// Example:
//
//	type User struct {
//	    ID       int    `db:"user_id"`
//	    Username string `db:"username"`
//	    Password string `db:"-"`
//	}
//	naming := &StructFieldNaming{Tag: "db", Ignore: "-"}
//	cols := naming.Columns(&User{}) // ["user_id", "username"]
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

// NewView returns a View for a table made up of a slice or array of structs.
//
// This method implements the Viewer interface for StructFieldNaming, creating
// a StructRowsView by delegating to StructRowsViewer with this naming configuration.
//
// Parameters:
//   - title: The title for the table view
//   - table: A slice or array of structs (or pointers to structs)
//
// Returns an error if table is not a slice or array, or if the element type
// is not a struct or pointer to struct.
//
// Example:
//
//	type Product struct {
//	    Name  string `json:"name"`
//	    Price float64 `json:"price"`
//	}
//	naming := &StructFieldNaming{Tag: "json"}
//	products := []Product{{"Widget", 9.99}, {"Gadget", 19.99}}
//	view, err := naming.NewView("Products", products)
func (n *StructFieldNaming) NewView(title string, table any) (View, error) {
	viewer := StructRowsViewer{StructFieldNaming: *n}
	return viewer.NewView(title, table)
}

// WithTag returns a copy of the StructFieldNaming with the Tag field set
// to the specified value.
//
// This method allows fluent configuration of the naming rules.
//
// Example:
//
//	naming := (&StructFieldNaming{}).WithTag("json").WithIgnore("-")
func (n *StructFieldNaming) WithTag(tag string) *StructFieldNaming {
	mod := *n
	mod.Tag = tag
	return &mod
}

// WithIgnore returns a copy of the StructFieldNaming with the Ignore field
// set to the specified value.
//
// This method allows fluent configuration of the naming rules.
//
// Example:
//
//	naming := (&StructFieldNaming{Tag: "csv"}).WithIgnore("-")
func (n *StructFieldNaming) WithIgnore(ignore string) *StructFieldNaming {
	mod := *n
	mod.Ignore = ignore
	return &mod
}
