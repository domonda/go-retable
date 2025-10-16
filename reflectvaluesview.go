package retable

import (
	"errors"
	"reflect"
)

var _ ReflectCellView = new(ReflectValuesView)

// ReflectValuesView is a View implementation that stores cells as reflect.Value.
//
// This view type is designed for advanced use cases requiring reflection-based
// operations on cell data. Each cell is stored as a reflect.Value, providing
// access to type information and reflection capabilities while maintaining
// the View interface.
//
// The underlying data structure uses [][]reflect.Value, where each cell can
// be introspected, compared, and manipulated using Go's reflection package.
// This is particularly useful for generic operations, type discovery, and
// dynamic value manipulation.
//
// ReflectValuesView implements both View and ReflectCellView interfaces,
// providing both standard Cell() and specialized ReflectCell() methods.
//
// Performance characteristics:
//   - Direct memory access: O(1) for cell retrieval
//   - Reflection overhead for value operations
//   - Higher memory usage than AnyValuesView due to reflect.Value storage
//   - Efficient for reflection-heavy operations
//
// When to use ReflectValuesView:
//   - Need type introspection on cell values
//   - Implementing generic algorithms that work with arbitrary types
//   - Converting between different representation forms
//   - Caching reflection results for performance
//   - Building dynamic view transformations
//   - Working with code generation or analysis tools
//
// Example usage:
//
//	source := retable.NewStringsView("Data", data, "A", "B")
//	reflected, _ := retable.NewReflectValuesViewFrom(source)
//
//	// Access via standard interface
//	value := reflected.Cell(0, 0)
//
//	// Access via reflection interface
//	reflectVal := reflected.ReflectCell(0, 0)
//	fmt.Printf("Type: %s, Kind: %s\n", reflectVal.Type(), reflectVal.Kind())
//
// Thread safety: Not thread-safe. External synchronization required for
// concurrent access.
type ReflectValuesView struct {
	// Tit is the title of this view, returned by the Title() method.
	Tit string

	// Cols contains the column names defining both the column headers
	// and the number of columns in this view.
	Cols []string

	// Rows contains the data rows, where each row is a slice of reflect.Value.
	// Each cell is stored as a reflect.Value for introspection capabilities.
	Rows [][]reflect.Value
}

// NewReflectValuesViewFrom creates a ReflectValuesView by reading and caching
// all cells from the source View as reflect.Value instances.
//
// This function materializes all data from the source view into memory, converting
// each cell to a reflect.Value. If the source implements ReflectCellView, it uses
// ReflectCell() directly; otherwise, it wraps Cell() results with reflect.ValueOf().
//
// This is useful for:
//   - Caching reflection results for repeated introspection
//   - Converting any view to a reflection-based representation
//   - Preparing data for reflection-heavy operations
//   - Type analysis and schema discovery
//
// Parameters:
//   - source: The View to read data from
//
// Returns:
//   - A new ReflectValuesView containing all data as reflect.Values
//   - An error if source is nil
//
// Example:
//
//	original := retable.NewStringsView("Data", rows, "A", "B", "C")
//	reflected, err := retable.NewReflectValuesViewFrom(original)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// All cells are now stored as reflect.Value for introspection
//	for row := 0; row < reflected.NumRows(); row++ {
//	    for col := range reflected.Columns() {
//	        val := reflected.ReflectCell(row, col)
//	        fmt.Printf("Type: %s\n", val.Type())
//	    }
//	}
//
// Time complexity: O(rows * cols) for copying all cells
// Space complexity: O(rows * cols) for storing all reflect.Values
func NewReflectValuesViewFrom(source View) (*ReflectValuesView, error) {
	if source == nil {
		return nil, errors.New("view is nil")
	}
	view := &ReflectValuesView{
		Tit:  source.Title(),
		Cols: source.Columns(),
		Rows: make([][]reflect.Value, source.NumRows()),
	}
	reflectSource := AsReflectCellView(source)
	for row := 0; row < source.NumRows(); row++ {
		view.Rows[row] = make([]reflect.Value, len(source.Columns()))
		for col := range view.Rows[row] {
			view.Rows[row][col] = reflectSource.ReflectCell(row, col)
		}
	}
	return view, nil
}

// Title returns the title of this view.
func (view *ReflectValuesView) Title() string { return view.Tit }

// Columns returns the column names of this view.
func (view *ReflectValuesView) Columns() []string { return view.Cols }

// NumRows returns the number of data rows in this view.
func (view *ReflectValuesView) NumRows() int { return len(view.Rows) }

// Cell returns the interface value at the specified row and column indices.
//
// This method extracts the underlying value from the reflect.Value using
// Interface(). If you need the reflect.Value itself for introspection,
// use ReflectCell() instead.
//
// Parameters:
//   - row: Zero-based row index (0 to NumRows()-1)
//   - col: Zero-based column index (0 to len(Columns())-1)
//
// Returns:
//   - The cell's underlying value (via reflect.Value.Interface()) if indices are valid
//   - nil if row or col indices are out of bounds
//
// Time complexity: O(1)
func (view *ReflectValuesView) Cell(row, col int) any {
	if row < 0 || col < 0 || row >= len(view.Rows) || col >= len(view.Rows[row]) {
		return nil
	}
	return view.Rows[row][col].Interface()
}

// ReflectCell returns the reflect.Value at the specified row and column indices.
//
// This method provides direct access to the stored reflect.Value, allowing
// for type introspection, comparison, and other reflection operations without
// extracting the underlying interface value.
//
// Parameters:
//   - row: Zero-based row index (0 to NumRows()-1)
//   - col: Zero-based column index (0 to len(Columns())-1)
//
// Returns:
//   - The reflect.Value stored at the cell if indices are valid
//   - An invalid reflect.Value (reflect.Value{}) if indices are out of bounds
//
// To check if the returned value is valid, use reflectVal.IsValid().
//
// Example:
//
//	val := view.ReflectCell(0, 0)
//	if val.IsValid() {
//	    fmt.Printf("Type: %s, Kind: %s\n", val.Type(), val.Kind())
//	    if val.Kind() == reflect.Int {
//	        fmt.Printf("Int value: %d\n", val.Int())
//	    }
//	}
//
// Time complexity: O(1)
func (view *ReflectValuesView) ReflectCell(row, col int) reflect.Value {
	if row < 0 || col < 0 || row >= len(view.Rows) || col >= len(view.Rows[row]) {
		return reflect.Value{}
	}
	return view.Rows[row][col]
}

var _ ReflectCellView = new(ReflectValuesView)

// SingleReflectValueView is a View implementation containing exactly one cell
// stored as a reflect.Value.
//
// This specialized view type represents a 1x1 table (single row, single column)
// where the cell value is stored as a reflect.Value. It implements both View
// and ReflectCellView interfaces.
//
// SingleReflectValueView is useful for:
//   - Extracting and wrapping a single cell from a larger view
//   - Representing scalar values as a minimal view
//   - Type introspection on individual values
//   - Building view hierarchies where leaf nodes are single values
//
// Performance characteristics:
//   - Minimal memory footprint (one value + metadata)
//   - O(1) access time
//   - No allocations for repeated access
//
// Example usage:
//
//	// Extract a single cell from another view
//	source := retable.NewStringsView("Data", rows, "A", "B", "C")
//	singleCell := retable.NewSingleReflectValueView(source, 0, 1)
//	fmt.Println(singleCell.Cell(0, 0)) // Value from source row 0, col 1
//
// Thread safety: Immutable after creation, safe for concurrent reads.
type SingleReflectValueView struct {
	// Tit is the title of this view, usually inherited from the source.
	Tit string

	// Col is the name of the single column in this view.
	Col string

	// Val is the reflect.Value stored in the single cell.
	Val reflect.Value
}

// NewSingleReflectValueView creates a SingleReflectValueView by extracting
// a single cell from the source View at the specified row and column.
//
// The function reads one cell value from the source and wraps it in a minimal
// 1x1 view. The title is inherited from the source, and the column name is
// taken from the source's column at the specified index.
//
// Parameters:
//   - source: The View to extract the cell from
//   - row: Zero-based row index in the source view
//   - col: Zero-based column index in the source view
//
// Returns a new SingleReflectValueView containing the single cell value.
// If source is nil or indices are invalid, returns a view with only the
// title set (and an invalid reflect.Value).
//
// Example:
//
//	source := retable.NewStringsView(
//	    "Products",
//	    [][]string{
//	        {"Widget", "9.99"},
//	        {"Gadget", "19.99"},
//	    },
//	    "Name", "Price",
//	)
//	priceView := retable.NewSingleReflectValueView(source, 1, 1)
//	fmt.Println(priceView.Cell(0, 0)) // Output: 19.99
func NewSingleReflectValueView(source View, row, col int) *SingleReflectValueView {
	if source == nil || row < 0 || col < 0 || row >= source.NumRows() || col >= len(source.Columns()) {
		return &SingleReflectValueView{Tit: source.Title()}
	}
	return &SingleReflectValueView{
		Tit: source.Title(),
		Col: source.Columns()[col],
		Val: reflect.ValueOf(source.Cell(row, col)),
	}
}

// Title returns the title of this view.
func (view *SingleReflectValueView) Title() string { return view.Tit }

// Columns returns a slice containing the single column name.
func (view *SingleReflectValueView) Columns() []string { return []string{view.Col} }

// NumRows always returns 1 for SingleReflectValueView.
func (view *SingleReflectValueView) NumRows() int { return 1 }

// Cell returns the underlying interface value of the single cell.
//
// Parameters:
//   - row: Must be 0 (only one row exists)
//   - col: Must be 0 (only one column exists)
//
// Returns:
//   - The underlying value extracted via Val.Interface() if row and col are both 0
//   - nil if row is not 0 or col is not 0
//
// Time complexity: O(1)
func (view *SingleReflectValueView) Cell(row, col int) any {
	if row != 0 || col != 0 {
		return nil
	}
	return view.Val.Interface()
}

// ReflectCell returns the reflect.Value of the single cell.
//
// Parameters:
//   - row: Must be 0 (only one row exists)
//   - col: Must be 0 (only one column exists)
//
// Returns:
//   - The reflect.Value stored in Val if row and col are both 0
//   - An invalid reflect.Value (reflect.Value{}) if row is not 0 or col is not 0
//
// Time complexity: O(1)
func (view *SingleReflectValueView) ReflectCell(row, col int) reflect.Value {
	if row != 0 || col != 0 {
		return reflect.Value{}
	}
	return view.Val
}
