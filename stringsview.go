package retable

import (
	"strings"
)

// StringsView is a View implementation that uses strings as cell values.
// It is the most straightforward and commonly used view type for tabular string data.
//
// The Cols field defines the column names and determines the number of columns.
// Each element in Rows represents a row of data, where each row is a slice of strings.
//
// StringsView supports sparse data: a row within Rows can have fewer slice elements
// than Cols, in which case empty strings ("") are returned as values for missing cells.
// This allows for efficient storage of tables with many empty cells.
//
// Performance characteristics:
//   - Direct memory access: O(1) for cell retrieval
//   - Memory efficient for string-only data
//   - No type conversion overhead
//
// When to use StringsView:
//   - Working with CSV, TSV, or other text-based tabular data
//   - All cell values are strings or can be represented as strings
//   - Need simple, fast access to string data
//   - Memory efficiency is important for large string tables
//
// Example usage:
//
//	view := retable.NewStringsView(
//	    "Products",
//	    [][]string{
//	        {"ID", "Name", "Price"},
//	        {"1", "Widget", "9.99"},
//	        {"2", "Gadget", "19.99"},
//	    },
//	)
//	fmt.Println(view.Cell(0, 1)) // Output: Widget
//
// Sparse data example:
//
//	view := &retable.StringsView{
//	    Cols: []string{"A", "B", "C"},
//	    Rows: [][]string{
//	        {"1", "2", "3"},
//	        {"4"}, // Missing B and C - will return empty strings
//	    },
//	}
//	fmt.Println(view.Cell(1, 2)) // Output: "" (empty string)
type StringsView struct {
	// Tit is the title of this view, returned by the Title() method.
	Tit string

	// Cols contains the column names defining both the column headers
	// and the number of columns in this view.
	Cols []string

	// Rows contains the data rows, where each row is a slice of strings.
	// Rows can have fewer elements than len(Cols) for sparse data support.
	Rows [][]string
}

var _ View = new(StringsView)

// NewStringsView creates a new StringsView with flexible column specification.
//
// The function accepts column names in two ways:
//  1. Explicit columns: Pass column names via the cols variadic parameter
//  2. Header row: If no cols are provided and rows is not empty, the first row
//     is used as column names and removed from the data rows
//
// All column names have leading and trailing whitespace trimmed automatically.
//
// Parameters:
//   - title: The title for this view (can be empty string)
//   - rows: The data rows, or rows including a header row if cols is empty
//   - cols: Optional column names. If empty, first row is used as header
//
// Returns a new StringsView with the specified title, columns, and data rows.
//
// Example with explicit columns:
//
//	view := retable.NewStringsView(
//	    "Users",
//	    [][]string{
//	        {"alice", "30"},
//	        {"bob", "25"},
//	    },
//	    "Name", "Age",
//	)
//
// Example with header row:
//
//	view := retable.NewStringsView(
//	    "Users",
//	    [][]string{
//	        {"Name", "Age"}, // This becomes the header
//	        {"alice", "30"},
//	        {"bob", "25"},
//	    },
//	)
func NewStringsView(title string, rows [][]string, cols ...string) *StringsView {
	if len(cols) == 0 && len(rows) > 0 {
		cols = rows[0]
		rows = rows[1:]
	}
	for i, col := range cols {
		cols[i] = strings.TrimSpace(col)
	}
	return &StringsView{Tit: title, Cols: cols, Rows: rows}
}

// Title returns the title of this view.
func (view *StringsView) Title() string { return view.Tit }

// Columns returns the column names of this view.
func (view *StringsView) Columns() []string { return view.Cols }

// NumRows returns the number of data rows in this view.
func (view *StringsView) NumRows() int { return len(view.Rows) }

// Cell returns the value at the specified row and column indices.
//
// For StringsView, Cell returns:
//   - The string value at [row][col] if the cell exists
//   - An empty string "" if the row exists but has fewer columns (sparse data)
//   - nil if row or col indices are out of bounds
//
// Parameters:
//   - row: Zero-based row index (0 to NumRows()-1)
//   - col: Zero-based column index (0 to len(Columns())-1)
//
// Returns the cell value as any, which will be either string, "", or nil.
//
// Time complexity: O(1)
func (view *StringsView) Cell(row, col int) any {
	if row < 0 || col < 0 || row >= len(view.Rows) || col >= len(view.Cols) {
		return nil
	}
	if col >= len(view.Rows[row]) {
		return ""
	}
	return view.Rows[row][col]
}

// NewHeaderView creates a View containing only a header row.
//
// The header row displays the column names as data, creating a single-row view
// where the column names are also the cell values in that row. This is useful
// for displaying just the header information or combining with other views.
//
// All column names have leading and trailing whitespace trimmed automatically.
//
// Parameters:
//   - cols: Variable number of column names
//
// Returns a new HeaderView with one row containing the column names.
//
// Example:
//
//	view := retable.NewHeaderView("ID", "Name", "Email")
//	fmt.Println(view.NumRows())    // Output: 1
//	fmt.Println(view.Cell(0, 1))   // Output: Name
func NewHeaderView(cols ...string) *HeaderView {
	for i, col := range cols {
		cols[i] = strings.TrimSpace(col)
	}
	return &HeaderView{Cols: cols}
}

// NewHeaderViewFrom creates a HeaderView from an existing View's columns.
//
// This function extracts the column names and title from the source View
// and creates a new HeaderView where the column names appear as both
// the header and the single data row.
//
// Parameters:
//   - source: The View to extract column names from
//
// Returns a new HeaderView with the source's title and columns.
//
// Example:
//
//	original := retable.NewStringsView("Products", data, "ID", "Name", "Price")
//	headerOnly := retable.NewHeaderViewFrom(original)
//	// headerOnly contains just one row with "ID", "Name", "Price" as values
func NewHeaderViewFrom(source View) *HeaderView {
	return &HeaderView{Tit: source.Title(), Cols: source.Columns()}
}

// HeaderView is a specialized View that contains only a header row.
//
// This view type displays column names as both the column headers and the
// single data row, making it useful for showing just table structure or
// for combining multiple views where you need header information displayed.
//
// The view always has exactly one row (NumRows() returns 1), and that row
// contains the column names as values.
//
// When to use HeaderView:
//   - Displaying table structure/schema information
//   - Creating view combinations where headers need to be visible
//   - Testing or debugging column configurations
//   - Generating table metadata displays
//
// Example:
//
//	view := &retable.HeaderView{
//	    Tit:  "User Schema",
//	    Cols: []string{"ID", "Name", "Email", "Active"},
//	}
//	// Row 0 will contain: "ID", "Name", "Email", "Active"
type HeaderView struct {
	// Tit is the title of this view.
	Tit string

	// Cols contains the column names, which are also used as the data row.
	Cols []string
}

// Title returns the title of this view.
func (view *HeaderView) Title() string { return view.Tit }

// Columns returns the column names of this view.
func (view *HeaderView) Columns() []string { return view.Cols }

// NumRows always returns 1 for HeaderView since it contains only the header row.
func (view *HeaderView) NumRows() int { return 1 }

// Cell returns the column name at the specified column index for row 0.
//
// Since HeaderView has only one row, row must be 0. The returned value
// is the column name at the given column index.
//
// Parameters:
//   - row: Must be 0, otherwise nil is returned
//   - col: Zero-based column index (0 to len(Columns())-1)
//
// Returns:
//   - The column name as any if row is 0 and col is valid
//   - nil if row is not 0 or col is out of bounds
func (view *HeaderView) Cell(row, col int) any {
	if row != 0 || col < 0 || col >= len(view.Cols) {
		return nil
	}
	return view.Cols[col]
}
