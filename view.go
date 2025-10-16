package retable

import (
	"reflect"
)

// View is the central interface in the retable package that represents read-only tabular data
// in a uniform, type-agnostic way. It provides access to table metadata (title, columns)
// and cell data through a simple coordinate-based API.
//
// View is designed as an in-memory abstraction - implementations should load all data
// before wrapping it as a View. This design choice eliminates the need for context
// parameters and error handling in the core View methods, simplifying the API.
//
// # Design Philosophy
//
// The View interface follows these principles:
//   - Immutability: Views are read-only; modifications create new Views
//   - Simplicity: No error returns from data access methods (data already in memory)
//   - Uniformity: All tabular data sources expose the same interface
//   - Composability: Views can be wrapped and transformed via decorator types
//
// # Coordinate System
//
// Views use zero-based indexing:
//   - Columns are numbered 0 to len(Columns())-1
//   - Rows are numbered 0 to NumRows()-1
//
// # Cell Values
//
// Cell values are returned as any (empty interface) and can be:
//   - Go primitives (int, string, bool, float64, etc.)
//   - Complex types (time.Time, custom structs, etc.)
//   - nil (for missing/null values or out-of-bounds access)
//
// # Common Implementations
//
// The package provides several View implementations:
//   - StringsView: Backed by [][]string (from CSV, text)
//   - StructRowsView: Backed by []StructType (reflection-based)
//   - AnyValuesView: Backed by [][]any (from SQL, mixed types)
//   - ReflectValuesView: Backed by [][]reflect.Value (advanced)
//
// # View Wrappers
//
// Several wrapper types transform Views without copying data:
//   - FilteredView: Row/column filtering and remapping
//   - DerefView: Automatic pointer dereferencing
//   - ExtraColsView: Horizontal concatenation
//   - ExtraRowsView: Vertical concatenation
//   - ViewWithTitle: Custom title override
//
// # Example Usage
//
//	// Create a view from structs
//	type Person struct {
//	    Name string
//	    Age  int
//	}
//	people := []Person{{"Alice", 30}, {"Bob", 25}}
//	view := NewStructRowsView("People", people, nil, nil)
//
//	// Access data
//	fmt.Println(view.Title())              // "People"
//	fmt.Println(view.Columns())            // ["Name", "Age"]
//	fmt.Println(view.NumRows())            // 2
//	fmt.Println(view.Cell(0, 0))           // "Alice"
//	fmt.Println(view.Cell(0, 1))           // 30
//	fmt.Println(view.Cell(2, 0))           // nil (out of bounds)
type View interface {
	// Title returns the name/title of this table.
	// The title is used for display purposes and when writing to formats
	// that support named tables (e.g., Excel sheet names, HTML table captions).
	// Returns an empty string if the table has no title.
	Title() string

	// Columns returns the names of all columns in this table.
	// The length of the returned slice defines the number of columns.
	// Individual column names may be empty strings if unnamed.
	// The returned slice should not be modified by callers.
	// Column names are used as headers when writing to CSV, HTML, etc.
	Columns() []string

	// NumRows returns the total number of data rows in this table.
	// This does not include any header row - it's purely the count of data rows.
	// Returns 0 for an empty table.
	NumRows() int

	// Cell returns the value at the specified row and column coordinates.
	// Row and column indices are zero-based.
	//
	// Returns nil in these cases:
	//   - row is negative or >= NumRows()
	//   - col is negative or >= len(Columns())
	//   - the cell actually contains a nil value
	//
	// The returned value type depends on the View implementation and
	// the underlying data source. Common types include:
	//   - string (for text-based sources like CSV)
	//   - numeric types (int, float64, etc.)
	//   - time.Time (for date/time values)
	//   - bool
	//   - custom types (for struct-based Views)
	//
	// Callers should use type assertions or reflection to work with
	// the returned value. For more type-safe access, consider using
	// ViewToStructSlice or SmartAssign functions.
	Cell(row, col int) any
}

// ReflectCellView extends the View interface with reflection-based cell access.
// This interface is useful when working with generic code that needs to inspect
// or manipulate cell values using Go's reflect package.
//
// # Use Cases
//
//   - Type-safe data conversion via SmartAssign
//   - Generic formatters that inspect value types
//   - Building dynamic struct scanners
//   - Implementing custom CellFormatter instances
//
// # Automatic Wrapping
//
// Not all Views natively implement ReflectCellView. Use AsReflectCellView()
// to get a ReflectCellView from any View - it will either return the View
// directly (if it implements the interface) or wrap it automatically.
//
// # Performance Consideration
//
// Views that natively implement ReflectCellView (like ReflectValuesView)
// are more efficient than wrapped views, as they avoid repeated reflection
// operations via reflect.ValueOf().
//
// # Example Usage
//
//	view := NewStringsView("Data", [][]string{{"123", "true"}}, []string{"Number", "Flag"})
//	reflectView := AsReflectCellView(view)
//
//	val := reflectView.ReflectCell(0, 0)  // reflect.Value containing "123"
//	if val.Kind() == reflect.String {
//	    fmt.Println(val.String())          // "123"
//	}
type ReflectCellView interface {
	View

	// ReflectCell returns the reflect.Value of the cell at the specified position.
	// This provides direct access to the underlying value's reflection metadata.
	//
	// The returned reflect.Value can be:
	//   - A valid value with the cell's actual type and value
	//   - A zero Value (from reflect.Value{}) if row/col are out of bounds
	//   - A reflect.Value containing nil if the cell is nil
	//
	// To check if a returned Value is valid, use:
	//   val := view.ReflectCell(row, col)
	//   if val.IsValid() { /* use val */ }
	//
	// Row and column indices are zero-based, same as Cell().
	ReflectCell(row, col int) reflect.Value
}

// AsReflectCellView converts any View to a ReflectCellView.
//
// If the input View already implements ReflectCellView, it is returned as-is
// with no allocation. Otherwise, the View is wrapped in a lightweight adapter
// that implements ReflectCellView by calling reflect.ValueOf() on Cell() results.
//
// This function never returns nil - it always returns a valid ReflectCellView.
//
// # Example Usage
//
//	var view View = NewStringsView(...)
//	reflectView := AsReflectCellView(view)
//	// reflectView.ReflectCell() is now available
//
// # Performance Note
//
// The wrapper created for non-ReflectCellView inputs is very lightweight
// (just a struct embedding the original View), but calling ReflectCell()
// will perform reflection on each call. For performance-critical code with
// heavy reflection usage, prefer Views that natively implement ReflectCellView.
func AsReflectCellView(view View) ReflectCellView {
	if v, ok := view.(ReflectCellView); ok {
		return v
	}
	return wrapAsReflectCellView{view}
}

// wrapAsReflectCellView is an internal adapter that wraps a regular View
// to implement the ReflectCellView interface.
type wrapAsReflectCellView struct {
	View
}

// ReflectCell implements ReflectCellView by wrapping Cell() results with reflect.ValueOf().
func (w wrapAsReflectCellView) ReflectCell(row, col int) reflect.Value {
	return reflect.ValueOf(w.View.Cell(row, col))
}
