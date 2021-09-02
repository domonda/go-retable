package retable

import (
	"fmt"
	"reflect"
	"strings"
)

// Ensure ReflectColumnTitles implements ColumnMapper
var _ Viewer = new(StructRowsViewer)

// StructRowsViewer implements ColumnMapper with a struct field Tag
// to be used for naming and a UntaggedTitle in case the Tag is not set.
type StructRowsViewer struct {
	// Tag is the struct field tag to be used as column name
	Tag string
	// IgnoreTitle will result in a column index of -1
	IgnoreTitle string
	// UntaggedTitleFunc will be called with the struct field name to
	// return a column name in case the struct field has no tag named Tag.
	// If UntaggedTitleFunc is nil, then the struct field name with be used unchanged.
	UntaggedTitleFunc func(fieldName string) (columnTitle string)
	// MapIndices is a map from the index of a field in struct
	// to the column index returned by ColumnTitlesAndRowReflector.
	// If MapIndices is nil, then no mapping will be performed.
	// Map to the index -1 to not create a column for a struct field.
	MapIndices map[int]int
}

func (n *StructRowsViewer) WithTag(tag string) *StructRowsViewer {
	mod := new(StructRowsViewer)
	*mod = *n
	mod.Tag = tag
	return mod
}

func (n *StructRowsViewer) WithIgnoreTitle(ignoreTitle string) *StructRowsViewer {
	mod := new(StructRowsViewer)
	*mod = *n
	mod.IgnoreTitle = ignoreTitle
	return mod
}

func (n *StructRowsViewer) WithIgnoreTitleAndUntagged(ignoreTitle string) *StructRowsViewer {
	mod := new(StructRowsViewer)
	*mod = *n
	mod.IgnoreTitle = ignoreTitle
	mod.UntaggedTitleFunc = UseTitle(ignoreTitle)
	return mod
}

func (n *StructRowsViewer) WithMapIndex(fieldIndex, columnIndex int) *StructRowsViewer {
	mod := new(StructRowsViewer)
	*mod = *n
	// Clone MapIndices
	mod.MapIndices = make(map[int]int, len(n.MapIndices))
	for i, j := range n.MapIndices {
		mod.MapIndices[i] = j
	}
	mod.MapIndices[fieldIndex] = columnIndex
	return mod
}

func (n *StructRowsViewer) WithIgnoreIndex(fieldIndex int) *StructRowsViewer {
	mod := new(StructRowsViewer)
	*mod = *n
	// Clone MapIndices
	mod.MapIndices = make(map[int]int, len(n.MapIndices))
	for i, j := range n.MapIndices {
		mod.MapIndices[i] = j
	}
	mod.MapIndices[fieldIndex] = -1
	return mod
}

func (n *StructRowsViewer) WithMapIndices(mapIndices map[int]int) *StructRowsViewer {
	mod := new(StructRowsViewer)
	*mod = *n
	mod.MapIndices = mapIndices
	return mod
}

func (n *StructRowsViewer) NewView(table interface{}) (View, error) {
	rows := reflect.ValueOf(table)
	for rows.Kind() == reflect.Ptr && !rows.IsNil() {
		rows = rows.Elem()
	}
	if rows.Kind() != reflect.Slice || rows.Kind() == reflect.Array {
		return nil, fmt.Errorf("table must be slice or array kind but is %T", table)
	}
	structType := rows.Type().Elem()
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}
	if structType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("row type must be a struct but is %s", structType)
	}

	structFields := StructFieldTypes(structType)
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
		title := n.titleFromStructField(structField)
		if title == n.IgnoreTitle {
			indices[i] = -1
			continue
		}

		index := getNextFreeColumnIndex()
		if n.MapIndices != nil {
			mappedIndex, ok := n.MapIndices[i]
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

func (n *StructRowsViewer) titleFromStructField(structField reflect.StructField) string {
	if tag, ok := structField.Tag.Lookup(n.Tag); ok {
		if i := strings.IndexByte(tag, ','); i != -1 {
			tag = tag[:i]
		}
		if tag != "" {
			return tag
		}
	}
	if n.UntaggedTitleFunc == nil {
		return structField.Name
	}
	return n.UntaggedTitleFunc(structField.Name)
}

func (n *StructRowsViewer) String() string {
	return fmt.Sprintf("Tag: %q, Ignore: %q", n.Tag, n.IgnoreTitle)
}

type structRowsView struct {
	columns []string
	indices []int
	rows    reflect.Value
}

func (v *structRowsView) Columns() []string { return v.columns }
func (v *structRowsView) NumRows() int      { return v.rows.Len() }

func (v *structRowsView) ReflectRow(index int) ([]reflect.Value, error) {
	if index < 0 || index >= v.rows.Len() {
		return nil, fmt.Errorf("row index %d out of bounds [0..%d)", index, v.rows.Len())
	}
	columnValues := make([]reflect.Value, len(v.columns))
	structFields := StructFieldValues(v.rows.Index(index))
	for i, index := range v.indices {
		if index >= 0 && index < len(v.columns) {
			columnValues[index] = structFields[i]
		}
	}
	return columnValues, nil
}

/*

// RowReflector is used to reflect column values from the fields of a struct
// representing a table row.
type RowReflector interface {
	// ReflectRow returns reflection values for struct fields
	// of structValue representing a table row.
	ReflectRow(structValue reflect.Value) (columnValues []reflect.Value)
}

// RowReflectorFunc implements RowReflector with a function
type RowReflectorFunc func(structValue reflect.Value) (columnValues []reflect.Value)

func (f RowReflectorFunc) ReflectRow(structValue reflect.Value) (columnValues []reflect.Value) {
	return f(structValue)
}

// ColumnMapper is used to map struct type fields to column names
type ColumnMapper interface {
	// ColumnTitlesAndRowReflector returns the column titles and indices for structFields.
	// The length of the titles and indices slices must be identical to the length of structFields.
	// The indices start at zero, the special index -1 filters removes the column
	// for the corresponding struct field.
	ColumnTitlesAndRowReflector(structType reflect.Type) (titles []string, rowReflector RowReflector)
}

// ColumnMapperFunc implements the ColumnMapper interface with a function
type ColumnMapperFunc func(structType reflect.Type) (titles []string, rowReflector RowReflector)

func (f ColumnMapperFunc) ColumnTitlesAndRowReflector(structType reflect.Type) (titles []string, rowReflector RowReflector) {
	return f(structType)
}

// ColumnTitles implements ColumnMapper by returning the underlying string slice as column titles
// and the StructFieldValues function of this package as RowReflector.
// It does not check if the number of column titles and the reflected row values are identical
// and re-mapping or ignoring of columns is not possible.
type ColumnTitles []string

func (t ColumnTitles) ColumnTitlesAndRowReflector(structType reflect.Type) (titles []string, rowReflector RowReflector) {
	return t, RowReflectorFunc(StructFieldValues)
}

// NoColumnTitles returns a ColumnMapper that returns nil as column titles
// and the StructFieldValues function of this package as RowReflector.
func NoColumnTitles() ColumnMapper {
	return noColumnTitles{}
}

// noColumnTitles implements ColumnMapper by returning nil as column titles
// and the StructFieldValues function of this package as RowReflector.
type noColumnTitles struct{}

func (noColumnTitles) ColumnTitlesAndRowReflector(structType reflect.Type) (titles []string, rowReflector RowReflector) {
	return nil, RowReflectorFunc(StructFieldValues)
}
*/
