package retable

// Viewer implementations create a View for a table.
type Viewer interface {
	// NewView creates a View with the passed title
	// for the passed table.
	NewView(title string, table any) (View, error)
}
