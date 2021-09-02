package retable

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
