package retable

import "reflect"

// SingleColView creates a View containing a single column with typed data.
//
// This generic function provides a type-safe way to create a one-column view
// from a slice of values. The generic type parameter T determines the type
// of values in the column, providing compile-time type safety.
//
// The resulting view has one column and len(rows) rows, where each row
// contains one value from the rows slice. The view implements both View
// and ReflectCellView interfaces.
//
// Performance characteristics:
//   - Zero-copy: directly wraps the provided slice
//   - O(1) cell access
//   - Memory efficient (no data duplication)
//   - Generic type instantiation overhead at compile time only
//
// When to use SingleColView:
//   - Working with a single column of homogeneous typed data
//   - Converting a slice into a View for compatibility
//   - Building columnar data structures
//   - Type-safe single-column operations
//
// Parameters:
//   - column: The name of the single column
//   - rows: Slice of values of type T, one per row
//
// Returns a View with one column containing the provided values.
//
// Example:
//
//	// Create a single-column view of integers
//	numbers := []int{10, 20, 30, 40}
//	view := retable.SingleColView("Value", numbers)
//	fmt.Println(view.NumRows())    // Output: 4
//	fmt.Println(view.Cell(2, 0))   // Output: 30
//
//	// Create a single-column view of strings
//	names := []string{"Alice", "Bob", "Charlie"}
//	view := retable.SingleColView("Name", names)
//	fmt.Println(view.Cell(1, 0))   // Output: Bob
//
// Thread safety: Not thread-safe if the underlying rows slice is modified.
// The view does not copy the data, so concurrent modifications to the slice
// will be visible through the view.
func SingleColView[T any](column string, rows []T) View {
	return &singleColsView[T]{
		columns:        []string{column},
		rows:           rows,
		isReflectValue: reflect.TypeOf(rows).Elem() == reflect.TypeOf(reflect.Value{}),
	}
}

// SingleCellView creates a View containing exactly one cell with a typed value.
//
// This generic function creates a minimal 1x1 view (one row, one column) containing
// a single value of type T. It provides type safety and can be used to wrap
// scalar values as Views for compatibility with view-based APIs.
//
// The resulting view has one row and one column. It implements both View
// and ReflectCellView interfaces.
//
// Performance characteristics:
//   - Minimal memory footprint (one value + metadata)
//   - O(1) cell access
//   - No allocations for repeated access
//
// When to use SingleCellView:
//   - Wrapping a single scalar value as a View
//   - Creating minimal test fixtures
//   - Representing single-value results in a tabular format
//   - Building view hierarchies where leaf nodes are single values
//
// Parameters:
//   - title: The title of the view
//   - column: The name of the single column
//   - value: The value of type T to store in the single cell
//
// Returns a View with one row and one column containing the provided value.
//
// Example:
//
//	// Create a single-cell view with an integer
//	view := retable.SingleCellView("Count", "Total", 42)
//	fmt.Println(view.Title())      // Output: Count
//	fmt.Println(view.NumRows())    // Output: 1
//	fmt.Println(view.Cell(0, 0))   // Output: 42
//
//	// Create a single-cell view with a string
//	view := retable.SingleCellView("Status", "Message", "Success")
//	fmt.Println(view.Cell(0, 0))   // Output: Success
//
// Thread safety: Immutable after creation (assuming T is not a reference type
// that is modified externally), safe for concurrent reads.
func SingleCellView[T any](title, column string, value T) View {
	return &singleColsView[T]{
		columns:        []string{column},
		rows:           []T{value},
		isReflectValue: reflect.TypeOf(value) == reflect.TypeOf(reflect.Value{}),
	}
}

// singleColsView is the internal implementation for SingleColView and SingleCellView.
//
// This generic type provides the underlying implementation for single-column views.
// It stores a slice of typed values and implements both View and ReflectCellView
// interfaces.
//
// The isReflectValue field is used to handle the special case where T is reflect.Value.
// Due to Go's lack of generic type specialization, this requires runtime type checking
// and dynamic type assertions to properly unwrap reflect.Value types.
type singleColsView[T any] struct {
	// columns contains the column name(s). Always has length 1.
	columns []string

	// rows contains the data values, one per row.
	rows []T

	// isReflectValue is true if T is reflect.Value, requiring special handling.
	isReflectValue bool
}

// Title returns the column name as the title for single-column views.
func (s *singleColsView[T]) Title() string {
	return s.columns[0]
}

// Columns returns the column names (always a single-element slice).
func (s *singleColsView[T]) Columns() []string {
	return s.columns
}

// NumRows returns the number of rows (equal to len(rows)).
func (s *singleColsView[T]) NumRows() int {
	return len(s.rows)
}

// Cell returns the value at the specified row index.
//
// Since this is a single-column view, col must be 0. The method returns
// the value from the rows slice at the given row index.
//
// Special handling for reflect.Value: If T is reflect.Value, the method
// unwraps it using Interface() to return the underlying value. Invalid
// reflect.Values return nil.
//
// Parameters:
//   - row: Zero-based row index (0 to NumRows()-1)
//   - col: Must be 0 (only one column exists)
//
// Returns:
//   - The value at the specified row if indices are valid
//   - nil if row is out of bounds or col is not 0
//   - nil if T is reflect.Value and the value is invalid
//
// Time complexity: O(1)
func (s *singleColsView[T]) Cell(row, col int) any {
	if row < 0 || row >= len(s.rows) || col != 0 {
		return nil
	}
	if !s.isReflectValue {
		return s.rows[row]
	}
	// Lack of generic type specialization requires
	// dynamic type assertion
	v := any(s.rows[row]).(reflect.Value)
	if !v.IsValid() {
		return nil
	}
	return v.Interface()
}

// ReflectCell returns the reflect.Value for the value at the specified row index.
//
// Since this is a single-column view, col must be 0. The method returns
// a reflect.Value wrapping the value from the rows slice.
//
// Special handling for reflect.Value: If T is reflect.Value, the method
// returns the reflect.Value directly without double-wrapping it.
//
// Parameters:
//   - row: Zero-based row index (0 to NumRows()-1)
//   - col: Must be 0 (only one column exists)
//
// Returns:
//   - A reflect.Value for the cell if indices are valid
//   - An invalid reflect.Value (reflect.Value{}) if indices are out of bounds
//
// Example:
//
//	view := retable.SingleColView("Numbers", []int{10, 20, 30})
//	val := view.(retable.ReflectCellView).ReflectCell(1, 0)
//	fmt.Printf("Type: %s, Value: %d\n", val.Type(), val.Int())
//	// Output: Type: int, Value: 20
//
// Time complexity: O(1)
func (s *singleColsView[T]) ReflectCell(row, col int) reflect.Value {
	if row < 0 || row >= len(s.rows) || col != 0 {
		return reflect.Value{}
	}
	if !s.isReflectValue {
		return reflect.ValueOf(s.rows[row])
	}
	// Lack of generic type specialization requires
	// dynamic type assertion
	return any(s.rows[row]).(reflect.Value)
}
