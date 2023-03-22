package retable

import (
	"context"
	"reflect"
)

var (
	// DefaultStructRowsViewer provides the default ReflectColumnTitles
	// using "col" as Tag and the SpacePascalCase function for UntaggedTitle.
	// Implements ColumnMapper.
	DefaultStructRowsViewer = &StructRowsViewer{
		Tag:               "col",
		IgnoreTitle:       "-",
		UntaggedTitleFunc: SpacePascalCase,
	}

	DefaultStructRowsViewerIgnoreUntagged = &StructRowsViewer{
		Tag:               "col",
		IgnoreTitle:       "-",
		UntaggedTitleFunc: UseTitle("-"),
	}

	// SelectViewer selects the best matching Viewer implementation
	// for the passed table.
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
