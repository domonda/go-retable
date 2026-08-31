# go-retable

[![Go Reference](https://pkg.go.dev/badge/github.com/domonda/go-retable.svg)](https://pkg.go.dev/github.com/domonda/go-retable)
[![Go Report Card](https://goreportcard.com/badge/github.com/domonda/go-retable)](https://goreportcard.com/report/github.com/domonda/go-retable)

A powerful Go library for working with tabular data using reflection. `go-retable` provides a unified interface for reading, transforming, and writing tables from various sources and formats including CSV, Excel, HTML, and SQL.

## Features

- **Unified Table Interface**: Work with all tabular data through a single `View` interface
- **Multiple Format Support**: Read/write CSV, Excel (XLSX), HTML tables, and SQL result sets
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
header and trailer lines around the table, see [csvtable](#csvtable).

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
        sheet.NumCols())
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
    nil,  // Use default field naming
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
    ColNames() []string    // Column names
    NumCols() int          // Number of columns
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

See [csvtable](#csvtable) for how the separator and line endings are scored, how
header and trailer lines are skipped, and which malformed files still parse.

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
- Writes CSV that reads back, both here and in `encoding/csv`

Full API on [pkg.go.dev](https://pkg.go.dev/github.com/domonda/go-retable/csvtable).

#### Import a CSV file into structs

This is the shortest path from bytes to typed data. The input below is a bank
statement with a title line, a period line, blank lines and a trailer, exported
with `;` separators and Windows line endings — none of which you have to tell the
parser.

```go
package main

import (
    "fmt"

    "github.com/domonda/go-retable"
    "github.com/domonda/go-retable/csvtable"
)

type Booking struct {
    Datum  string `csv:"Datum"`
    Text   string `csv:"Text"`
    Betrag string `csv:"Betrag"`
}

func main() {
    statement := []byte("Kontoauszug Nr. 4\r\n" +
        "Zeitraum;01.01.2025;31.01.2025\r\n" +
        "\r\n" +
        "Datum;Text;Betrag\r\n" +
        "01.01.2025;Miete;-500,00\r\n" +
        "02.01.2025;\"Gehalt, Januar\";2000,00\r\n" +
        "\r\n" +
        "Erstellt am 03.01.2025\r\n")

    rows, format, err := csvtable.ParseDetectFormat(statement, nil)
    if err != nil {
        panic(err)
    }
    fmt.Printf("%s, separator %q\n", format.Encoding, format.Separator)
    // UTF-8, separator ";"

    bookings, err := csvtable.ReadStringsToStructSlice[Booking](
        rows,
        &retable.StructFieldNaming{Tag: "csv"},
        nil, nil, nil, nil, // scanner, parser, formatter, validate
        "Datum", "Text", "Betrag", // required columns
    )
    if err != nil {
        panic(err)
    }
    for _, b := range bookings {
        fmt.Printf("%+v\n", b)
    }
    // {Datum:01.01.2025 Text:Miete Betrag:-500,00}
    // {Datum:02.01.2025 Text:Gehalt, Januar Betrag:2000,00}
}
```

Three things happened without configuration: the `;` separator was picked over the
comma inside `"Gehalt, Januar"`, the title and trailer lines were dropped, and
`Datum;Text;Betrag` was found as the column titles rather than `Kontoauszug Nr. 4`.
The required column names are what makes the last one work — see
[Finding the table](#finding-the-table-inside-header-and-trailer-lines).

To read straight from a file or from bytes, skip the two-step and use
`ReadFileDetectFormatToStructSlice` or `ReadBytesDetectFormatToStructSlice`.
Both take a `*FormatDetectionConfig` after the file or the bytes, nil for the
defaults, then the same naming, scanner, parser, formatter, validate and
required-column arguments, and also return the detected `*Format`. The
`WithFormat` variants take an explicit `*Format` instead of detecting one.

#### Parsing

```go
// Detect encoding, separator and line endings. A nil config means
// NewDefaultFormatDetectionConfig().
rows, format, err := csvtable.ParseDetectFormat(csvBytes, nil)

// Or parse with a format you already know.
rows, err := csvtable.ParseWithFormat(csvBytes, csvtable.NewFormat(","))
```

Both return `rows [][]string`. **A row can be `nil`**, for a blank line in the file
and for a line that was absorbed into a field containing a newline. The `nil` rows
keep row indices aligned with line numbers in the source file, which is what you
want when reporting an error back to whoever produced the file. Call
`RemoveEmptyRows` when you just want the data.

Parsing does not fail on a malformed field. A field that cannot be interpreted is
returned as its literal text rather than aborting the file — see
[How malformed CSV is handled](#how-malformed-csv-is-handled).

#### Format and detection

```go
type Format struct {
    Encoding  string `json:"encoding"`  // "UTF-8", "Windows 1252", ...
    Separator string `json:"separator"` // exactly one byte
    Newline   string `json:"newline"`   // "\n", "\r\n" or "\n\r"
}
```

| Function | Purpose |
|---|---|
| `NewFormat(separator string) *Format` | UTF-8 with `\r\n` line endings and the given separator |
| `(*Format).Validate() error` | Nil-safe. Rejects an empty encoding, a separator that is not exactly one byte, a separator that is a quote or a control character other than tab, and any newline other than `\n`, `\r\n`, `\n\r` |
| `NewDefaultFormatDetectionConfig() *FormatDetectionConfig` | The defaults used when `ParseDetectFormat` gets a nil config |
| `EscapeQuotes(val string) string` | Doubles every `"` in a value, per RFC 4180 |

What detection covers:

- **Encoding** — a byte order mark decides on its own, and also reports the
  UTF-16BE, UTF-32LE and UTF-32BE that are not in the candidate list. The marks
  are matched longest first, so `FF FE 00 00` is UTF-32LE rather than UTF-16LE
  followed by a NUL. Otherwise UTF-8, UTF-16LE, ISO 8859-1, Windows 1252 and
  Macintosh are tried in order, and each candidate is validated by decoding a set
  of test characters (umlauts, `§`, `€`, Cyrillic). Restrict either list to
  narrow the guess:

  ```go
  config := &csvtable.FormatDetectionConfig{
      Encodings:     []string{"UTF-8", "Windows 1252"},
      EncodingTests: []string{"ä", "ö", "ü", "€"},
  }
  rows, format, err := csvtable.ParseDetectFormat(csvBytes, config)
  ```

- **Separator** — `,` `;` tab and `|` are candidates. A `sep=;` header line, which
  Excel writes, is honored and removed from the output.
- **Line endings** — `\r\n`, `\n` and `\n\r`.

The returned `Format` always passes `Validate()`, including for empty input, so you
can reuse it for `ParseWithFormat` or hand its separator to `NewWriter`.

#### Finding the table inside header and trailer lines

Exports put a title, a reporting period or a total around the table. Taking the
first row of the file as the column titles gets all of those wrong.

```go
bounds := csvtable.DetectTableBounds(rows, "Datum", "Betrag")
// TableBounds{TitleRow: 3, EndRow: 6, NumColumns: 3}

if bounds.Valid() {
    table := bounds.Rows(rows) // column titles row + data rows
    view := retable.NewStringsView("Bookings", table)
    _ = view
}
```

| Member | Meaning |
|---|---|
| `TableBounds.TitleRow` | Index of the column titles row, `-1` when no table was found |
| `TableBounds.EndRow` | Index after the last data row |
| `TableBounds.NumColumns` | Column count of the table |
| `TableBounds.Valid() bool` | Whether a table was found |
| `TableBounds.Rows(rows) [][]string` | The titles row followed by the data rows, ready for `retable.NewStringsView` |

How it decides:

1. The table width is the column count the majority of rows have. Single column
   rows only get a vote when no row has more than one column, so a stack of title
   lines cannot outvote three data rows.
2. The column titles are the row of that width matching the most `expectedTitles`,
   compared trimmed and without case. A row matching none is never used.
3. The data is the rows of that width after the titles, up to the first row of a
   different width. Empty rows in between are skipped rather than ending the table,
   because joining a field that contains a newline leaves empty rows behind.

`ReadStringsToStructSlice` calls this for you when you pass `requiredCols` — those
are exactly the titles to find the table by. Without them it falls back to the first
non-empty row, so existing callers that pass a plain table are unaffected.

Two limits worth knowing:

- Without `expectedTitles`, only the column count and non-empty fields are
  available, so a header line as wide as the table is taken as the column titles.
- A trailer row as wide as the table, such as `Summe;;1500,00`, cannot be told
  apart from a data row and is included. Validate the scanned values to reject it;
  the blank line before it is not usable as a signal, because an empty row is also
  what a joined multi-line field leaves behind.

#### Cleaning up parsed rows

All of these operate on `[][]string` straight out of the parser.

| Function | Effect |
|---|---|
| `RemoveEmptyRows(rows) [][]string` | Drops rows with no columns and rows whose columns are all empty. Returns the input unchanged if there is nothing to drop |
| `SetRowsWithNonUniformColumnsNil(rows) [][]string` | Sets every row that does not have the majority column count to `nil`, keeping indices aligned. Single column rows only vote when no row is wider |
| `SetEmptyRowsNil(rows) [][]string` | Sets rows whose columns are all empty to `nil` |
| `TrimSpace(rows)` | In place. Trims leading and trailing space from every field |
| `ReplaceNewlineWithSpace(rows)` | In place. Replaces `\r\n`, `\n` and `\r` in every field with one space |
| `CompactSpacedStrings(rows) int` | In place. Rewrites `"S h i n e r g y"` as `"Shinergy"` when every second rune is a space, and returns how many fields changed. PDF-extracted CSV needs this |

`ReplaceNewlineWithSpacefunc` is a deprecated alias of `ReplaceNewlineWithSpace`,
kept because the misspelled name is part of the published API.

#### Writing CSV

```go
writer := csvtable.NewWriter[[]Product]().
    WithHeaderRow(true).
    WithDelimiter(',')

err := writer.Write(context.Background(), dest, products)
// SKU,Name,Notes
// A-1,Widget,"He said ""hi"""
// B-2,Gadget,"line1
// line2"
```

`Writer[T]` is an immutable builder: every `With…` method returns a new writer, so
keep the returned value. `Write` builds a view with `retable.SelectViewer`,
`WriteWithViewer` uses one you supply, and `WriteView` takes a `retable.View`
directly. `ViewStrings` returns the formatted cells as `[][]string` without writing
anything, which is useful for tests.

Defaults from `NewWriter`:

| Option | Default | Setter |
|---|---|---|
| Delimiter | `;` | `WithDelimiter(rune)` |
| Line ending | `\r\n` | `WithNewLine(string)` |
| Header row | off | `WithHeaderRow(bool)` |
| Quote escaping | `""` | `WithEscapeQuotes(string)` |
| Nil value | `""` | `WithNilValue(string)` |
| Padding | `NoPadding` | `WithPadding(NoPadding\|AlignLeft\|AlignRight\|AlignCenter)` |
| Quote all fields | off | `WithQuoteAllFields(bool)` |
| Quote empty fields | off | `WithQuoteEmptyFields(bool)` |
| Output encoder | none | `WithEncoder(Encoder)` |

A field is quoted when it contains the delimiter, a newline or a quote, so the
output reads back both here and in `encoding/csv`. Formatting is controlled with
the same `retable` formatters used elsewhere: `WithTypeFormatter`,
`WithKindFormatter`, `WithInterfaceTypeFormatter`, `WithColumnFormatter` and their
`…Func` variants. Column formatters apply to cell values only, never to the column
titles of the header row.

`WithPadding` aligns columns for human-readable output:

```go
csvtable.NewWriter[[]Product]().
    WithHeaderRow(true).
    WithDelimiter('|').
    WithPadding(csvtable.AlignLeft)
// SKU|Name  |Notes
// A-1|Widget|"He said ""hi"""
```

#### How malformed CSV is handled

**Quoting is decided by parity, not by shape.** A field opens a quoted field when
its leading run of quotes is odd, and that field is still open when its total quote
count is odd. This covers every combination of leading and trailing quotes, so no
quote pattern can abort a file. It also keeps working for the two escaping
conventions real exporters mix:

```go
`"a","{""k"":""v"",""k2"":""v2""}","b"`   // JSON in a quoted field
// ["a" `{"k":"v","k2":"v2"}` "b"]

`1997,""Ford"",E350,"Super, luxurious truck"`  // unquoted field, doubled quotes
// ["1997" `"Ford"` "E350" "Super, luxurious truck"]
```

The second shape is not RFC 4180 — `encoding/csv` rejects it with
`bare " in non-quoted-field` — but bank exports emit it, so it is supported.

**An unterminated quote costs one row, not the rest of the file.**

```go
"a;\"oops\nb;c\nd;e\n"
// [["a" `"oops`] ["b" "c"] ["d" "e"]]
```

The unclosed field keeps its literal text and the following rows are parsed
normally.

**The separator is scored by how uniform the column count is**, not by how often a
candidate occurs, because the right separator is the one that makes the data
rectangular. Counting alone picks the comma here; uniformity picks the semicolon:

```
Name;Beschreibung
Meier;Wien, Graz, Linz         3 semicolons vs 4 commas
Huber;Wels, Steyr, Melk

;  ->  2/2/2 columns, uniform      <- chosen
,  ->  1/3/3 columns, not uniform
```

Separators and newlines inside quoted fields are not counted at all, so a quoted
`\r\n` cannot switch an otherwise `\n` file to Windows line endings.

**Known limits.** These are deliberate, and documented in the code:

- A `\r` directly before the newline that splits the lines is lost, as in
  `A;"x\r\ny";B` in a `\n` file. There it cannot be told apart from the residue of a
  file with mixed line endings.
- CR-only (classic Mac) line endings are not supported; `\r` alone is not a valid
  `Format.Newline`.
- `sanitizeUTF8` replaces undecodable bytes with spaces instead of reporting them,
  so a failed encoding guess turns `Müller` into `M ller` rather than an error.
  Check the decoded data yourself if that distinction matters.

All three come from the same design: lines are split first and quoted fields are
joined back together afterwards, so the parser has no quote state while splitting.

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
