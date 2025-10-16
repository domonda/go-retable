package retable

var _ View = ExtraColsView(nil)

// ExtraColsView is a zero-copy decorator that horizontally concatenates multiple Views,
// combining their columns into a single unified View. This is the columnar equivalent
// of a SQL JOIN operation, allowing you to merge data from different sources side-by-side.
//
// # Key Features
//
//   - Horizontal concatenation: Columns from all views appear sequentially
//   - Zero-copy: No data duplication, references original Views
//   - Automatic row padding: Shorter views treated as having nil cells for extra rows
//   - Preserves types: Each cell maintains its original type from source View
//
// # Use Cases
//
//   - Joining related data: Combine user profiles with their statistics
//   - Adding computed columns: Append calculated values to existing data
//   - Merging partial data: Combine views from different data sources
//   - Building composite reports: Unite data from multiple queries
//
// # Row Count Behavior
//
// The resulting view has as many rows as the longest input View.
// Shorter views are implicitly padded with nil values:
//
//	View1: 10 rows, 2 columns
//	View2: 5 rows, 3 columns
//	ExtraColsView: 10 rows, 5 columns (View2 rows 5-9 are all nil)
//
// # Column Order
//
// Columns appear in the order of views in the slice:
//
//	ExtraColsView{view1, view2, view3}
//	Columns: [view1.col0, view1.col1, view2.col0, view2.col1, view3.col0]
//
// # Performance Characteristics
//
//   - No data copying: Only stores View references
//   - Linear column lookup: O(n) where n is number of views
//   - Constant row lookup: O(1) after finding the right view
//   - Memory efficient: Only overhead is the slice of View references
//
// # Example: Basic Column Concatenation
//
//	// View1: Name and Age
//	people := NewStringsView("People", [][]string{
//	    {"Alice", "30"},
//	    {"Bob", "25"},
//	}, []string{"Name", "Age"})
//
//	// View2: City and Country
//	locations := NewStringsView("", [][]string{
//	    {"NYC", "USA"},
//	    {"London", "UK"},
//	}, []string{"City", "Country"})
//
//	// Combine them
//	combined := ExtraColsView{people, locations}
//	// Columns: ["Name", "Age", "City", "Country"]
//	// Row 0: ["Alice", "30", "NYC", "USA"]
//	// Row 1: ["Bob", "25", "London", "UK"]
//
// # Example: Unequal Row Counts
//
//	view1 := NewStringsView("", [][]string{
//	    {"A1"}, {"A2"}, {"A3"},
//	}, []string{"Col1"})
//
//	view2 := NewStringsView("", [][]string{
//	    {"B1"},
//	}, []string{"Col2"})
//
//	combined := ExtraColsView{view1, view2}
//	// 3 rows total (max of 3 and 1)
//	combined.Cell(0, 0) -> "A1"
//	combined.Cell(0, 1) -> "B1"
//	combined.Cell(1, 0) -> "A2"
//	combined.Cell(1, 1) -> nil  // view2 has no row 1
//	combined.Cell(2, 0) -> "A3"
//	combined.Cell(2, 1) -> nil  // view2 has no row 2
//
// # Example: Adding Computed Columns
//
//	// Original data
//	baseView := NewStructRowsView("Sales", salesData, nil, nil)
//
//	// Add computed columns via a custom view
//	computedView := ExtraColsAnyValueFuncView(nil, []string{"Total", "Tax"},
//	    func(row, col int) any {
//	        // Calculate based on baseView data
//	        if col == 0 { return calculateTotal(row) }
//	        return calculateTax(row)
//	    })
//
//	// Combine
//	enriched := ExtraColsView{baseView, computedView}
//	// Now has original columns plus Total and Tax
//
// # Example: Multi-Source Join
//
//	users := loadUsersView()      // id, name, email
//	profiles := loadProfilesView() // bio, avatar
//	stats := loadStatsView()       // post_count, follower_count
//
//	fullProfile := ExtraColsView{users, profiles, stats}
//	// Columns: id, name, email, bio, avatar, post_count, follower_count
//
// # Edge Cases
//
//   - Empty ExtraColsView{} results in 0 rows, 0 columns, empty title
//   - Single view ExtraColsView{view} behaves identically to view
//   - Views with 0 rows contribute 0 to final row count
//   - Negative row/col indices return nil
//   - Column index beyond total column count returns nil
//
// # Composition
//
// ExtraColsView can be nested and combined with other decorators:
//
//	base := NewStringsView(...)
//	extra1 := ExtraColsView{base, computedView1}
//	extra2 := ExtraColsView{extra1, computedView2}
//	filtered := &FilteredView{Source: extra2, RowLimit: 10}
//	// First 10 rows with all computed columns
//
// # Title Behavior
//
// The title is taken from the first view in the slice. If you need a custom
// title, wrap the result with ViewWithTitle.
type ExtraColsView []View

// Title returns the title of the first View in the slice.
// Returns an empty string if the slice is empty.
//
// The title is not a combination of all view titles - only the first is used.
// To set a custom title, use ViewWithTitle to wrap the ExtraColsView.
func (e ExtraColsView) Title() string {
	if len(e) == 0 {
		return ""
	}
	return e[0].Title()
}

// Columns returns all column names from all Views concatenated in order.
//
// The resulting slice contains columns from each view sequentially:
//   [view0.col0, view0.col1, ..., view1.col0, view1.col1, ..., viewN.colM]
//
// Returns an empty slice if ExtraColsView is empty.
//
// Example:
//
//	view1.Columns() -> ["A", "B"]
//	view2.Columns() -> ["C", "D", "E"]
//	ExtraColsView{view1, view2}.Columns() -> ["A", "B", "C", "D", "E"]
func (e ExtraColsView) Columns() []string {
	var columns []string
	for _, view := range e {
		columns = append(columns, view.Columns()...)
	}
	return columns
}

// NumRows returns the maximum row count across all Views.
//
// The combined view has as many rows as its longest component View.
// Shorter views are implicitly treated as having nil cells for additional rows.
//
// Returns 0 if ExtraColsView is empty or all views have 0 rows.
//
// Example:
//
//	view1.NumRows() -> 10
//	view2.NumRows() -> 5
//	view3.NumRows() -> 15
//	ExtraColsView{view1, view2, view3}.NumRows() -> 15
func (e ExtraColsView) NumRows() int {
	maxNumRows := 0
	for _, view := range e {
		maxNumRows = max(maxNumRows, view.NumRows())
	}
	return maxNumRows
}

// Cell returns the value at the specified row and column position.
//
// The column parameter is translated to the appropriate source view:
//  1. Iterates through views to find which one contains the requested column
//  2. Translates col to the local column index within that view
//  3. Returns view.Cell(row, localCol)
//
// Returns nil if:
//   - row or col are negative
//   - col >= total number of columns
//   - row >= NumRows() of the responsible view (implicit nil padding)
//   - The underlying view cell is nil
//
// Example:
//
//	view1 has columns ["A", "B"] (indices 0-1)
//	view2 has columns ["C", "D", "E"] (indices 2-4 in combined view)
//
//	combined := ExtraColsView{view1, view2}
//
//	combined.Cell(0, 0) -> view1.Cell(0, 0)  // Column "A"
//	combined.Cell(0, 1) -> view1.Cell(0, 1)  // Column "B"
//	combined.Cell(0, 2) -> view2.Cell(0, 0)  // Column "C"
//	combined.Cell(0, 3) -> view2.Cell(0, 1)  // Column "D"
//	combined.Cell(0, 4) -> view2.Cell(0, 2)  // Column "E"
//	combined.Cell(0, 5) -> nil               // Out of bounds
//
// Performance: O(n) where n is the number of views, as it must iterate
// to find the correct view. For performance-critical code with many views,
// consider caching column offsets.
func (e ExtraColsView) Cell(row, col int) any {
	if row < 0 || col < 0 {
		return nil
	}
	colLeft := 0
	for _, view := range e {
		numCols := len(view.Columns())
		colRight := colLeft + numCols
		if col < colRight {
			return view.Cell(row, col-colLeft)
		}
		colLeft = colRight
	}
	return nil
}
