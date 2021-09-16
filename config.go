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

	// DefaultViewer references DefaultStructRowsViewer by default
	// but can be changed to another Viewer implementation.
	DefaultViewer Viewer = DefaultStructRowsViewer
)

var (
	typeOfError   = reflect.TypeOf((*error)(nil)).Elem()
	typeOfContext = reflect.TypeOf((*context.Context)(nil)).Elem()
	typeOfCellPtr = reflect.TypeOf((*Cell)(nil))
)
