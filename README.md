# go-retable

[![Go Reference](https://pkg.go.dev/badge/github.com/domonda/go-retable.svg)](https://pkg.go.dev/github.com/domonda/go-retable)
[![Go Report Card](https://goreportcard.com/badge/github.com/domonda/go-retable)](https://goreportcard.com/report/github.com/domonda/go-retable)

A powerful Go library for working with tabular data using reflection. `go-retable` provides a unified interface for reading, transforming, and writing tables from various sources and formats including CSV, Excel, HTML, and SQL.

## Features

- **Unified Table Interface**: Work with all tabular data through a single `View` interface
- **Multiple Format Support**: Read/write CSV and Excel (XLSX) with typed cells, write HTML tables, read SQL result sets
- **Type-Safe Conversions**: Convert between struct slices, string slices, and generic values
- **Smart Type Conversion**: Intelligent value assignment across different Go types
- **Configurable Parsing**: Locale number formats, custom time layouts and null-value spellings, with scanners for your own types
- **Zero-Copy Transformations**: Efficient view wrappers for filtering, mapping, and combining data
- **Flexible Formatting**: Customizable cell formatters with type-based routing
- **Struct Tag Support**: Map struct fields to columns using tags
- **Format Auto-Detection**: Automatic detection of CSV encoding, separator, and line endings

## Installation

```bash
go get github.com/domonda/go-retable
```

## Documentation

This README introduces every part of the library. The guides in `docs/` go
deeper:

| Topic | How-to | Reference | Explanation |
| --- | --- | --- | --- |
| Importing files typed by people | [How to import tables typed by people](docs/retable/howto-import-tables-typed-by-people.md) | [StructFieldNaming](docs/retable/reference-struct-field-naming.md) | |
| CSV | [How to read and write CSV files](docs/csvtable/howto-read-and-write-csv.md) | [csvtable](docs/csvtable/reference.md) | [How csvtable reads malformed CSV](docs/csvtable/explanation-malformed-csv.md) |
| Excel | [How to write Excel workbooks](docs/exceltable/howto-write-workbooks.md) | [exceltable Writer](docs/exceltable/reference-writer.md) | [Why exceltable writes typed cells](docs/exceltable/explanation-typed-cells.md) |

The generated API docs are on [pkg.go.dev](https://pkg.go.dev/github.com/domonda/go-retable).

## Quick Start

### Working with Struct Slices

```go
package main

import (
    "log"

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
    view, err := retable.DefaultStructRowsViewer().NewView("People", people)
    if err != nil {
        log.Fatal(err)
    }

    // Print the table
    retable.PrintlnView(view)
    // Output:
    // People:
    // | Full Name   | Age | City     |
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

// Read a CSV file, detecting encoding, separator and line endings
csvBytes, err := os.ReadFile("data.csv")
if err != nil {
    log.Fatal(err)
}
data, format, err := csvtable.ParseDetectFormat(csvBytes, nil)
if err != nil {
    log.Fatal(err)
}

// Auto-detected format
fmt.Printf("Detected: %s encoding, '%s' separator\n",
    format.Encoding, format.Separator)

// Write CSV with custom formatting
writer := csvtable.NewWriter[[]Person]().
    WithHeaderRow(true).
    WithDelimiter(';').
    WithPadding(csvtable.AlignLeft)

err = writer.Write(context.Background(), file, people)
```

To go straight from a file to typed structs, and for reading exports that carry
header and trailer lines around the table, see
[How to read and write CSV files](docs/csvtable/howto-read-and-write-csv.md).

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
        sheet.NumColumns())
}

// Read first sheet only
firstSheet, err := exceltable.ReadLocalFileFirstSheet("data.xlsx", false)
```

Writing keeps cells typed, so numbers stay numbers and dates stay Excel dates
that can be sorted, filtered and summed:

```go
type Invoice struct {
    Number string    `col:"Invoice"`
    Date   time.Time `col:"Date"`
    Total  float64   `col:"Total"`
}

writer := exceltable.NewWriter[[]Invoice]().
    WithHeaderRow(true).
    WithTimeFormat("dd.mm.yyyy").
    WithColumnFormatter(2, exceltable.CellFormatterFunc(
        func(ctx context.Context, view retable.View, row, col int) (exceltable.Cell, error) {
            return exceltable.NumberCell(view.Cell(row, col).(float64), "#,##0.00"), nil
        },
    ))

// One sheet
err := writer.Write(ctx, w, invoices, "Invoices")

// Several sheets, each with its own row type
wb := exceltable.NewWorkbook()
defer wb.Close()
err = overviewWriter.WriteSheet(ctx, wb, overview, "Overview")
err = writer.WriteSheet(ctx, wb, invoices, "Invoices")
_, err = wb.WriteTo(w) // w.Header().Set("Content-Type", exceltable.ContentType)
```

More in [How to write Excel workbooks](docs/exceltable/howto-write-workbooks.md).

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
// <table class='table table-striped'>
//   <caption>People</caption>
//   <tr><th>Full Name</th><th>Age</th><th>City</th></tr>
//   <tr><td>Alice Smith</td><td>30</td><td>New York</td></tr>
//   ...
// </table>
```

The default templates emit the rows directly inside `<table>`, without `<thead>`
or `<tbody>`. Pass your own templates to `WithTemplate` for a different
structure; a nil argument keeps the current template for that position.

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
    nil,  // No naming: columns match field names, the col tag is not read
    nil,  // No custom scanner
    nil,  // No custom parser
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
    Title() string         // Table name/title
    ColumnNames() []string // Column names
    NumColumns() int       // Number of columns
    NumRows() int          // Number of data rows
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
view := retable.NewStringsView("People", data, "Name", "Age")

// From structs
people := []Person{{"Alice", 30}, {"Bob", 25}}
view, err := retable.DefaultStructRowsViewer().NewView("People", people)
```

### View Wrappers (Decorators)

Transform Views without copying data:

```go
// Filter rows and columns
filtered := retable.FilteredView{
    Source:        view,
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
    func(row, col int) reflect.Value {
        price := view.Cell(row, 1).(float64)
        qty := view.Cell(row, 2).(int)
        return reflect.ValueOf(price * float64(qty))
    },
)

// Concatenate views horizontally (like SQL JOIN)
joined := retable.ExtraColsView{view1, view2, view3}

// Concatenate views vertically (like SQL UNION)
combined := retable.ExtraRowsView{viewA, viewB, viewC}
```

### Cell Formatters

Customize how values are formatted:

```go
// Type-based formatter
formatter := retable.NewReflectTypeCellFormatter().
    WithKindFormatter(reflect.Float64,
        retable.PrintfCellFormatter("%.2f")).
    WithTypeFormatter(reflect.TypeOf(time.Time{}),
        retable.LayoutFormatter("2006-01-02"))

// Use in CSV writer
writer := csvtable.NewWriter[[]Product]().
    WithTypeFormatters(formatter)

// Column-specific formatter. Writers are immutable builders,
// so keep the returned writer. Column formatters are applied to
// cell values only, never to the column titles of the header row.
writer = writer.WithColumnFormatter(2, // Column index
    retable.PrintfCellFormatter("$%.2f"))
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

view, err := naming.NewView("Products", products)
// Columns: ["Product Code", "Product Name", "Unit Price"]
```

Reading matches view column titles to struct fields exactly by default. Column
titles typed by people come with stray spaces and any case, so set `MatchTitle`
to `retable.TrimEqualFold` to compare them trimmed and case-insensitively.
`ColumnStructFieldValue`, `ViewToStructSlice` (including its `requiredCols`) and
the `csvtable` `Read*ToStructSlice` functions all use it:

```go
naming := &retable.StructFieldNaming{
    Tag:        "col",
    Ignore:     "-",
    Untagged:   func(string) string { return "" }, // only tagged fields are columns
    MatchTitle: retable.TrimEqualFold,
}

// Rejects two fields whose titles match each other, like "Name" and "name"
err := naming.CheckUniqueColumns(Account{}) // errors.Is(err, retable.ErrDuplicateColumnTitle)

// The first row containing every column title, in any order, with extra cells allowed
titleRow, columns, err := naming.FindTitleRow(rows, Account{})
view := &retable.FilteredView{
    Source:        retable.NewStringsView("", rows[titleRow+1:], rows[titleRow]...),
    ColumnMapping: columns, // columns[i] is the cell of naming.Columns(Account{})[i]
}
accounts, err := retable.ViewToStructSlice[Account](view, naming, nil, nil, nil, nil)
```

`FindTitleRow` returns `ErrNoColumns`, `ErrDuplicateColumnTitle` or
`ErrTitleRowNotFound`. Unlike `csvtable.DetectTableBounds`, which takes the row
matching the most titles among the rows of the majority column count, it requires
every title and ignores the column count.

More in [How to import tables typed by people](docs/retable/howto-import-tables-typed-by-people.md),
with recipes for CSV exports and Excel sheets, and in the
[StructFieldNaming reference](docs/retable/reference-struct-field-naming.md).

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
    nil, // parser
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

#### A number that does not fit is an error

Go's conversion rules truncate and wrap without complaining, so `int64(300)`
would become `int8(44)` and `float64(1234.56)` would become `int(1234)`: the cell
says one number and the struct field holds another, with nothing to notice it by.
`SmartAssign` reports both instead of converting them.

```go
var count int8
err := retable.SmartAssign(
    reflect.ValueOf(&count).Elem(), reflect.ValueOf(int64(300)), nil, nil, nil)
// 300 overflows int8: value out of range
// errors.Is(err, strconv.ErrRange)

var cents int
err = retable.SmartAssign(
    reflect.ValueOf(&cents).Elem(), reflect.ValueOf(1234.56), nil, nil, nil)
// 1234.56 is not a whole number and cannot be assigned to int: invalid syntax
// errors.Is(err, strconv.ErrSyntax)
```

The same rule covers a negative number assigned to an unsigned type, and a float
too large for its destination, which would otherwise become an infinity. Only the
conversions that land on the nearest representable number rather than a wrapped
one are still allowed: an integer into a float, and a float too small for its
destination, which becomes `0`.

This is a behavior change. Assignments that used to succeed with a silently
altered number now return an error, which is the point — the string side of
`SmartAssign` already rejected every one of them, so the two routes into the same
struct field now agree.

### Parsers and Scanners

`SmartAssign`, `ViewToStructSlice` and the `csvtable` read functions all take a
`Scanner` and a `Parser`. Both are optional: `nil` for either uses the defaults.

A `Parser` converts a string into one of Go's basic types and classifies a string
that means "no value". A `Scanner` sees the destination type and decides what the
string means for it, calling the `Parser` for the conversions. Pass a `Parser` to
change which formats are accepted; pass a `Scanner` to handle a type the package
does not know.

#### Configuring the parser

`StringParser` is the default implementation, shared as `DefaultParser`. Build
your own with `NewStringParser` and change what it accepts:

```go
type Booking struct {
    Date   time.Time
    Amount float64
    Count  int
}

parser := retable.NewStringParser()
parser.NilStrings = []string{"", "N/A", "-"}
parser.TimeFormats = append([]string{"02.01.2006"}, parser.TimeFormats...)

rows, err := retable.ViewToStructSlice[Booking](view, nil, nil, parser, nil, nil)
```

Leaving a field nil uses that field's defaults, so a parser unmarshalled from a
config file that only sets `TimeFormats` still parses numbers and booleans. Set a
field to an empty non-nil slice to really accept nothing. `DefaultParser` is
package state, so pass your own parser rather than modifying it from a library or
from concurrent code.

`ParseFloat` reads the number formats other locales write, not only Go's float
literal syntax:

```go
retable.DefaultParser.ParseFloat("1,234.56")  // 1234.56
retable.DefaultParser.ParseFloat("1.234,56")  // 1234.56
retable.DefaultParser.ParseFloat("1 234")     // 1234
retable.DefaultParser.ParseFloat("1.234,56-") // -1234.56, trailing minus
```

Groups before the decimal separator have to be 3 digits long, so a wrongly
grouped `12.34,56` is rejected rather than read as an arbitrary number. A lone
`,` or `.` stays the decimal separator, so `1,234` is 1.234 — nothing in the
string can resolve that ambiguity.

Set `StdlibFloatsOnly` for a source that writes Go float literals, where a
decimal comma means the cell is corrupt rather than German. Otherwise the fallback
guesses a value for that cell instead of failing the row, and the guess can be
off by a factor of a thousand: `1,234` reads as 1.234 where 1234 was meant.

```go
parser := &retable.StringParser{StdlibFloatsOnly: true}
parser.ParseFloat("3.14")  // 3.14
parser.ParseFloat("3,14")  // error
parser.ParseFloat(" 3.14") // error, strconv does not trim
parser.ParseFloat("NaN")   // NaN, strconv parses more than Go literals
```

It is the standard library's parsing, not a validating one, and it removes only
the second reading: a `1.234` that meant 1234 still reads as 1.234, because
`strconv` accepts it and the fallback never runs.

#### Empty cells

`Parser.IsNil` reports whether a string means "no value" — by default `""`,
`nil`, `<nil>`, `null`, `NULL`, `None`, `N/A`, `n/a` and `NA`: the absent value
of Go, SQL, JSON, Python and a spreadsheet. `SmartAssign` assigns the zero value for such
a string, so an empty CSV cell reads as `0` instead of failing the whole file.
The cost is that the parsed data can no longer tell an empty cell from a cell
containing `0`.

Pass the `StrictNilStrings` scanner to make that an error for the types that
cannot represent an absent value — the numeric types, `bool`, `time.Time` and
`time.Duration`:

```go
_, err := retable.ViewToStructSlice[Booking](view, nil, retable.StrictNilStrings, nil, nil, nil)
// cannot assign "" to int, use a pointer type for an optional column
```

The fix it asks for is to declare the optional column as a pointer. Pointer
destinations are left alone and still get `nil`, so the absence stays visible in
the parsed data and the type states which columns are optional.

#### Combining scanners

`MultiScanner` calls each `Scanner` in order until one handles the destination
type, so `StrictNilStrings` composes with a scanner of your own:

```go
scanner := retable.MultiScanner(retable.StrictNilStrings, myScanner)
rows, err := retable.ViewToStructSlice[Booking](view, nil, scanner, nil, nil, nil)
```

A `Scanner` returns `errors.ErrUnsupported` for a type it does not handle, which
lets the chain continue to the next one and finally to `SmartAssign`'s own
conversions. Any other error stops the chain.

### SQL Integration

Query in-memory Views using SQL:

```go
import "github.com/domonda/go-retable/sqltable"

// Create virtual database
view, err := retable.DefaultStructRowsViewer().NewView("users", users)
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

// A nil config uses NewDefaultFormatDetectionConfig()
data, format, err := csvtable.ParseDetectFormat(csvBytes, nil)
// Detects: UTF-8/UTF-16LE/ISO 8859-1/Windows 1252/Macintosh, plus
//          UTF-16BE/UTF-32LE/UTF-32BE from a byte order mark
// Detects: , or ; or \t or | separators
// Detects: \n or \r\n or \n\r line endings

// Or restrict which encodings are tried and how they are validated
config := &csvtable.FormatDetectionConfig{
    // Encoding names are written with a space, not with a hyphen
    Encodings:     []string{"UTF-8", "Windows 1252"},
    EncodingTests: []string{"äöü", "€"},
}
data, format, err = csvtable.ParseDetectFormat(csvBytes, config)
```

A CSV file may declare its separator in a `sep=;` header line, which is honored
during detection. A quote or a control character other than tab is never accepted
as a separator, so the returned `Format` always passes `Format.Validate()` and can
be reused for `ParseWithFormat` and `NewWriter`.

The [csvtable reference](docs/csvtable/reference.md#formatdetectionconfig) lists
how the encoding, line ending and separator are scored, and
[How csvtable reads malformed CSV](docs/csvtable/explanation-malformed-csv.md)
explains why, and which malformed files still parse.

## Subpackages

### csvtable

CSV reading and writing built for the files real systems export, not only the ones
that follow RFC 4180. Bank and payment exports arrive with the wrong encoding, a
separator you have to guess, quotes escaped two different ways, and a title line
sitting above the actual table. `csvtable` is written to get usable rows out of
those files instead of failing on them.

- Detects encoding, separator and line endings
- Parses RFC 4180 and the doubled-quote conventions real exporters emit
- Joins quoted fields that were split by a separator, a newline, or both
- Finds the actual table inside header and trailer lines
- Writes CSV that reads back, both here and in `encoding/csv`, and that Excel
  opens as UTF-8 with `WithUTF8BOM(true)`

```go
bookings, format, err := csvtable.ReadBytesDetectFormatToStructSlice[Booking](
    statement,
    nil, // default format detection
    &retable.StructFieldNaming{Tag: "csv"},
    nil, nil, nil, nil, // scanner, parser, formatter, validate
    "Datum", "Text", "Betrag", // required columns find the table between header and trailer lines
)
```

Documentation:
- [How to read and write CSV files](docs/csvtable/howto-read-and-write-csv.md):
  importing into structs, encodings, stored formats, cleanup, CSV for Excel,
  other encodings, HTTP downloads, troubleshooting
- [csvtable reference](docs/csvtable/reference.md): every function and writer
  option, the detection rules, table bounds, errors and panics
- [How csvtable reads malformed CSV](docs/csvtable/explanation-malformed-csv.md):
  quote parity, separator scoring, encoding detection, known limits, trade-offs
- API on [pkg.go.dev](https://pkg.go.dev/github.com/domonda/go-retable/csvtable)

### exceltable

Excel `.xlsx` reading and writing using excelize:
- Read XLSX files from filesystem or `io.Reader`, all sheets or the first,
  with raw or formatted cell values and empty rows and columns removed
- `Writer[T]` writes typed cells, so numbers, dates, durations and booleans
  stay sortable and summable in Excel; formatters by column, type, interface
  and kind return an `exceltable.Cell`
- A `Workbook` takes several sheets of different row types, and a sheet that
  fails midway, with an error or a panic, makes the workbook refuse to write
  a truncated file

Documentation:
- [How to write Excel workbooks](docs/exceltable/howto-write-workbooks.md):
  several sheets, HTTP downloads, money and date columns, enums, troubleshooting
- [Writer reference](docs/exceltable/reference-writer.md): every method and
  default, the conversion of each Go type, sheet name rules, errors
- [Why exceltable writes typed cells](docs/exceltable/explanation-typed-cells.md):
  date serials, time zones, precision, the unusable workbook, trade-offs
- API on [pkg.go.dev](https://pkg.go.dev/github.com/domonda/go-retable/exceltable)

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
retable.PrintlnTable("Title", data)

// Get struct field types including embedded fields
fields := retable.StructFieldTypes(reflect.TypeOf(MyStruct{}))

// Convert PascalCase to spaced names
title := retable.SpacePascalCase("UserID")  // "User ID"

// Calculate column widths for alignment, -1 for all columns
widths := retable.StringColumnWidths([][]string{...}, -1)
```

## Examples

### Example 1: CSV to Excel Conversion

```go
// Read CSV
csvBytes, err := os.ReadFile("input.csv")
check(err)

csvData, _, err := csvtable.ParseDetectFormat(csvBytes, nil)
check(err)

csvView := retable.NewStringsView("Data", csvData)

// Convert to structs for processing
type Record struct {
    ID   int
    Name string
    Date time.Time
}

records, err := retable.ViewToStructSlice[Record](csvView, nil, nil, nil, nil, nil)
check(err)

// Process data...
for i := range records {
    records[i].Name = strings.ToUpper(records[i].Name)
}

// Write to Excel
xlsxFile, err := os.Create("output.xlsx")
check(err)
defer xlsxFile.Close()
err = exceltable.NewWriter[[]Record]().
    WithHeaderRow(true).
    Write(context.Background(), xlsxFile, records, "Data")
check(err)
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
    nil, // parser
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
    func(row, col int) reflect.Value {
        cost := joined.Cell(row, 2).(float64)
        price := joined.Cell(row, 3).(float64)
        margin := ((price - cost) / price) * 100
        return reflect.ValueOf(margin)
    },
)

// Format as HTML report
writer := htmltable.NewWriter[retable.View]().
    WithHeaderRow(true).
    WithTableClass("report-table").
    WithColumnFormatter(4, retable.PrintfCellFormatter("%.1f%%"))

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
