package retable

var _ View = new(FilteredView)

// FilteredView is a decorator that provides zero-copy filtering, slicing, and column
// remapping of an underlying View. It implements pagination, column selection, and
// column reordering without duplicating the underlying data.
//
// # Key Features
//
//   - Row slicing: Skip rows (RowOffset) and limit results (RowLimit)
//   - Column filtering: Select specific columns via ColumnMapping
//   - Column reordering: Rearrange columns by mapping indices
//   - Zero-copy: No data duplication, just index manipulation
//
// # Use Cases
//
//   - Pagination: Implement offset/limit patterns for large datasets
//   - Column selection: Export only specific columns (e.g., for CSV or API responses)
//   - Column reordering: Change column display order without modifying source data
//   - Data projection: Create different views of the same underlying data
//
// # Performance Characteristics
//
// FilteredView is extremely lightweight:
//   - No data copying: Source data is never duplicated
//   - Constant overhead: Cell access adds only index arithmetic
//   - Memory efficient: Only stores integers for offset, limit, and mapping
//
// # Example: Pagination
//
//	// Original view with 1000 rows
//	source := NewStringsView("Products", productData, columns)
//
//	// Get page 3 (rows 20-29, assuming page size of 10)
//	page3 := &FilteredView{
//	    Source:    source,
//	    RowOffset: 20,
//	    RowLimit:  10,
//	}
//	fmt.Println(page3.NumRows()) // 10
//
// # Example: Column Selection
//
//	// Select only columns 0, 2, and 4 from source
//	filtered := &FilteredView{
//	    Source:        source,
//	    ColumnMapping: []int{0, 2, 4},
//	}
//	// Accessing column 0 of filtered reads column 0 of source
//	// Accessing column 1 of filtered reads column 2 of source
//	// Accessing column 2 of filtered reads column 4 of source
//
// # Example: Column Reordering
//
//	// Reverse column order (assuming 3 columns)
//	reversed := &FilteredView{
//	    Source:        source,
//	    ColumnMapping: []int{2, 1, 0},
//	}
//
// # Example: Combined Operations
//
//	// Select specific columns AND paginate
//	view := &FilteredView{
//	    Source:        source,
//	    RowOffset:     100,
//	    RowLimit:      25,
//	    ColumnMapping: []int{0, 3, 5}, // Only columns 0, 3, 5
//	}
//
// # Field Details
//
//   - Source: The underlying View to filter/transform (required)
//   - RowOffset: Number of rows to skip from the beginning (0 = no offset)
//   - RowLimit: Maximum number of rows to include (0 = no limit, show all remaining)
//   - ColumnMapping: nil = all columns; []int = column indices to include/reorder
//
// # Edge Cases
//
//   - Negative RowOffset is treated as 0
//   - RowOffset beyond source length results in 0 rows
//   - Out-of-bounds cell access returns nil
//   - Invalid column mapping indices will panic on Cell() access
//
// # Composition
//
// FilteredView can wrap other FilteredViews for complex transformations:
//
//	base := NewStringsView(...)
//	step1 := &FilteredView{Source: base, ColumnMapping: []int{0, 2, 4, 6}}
//	step2 := &FilteredView{Source: step1, RowOffset: 10, RowLimit: 5}
//	// step2 shows rows 10-14 of base, with columns 0, 2, 4, 6
type FilteredView struct {
	// Source is the underlying View to filter and transform.
	// All Cell() calls are delegated to this View after applying
	// row offset and column mapping transformations.
	Source View

	// RowOffset is the index of the first row from Source to include.
	// Rows 0 to RowOffset-1 are skipped.
	// Negative values are treated as 0.
	// If RowOffset >= Source.NumRows(), the view will have 0 rows.
	//
	// Example:
	//   RowOffset: 0  -> starts at first row (no offset)
	//   RowOffset: 10 -> skips first 10 rows, starts at row 10
	RowOffset int

	// RowLimit caps the maximum number of rows in this view.
	// If RowLimit is 0 or negative, no limit is applied (all rows shown).
	// If RowLimit > 0, at most RowLimit rows are shown.
	// The actual row count may be less if Source has fewer rows available.
	//
	// Example:
	//   RowLimit: 0  -> no limit, show all rows after RowOffset
	//   RowLimit: 10 -> show at most 10 rows
	RowLimit int

	// ColumnMapping controls which columns are visible and in what order.
	//
	// If nil:
	//   - All columns from Source are included
	//   - Columns appear in their original order
	//   - NumCols() returns len(Source.Columns())
	//
	// If not nil:
	//   - Only mapped columns are included
	//   - len(ColumnMapping) determines NumCols()
	//   - Each element is an index into Source's columns
	//   - Columns can be reordered or duplicated
	//
	// Example:
	//   ColumnMapping: nil        -> [0, 1, 2, 3] (all columns)
	//   ColumnMapping: []int{0}   -> [0] (only first column)
	//   ColumnMapping: []int{2,0} -> [2, 0] (columns 2 and 0, reordered)
	//   ColumnMapping: []int{1,1} -> [1, 1] (column 1 duplicated)
	//
	// WARNING: Invalid indices (negative or >= Source.NumCols()) will cause
	// panics when accessing cells or column names.
	ColumnMapping []int
}

// Title returns the title from the underlying Source view.
// FilteredView does not modify the title.
func (view *FilteredView) Title() string {
	return view.Source.Title()
}

// Columns returns the column names for this filtered view.
//
// If ColumnMapping is nil, returns all columns from Source in their original order.
// If ColumnMapping is set, returns only the mapped columns in the specified order.
//
// The returned slice is newly allocated and safe to modify.
//
// Example:
//
//	source.Columns() -> ["A", "B", "C", "D"]
//	view := &FilteredView{Source: source, ColumnMapping: []int{3, 1}}
//	view.Columns() -> ["D", "B"]
func (view *FilteredView) Columns() []string {
	sourceCols := view.Source.Columns()
	if view.ColumnMapping == nil {
		return sourceCols
	}
	mappedCols := make([]string, len(view.ColumnMapping))
	for i, iSource := range view.ColumnMapping {
		mappedCols[i] = sourceCols[iSource]
	}
	return mappedCols
}

// NumCols returns the number of columns in this filtered view.
//
// If ColumnMapping is nil, returns the number of columns in Source.
// If ColumnMapping is set, returns len(ColumnMapping).
//
// This method is more efficient than len(view.Columns()) as it avoids
// allocating the column name slice.
func (view *FilteredView) NumCols() int {
	if view.ColumnMapping != nil {
		return len(view.ColumnMapping)
	}
	return len(view.Source.Columns())
}

// NumRows returns the number of rows visible in this filtered view after
// applying RowOffset and RowLimit.
//
// The calculation:
//  1. Start with Source.NumRows()
//  2. Subtract RowOffset (treated as 0 if negative)
//  3. Apply RowLimit if > 0
//  4. Return 0 if result would be negative
//
// Examples:
//
//	Source has 100 rows:
//	  RowOffset: 0,  RowLimit: 0   -> 100 rows
//	  RowOffset: 10, RowLimit: 0   -> 90 rows
//	  RowOffset: 10, RowLimit: 20  -> 20 rows
//	  RowOffset: 95, RowLimit: 20  -> 5 rows (limited by available data)
//	  RowOffset: 200, RowLimit: 0  -> 0 rows (offset beyond data)
func (view *FilteredView) NumRows() int {
	n := view.Source.NumRows() - max(view.RowOffset, 0)
	if n < 0 {
		return 0
	}
	if view.RowLimit > 0 && n > view.RowLimit {
		return view.RowLimit
	}
	return n
}

// Cell returns the value at the specified row and column position in this filtered view.
//
// The row and col parameters are relative to this view's coordinate space
// (after filtering), not the Source view's coordinates.
//
// Returns nil if:
//   - row or col are negative
//   - row >= NumRows()
//   - col >= NumCols()
//   - The underlying Source cell is nil
//
// The method translates coordinates before accessing Source:
//   - Row: adds RowOffset to translate to Source's row space
//   - Col: maps through ColumnMapping if set, otherwise uses col directly
//
// Example:
//
//	source := NewStringsView("Data", [][]string{
//	    {"A0", "B0", "C0"},  // row 0
//	    {"A1", "B1", "C1"},  // row 1
//	    {"A2", "B2", "C2"},  // row 2
//	    {"A3", "B3", "C3"},  // row 3
//	}, []string{"A", "B", "C"})
//
//	filtered := &FilteredView{
//	    Source:        source,
//	    RowOffset:     1,           // skip first row
//	    RowLimit:      2,           // show 2 rows
//	    ColumnMapping: []int{2, 0}, // columns C, A
//	}
//
//	filtered.Cell(0, 0) -> "C1"  (row 1, col 2 in source)
//	filtered.Cell(0, 1) -> "A1"  (row 1, col 0 in source)
//	filtered.Cell(1, 0) -> "C2"  (row 2, col 2 in source)
//	filtered.Cell(2, 0) -> nil   (row 2 is out of bounds in filtered view)
func (view *FilteredView) Cell(row, col int) any {
	numRows := view.NumRows()
	numCols := view.NumCols()
	if row < 0 || col < 0 || row >= numRows || col >= numCols {
		return nil
	}
	row += max(view.RowOffset, 0)
	if view.ColumnMapping != nil {
		col = view.ColumnMapping[col]
	}
	return view.Source.Cell(row, col)
}
