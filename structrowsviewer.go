package retable

import (
	"fmt"
	"reflect"
)

// Ensure StructRowsViewer implements Viewer
var _ Viewer = new(StructRowsViewer)

// StructRowsViewer implements Viewer for tables
// represented by a slice or array of struct rows.
type StructRowsViewer struct {
	StructFieldNaming

	// MapIndices is a map from the index of a field in struct
	// to the column index returned by the function StructFieldTypes.
	// If MapIndices is nil, then no mapping will be performed.
	// Mapping a struct field index to -1 will ignore this field
	// and not create a column for it..
	MapIndices map[int]int
}

func (v *StructRowsViewer) clone() *StructRowsViewer {
	c := new(StructRowsViewer)
	*c = *v
	c.MapIndices = make(map[int]int, len(v.MapIndices))
	for i, j := range v.MapIndices {
		c.MapIndices[i] = j
	}
	return c
}

// String implements the fmt.Stringer interface for StructRowsViewer.
func (v *StructRowsViewer) String() string {
	return fmt.Sprintf("StructRowsViewer{Tag: %q, Ignore: %q}", v.Tag, v.Ignore)
}

// NewView returns a View for a table made up of
// a slice or array of structs.
// NewView implements the Viewer interface for StructRowsViewer.
func (v *StructRowsViewer) NewView(title string, table any) (View, error) {
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
	columns := make([]string, 0, len(structFields))

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
		column := v.StructFieldColumn(structField)
		if column == v.Ignore {
			indices[i] = -1
			continue
		}

		index := getNextFreeColumnIndex()
		if v.MapIndices != nil {
			mappedIndex, ok := v.MapIndices[i]
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

		columns = append(columns, column)
	}

	return NewStructRowsView(title, columns, indices, rows), nil
}

func (v *StructRowsViewer) WithTag(tag string) *StructRowsViewer {
	mod := v.clone()
	mod.Tag = tag
	return mod
}

func (v *StructRowsViewer) WithIgnore(ignore string) *StructRowsViewer {
	mod := v.clone()
	mod.Ignore = ignore
	return mod
}

func (v *StructRowsViewer) WithMapIndex(fieldIndex, columnIndex int) *StructRowsViewer {
	mod := v.clone()
	mod.MapIndices[fieldIndex] = columnIndex
	return mod
}

func (v *StructRowsViewer) WithIgnoreFieldIndex(fieldIndex int) *StructRowsViewer {
	mod := v.clone()
	mod.MapIndices[fieldIndex] = -1
	return mod
}

func (v *StructRowsViewer) WithIgnoreFieldIndices(fieldIndices ...int) *StructRowsViewer {
	mod := v.clone()
	for _, fieldIndex := range fieldIndices {
		mod.MapIndices[fieldIndex] = -1
	}
	return mod
}

func (v *StructRowsViewer) WithIgnoreField(structPtr, fieldPtr any) *StructRowsViewer {
	return v.WithIgnoreFieldIndex(MustStructFieldIndex(structPtr, fieldPtr))
}

func (v *StructRowsViewer) WithMapIndices(mapIndices map[int]int) *StructRowsViewer {
	mod := v.clone()
	mod.MapIndices = mapIndices
	return mod
}
