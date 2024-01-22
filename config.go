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
	DefaultStructFieldNaming = StructFieldNaming{
		Tag:      "col",
		Ignore:   "-",
		Untagged: SpacePascalCase,
	}

	// DefaultStructRowsViewer provides the default StructRowsViewer
	// using "col" as title tag, ignores "-" titled fields,
	// and uses SpacePascalCase for untagged fields.
	DefaultStructRowsViewer = &StructRowsViewer{
		StructFieldNaming: DefaultStructFieldNaming,
	}

	// DefaultStructFieldNamingIgnoreUntagged provides the default StructFieldNaming
	// using "col" as title tag, ignores "-" titled as well as untitled fields.
	DefaultStructFieldNamingIgnoreUntagged = StructFieldNaming{
		Tag:      "col",
		Ignore:   "-",
		Untagged: UseTitle("-"),
	}

	// DefaultStructRowsViewerIgnoreUntagged provides the default StructRowsViewer
	// using "col" as title tag, ignores "-" titled as well as untitled fields.
	DefaultStructRowsViewerIgnoreUntagged = &StructRowsViewer{
		StructFieldNaming: DefaultStructFieldNamingIgnoreUntagged,
	}

	// SelectViewer selects the best matching Viewer implementation
	// for the passed table type.
	// By default it returns a StringsViewer for a [][]string table
	// and the DefaultStructRowsViewer for all other cases.
	SelectViewer = func(table any) (Viewer, error) {
		if _, ok := table.([][]string); ok {
			return new(StringsViewer), nil
		}
		return DefaultStructRowsViewer, nil
	}

	noTagsStructRowsViewer StructRowsViewer
)

// NoTagsStructRowsViewer returns a viewer
// that uses the struct field names as column titles
// without considering struct field tags.
func NoTagsStructRowsViewer() Viewer {
	return &noTagsStructRowsViewer
}

var (
	typeOfError       = reflect.TypeOf((*error)(nil)).Elem()
	typeOfContext     = reflect.TypeOf((*context.Context)(nil)).Elem()
	typeOfView        = reflect.TypeOf((*View)(nil)).Elem()
	typeOfTime        = reflect.TypeOf(time.Time{})
	typeOfEmptyStruct = reflect.TypeOf(struct{}{})
)
