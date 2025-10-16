package retable

import (
	"reflect"
)

// ExtraColsAnyValueFuncView creates a View that appends dynamically computed columns
// to a base View. The additional column values are generated on-demand by a user-provided
// function, enabling lazy evaluation and computed fields without data duplication.
//
// # Use Cases
//
//   - Adding calculated fields: totals, percentages, derived metrics
//   - Generating display columns: formatted strings, URLs, labels
//   - Implementing virtual columns: values computed from other columns
//   - Creating composite views: enrich data with external lookups
//
// # How It Works
//
// The function receives relative column indices (0-based within the added columns)
// and row indices, returning the computed value for that position. The left View
// provides the base columns, and the function provides the extra columns on the right.
//
// # Performance
//
//   - Zero base data copying: Left view data is never duplicated
//   - Lazy evaluation: Values computed only when accessed
//   - Function overhead: Each Cell() call invokes the function
//   - Efficient for sparse access: Only computes accessed cells
//
// # Parameters
//
//   - left: The base View to extend (can be nil for a pure computed view)
//   - columns: Names for the computed columns (determines column count)
//   - anyValue: Function that computes cell values (row, col) -> any
//
// # Column Indices
//
// The col parameter passed to anyValue is 0-based within the new columns only:
//
//	left has 3 columns: ["A", "B", "C"]
//	Adding 2 columns: ["D", "E"]
//
//	Combined view columns: ["A", "B", "C", "D", "E"]
//	Cell(0, 3) -> anyValue(0, 0)  // col 3 is "D", passed as 0 to anyValue
//	Cell(0, 4) -> anyValue(0, 1)  // col 4 is "E", passed as 1 to anyValue
//
// # Example: Adding Calculated Columns
//
//	// Base data: product prices
//	products := NewStringsView("Products", [][]string{
//	    {"Widget", "100"},
//	    {"Gadget", "200"},
//	}, []string{"Name", "Price"})
//
//	// Add computed tax and total columns
//	withTax := ExtraColsAnyValueFuncView(products, []string{"Tax", "Total"},
//	    func(row, col int) any {
//	        priceStr := products.Cell(row, 1).(string)
//	        price, _ := strconv.ParseFloat(priceStr, 64)
//	        if col == 0 { // Tax column
//	            return price * 0.1
//	        }
//	        // Total column
//	        return price * 1.1
//	    })
//
//	// Result columns: ["Name", "Price", "Tax", "Total"]
//	// Row 0: ["Widget", "100", 10.0, 110.0]
//	// Row 1: ["Gadget", "200", 20.0, 220.0]
//
// # Example: Pure Computed View (No Left View)
//
//	// Generate a multiplication table without any base data
//	table := ExtraColsAnyValueFuncView(nil, []string{"A", "B", "Product"},
//	    func(row, col int) any {
//	        if col == 0 {
//	            return row + 1
//	        }
//	        if col == 1 {
//	            return row + 2
//	        }
//	        return (row + 1) * (row + 2)
//	    })
//
//	// When NumRows() is called, returns 0 because left is nil
//	// Typically you'd wrap this in a FilteredView to set row count
//
// # Example: Lookups and External Data
//
//	userData := loadUserView() // id, name
//
//	withStatus := ExtraColsAnyValueFuncView(userData, []string{"Status", "LastSeen"},
//	    func(row, col int) any {
//	        userID := userData.Cell(row, 0).(int)
//	        if col == 0 {
//	            return statusCache.Get(userID)
//	        }
//	        return lastSeenCache.Get(userID)
//	    })
//
// # Example: Conditional Formatting
//
//	scores := loadScoresView() // name, score
//
//	withGrade := ExtraColsAnyValueFuncView(scores, []string{"Grade"},
//	    func(row, col int) any {
//	        score := scores.Cell(row, 1).(int)
//	        if score >= 90 { return "A" }
//	        if score >= 80 { return "B" }
//	        if score >= 70 { return "C" }
//	        return "F"
//	    })
//
// # Edge Cases
//
//   - If left is nil, NumRows() returns 0 (function never called)
//   - Empty columns slice is valid (adds no columns)
//   - Function can return nil for any cell
//   - Out-of-bounds access returns nil (function not called)
//
// # Composition
//
// Can be chained to add multiple sets of computed columns:
//
//	base := loadDataView()
//	step1 := ExtraColsAnyValueFuncView(base, []string{"A"}, funcA)
//	step2 := ExtraColsAnyValueFuncView(step1, []string{"B", "C"}, funcBC)
//	// step2 has base columns plus A, B, C
//
// # Return Type
//
// Returns a ReflectCellView, providing both Cell() and ReflectCell() access.
// This enables efficient integration with reflection-based operations.
func ExtraColsAnyValueFuncView(left View, columns []string, anyValue func(row, col int) any) ReflectCellView {
	return &extraColsFuncView{
		left:     AsReflectCellView(left),
		columns:  columns,
		anyValue: anyValue,
		reflectValue: func(row, col int) reflect.Value {
			return reflect.ValueOf(anyValue(row, col))
		},
	}
}

// ExtraColsReflectValueFuncView creates a View that appends dynamically computed columns
// using a reflection-based value function. This is similar to ExtraColsAnyValueFuncView
// but provides reflect.Value access for efficiency when working with reflection.
//
// # When To Use
//
// Use this instead of ExtraColsAnyValueFuncView when:
//   - You're already working with reflect.Value in your code
//   - You need to avoid boxing/unboxing via Interface()
//   - You want to return invalid reflect.Value to represent nil
//   - Performance-critical reflection operations
//
// # Key Difference
//
// The function receives and returns reflect.Value instead of any:
//   - More efficient: Avoids Interface() conversions
//   - More control: Can return invalid reflect.Values for nil
//   - More complex: Requires reflection knowledge
//
// # Parameters
//
//   - left: The base View to extend (can be nil)
//   - columns: Names for the computed columns
//   - reflectValue: Function that computes cell values (row, col) -> reflect.Value
//
// # Example: Efficient Reflection-Based Computation
//
//	type Product struct {
//	    Name  string
//	    Price float64
//	}
//
//	products := NewStructRowsView("Products", productSlice, nil, nil)
//
//	withFormatted := ExtraColsReflectValueFuncView(products, []string{"FormattedPrice"},
//	    func(row, col int) reflect.Value {
//	        rv := products.(ReflectCellView).ReflectCell(row, 1) // Price field
//	        price := rv.Float()
//	        formatted := fmt.Sprintf("$%.2f", price)
//	        return reflect.ValueOf(formatted)
//	    })
//
// # Example: Returning nil as Invalid Value
//
//	withOptional := ExtraColsReflectValueFuncView(base, []string{"Optional"},
//	    func(row, col int) reflect.Value {
//	        if shouldBeNil(row) {
//	            return reflect.Value{} // Invalid value represents nil
//	        }
//	        return reflect.ValueOf(computeValue(row))
//	    })
//
// # Example: Type Conversion with Reflection
//
//	withConverted := ExtraColsReflectValueFuncView(base, []string{"AsString"},
//	    func(row, col int) reflect.Value {
//	        val := base.(ReflectCellView).ReflectCell(row, 0)
//	        // Use reflection to convert to string
//	        str := fmt.Sprint(val.Interface())
//	        return reflect.ValueOf(str)
//	    })
//
// # Invalid Reflect Values
//
// If reflectValue returns an invalid reflect.Value (reflect.Value{}),
// the Cell() method will return nil. This is the recommended way to
// represent null/missing values.
//
// # Performance Advantages
//
// When implementing complex reflection-based transformations:
//
//	// Less efficient: Using ExtraColsAnyValueFuncView
//	ExtraColsAnyValueFuncView(v, cols, func(r, c int) any {
//	    rv := v.(ReflectCellView).ReflectCell(r, c)
//	    // ... reflection operations ...
//	    return rv.Interface() // Extra boxing
//	})
//
//	// More efficient: Using ExtraColsReflectValueFuncView
//	ExtraColsReflectValueFuncView(v, cols, func(r, c int) reflect.Value {
//	    rv := v.(ReflectCellView).ReflectCell(r, c)
//	    // ... reflection operations ...
//	    return rv // No boxing
//	})
//
// # Return Type
//
// Returns a ReflectCellView, providing efficient reflection-based access
// through both Cell() and ReflectCell() methods.
func ExtraColsReflectValueFuncView(left View, columns []string, reflectValue func(row, col int) reflect.Value) ReflectCellView {
	return &extraColsFuncView{
		left:    AsReflectCellView(left),
		columns: columns,
		anyValue: func(row, col int) any {
			v := reflectValue(row, col)
			if !v.IsValid() {
				return nil
			}
			return v.Interface()
		},
		reflectValue: reflectValue,
	}
}

// extraColsFuncView is the internal implementation for function-based extra columns.
// It stores both function variants (any and reflect.Value) to support both access patterns.
type extraColsFuncView struct {
	// left is the base View providing initial columns
	left ReflectCellView

	// columns are the names of the computed columns to append
	columns []string

	// anyValue computes cell values as any (used by Cell method)
	anyValue func(row, col int) any

	// reflectValue computes cell values as reflect.Value (used by ReflectCell method)
	reflectValue func(row, col int) reflect.Value
}

// Title returns the title from the left View.
// Returns an empty string if left is nil.
func (e *extraColsFuncView) Title() string {
	return e.left.Title()
}

// Columns returns all column names: left columns followed by computed columns.
//
// Example:
//
//	left.Columns() -> ["A", "B"]
//	e.columns -> ["C", "D"]
//	e.Columns() -> ["A", "B", "C", "D"]
func (e *extraColsFuncView) Columns() []string {
	return append(e.left.Columns(), e.columns...)
}

// NumRows returns the row count from the left View.
// Returns 0 if left is nil.
//
// Note: The computed columns do not affect row count - they are evaluated
// for the same rows as the left View.
func (e *extraColsFuncView) NumRows() int {
	return e.left.NumRows()
}

// Cell returns the value at the specified position.
//
// If col is within the left View's columns, delegates to left.Cell(row, col).
// If col is in the computed columns range, calls anyValue(row, col-numLeftCols).
//
// Returns nil if row/col are out of bounds or if the computed function returns nil.
//
// Example:
//
//	left has 3 columns
//	e has 2 computed columns (total 5 columns)
//
//	e.Cell(0, 0) -> left.Cell(0, 0)
//	e.Cell(0, 2) -> left.Cell(0, 2)
//	e.Cell(0, 3) -> anyValue(0, 0)  // First computed column
//	e.Cell(0, 4) -> anyValue(0, 1)  // Second computed column
func (e *extraColsFuncView) Cell(row, col int) any {
	numLeftCols := len(e.left.Columns())
	if col < numLeftCols {
		return e.left.Cell(row, col)
	}
	return e.anyValue(row, col-numLeftCols)
}

// ReflectCell returns the reflect.Value at the specified position.
//
// If col is within the left View's columns, delegates to left.ReflectCell(row, col).
// If col is in the computed columns range, calls reflectValue(row, col-numLeftCols).
//
// This method is more efficient than Cell() when working with reflection-based code,
// as it avoids the Interface() boxing operation.
//
// Example:
//
//	left has 3 columns
//	e has 2 computed columns
//
//	e.ReflectCell(0, 0) -> left.ReflectCell(0, 0)
//	e.ReflectCell(0, 3) -> reflectValue(0, 0)  // First computed column
//	e.ReflectCell(0, 4) -> reflectValue(0, 1)  // Second computed column
func (e *extraColsFuncView) ReflectCell(row, col int) reflect.Value {
	numLeftCols := len(e.left.Columns())
	if col < numLeftCols {
		return e.left.ReflectCell(row, col)
	}
	return e.reflectValue(row, col-numLeftCols)
}
