package retable

import "fmt"

var _ Viewer = new(StringsViewer)

// StringsViewer is a Viewer implementation that creates Views from
// two-dimensional string slices ([][]string).
//
// This is one of the simplest Viewer implementations and is useful when
// working with raw string data, such as CSV files, database query results
// converted to strings, or any tabular data represented as strings.
//
// Column Names:
//
// The Cols field can be used to specify column names for the Views.
// If provided, these names will be used as the column headers in the
// resulting View. If Cols is empty or nil, NewStringsView will use
// default column names or the table may be treated as having no headers.
//
// The behavior when Cols is provided:
//   - If Cols has fewer entries than columns in the data, remaining columns
//     will use default names
//   - If Cols has more entries than columns in the data, extra names are ignored
//
// Example:
//
//	// Create a StringsViewer with column names
//	viewer := StringsViewer{
//	    Cols: []string{"Name", "Age", "City"},
//	}
//
//	// Use it to create a View from string data
//	data := [][]string{
//	    {"Alice", "30", "NYC"},
//	    {"Bob", "25", "LA"},
//	}
//	view, err := viewer.NewView("People", data)
//
//	// Create a StringsViewer without predefined columns
//	viewer := StringsViewer{}
//	view, err := viewer.NewView("Data", data)
type StringsViewer struct {
	// Cols specifies the column names to use when creating Views.
	// Can be empty or nil, in which case column names will be handled
	// by the underlying View implementation (NewStringsView).
	Cols []string
}

// NewView creates a View from a [][]string table.
//
// This method implements the Viewer interface and is the primary way to
// create Views from string data using StringsViewer.
//
// The table parameter must be of type [][]string where:
//   - Each inner slice represents one row of data
//   - All rows should have the same number of elements (columns) for best results
//   - Empty tables (nil or zero length) are accepted and will create empty Views
//
// Column names are taken from the Cols field of the StringsViewer.
// See the StringsViewer type documentation for details on column naming.
//
// Parameters:
//   - title: The title for the resulting View. Can be empty. This title
//     is returned by the View's Title() method.
//   - table: The data to create the View from. Must be of type [][]string.
//
// Returns:
//   - View: The created View containing the string data with the specified title.
//   - error: nil on success, or an error if table is not of type [][]string.
//
// Example:
//
//	viewer := StringsViewer{Cols: []string{"Product", "Price", "Stock"}}
//
//	inventory := [][]string{
//	    {"Widget", "9.99", "100"},
//	    {"Gadget", "19.99", "50"},
//	    {"Doohickey", "29.99", "25"},
//	}
//
//	view, err := viewer.NewView("Inventory", inventory)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Println(view.Title())    // "Inventory"
//	fmt.Println(view.Columns())  // ["Product", "Price", "Stock"]
//	fmt.Println(view.NumRows())  // 3
//
//	// Access data through the View interface
//	cell := view.Cell(0, 0)  // "Widget"
func (v StringsViewer) NewView(title string, table any) (View, error) {
	rows, ok := table.([][]string)
	if !ok {
		return nil, fmt.Errorf("expected table of type [][]string, but got %T", table)
	}
	return NewStringsView(title, rows, v.Cols...), nil
}
