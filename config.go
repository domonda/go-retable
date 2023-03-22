package retable

import (
	"context"
	"reflect"
)

var (
	// DefaultStructRowsViewer provides the default StructRowsViewer
	// using "col" as title tag, ignores "-" titled fields,
	// and uses SpacePascalCase for untagged fields.
	DefaultStructRowsViewer = &StructRowsViewer{
		Tag:      "col",
		Ignore:   "-",
		Untagged: SpacePascalCase,
	}

	// DefaultStructRowsViewerIgnoreUntagged provides the default StructRowsViewer
	// using "col" as title tag, ignores "-" titled and untitled fields.
	DefaultStructRowsViewerIgnoreUntagged = &StructRowsViewer{
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
		return DefaultStructRowsViewer, nil
	}
)

var (
	typeOfError   = reflect.TypeOf((*error)(nil)).Elem()
	typeOfContext = reflect.TypeOf((*context.Context)(nil)).Elem()
	typeOfCellPtr = reflect.TypeOf((*Cell)(nil))
)
