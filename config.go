// Package retable provides utilities for working with tabular data in Go,
// including formatting structs and slices as tables with customizable column naming,
// cell formatting, and output rendering.
package retable

import (
	"context"
	"reflect"
	"time"
)

var (
	// DefaultStructFieldNaming provides the default StructFieldNaming configuration
	// for converting struct fields to table columns.
	//
	// Configuration:
	//   - Uses "col" as the struct tag to read column titles
	//   - Ignores fields tagged with "-"
	//   - Uses SpacePascalCase to convert untagged field names to titles
	//
	// This is the recommended configuration for most use cases and implements the Viewer interface.
	//
	// Example:
	//
	//	type Person struct {
	//	    FirstName string `col:"First Name"`
	//	    LastName  string `col:"Last Name"`
	//	    Age       int    `col:"Age"`
	//	    password  string `col:"-"` // ignored (also unexported)
	//	}
	DefaultStructFieldNaming = StructFieldNaming{
		Tag:      "col",
		Ignore:   "-",
		Untagged: SpacePascalCase,
	}

	// DefaultStructFieldNamingIgnoreUntagged provides a StructFieldNaming configuration
	// that only includes explicitly tagged fields.
	//
	// Configuration:
	//   - Uses "col" as the struct tag to read column titles
	//   - Ignores fields tagged with "-"
	//   - Ignores all untagged fields (by treating them as "-")
	//
	// This is useful when you want strict control over which fields are included in the table,
	// requiring explicit opt-in via struct tags. Implements the Viewer interface.
	//
	// Example:
	//
	//	type Person struct {
	//	    FirstName string `col:"First Name"` // included
	//	    LastName  string `col:"Last Name"`  // included
	//	    Age       int                       // excluded (no tag)
	//	    internal  string `col:"-"`          // excluded (tagged with "-")
	//	}
	DefaultStructFieldNamingIgnoreUntagged = StructFieldNaming{
		Tag:      "col",
		Ignore:   "-",
		Untagged: UseTitle("-"),
	}

	// SelectViewer is a function variable that selects the most appropriate Viewer implementation
	// for a given table type.
	//
	// Default behavior:
	//   - Returns StringsViewer for [][]string tables
	//   - Returns DefaultStructFieldNaming (as Viewer) for all other types
	//
	// This variable can be reassigned to customize viewer selection logic.
	// The function should return an error if the table type is not supported.
	//
	// Example custom selector:
	//
	//	SelectViewer = func(table any) (Viewer, error) {
	//	    switch table.(type) {
	//	    case [][]string:
	//	        return new(StringsViewer), nil
	//	    case []MyCustomType:
	//	        return &MyCustomViewer{}, nil
	//	    default:
	//	        return &DefaultStructFieldNaming, nil
	//	    }
	//	}
	SelectViewer = func(table any) (Viewer, error) {
		if _, ok := table.([][]string); ok {
			return new(StringsViewer), nil
		}
		return &DefaultStructFieldNaming, nil
	}
)

// DefaultStructRowsViewer returns a new StructRowsViewer configured with DefaultStructFieldNaming.
//
// The returned viewer:
//   - Uses "col" struct tag for column titles
//   - Ignores fields tagged with "-"
//   - Uses SpacePascalCase for untagged field names
//   - Has no column index mapping (MapIndices is nil)
//
// This is a convenience function for creating a standard viewer for struct slices.
//
// Example:
//
//	type Person struct {
//	    Name string `col:"Full Name"`
//	    Age  int
//	}
//	viewer := DefaultStructRowsViewer()
//	view, err := viewer.NewView("People", []Person{{Name: "John", Age: 30}})
func DefaultStructRowsViewer() *StructRowsViewer {
	return &StructRowsViewer{StructFieldNaming: DefaultStructFieldNaming}
}

// NoTagsStructRowsViewer returns a new StructRowsViewer that ignores struct tags
// and uses raw field names as column titles.
//
// The returned viewer:
//   - Does not read any struct tags
//   - Uses the exact struct field names as column titles (e.g., "FirstName", "LastName")
//   - Has no column index mapping (MapIndices is nil)
//
// This is useful when you want to quickly display struct data without setting up tags,
// or when the raw field names are already suitable as column headers.
//
// Example:
//
//	type Person struct {
//	    Name string
//	    Age  int
//	}
//	viewer := NoTagsStructRowsViewer()
//	view, err := viewer.NewView("People", []Person{{Name: "John", Age: 30}})
//	// Column titles will be: "Name", "Age"
func NoTagsStructRowsViewer() *StructRowsViewer {
	return &StructRowsViewer{}
}

var (
	// typeOfError is the reflect.Type of the error interface.
	// Used internally to identify error types during reflection operations.
	typeOfError = reflect.TypeOf((*error)(nil)).Elem()

	// typeOfContext is the reflect.Type of the context.Context interface.
	// Used internally to identify context types during reflection operations.
	typeOfContext = reflect.TypeOf((*context.Context)(nil)).Elem()

	// typeOfTime is the reflect.Type of time.Time.
	// Used internally to identify time.Time values for special formatting.
	typeOfTime = reflect.TypeOf(time.Time{})

	// typeOfEmptyStruct is the reflect.Type of struct{}.
	// Used internally to identify empty structs, which are often treated as null-like values.
	typeOfEmptyStruct = reflect.TypeOf(struct{}{})
)
