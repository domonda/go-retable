package retable

import (
	"context"
	"reflect"
	"time"
)

var (
	// DefaultStructFieldNaming provides the default StructFieldNaming
	// using "col" as title tag, ignores "-" titled fields,
	// and uses SpacePascalCase for untagged fields.
	// Implements the Viewer interface.
	DefaultStructFieldNaming = StructFieldNaming{
		Tag:      "col",
		Ignore:   "-",
		Untagged: SpacePascalCase,
	}

	// DefaultStructFieldNamingIgnoreUntagged provides the default StructFieldNaming
	// using "col" as title tag, ignores "-" titled as well as untitled fields.
	// Implements the Viewer interface.
	DefaultStructFieldNamingIgnoreUntagged = StructFieldNaming{
		Tag:      "col",
		Ignore:   "-",
		Untagged: UseTitle("-"),
	}

	// SelectViewer selects the best matching Viewer implementation
	// for the passed table type.
	// By default it returns a StringsViewer for a [][]string table
	// and the DefaultStructRowsViewer for all other cases.
	SelectViewer = func(table any) (Viewer, error) {
		if _, ok := table.([][]string); ok {
			return new(StringsViewer), nil
		}
		return &DefaultStructFieldNaming, nil
	}
)

// DefaultStructRowsViewer returns a StructRowsViewer
// that uses DefaultStructFieldNaming and no MapIndices.
func DefaultStructRowsViewer() *StructRowsViewer {
	return &StructRowsViewer{StructFieldNaming: DefaultStructFieldNaming}
}

// NoTagsStructRowsViewer returns a StructRowsViewer
// that uses the struct field names as column titles
// without considering struct field tags.
func NoTagsStructRowsViewer() *StructRowsViewer {
	return &StructRowsViewer{}
}

var (
	typeOfError       = reflect.TypeOf((*error)(nil)).Elem()
	typeOfContext     = reflect.TypeOf((*context.Context)(nil)).Elem()
	typeOfTime        = reflect.TypeOf(time.Time{})
	typeOfEmptyStruct = reflect.TypeOf(struct{}{})
)
