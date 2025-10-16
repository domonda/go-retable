package retable

import (
	"fmt"
	"reflect"
)

// Ensure StructRowsViewer implements Viewer
var _ Viewer = new(StructRowsViewer)

// StructRowsViewer implements Viewer for tables represented by a slice or array
// of struct rows, with advanced control over column ordering and field inclusion.
//
// StructRowsViewer extends StructFieldNaming with the ability to rearrange columns
// and selectively exclude specific struct fields by their index. This is useful when:
//   - You need to reorder columns without changing the struct definition
//   - You want to exclude specific fields beyond what struct tags provide
//   - You need programmatic control over which fields appear in the view
//
// The viewer uses reflection to extract field values from each struct in the slice,
// mapping them to columns according to the StructFieldNaming rules and MapIndices
// configuration.
//
// Column mapping process:
//  1. All exported struct fields are identified (including embedded struct fields)
//  2. StructFieldNaming rules determine the column title for each field
//  3. MapIndices (if set) remaps field indices to column positions
//  4. Fields mapped to -1 are excluded from the output
//  5. A StructRowsView is created to provide access to the data
//
// Example with field reordering:
//
//	type Employee struct {
//	    ID        int
//	    FirstName string
//	    LastName  string
//	    Salary    float64
//	}
//
//	viewer := &StructRowsViewer{
//	    StructFieldNaming: StructFieldNaming{
//	        Untagged: SpacePascalCase,
//	    },
//	    MapIndices: map[int]int{
//	        0: 2,  // ID appears in 3rd column
//	        1: 0,  // FirstName appears in 1st column
//	        2: 1,  // LastName appears in 2nd column
//	        3: -1, // Salary is excluded
//	    },
//	}
//	// Columns will be: ["First Name", "Last Name", "ID"]
//
// Example with field exclusion:
//
//	viewer := &StructRowsViewer{}
//	viewer = viewer.WithIgnoreFieldIndex(3) // Ignore field at index 3
//
// StructRowsViewer implements the Viewer interface.
type StructRowsViewer struct {
	StructFieldNaming

	// MapIndices maps struct field indices to column indices in the output view.
	//
	// The key is the index of a field as returned by StructFieldTypes (exported
	// fields in declaration order, with embedded struct fields inlined).
	// The value is the column index where that field's data should appear.
	//
	// If MapIndices is nil, fields map 1:1 to columns in their natural order.
	//
	// Mapping a field index to -1 excludes that field from the view entirely.
	//
	// All column indices from 0 to N-1 must be mapped exactly once (excluding -1),
	// where N is the number of columns in the output.
	//
	// Example:
	//   MapIndices: map[int]int{
	//       0: 1,  // field 0 -> column 1
	//       1: 0,  // field 1 -> column 0 (swap with field 0)
	//       2: -1, // field 2 -> excluded
	//       3: 2,  // field 3 -> column 2
	//   }
	MapIndices map[int]int
}

// clone creates a deep copy of the StructRowsViewer, including a copy
// of the MapIndices map. This is used internally by the With* methods
// to ensure immutability when modifying configuration.
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

// NewView returns a View for a table made up of a slice or array of structs.
//
// This method implements the Viewer interface for StructRowsViewer. It uses
// reflection to analyze the struct type, apply the naming configuration,
// and create a StructRowsView that provides efficient access to the data.
//
// The method performs the following steps:
//  1. Validates that table is a slice or array
//  2. Extracts the element type and verifies it's a struct
//  3. Gets all exported struct fields using StructFieldTypes
//  4. Applies StructFieldNaming rules to determine column titles
//  5. Applies MapIndices remapping if configured
//  6. Creates and returns a StructRowsView with the computed mapping
//
// Parameters:
//   - title: The title for the table view
//   - table: Must be a slice or array of structs (or pointers to structs)
//
// Returns an error if:
//   - table is not a slice or array type
//   - the element type is not a struct or pointer to struct
//
// Example:
//
//	type Order struct {
//	    OrderID   int    `csv:"id"`
//	    Customer  string `csv:"customer"`
//	    Total     float64 `csv:"total"`
//	}
//	viewer := &StructRowsViewer{
//	    StructFieldNaming: StructFieldNaming{Tag: "csv"},
//	}
//	orders := []Order{{1, "Alice", 99.99}, {2, "Bob", 149.99}}
//	view, err := viewer.NewView("Orders", orders)
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

// WithTag returns a copy of the StructRowsViewer with the Tag field set
// to the specified value.
//
// This method allows fluent configuration of the viewer.
//
// Example:
//
//	viewer := (&StructRowsViewer{}).WithTag("json").WithIgnore("-")
func (v *StructRowsViewer) WithTag(tag string) *StructRowsViewer {
	mod := v.clone()
	mod.Tag = tag
	return mod
}

// WithIgnore returns a copy of the StructRowsViewer with the Ignore field
// set to the specified value.
//
// This method allows fluent configuration of the viewer.
//
// Example:
//
//	viewer := (&StructRowsViewer{Tag: "csv"}).WithIgnore("-")
func (v *StructRowsViewer) WithIgnore(ignore string) *StructRowsViewer {
	mod := v.clone()
	mod.Ignore = ignore
	return mod
}

// WithMapIndex returns a copy of the StructRowsViewer with an additional
// field-to-column mapping added to MapIndices.
//
// The fieldIndex is the index of the struct field (as returned by StructFieldTypes),
// and columnIndex is the desired column position in the output (or -1 to exclude).
//
// Example:
//
//	viewer := (&StructRowsViewer{}).
//	    WithMapIndex(0, 2).  // field 0 -> column 2
//	    WithMapIndex(1, 0).  // field 1 -> column 0
//	    WithMapIndex(2, 1)   // field 2 -> column 1
func (v *StructRowsViewer) WithMapIndex(fieldIndex, columnIndex int) *StructRowsViewer {
	mod := v.clone()
	mod.MapIndices[fieldIndex] = columnIndex
	return mod
}

// WithIgnoreFieldIndex returns a copy of the StructRowsViewer with the specified
// field index mapped to -1, effectively excluding it from the view.
//
// This is a convenience method equivalent to WithMapIndex(fieldIndex, -1).
//
// The fieldIndex is the index of the struct field as returned by StructFieldTypes.
//
// Example:
//
//	// Exclude the 3rd field (index 2)
//	viewer := (&StructRowsViewer{}).WithIgnoreFieldIndex(2)
func (v *StructRowsViewer) WithIgnoreFieldIndex(fieldIndex int) *StructRowsViewer {
	mod := v.clone()
	mod.MapIndices[fieldIndex] = -1
	return mod
}

// WithIgnoreFieldIndices returns a copy of the StructRowsViewer with multiple
// field indices mapped to -1, effectively excluding them from the view.
//
// This is a convenience method for excluding multiple fields at once.
//
// Example:
//
//	// Exclude fields at indices 2, 5, and 7
//	viewer := (&StructRowsViewer{}).WithIgnoreFieldIndices(2, 5, 7)
func (v *StructRowsViewer) WithIgnoreFieldIndices(fieldIndices ...int) *StructRowsViewer {
	mod := v.clone()
	for _, fieldIndex := range fieldIndices {
		mod.MapIndices[fieldIndex] = -1
	}
	return mod
}

// WithIgnoreField returns a copy of the StructRowsViewer with the specified
// struct field excluded from the view.
//
// This method uses reflection to determine the field index by comparing
// pointer addresses. It requires pointers to both a struct instance and
// one of its fields.
//
// Panics if:
//   - structPtr is not a pointer to a struct
//   - fieldPtr is not a pointer to a field of that struct
//   - the field cannot be found in the struct
//
// Example:
//
//	type Person struct {
//	    Name   string
//	    Age    int
//	    Secret string
//	}
//	var p Person
//	viewer := (&StructRowsViewer{}).WithIgnoreField(&p, &p.Secret)
func (v *StructRowsViewer) WithIgnoreField(structPtr, fieldPtr any) *StructRowsViewer {
	return v.WithIgnoreFieldIndex(MustStructFieldIndex(structPtr, fieldPtr))
}

// WithMapIndices returns a copy of the StructRowsViewer with the MapIndices
// field replaced by the provided map.
//
// This replaces any existing mappings with the new map. Use WithMapIndex
// to add individual mappings incrementally.
//
// Example:
//
//	viewer := (&StructRowsViewer{}).WithMapIndices(map[int]int{
//	    0: 2,  // field 0 -> column 2
//	    1: 0,  // field 1 -> column 0
//	    2: 1,  // field 2 -> column 1
//	    3: -1, // field 3 -> excluded
//	})
func (v *StructRowsViewer) WithMapIndices(mapIndices map[int]int) *StructRowsViewer {
	mod := v.clone()
	mod.MapIndices = mapIndices
	return mod
}
