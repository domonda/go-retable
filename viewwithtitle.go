package retable

import "reflect"

// ViewWithTitle creates a new View that wraps a source View with a different title.
//
// This function provides a way to change the title of an existing view without
// modifying the original or copying the data. The returned view delegates all
// operations to the source view except for Title(), which returns the provided
// title string.
//
// The wrapper implements both View and ReflectCellView interfaces, providing
// transparent access to the underlying data while presenting a different title.
//
// Performance characteristics:
//   - Zero-copy: no data duplication
//   - O(1) overhead for all operations (simple delegation)
//   - Minimal memory footprint (wrapper object only)
//
// When to use ViewWithTitle:
//   - Need to display the same data with different titles in different contexts
//   - Building view pipelines where title changes are required
//   - Implementing view transformations that only affect metadata
//   - Creating aliases or alternative presentations of existing views
//
// Parameters:
//   - source: The View to wrap (will be converted to ReflectCellView)
//   - title: The new title to use for the wrapped view
//
// Returns a View that has the specified title but otherwise behaves like source.
//
// Example:
//
//	original := retable.NewStringsView("Original Title", data, "A", "B")
//	renamed := retable.ViewWithTitle(original, "New Title")
//	fmt.Println(renamed.Title())       // Output: New Title
//	fmt.Println(renamed.Cell(0, 0))    // Delegates to original
//	fmt.Println(original.Title())      // Output: Original Title (unchanged)
//
// Thread safety: Safe for concurrent reads if the source view is safe for
// concurrent reads. The wrapper is immutable after creation.
func ViewWithTitle(source View, title string) View {
	return viewWithTitle{source: AsReflectCellView(source), title: title}
}

// viewWithTitle is the internal implementation of ViewWithTitle.
//
// This wrapper type delegates all operations to the source ReflectCellView
// except for Title(), which returns the wrapped title. It implements both
// View and ReflectCellView interfaces.
type viewWithTitle struct {
	// source is the wrapped view, converted to ReflectCellView for full functionality.
	source ReflectCellView

	// title is the replacement title returned by Title().
	title string
}

// Title returns the wrapped title (not the source's title).
func (v viewWithTitle) Title() string { return v.title }

// Columns delegates to the source view's Columns method.
func (v viewWithTitle) Columns() []string { return v.source.Columns() }

// NumRows delegates to the source view's NumRows method.
func (v viewWithTitle) NumRows() int { return v.source.NumRows() }

// Cell delegates to the source view's Cell method.
//
// Parameters:
//   - row: Zero-based row index
//   - col: Zero-based column index
//
// Returns the cell value from the source view.
func (v viewWithTitle) Cell(row, col int) any {
	return v.source.Cell(row, col)
}

// ReflectCell delegates to the source view's ReflectCell method and extracts
// the underlying value.
//
// Note: This method calls Elem() on the result from source.ReflectCell().
// This assumes the source returns a pointer or interface that needs to be
// dereferenced.
//
// Parameters:
//   - row: Zero-based row index
//   - col: Zero-based column index
//
// Returns the reflect.Value from the source view after calling Elem().
func (v viewWithTitle) ReflectCell(row, col int) reflect.Value {
	return v.source.ReflectCell(row, col).Elem()
}
