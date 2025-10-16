package retable

// Viewer is a factory interface for creating Views from arbitrary table data.
// Viewers are responsible for recognizing specific data structures (like [][]string
// or []StructType) and wrapping them as View implementations.
//
// # Purpose
//
// The Viewer interface provides a uniform way to create Views from different
// data representations without needing to know the exact View type to construct.
// This is especially useful in generic code that works with multiple data formats.
//
// # Standard Implementations
//
//   - StringsViewer: Creates views from [][]string (CSV, text data)
//   - StructRowsViewer: Creates views from struct slices via reflection
//
// # Example Usage
//
//	// Using StringsViewer
//	viewer := StringsViewer{FirstRowIsHeader: true}
//	data := [][]string{
//	    {"Name", "Age"},
//	    {"Alice", "30"},
//	    {"Bob", "25"},
//	}
//	view, err := viewer.NewView("People", data)
//
//	// Using StructRowsViewer
//	type Person struct {
//	    Name string
//	    Age  int
//	}
//	people := []Person{{"Alice", 30}, {"Bob", 25}}
//	viewer := NewStructRowsViewer(nil)
//	view, err := viewer.NewView("People", people)
//
// # Custom Viewers
//
// Implement this interface to create Views from custom data structures:
//
//	type MyViewer struct{}
//
//	func (v MyViewer) NewView(title string, table any) (View, error) {
//	    myData, ok := table.(MyDataType)
//	    if !ok {
//	        return nil, fmt.Errorf("expected MyDataType, got %T", table)
//	    }
//	    return myViewFromData(title, myData), nil
//	}
type Viewer interface {
	// NewView creates a View with the specified title from the given table data.
	//
	// The table parameter should contain tabular data in a format that this
	// Viewer implementation recognizes. Different Viewer implementations accept
	// different types:
	//   - StringsViewer accepts [][]string
	//   - StructRowsViewer accepts []StructType or []*StructType
	//
	// Returns an error if:
	//   - table is not a supported type for this Viewer
	//   - table is nil or empty (implementation-dependent)
	//   - the data structure is invalid or malformed
	//
	// The title parameter becomes the result of the View's Title() method.
	// It can be an empty string if no title is needed.
	NewView(title string, table any) (View, error)
}
