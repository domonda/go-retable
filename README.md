# go-retable

[![Go Reference](https://pkg.go.dev/badge/github.com/domonda/go-retable.svg)](https://pkg.go.dev/github.com/domonda/go-retable)
[![Go Report Card](https://goreportcard.com/badge/github.com/domonda/go-retable)](https://goreportcard.com/report/github.com/domonda/go-retable)

A powerful Go library for working with tabular data using reflection. `go-retable` provides a unified interface for reading, transforming, and writing tables from various sources and formats including CSV, Excel, HTML, and SQL.

## Features

- **Unified Table Interface**: Work with all tabular data through a single `View` interface
- **Multiple Format Support**: Read/write CSV, Excel (XLSX), HTML tables, and SQL result sets
- **Type-Safe Conversions**: Convert between struct slices, string slices, and generic values
- **Smart Type Conversion**: Intelligent value assignment across different Go types
- **Zero-Copy Transformations**: Efficient view wrappers for filtering, mapping, and combining data
- **Flexible Formatting**: Customizable cell formatters with type-based routing
- **Struct Tag Support**: Map struct fields to columns using tags
- **Format Auto-Detection**: Automatic detection of CSV encoding, separator, and line endings

## Installation

```bash
go get github.com/domonda/go-retable
```

## Quick Start

### Working with Struct Slices

```go
package main

import (
    "fmt"
    "github.com/domonda/go-retable"
)

type Person struct {
    Name string `col:"Full Name"`
    Age  int    `col:"Age"`
    City string `col:"City"`
}

func main() {
    people := []Person{
        {"Alice Smith", 30, "New York"},
        {"Bob Jones", 25, "London"},
        {"Carol White", 35, "Tokyo"},
    }

    // Create a view from struct slice
    view := retable.NewStructRowsView("People", people, nil, nil)

    // Print the table
    retable.PrintlnView(view)
    // Output:
    // | Full Name   | Age | City     |
    // |-------------|-----|----------|
    // | Alice Smith | 30  | New York |
    // | Bob Jones   | 25  | London   |
    // | Carol White | 35  | Tokyo    |
}
```

### Reading and Writing CSV

```go
import (
    "github.com/domonda/go-retable/csvtable"
)

// Read CSV file
data, format, err := csvtable.ParseFile("data.csv")
if err != nil {
    log.Fatal(err)
}

// Auto-detected format
fmt.Printf("Detected: %s encoding, '%s' separator\n",
    format.Encoding, format.Separator)

// Write CSV with custom formatting
writer := csvtable.NewWriter[[]Person]().
    WithHeaderRow(true).
    WithDelimiter(";").
    WithPadding(csvtable.PadRight)

err = writer.Write(context.Background(), file, people, "People")
```

### Working with Excel Files

```go
import (
    "github.com/domonda/go-retable/exceltable"
)

// Read all sheets from Excel file
sheets, err := exceltable.ReadLocalFile("data.xlsx", false)
if err != nil {
    log.Fatal(err)
}

for _, sheet := range sheets {
    fmt.Printf("Sheet: %s (%d rows, %d cols)\n",
        sheet.Title(),
        sheet.NumRows(),
        len(sheet.Columns()))
}

// Read first sheet only
firstSheet, err := exceltable.ReadLocalFileFirstSheet("data.xlsx", false)
```

### Generating HTML Tables

```go
import (
    "github.com/domonda/go-retable/htmltable"
)

writer := htmltable.NewWriter[[]Person]().
    WithHeaderRow(true).
    WithTableClass("table table-striped")

err := writer.Write(context.Background(), os.Stdout, people, "People")
// Outputs:
// <table class="table table-striped">
//   <thead><tr><th>Full Name</th><th>Age</th><th>City</th></tr></thead>
//   <tbody>
//     <tr><td>Alice Smith</td><td>30</td><td>New York</td></tr>
//     ...
//   </tbody>
// </table>
```

### Converting Between Types

```go
// Convert View back to struct slice
type Employee struct {
    Name     string
    Age      int
    Position string
}

employees, err := retable.ViewToStructSlice[Employee](
    view,
    nil,  // Use default field naming
    nil,  // No custom scanner
    nil,  // No custom formatter
    nil,  // No validation
    "Name", "Age", // Required columns
)
```

## Core Concepts

### View Interface

The `View` interface is the heart of go-retable. It represents any tabular data with rows and columns:

```go
type View interface {
    Title() string      // Table name/title
    Columns() []string  // Column names
    NumRows() int       // Number of data rows
    Cell(row, col int) any // Get cell value
}
```

All table operations work with Views, making the library highly composable.

### View Implementations

**Built-in View Types:**

- `StringsView` - Backed by `[][]string` (CSV, text data)
- `StructRowsView` - Backed by struct slices using reflection
- `AnyValuesView` - Backed by `[][]any` (mixed types, SQL results)
- `ReflectValuesView` - Backed by `[][]reflect.Value` (advanced reflection)

**Example:**

```go
// From strings
data := [][]string{
    {"Alice", "30"},
    {"Bob", "25"},
}
view := retable.NewStringsView("People", data, []string{"Name", "Age"})

// From structs
people := []Person{{"Alice", 30}, {"Bob", 25}}
view := retable.NewStructRowsView("People", people, nil, nil)
```

### View Wrappers (Decorators)

Transform Views without copying data:

```go
// Filter rows and columns
filtered := retable.FilteredView{
    View:          view,
    RowOffset:     10,
    RowLimit:      20,
    ColumnMapping: []int{0, 2, 3}, // Select specific columns
}

// Dereference pointers automatically
deref := retable.DerefView(pointerView)

// Add computed columns
withTotal := retable.ExtraColsReflectValueFuncView(
    view,
    []string{"Total"},
    func(row int) []reflect.Value {
        price := view.Cell(row, 1).(float64)
        qty := view.Cell(row, 2).(int)
        return []reflect.Value{reflect.ValueOf(price * float64(qty))}
    },
)

// Concatenate views horizontally (like SQL JOIN)
joined := retable.ExtraColsView{view1, view2, view3}

// Concatenate views vertically (like SQL UNION)
combined := retable.ExtraRowView{viewA, viewB, viewC}
```

### Cell Formatters

Customize how values are formatted:

```go
// Type-based formatter
formatter := retable.NewReflectTypeCellFormatter().
    WithKindFormatter(reflect.Float64,
        retable.PrintfCellFormatter("%.2f", false)).
    WithTypeFormatter(reflect.TypeOf(time.Time{}),
        retable.LayoutFormatter("2006-01-02"))

// Use in CSV writer
writer := csvtable.NewWriter[[]Product]().
    WithTypeFormatter(formatter)

// Column-specific formatter
writer.WithColumnFormatter(2, // Column index
    retable.PrintfCellFormatter("$%.2f", false))
```

### Struct Field Naming

Control how struct fields map to columns:

```go
type Product struct {
    SKU         string  `csv:"Product Code"`
    Name        string  `csv:"Product Name"`
    Price       float64 `csv:"Unit Price"`
    InternalID  int     `csv:"-"` // Ignored
}

// Use custom naming
naming := &retable.StructFieldNaming{
    Tag:    "csv",
    Ignore: "-",
}

view := retable.NewStructRowsView("Products", products, nil, naming)
// Columns: ["Product Code", "Product Name", "Unit Price"]
```

## Advanced Features

### Smart Type Assignment

`SmartAssign` intelligently converts between different Go types:

```go
var dest int
src := "42"

err := retable.SmartAssign(
    reflect.ValueOf(&dest).Elem(),
    reflect.ValueOf(src),
    nil, // scanner
    nil, // formatter
)
// dest is now 42
```

Supports:
- Direct type conversions
- String parsing (numbers, bools, times, durations)
- Interface unwrapping (`TextMarshaler`, `Stringer`)
- Pointer dereferencing
- Null-like value handling
- Custom formatters and scanners

### SQL Integration

Query in-memory Views using SQL:

```go
import "github.com/domonda/go-retable/sqltable"

// Create virtual database
view := retable.NewStructRowsView("users", users, nil, nil)
db := sqltable.NewViewDB("users", view)
defer db.Close()

// Use standard database/sql
rows, err := db.Query("SELECT name, age FROM users WHERE age > 25")
defer rows.Close()

for rows.Next() {
    var name string
    var age int
    rows.Scan(&name, &age)
    fmt.Printf("%s: %d\n", name, age)
}
```

### Format Detection

Automatically detect CSV format:

```go
import "github.com/domonda/go-retable/csvtable"

config := csvtable.FormatDetectionConfig{
    DetectEncoding:  true,
    DetectSeparator: true,
    DetectNewline:   true,
}

data, format, err := csvtable.ParseFile("unknown.csv")
// Detects: UTF-8/UTF-16LE/ISO-8859-1/Windows-1252/Macintosh
// Detects: , or ; or \t separators
// Detects: \n or \r\n or \n\r line endings
```

## Subpackages

### csvtable

CSV reading and writing with format detection:
- Auto-detect encoding, separator, line endings
- RFC 4180 compliant parsing
- Multi-line field support
- Configurable padding and quoting
- Row modification utilities

### exceltable

Excel file reading using excelize:
- Read XLSX files from filesystem or `io.Reader`
- Multiple sheet support
- Raw or formatted cell values
- Automatic empty row/column cleanup

### htmltable

HTML table generation:
- Template-based output
- Custom CSS classes
- Column and type-based formatters
- Automatic HTML escaping
- Raw HTML output support

### sqltable

Virtual SQL driver for in-memory Views:
- Query Views with SQL syntax
- Standard `database/sql` interface
- Column selection and filtering
- No actual database required

## Utility Functions

```go
// Pretty-print any table data
retable.PrintlnTable(data)

// Get struct field types including embedded fields
fields := retable.StructFieldTypes(reflect.TypeOf(MyStruct{}))

// Convert PascalCase to spaced names
title := retable.SpacePascalCase("UserID")  // "User ID"

// Calculate column widths for alignment
widths := retable.StringColumnWidths([][]string{...})
```

## Examples

### Example 1: CSV to Excel Conversion

```go
// Read CSV
csvData, _, err := csvtable.ParseFile("input.csv")
check(err)

csvView := retable.NewStringsView("Data", csvData, nil)

// Convert to structs for processing
type Record struct {
    ID   int
    Name string
    Date time.Time
}

records, err := retable.ViewToStructSlice[Record](csvView, nil, nil, nil, nil)
check(err)

// Process data...
for i := range records {
    records[i].Name = strings.ToUpper(records[i].Name)
}

// Write to Excel (via another library or export as HTML/CSV)
```

### Example 2: Data Validation Pipeline

```go
type User struct {
    Email string
    Age   int
}

func (u User) Validate() error {
    if !strings.Contains(u.Email, "@") {
        return fmt.Errorf("invalid email: %s", u.Email)
    }
    if u.Age < 18 || u.Age > 120 {
        return fmt.Errorf("invalid age: %d", u.Age)
    }
    return nil
}

// Read and validate
users, err := retable.ViewToStructSlice[User](
    csvView,
    nil, // naming
    nil, // scanner
    nil, // formatter
    retable.CallValidateMethod, // validation
    "Email", "Age", // required columns
)
// Returns error if any user fails validation
```

### Example 3: Report Generation

```go
// Load data from multiple sources
salesView := loadSalesData()
inventoryView := loadInventoryData()

// Join data (add inventory info to sales)
joined := retable.ExtraColsView{salesView, inventoryView}

// Add computed columns
withMargin := retable.ExtraColsReflectValueFuncView(
    joined,
    []string{"Margin %"},
    func(row int) []reflect.Value {
        cost := joined.Cell(row, 2).(float64)
        price := joined.Cell(row, 3).(float64)
        margin := ((price - cost) / price) * 100
        return []reflect.Value{reflect.ValueOf(margin)}
    },
)

// Format as HTML report
writer := htmltable.NewWriter[retable.View]().
    WithHeaderRow(true).
    WithTableClass("report-table").
    WithColumnFormatter(4, retable.PrintfCellFormatter("%.1f%%", false))

writer.Write(ctx, reportFile, withMargin, "Sales Report")
```

## Design Philosophy

### In-Memory Architecture

**go-retable** is designed around a fundamental principle: **tables are completely loaded into memory before being wrapped as Views**. This design decision prioritizes simplicity and performance over streaming capabilities.

**Key implications:**

- **No context cancellation**: View methods don't accept `context.Context` parameters since data is already in memory
- **No error handling in reads**: `Cell()` and other read methods don't return errors - the data is guaranteed to be available
- **Simple API**: The absence of error propagation makes the API cleaner and easier to use
- **Better performance**: Random access to any cell is O(1) without I/O overhead
- **Composability**: Views can be freely composed, transformed, and reused without side effects

**Trade-offs:**

This approach makes go-retable **not suitable for gigantic tables** like those commonly found in large SQL databases (millions+ rows). For such use cases, consider streaming solutions that process data row-by-row.

**Ideal use cases:**
- CSV files (typically < 100K rows)
- Excel spreadsheets (< 1M rows)
- Report generation and data transformation
- Configuration and reference data
- API responses and data exports
- Data validation pipelines

**When to use streaming instead:**
- Processing SQL tables with millions of rows
- ETL pipelines for large datasets
- Real-time data processing
- Memory-constrained environments

## Performance Considerations

- **Views are lightweight**: Most views are just wrappers around existing data
- **Zero-copy transformations**: View decorators don't duplicate data
- **Caching**: `StructRowsView` caches reflected values for efficiency
- **Reflection overhead**: Type-based operations use reflection; consider caching for tight loops
- **Memory footprint**: Entire table loaded in memory - typical CSV/Excel files fit comfortably, but be mindful of very large datasets

## Thread Safety

- **Views are generally not thread-safe** for concurrent modifications to underlying data
- **Immutable operations**: Reading from Views is safe if underlying data doesn't change
- **Writers use immutable builder pattern**: Safe to share writer configurations

## Best Practices

1. **Use struct tags** for explicit column mapping: `col:"Column Name"`
2. **Validate data** using `ViewToStructSlice` with validation functions
3. **Choose the right View type**:
   - `StringsView` for CSV/text data
   - `StructRowsView` for typed data
   - `AnyValuesView` for mixed types
4. **Compose View wrappers** for complex transformations
5. **Reuse formatters** rather than creating new ones per cell
6. **Use type-based formatters** for consistency across columns

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

[MIT License](LICENSE)

## Related Projects

- [excelize](https://github.com/qax-os/excelize) - Excel file library (used by exceltable)
- [charset](https://pkg.go.dev/golang.org/x/text/encoding) - Character encoding support
