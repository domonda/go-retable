package retable

import (
	"fmt"
	"reflect"
	"strings"
)

// Ensure StructRowsViewer implements Viewer
var _ Viewer = new(StructRowsViewer)

// StructRowsViewer implements Viewer for tables
// represented by a slice or array of struct rows.
type StructRowsViewer struct {
	// Tag is the struct field tag to be used as column title
	Tag string
	// Ignore will result in a column index of -1
	// for columns with that title
	Ignore string
	// Untagged will be called with the struct field name to
	// return a column title in case the struct field has no tag named Tag.
	// If Untagged is nil, then the struct field name with be used unchanged.
	Untagged func(fieldName string) (columnTitle string)
	// MapIndices is a map from the index of a field in struct
	// to the column index returned by StructFieldTypes.
	// If MapIndices is nil, then no mapping will be performed.
	// Mapping a struct field index to -1 will ignore this field
	// and not create a column for it..
	MapIndices map[int]int
}

func (viewer *StructRowsViewer) clone() *StructRowsViewer {
	c := new(StructRowsViewer)
	*c = *viewer
	c.MapIndices = make(map[int]int, len(viewer.MapIndices))
	for i, j := range viewer.MapIndices {
		c.MapIndices[i] = j
	}
	return c
}

// String implements the fmt.Stringer interface for StructRowsViewer.
func (viewer *StructRowsViewer) String() string {
	return fmt.Sprintf("StructRowsViewer{Tag: %q, Ignore: %q}", viewer.Tag, viewer.Ignore)
}

// NewView returns a View for a table made up of a slice
// or array of structs.
// NewView implements the Viewer interface for StructRowsViewer.
func (viewer *StructRowsViewer) NewView(table any) (View, error) {
	rows := reflect.ValueOf(table)
	for rows.Kind() == reflect.Pointer && !rows.IsNil() {
		rows = rows.Elem()
	}
	if rows.Kind() != reflect.Slice || rows.Kind() == reflect.Array {
		return nil, fmt.Errorf("table must be slice or array kind but is %T", table)
	}
	rowType := rows.Type().Elem()
	if rowType.Kind() == reflect.Pointer {
		rowType = rowType.Elem()
	}
	if rowType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("row type must be a struct but is %s", rowType)
	}

	structFields := StructFieldTypes(rowType)
	indices := make([]int, len(structFields))
	titles := make([]string, 0, len(structFields))

	columnIndexUsed := make(map[int]bool)
	getNextFreeColumnIndex := func() int {
		for i := range structFields {
			if !columnIndexUsed[i] {
				return i
			}
		}
		panic("getNextFreeColumnIndex should always find a free column index")
	}

	for i, structField := range structFields {
		title := viewer.titleFromStructField(structField)
		if title == viewer.Ignore {
			indices[i] = -1
			continue
		}

		index := getNextFreeColumnIndex()
		if viewer.MapIndices != nil {
			mappedIndex, ok := viewer.MapIndices[i]
			if ok && !columnIndexUsed[mappedIndex] {
				index = mappedIndex
			}
		}
		if index < 0 || index >= len(structFields) {
			indices[i] = -1
			continue
		}

		indices[i] = index
		columnIndexUsed[index] = true

		titles = append(titles, title)
	}

	return &structRowsView{titles, indices, rows}, nil
}

func (viewer *StructRowsViewer) titleFromStructField(structField reflect.StructField) string {
	if tag, ok := structField.Tag.Lookup(viewer.Tag); ok {
		if i := strings.IndexByte(tag, ','); i != -1 {
			tag = tag[:i]
		}
		if tag != "" {
			return tag
		}
	}
	if viewer.Untagged == nil {
		return structField.Name
	}
	return viewer.Untagged(structField.Name)
}

func (viewer *StructRowsViewer) WithTag(tag string) *StructRowsViewer {
	mod := viewer.clone()
	mod.Tag = tag
	return mod
}

func (viewer *StructRowsViewer) WithIgnore(ignore string) *StructRowsViewer {
	mod := viewer.clone()
	mod.Ignore = ignore
	return mod
}

func (viewer *StructRowsViewer) WithMapIndex(fieldIndex, columnIndex int) *StructRowsViewer {
	mod := viewer.clone()
	mod.MapIndices[fieldIndex] = columnIndex
	return mod
}

func (viewer *StructRowsViewer) WithIgnoreFieldIndex(fieldIndex int) *StructRowsViewer {
	mod := viewer.clone()
	mod.MapIndices[fieldIndex] = -1
	return mod
}

func (viewer *StructRowsViewer) WithIgnoreFieldIndices(fieldIndices ...int) *StructRowsViewer {
	mod := viewer.clone()
	for _, fieldIndex := range fieldIndices {
		mod.MapIndices[fieldIndex] = -1
	}
	return mod
}

func (viewer *StructRowsViewer) WithIgnoreField(structPtr, fieldPtr any) *StructRowsViewer {
	return viewer.WithIgnoreFieldIndex(MustStructFieldIndex(structPtr, fieldPtr))
}

func (viewer *StructRowsViewer) WithMapIndices(mapIndices map[int]int) *StructRowsViewer {
	mod := viewer.clone()
	mod.MapIndices = mapIndices
	return mod
}
