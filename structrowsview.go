package retable

import (
	"fmt"
	"reflect"
	"sync"
)

// StructRowsView is a View implementation that provides efficient access
// to tabular data stored in a slice or array of structs.
//
// StructRowsView is created by StructRowsViewer.NewView and implements
// the View interface, allowing struct data to be formatted, exported, or
// displayed as tables.
//
// The view uses reflection to extract field values on demand, with caching
// to optimize repeated access to the same row. This makes it efficient for
// scenarios where rows are accessed sequentially or where the same row is
// queried multiple times.
//
// Field-to-column mapping:
//   - If indices is nil, struct fields map 1:1 to columns in declaration order
//   - If indices is non-nil, it specifies which struct field index maps to each column
//   - The indices slice has one entry per struct field, with values indicating column positions
//   - An index value of -1 means that field is excluded from the view
//
// Performance characteristics:
//   - Row access is O(1) - direct slice indexing
//   - Cell access includes reflection overhead on first access per row
//   - Subsequent cell access for the same row uses cached values (O(1))
//   - Memory usage scales with the number of structs in the underlying slice
//
// This type is thread-safe for concurrent read access.
type StructRowsView struct {
	title   string
	columns []string
	indices []int         // nil for 1:1 mapping of columns to struct fields
	rows    reflect.Value // slice of structs

	// Caching fields to optimize repeated access to the same row
	// Protected by mutex for thread-safe concurrent access
	mutex               sync.RWMutex
	cachedRow           int             // -1 when cache is invalid
	cachedValues        []any           // cached any values for cachedRow
	cachedReflectValues []reflect.Value // cached reflect.Value values for cachedRow
}

// NewStructRowsView creates a new StructRowsView for the given struct slice data.
//
// This function is typically called by StructRowsViewer.NewView after determining
// the column names and field-to-column mapping through reflection and naming rules.
//
// Parameters:
//   - title: The title for the table view
//   - columns: Slice of column names in display order
//   - indices: Maps struct field indices to column indices (or nil for 1:1 mapping)
//   - rows: reflect.Value containing a slice or array of structs
//
// The indices parameter, when non-nil, must satisfy these constraints:
//   - Length equals the number of struct fields (exported, with embedded fields inlined)
//   - Each value is either -1 (field excluded) or a valid column index [0, len(columns))
//   - Each column index from 0 to len(columns)-1 appears exactly once
//   - No column index appears more than once
//
// If indices represents a simple 1:1 mapping where indices[i] == i for all i,
// it will be optimized to nil internally.
//
// Panics if:
//   - rows is not a slice or array
//   - indices violates the mapping constraints
//   - any column is unmapped
//   - any column is mapped more than once
//
// Example:
//
//	// For struct: {Field0, Field1, Field2, Field3}
//	// To create columns: [Field1, Field3, Field0] (Field2 excluded)
//	indices := []int{2, 0, -1, 1}
//	view := NewStructRowsView("Data", []string{"Col0", "Col1", "Col2"}, indices, rowsValue)
func NewStructRowsView(title string, columns []string, indices []int, rows reflect.Value) View {
	if rows.Kind() != reflect.Slice && rows.Kind() != reflect.Array {
		panic(fmt.Errorf("rows must be a slice or array, got %s", rows.Type()))
	}
	if is1on1Mapping(columns, indices) {
		indices = nil
	} else if indices != nil {
		colMapped := make([]bool, len(columns))
		for _, index := range indices {
			if index < 0 {
				continue
			}
			if index >= len(columns) {
				panic(fmt.Errorf("index %d out of range for %d columns", index, len(columns)))
			}
			if colMapped[index] {
				panic(fmt.Errorf("index %d mapped to column %q more than once", index, columns[index]))
			}
			colMapped[index] = true
		}
		for col, mapped := range colMapped {
			if !mapped {
				panic(fmt.Errorf("column %q not mapped", columns[col]))
			}
		}
	}
	return &StructRowsView{
		title:     title,
		columns:   columns,
		indices:   indices,
		rows:      rows,
		cachedRow: -1, // -1 indicates no cached row
	}
}

// is1on1Mapping checks if the indices represent a simple 1:1 mapping
// where each field maps to the corresponding column at the same index.
//
// Returns true if indices is nil or if indices[i] == i for all i.
// This optimization allows the view to skip index lookup when accessing cells.
func is1on1Mapping(columns []string, indices []int) bool {
	if indices == nil {
		return true
	}
	if len(columns) != len(indices) {
		return false
	}
	for i, index := range indices {
		if index != i {
			return false
		}
	}
	return true
}

// Title returns the title of the view.
// This implements the View interface.
func (view *StructRowsView) Title() string { return view.title }

// Columns returns the column names of the view in display order.
// This implements the View interface.
func (view *StructRowsView) Columns() []string { return view.columns }

// NumRows returns the number of rows in the view, which equals
// the length of the underlying struct slice or array.
// This implements the View interface.
func (view *StructRowsView) NumRows() int { return view.rows.Len() }

// Cell returns the value at the specified row and column as an any (interface{}).
//
// This method implements the View interface and uses reflection to extract
// the struct field value corresponding to the requested cell.
//
// Caching behavior:
//   - On first access to a row, all field values are extracted and cached
//   - Subsequent Cell calls for the same row use the cached values
//   - Accessing a different row invalidates the cache and extracts that row's values
//
// Parameters:
//   - row: Row index (0-based)
//   - col: Column index (0-based)
//
// Returns nil if row or col is out of bounds.
//
// Performance: O(1) for cached rows, O(n) where n is the number of struct fields
// for the first access to a new row.
func (view *StructRowsView) Cell(row, col int) any {
	if row < 0 || col < 0 || row >= view.rows.Len() || col >= len(view.columns) {
		return nil
	}

	view.mutex.Lock()
	defer view.mutex.Unlock()

	if row != view.cachedRow {
		view.cachedRow = row
		view.cachedValues = nil
		view.cachedReflectValues = nil
	}
	if view.cachedValues == nil {
		if view.indices != nil {
			view.cachedValues = IndexedStructFieldAnyValues(view.rows.Index(row), len(view.columns), view.indices)
		} else {
			view.cachedValues = StructFieldAnyValues(view.rows.Index(row))
		}
	}
	return view.cachedValues[col]
}

// ReflectCell returns the reflect.Value at the specified row and column.
//
// This method is similar to Cell but returns a reflect.Value instead of any,
// allowing callers to perform type-specific operations or avoid unnecessary
// Interface() conversions.
//
// Caching behavior:
//   - On first access to a row, all field reflect.Values are extracted and cached
//   - Subsequent ReflectCell calls for the same row use the cached values
//   - Accessing a different row invalidates the cache and extracts that row's values
//
// Parameters:
//   - row: Row index (0-based)
//   - col: Column index (0-based)
//
// Returns an invalid reflect.Value (reflect.Value{}) if row or col is out of bounds.
// Use reflect.Value.IsValid() to check if the returned value is valid.
//
// Performance: O(1) for cached rows, O(n) where n is the number of struct fields
// for the first access to a new row.
//
// Example:
//
//	val := view.ReflectCell(0, 0)
//	if val.IsValid() && val.CanInt() {
//	    fmt.Println("Integer value:", val.Int())
//	}
func (view *StructRowsView) ReflectCell(row, col int) reflect.Value {
	if row < 0 || col < 0 || row >= view.rows.Len() || col >= len(view.columns) {
		return reflect.Value{}
	}

	view.mutex.Lock()
	defer view.mutex.Unlock()

	if row != view.cachedRow {
		view.cachedRow = row
		view.cachedValues = nil
		view.cachedReflectValues = nil
	}
	if view.cachedReflectValues == nil {
		if view.indices != nil {
			view.cachedReflectValues = IndexedStructFieldReflectValues(view.rows.Index(row), len(view.columns), view.indices)
		} else {
			view.cachedReflectValues = StructFieldReflectValues(view.rows.Index(row))
		}
	}
	return view.cachedReflectValues[col]
}
