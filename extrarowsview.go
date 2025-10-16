package retable

var _ View = ExtraRowView(nil)

// ExtraRowView is a zero-copy decorator that vertically concatenates multiple Views,
// stacking their rows into a single unified View. This is the row equivalent of a
// SQL UNION operation, allowing you to combine data from different sources with the
// same column structure.
//
// # Key Features
//
//   - Vertical concatenation: Rows from all views appear sequentially
//   - Zero-copy: No data duplication, references original Views
//   - Column alignment: All views must have compatible column structures
//   - Preserves types: Each cell maintains its original type from source View
//
// # Use Cases
//
//   - Combining result sets: Merge data from multiple queries
//   - Appending batches: Stack incrementally loaded data
//   - Unifying data sources: Combine test + prod data for reports
//   - Building composite datasets: Unite similar data from different time periods
//
// # Column Structure Requirements
//
// ExtraRowView uses the column structure from the first view in the slice.
// All subsequent views should have the same column count and compatible types.
// The implementation does NOT validate column compatibility - it's the caller's
// responsibility to ensure views have matching structures.
//
// Column names are taken only from the first view:
//
//	View1: ["Name", "Age"]  <- columns used
//	View2: ["Name", "Age"]
//	ExtraRowView columns: ["Name", "Age"]
//
// If views have different columns, cells are accessed by position only:
//
//	View1: ["A", "B"]
//	View2: ["X", "Y"]
//	ExtraRowView: columns ["A", "B"], but view2.Cell(0, 0) returns "X" data
//
// # Row Count Behavior
//
// The resulting view has the sum of all input Views' row counts:
//
//	View1: 10 rows
//	View2: 5 rows
//	View3: 15 rows
//	ExtraRowView: 30 rows total
//
// # Performance Characteristics
//
//   - No data copying: Only stores View references
//   - Linear row lookup: O(n) where n is number of views
//   - Constant column lookup: O(1)
//   - Memory efficient: Only overhead is the slice of View references
//
// # Example: Basic Row Concatenation
//
//	// Historical data
//	past := NewStringsView("", [][]string{
//	    {"Alice", "30"},
//	    {"Bob", "25"},
//	}, []string{"Name", "Age"})
//
//	// Recent data
//	recent := NewStringsView("", [][]string{
//	    {"Charlie", "35"},
//	    {"Diana", "28"},
//	}, []string{"Name", "Age"})
//
//	// Combine them
//	all := ExtraRowView{past, recent}
//	// 4 rows total:
//	// Row 0: ["Alice", "30"]   (from past)
//	// Row 1: ["Bob", "25"]     (from past)
//	// Row 2: ["Charlie", "35"] (from recent)
//	// Row 3: ["Diana", "28"]   (from recent)
//
// # Example: Combining Multiple Data Sources
//
//	usData := loadUSCustomers()      // 100 rows
//	euData := loadEUCustomers()      // 75 rows
//	asiaData := loadAsiaCustomers()  // 50 rows
//
//	allCustomers := ExtraRowView{usData, euData, asiaData}
//	// 225 rows total, all with same columns
//
// # Example: Appending Summary Rows
//
//	dataRows := loadDataView()
//
//	// Create a summary row view
//	summaryData := [][]any{
//	    {"TOTAL", sumValues(), avgValues()},
//	}
//	summary := NewAnyValuesView("", summaryData, dataRows.Columns())
//
//	// Append summary at the bottom
//	withSummary := ExtraRowView{dataRows, summary}
//	// Last row contains the totals
//
// # Example: Combining Filtered Subsets
//
//	allData := loadDataView()
//
//	// Split into categories
//	categoryA := &FilteredView{
//	    Source: allData,
//	    RowOffset: 0,
//	    RowLimit: 10,
//	}
//	categoryB := &FilteredView{
//	    Source: allData,
//	    RowOffset: 50,
//	    RowLimit: 10,
//	}
//
//	// Recombine selected rows
//	selected := ExtraRowView{categoryA, categoryB}
//	// 20 rows: first 10 from categoryA, then 10 from categoryB
//
// # Example: Multi-Batch Loading
//
//	var batches []View
//	for batch := range loadIncrementally() {
//	    batches = append(batches, batch)
//	}
//	allData := ExtraRowView(batches)
//	// All batches combined into single view
//
// # Row Index Translation
//
// Cell access translates row indices to the appropriate source view:
//
//	view1 has 10 rows (indices 0-9)
//	view2 has 5 rows (indices 10-14 in combined view)
//	view3 has 8 rows (indices 15-22 in combined view)
//
//	combined := ExtraRowView{view1, view2, view3}
//
//	combined.Cell(5, 0)  -> view1.Cell(5, 0)   // Within view1
//	combined.Cell(12, 0) -> view2.Cell(2, 0)   // Within view2 (12-10=2)
//	combined.Cell(20, 0) -> view3.Cell(5, 0)   // Within view3 (20-15=5)
//
// # Edge Cases
//
//   - Empty ExtraRowView{} results in 0 rows, 0 columns, empty title
//   - Single view ExtraRowView{view} behaves identically to view
//   - Views with 0 rows contribute nothing to final result
//   - Negative row/col indices return nil
//   - Column index beyond first view's column count returns nil
//   - Row index beyond total row count returns nil
//
// # Column Mismatch Behavior
//
// If views have different column counts, no error occurs:
//   - Columns() returns the first view's columns only
//   - Accessing a column index >= a view's column count returns nil for that view's rows
//
//	view1 := NewStringsView("", [][]string{{"A", "B"}}, []string{"Col1", "Col2"})
//	view2 := NewStringsView("", [][]string{{"X"}}, []string{"Col1"})
//
//	combined := ExtraRowView{view1, view2}
//	combined.Columns() -> ["Col1", "Col2"]
//	combined.Cell(0, 0) -> "A"
//	combined.Cell(0, 1) -> "B"
//	combined.Cell(1, 0) -> "X"
//	combined.Cell(1, 1) -> nil  // view2 has no column 1
//
// # Composition
//
// ExtraRowView can be nested and combined with other decorators:
//
//	batch1 := loadBatch1()
//	batch2 := loadBatch2()
//	combined := ExtraRowView{batch1, batch2}
//
//	// Filter the combined result
//	filtered := &FilteredView{
//	    Source: combined,
//	    RowLimit: 100,
//	}
//
//	// Add computed columns to the combination
//	enriched := ExtraColsAnyValueFuncView(filtered, []string{"Score"}, scoreFunc)
//
// # Title Behavior
//
// The title is taken from the first view in the slice. If you need a custom
// title, wrap the result with ViewWithTitle.
type ExtraRowView []View

// Title returns the title of the first View in the slice.
// Returns an empty string if the slice is empty.
//
// The title is not a combination of all view titles - only the first is used.
// To set a custom title, use ViewWithTitle to wrap the ExtraRowView.
func (e ExtraRowView) Title() string {
	if len(e) == 0 {
		return ""
	}
	return e[0].Title()
}

// Columns returns the column names from the first View in the slice.
// Returns nil if the slice is empty.
//
// All views in ExtraRowView should have the same column structure,
// but this is not enforced. Only the first view's columns are used
// to define the structure of the combined view.
//
// Example:
//
//	view1.Columns() -> ["A", "B", "C"]
//	view2.Columns() -> ["A", "B", "C"]
//	ExtraRowView{view1, view2}.Columns() -> ["A", "B", "C"]
func (e ExtraRowView) Columns() []string {
	if len(e) == 0 {
		return nil
	}
	return e[0].Columns()
}

// NumRows returns the total row count across all Views.
//
// The combined view has the sum of row counts from all component Views.
// Views with 0 rows contribute nothing to the total.
//
// Returns 0 if ExtraRowView is empty or all views have 0 rows.
//
// Example:
//
//	view1.NumRows() -> 10
//	view2.NumRows() -> 5
//	view3.NumRows() -> 15
//	ExtraRowView{view1, view2, view3}.NumRows() -> 30
func (e ExtraRowView) NumRows() int {
	numRows := 0
	for _, view := range e {
		numRows += view.NumRows()
	}
	return numRows
}

// Cell returns the value at the specified row and column position.
//
// The row parameter is translated to the appropriate source view:
//  1. Iterates through views to find which one contains the requested row
//  2. Translates row to the local row index within that view
//  3. Returns view.Cell(localRow, col)
//
// Returns nil if:
//   - row or col are negative
//   - row >= total number of rows
//   - col >= number of columns (from first view)
//   - The underlying view cell is nil
//
// Example:
//
//	view1 has 2 rows (indices 0-1 in combined view)
//	view2 has 3 rows (indices 2-4 in combined view)
//
//	combined := ExtraRowView{view1, view2}
//
//	combined.Cell(0, 0) -> view1.Cell(0, 0)  // First row of view1
//	combined.Cell(1, 0) -> view1.Cell(1, 0)  // Second row of view1
//	combined.Cell(2, 0) -> view2.Cell(0, 0)  // First row of view2
//	combined.Cell(3, 0) -> view2.Cell(1, 0)  // Second row of view2
//	combined.Cell(4, 0) -> view2.Cell(2, 0)  // Third row of view2
//	combined.Cell(5, 0) -> nil               // Out of bounds
//
// Performance: O(n) where n is the number of views, as it must iterate
// to find the correct view. For performance-critical code with many views,
// consider pre-computing row offsets.
func (e ExtraRowView) Cell(row, col int) any {
	if row < 0 || col < 0 || col >= len(e.Columns()) {
		return nil
	}
	rowTop := 0
	for _, view := range e {
		numRows := view.NumRows()
		rowBottom := rowTop + numRows
		if row < rowBottom {
			return view.Cell(row-rowTop, col)
		}
		rowTop = rowBottom
	}
	return nil
}
