package retable

var _ View = new(AnyValuesView)

// AnyValuesView is a View implementation that stores cells as values of any type.
//
// Unlike StringsView which is restricted to string values, AnyValuesView can hold
// heterogeneous data types within the same table. Each cell can contain values of
// different types (int, string, bool, struct, etc.), making it ideal for mixed-type
// data or when converting from other view types.
//
// The underlying data structure uses [][]any, where each row is a slice of
// interface{} (any) values. This provides maximum flexibility at the cost of
// type safety and some performance overhead due to interface boxing.
//
// Performance characteristics:
//   - Direct memory access: O(1) for cell retrieval
//   - Interface boxing overhead for value storage and retrieval
//   - Higher memory usage than StringsView due to interface overhead
//   - Type assertions required when accessing specific types
//
// When to use AnyValuesView:
//   - Working with mixed-type tabular data (numbers, strings, booleans, etc.)
//   - Caching data from another View with NewAnyValuesViewFrom
//   - Need to store arbitrary Go values in table cells
//   - Converting between different view types
//   - Building dynamic tables where cell types aren't known at compile time
//
// Example usage:
//
//	view := &retable.AnyValuesView{
//	    Tit:  "Mixed Data",
//	    Cols: []string{"ID", "Name", "Active", "Score"},
//	    Rows: [][]any{
//	        {1, "Alice", true, 95.5},
//	        {2, "Bob", false, 87.3},
//	    },
//	}
//	// Type assertion needed for specific types
//	if score, ok := view.Cell(0, 3).(float64); ok {
//	    fmt.Printf("Score: %.1f\n", score)
//	}
//
// Thread safety: Not thread-safe. External synchronization required for
// concurrent access.
type AnyValuesView struct {
	// Tit is the title of this view, returned by the Title() method.
	Tit string

	// Cols contains the column names defining both the column headers
	// and the number of columns in this view.
	Cols []string

	// Rows contains the data rows, where each row is a slice of any-typed values.
	// Each cell can hold a value of any type.
	Rows [][]any
}

// NewAnyValuesViewFrom creates an AnyValuesView by reading and caching all cells
// from the source View.
//
// This function materializes all data from the source view into memory as an
// AnyValuesView. This is useful for:
//   - Caching data from views with expensive Cell() operations
//   - Converting computed or dynamic views into static data
//   - Creating a snapshot of view data at a point in time
//   - Ensuring consistent data access when the source might change
//
// The function calls source.Cell() for every cell in the view, so for large
// views this can be memory-intensive. The resulting view is completely independent
// of the source.
//
// Parameters:
//   - source: The View to read data from
//
// Returns a new AnyValuesView containing all data from the source.
//
// Example:
//
//	// Cache an expensive computed view
//	expensiveView := retable.NewComputedView(...)
//	cached := retable.NewAnyValuesViewFrom(expensiveView)
//	// Now cached can be accessed multiple times without recomputation
//
// Time complexity: O(rows * cols) for copying all cells
// Space complexity: O(rows * cols) for storing all values
func NewAnyValuesViewFrom(source View) *AnyValuesView {
	view := &AnyValuesView{
		Tit:  source.Title(),
		Cols: source.Columns(),
		Rows: make([][]any, source.NumRows()),
	}
	for row := 0; row < source.NumRows(); row++ {
		view.Rows[row] = make([]any, len(source.Columns()))
		for col := range view.Rows[row] {
			view.Rows[row][col] = source.Cell(row, col)
		}
	}
	return view
}

// Title returns the title of this view.
func (view *AnyValuesView) Title() string { return view.Tit }

// Columns returns the column names of this view.
func (view *AnyValuesView) Columns() []string { return view.Cols }

// NumRows returns the number of data rows in this view.
func (view *AnyValuesView) NumRows() int { return len(view.Rows) }

// Cell returns the value at the specified row and column indices.
//
// The returned value can be of any type depending on what was stored in that cell.
// Callers typically need to use type assertions or type switches to work with
// the specific types.
//
// Parameters:
//   - row: Zero-based row index (0 to NumRows()-1)
//   - col: Zero-based column index (0 to len(Columns())-1)
//
// Returns:
//   - The cell value (of any type) if indices are valid
//   - nil if row or col indices are out of bounds
//
// Example with type assertion:
//
//	value := view.Cell(0, 2)
//	switch v := value.(type) {
//	case int:
//	    fmt.Printf("Integer: %d\n", v)
//	case string:
//	    fmt.Printf("String: %s\n", v)
//	case bool:
//	    fmt.Printf("Boolean: %t\n", v)
//	}
//
// Time complexity: O(1)
func (view *AnyValuesView) Cell(row, col int) any {
	if row < 0 || col < 0 || row >= len(view.Rows) || col >= len(view.Rows[row]) {
		return nil
	}
	return view.Rows[row][col]
}
