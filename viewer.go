package retable

// Viewer implementations have a NewView method
// to create a View for a table.
type Viewer interface {
	// NewView creates a View for the passed table
	NewView(table any) (View, error)
}
