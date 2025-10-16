package retable

import "reflect"

// DerefView creates a zero-copy decorator that automatically dereferences pointer values
// in a source View. This is particularly useful when working with Views of pointer types
// (e.g., []*Person) but you need to access the underlying values.
//
// # Use Cases
//
//   - Converting Views of pointer slices ([]*T) to value access
//   - Unwrapping optional/nullable values represented as pointers
//   - Simplifying code that works with dereferenced values
//   - Preparing data for formatters that expect concrete values
//
// # How It Works
//
// DerefView wraps any View and calls reflect.Value.Elem() on every cell value,
// which dereferences pointers, interfaces, and other indirect types. The source
// View is automatically converted to ReflectCellView for efficient operation.
//
// # Performance
//
//   - Zero data copying: Only dereferences on access
//   - Reflection overhead: Each Cell() call performs reflection
//   - Efficient for sparse access: Only dereferences accessed cells
//
// # Panics
//
// DerefView will panic if:
//   - A cell value cannot be dereferenced (e.g., non-pointer primitive)
//   - A cell contains a nil pointer and you try to access it
//   - The reflect.Value.Elem() operation is invalid for the value type
//
// # Example: Dereferencing Pointer Structs
//
//	type Person struct {
//	    Name string
//	    Age  int
//	}
//
//	// Source with pointer values
//	people := []*Person{
//	    {Name: "Alice", Age: 30},
//	    {Name: "Bob", Age: 25},
//	}
//	source := NewStructRowsView("People", people, nil, nil)
//	// source.Cell(0, 0) returns &Person{Name: "Alice", Age: 30}
//
//	// Deref view automatically unwraps pointers
//	deref := DerefView(source)
//	// deref.Cell(0, 0) returns Person{Name: "Alice", Age: 30}
//
// # Example: Unwrapping Interface Values
//
//	// View with cells wrapped in interface{}
//	source := NewAnyValuesView("Data", [][]any{
//	    {any(&Person{Name: "Alice"})},
//	})
//
//	deref := DerefView(source)
//	// Dereferences the pointer inside the interface
//	deref.Cell(0, 0) // Returns Person{Name: "Alice"}
//
// # Example: Composition with Other Decorators
//
//	// Combine dereferencing with filtering
//	source := NewStructRowsView("People", peoplePointers, nil, nil)
//	deref := DerefView(source)
//	filtered := &FilteredView{
//	    Source:    deref,
//	    RowLimit:  10,
//	}
//	// filtered provides first 10 dereferenced values
//
// # Safety Considerations
//
// To safely use DerefView with potentially nil pointers, check values first:
//
//	val := source.Cell(row, col)
//	if val != nil {
//	    derefVal := deref.Cell(row, col)
//	    // Use derefVal
//	}
//
// Or use reflection to check before dereferencing:
//
//	rv := deref.(ReflectCellView).ReflectCell(row, col)
//	if rv.IsValid() && !rv.IsNil() {
//	    // Safe to use rv.Interface()
//	}
//
// # Return Type
//
// Returns a ReflectCellView (which embeds View), providing both Cell() and
// ReflectCell() methods for flexibility in value access.
func DerefView(source View) ReflectCellView {
	return derefView{source: AsReflectCellView(source)}
}

// derefView is the internal implementation of DerefView.
// It wraps a ReflectCellView and dereferences all cell values via reflection.
type derefView struct {
	source ReflectCellView
}

// Title returns the title from the underlying source View.
// DerefView does not modify the title.
func (v derefView) Title() string { return v.source.Title() }

// Columns returns the column names from the underlying source View.
// DerefView does not modify column names.
func (v derefView) Columns() []string { return v.source.Columns() }

// NumRows returns the row count from the underlying source View.
// DerefView does not modify the row count.
func (v derefView) NumRows() int { return v.source.NumRows() }

// Cell returns the dereferenced value at the specified position.
//
// The method:
//  1. Calls source.ReflectCell(row, col) to get a reflect.Value
//  2. Calls .Elem() to dereference it
//  3. Calls .Interface() to extract the concrete value
//
// Returns nil if the source cell is out of bounds.
//
// PANICS if:
//   - The cell value cannot be dereferenced (not a pointer/interface)
//   - The cell contains a nil pointer
//
// Example:
//
//	source.Cell(0, 0) -> &Person{Name: "Alice"}
//	deref.Cell(0, 0)  -> Person{Name: "Alice"}
func (v derefView) Cell(row, col int) any {
	return v.source.ReflectCell(row, col).Elem().Interface()
}

// ReflectCell returns the dereferenced reflect.Value at the specified position.
//
// This method is more efficient than Cell() when working with reflection,
// as it avoids the .Interface() conversion.
//
// The method:
//  1. Calls source.ReflectCell(row, col)
//  2. Calls .Elem() to dereference the value
//  3. Returns the dereferenced reflect.Value
//
// PANICS if:
//   - The cell value cannot be dereferenced
//   - The cell contains a nil pointer
//
// Use IsValid() and IsNil() checks to safely handle edge cases:
//
//	rv := deref.ReflectCell(row, col)
//	if rv.IsValid() && rv.Kind() == reflect.Ptr && !rv.IsNil() {
//	    // Safe to use
//	}
func (v derefView) ReflectCell(row, col int) reflect.Value {
	return v.source.ReflectCell(row, col).Elem()
}
