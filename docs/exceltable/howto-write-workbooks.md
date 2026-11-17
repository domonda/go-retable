# How to write Excel workbooks

Recipes for the tasks that come up when you export data as `.xlsx` with
`exceltable.Writer`: a file from a struct slice, several sheets in one
workbook, an HTTP download, money and date columns, and the settings that
change how cells look. Each recipe stands on its own.

## Prerequisites

- Go 1.26 or later.
- Both modules at the same version, because `exceltable` uses root API of the
  same release:

  ```sh
  go get github.com/domonda/go-retable@v1.0.0-beta.2 github.com/domonda/go-retable/exceltable@v1.0.0-beta.2
  ```

- The recipes import these two packages plus the standard library packages
  they name (`context`, `errors`, `reflect`, `time`, ...):

  ```go
  import (
      "github.com/domonda/go-retable"
      "github.com/domonda/go-retable/exceltable"
  )
  ```

  The money and date recipes also use `github.com/domonda/go-types/money` and
  `github.com/domonda/go-types/date`.

The complete API and the conversion of every Go type are in the
[reference](reference-writer.md).

## Write a struct slice to a file

1. Tag the struct fields with the column titles. An untagged field gets its
   name split at case changes (`DueDate` becomes `Due Date`); `col:"-"` skips a
   field.

   ```go
   type Invoice struct {
       Number   string    `col:"Invoice"`
       Customer string    `col:"Customer"`
       Date     time.Time `col:"Date"`
       Total    float64   `col:"Total"`
       Paid     bool      `col:"Paid"`
   }
   ```

2. Create the writer once, for example as a package variable. Writers are
   immutable and safe for concurrent use. The type parameter is the table
   type, the slice, not the row struct: `[]Invoice`, not `Invoice`. See
   [The type parameter T](reference-writer.md#the-type-parameter-t).

   ```go
   var invoiceWriter = exceltable.NewWriter[[]Invoice]().WithHeaderRow(true)
   ```

3. Write the sheet:

   ```go
   file, err := os.Create("invoices.xlsx")
   if err != nil {
       return err
   }
   defer file.Close()

   err = invoiceWriter.Write(ctx, file, invoices, "Invoices")
   if err != nil {
       return err
   }
   return file.Close()
   ```

The sheet has a bold header row, `Date` as an Excel date shown as
`dd.mm.yyyy hh:mm:ss`, `Total` as a number and `Paid` as `TRUE`/`FALSE`.

## Write several sheets into one workbook

Use one `Writer` per row type and one `Workbook` for the file. Sheets appear in
the order you add them.

```go
type CostCenter struct {
    Number string  `col:"Cost Center"`
    Total  float64 `col:"Total"`
}

type Booking struct {
    Date   time.Time `col:"Date"`
    Text   string    `col:"Text"`
    Amount float64   `col:"Amount"`
}

func writeCostCenters(ctx context.Context, dest io.Writer, centers []CostCenter, bookings map[string][]Booking) error {
    wb := exceltable.NewWorkbook()
    defer wb.Close()

    err := exceltable.NewWriter[[]CostCenter]().WithHeaderRow(true).
        WriteSheet(ctx, wb, centers, "Overview")
    if err != nil {
        return err
    }
    bookingWriter := exceltable.NewWriter[[]Booking]().WithHeaderRow(true)
    for _, center := range centers {
        err = bookingWriter.WriteSheet(ctx, wb, bookings[center.Number], center.Number)
        if err != nil {
            return err
        }
    }
    _, err = wb.WriteTo(dest)
    return err
}
```

Sheet names are sanitized for Excel: `KST: Vertrieb` becomes `KST_ Vertrieb`,
and names longer than 31 characters are cut to 30 plus `…`. See
[Sheet names](reference-writer.md#sheet-names) for all rules.

### With one writer for tables of any type

When the sheets arrive as values of different types that your code does not
know, use `NewWriter[any]()`. Each call writes the table it gets, with the
columns of that table's type. Put shared formatting into type formatters, not
column formatters, because a column index means a different field in every
table:

```go
type Sheet struct {
    Name  string
    Table any // a slice of structs or [][]string
}

func writeSheets(ctx context.Context, dest io.Writer, sheets []Sheet) error {
    writer := exceltable.NewWriter[any]().
        WithHeaderRow(true).
        WithTypeFormatter(reflect.TypeFor[money.Amount](), amountCell)

    wb := exceltable.NewWorkbook()
    defer wb.Close()
    for _, sheet := range sheets {
        err := writer.WriteSheet(ctx, wb, sheet.Table, sheet.Name)
        if err != nil {
            return err
        }
    }
    _, err := wb.WriteTo(dest)
    return err
}
```

A table the viewer can't handle, like a single struct instead of a slice,
returns an error at runtime instead of failing to compile. See
[Typed T or any](reference-writer.md#typed-t-or-any).

### When two sheet names collide

`WriteSheet` returns `exceltable.ErrDuplicateSheetName` when the sanitized
name equals an existing sheet name, ignoring case. The workbook is unchanged,
so you can retry with another name:

```go
name := center.Number
for i := 2; ; i++ {
    err = bookingWriter.WriteSheet(ctx, wb, bookings[center.Number], name)
    if !errors.Is(err, exceltable.ErrDuplicateSheetName) {
        break
    }
    name = fmt.Sprintf("%s (%d)", center.Number, i)
}
if err != nil {
    return err
}
```

Keep the suffix short when names can be long: a name cut at 30 characters
loses the suffix and collides again.

## Serve a workbook as an HTTP download

Build the workbook completely, then set the headers and write it. If a sheet
fails, nothing has been sent yet and you can still answer with an error status.

```go
func handleExport(w http.ResponseWriter, r *http.Request) {
    invoices, err := loadInvoices(r.Context())
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    wb := exceltable.NewWorkbook()
    defer wb.Close()
    err = invoiceWriter.WriteSheet(r.Context(), wb, invoices, "Invoices")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", exceltable.ContentType)
    w.Header().Set("Content-Disposition", `attachment; filename="invoices.xlsx"`)
    _, err = wb.WriteTo(w)
    if err != nil {
        log.Printf("writing invoices.xlsx: %v", err)
    }
}
```

The request context is checked before every row, so a client that disconnects
stops the export of a large table. Avoid `invoiceWriter.Write(ctx, w, ...)`
directly on the `http.ResponseWriter` when you set `Content-Disposition`
first: on an error, `http.Error` would send the error text with your
attachment header, and the browser would save it as `invoices.xlsx`.

## Format money amounts

A float field is already a number, shown in Excel's General format. Give the
column a number format to show thousands separators and two decimals.

For one column, use a column formatter. The column index is zero-based and
counts the columns of the view, so skipped fields do not count:

```go
writer := exceltable.NewWriter[[]Invoice]().
    WithHeaderRow(true).
    WithColumnFormatter(3, exceltable.CellFormatterFunc(
        func(ctx context.Context, view retable.View, row, col int) (exceltable.Cell, error) {
            return exceltable.NumberCell(view.Cell(row, col).(float64), "#,##0.00"), nil
        },
    ))
```

For every field of an amount type, register a type formatter. It also handles
non-nil pointers to the type, and it receives the dereferenced value, so the
type assertion works for `*money.Amount` too. This uses
`github.com/domonda/go-types/money`:

```go
var amountCell = exceltable.CellFormatterFunc(
    func(ctx context.Context, view retable.View, row, col int) (exceltable.Cell, error) {
        amount := view.Cell(row, col).(money.Amount)
        return exceltable.NumberCell(float64(amount), "#,##0.00"), nil
    },
)

var currencyAmountCell = exceltable.CellFormatterFunc(
    func(ctx context.Context, view retable.View, row, col int) (exceltable.Cell, error) {
        ca := view.Cell(row, col).(money.CurrencyAmount)
        if ca.Currency == "" {
            return exceltable.NumberCell(float64(ca.Amount), "#,##0.00"), nil
        }
        format := fmt.Sprintf("#,##0.00 [$%[1]s];-#,##0.00 [$%[1]s]", ca.Currency)
        return exceltable.NumberCell(float64(ca.Amount), format), nil
    },
)

writer := exceltable.NewWriter[[]Payment]().
    WithTypeFormatter(reflect.TypeFor[money.Amount](), amountCell).
    WithTypeFormatter(reflect.TypeFor[money.CurrencyAmount](), currencyAmountCell)
```

`#,##0.00 [$EUR];-#,##0.00 [$EUR]` shows `1,234.50 EUR`, with the separators of
the reader's locale. The format code is stored exactly as you pass it.

Without the `CurrencyAmount` formatter, the struct would be written as the text
of its `String` method, and Excel could not sum it.

## Write date-only values

A `time.Time` is written as a date and time. To show only the date for all
time columns, change the time format:

```go
writer := exceltable.NewWriter[[]Invoice]().WithTimeFormat("dd.mm.yyyy")
```

To show only the date in one column, return a `DateTimeCell` with its own
format from a column formatter:

```go
writer := exceltable.NewWriter[[]Invoice]().
    WithColumnFormatter(2, exceltable.CellFormatterFunc(
        func(ctx context.Context, view retable.View, row, col int) (exceltable.Cell, error) {
            return exceltable.DateTimeCell(view.Cell(row, col).(time.Time), "dd.mm.yyyy"), nil
        },
    ))
```

A date type based on `string`, such as `date.Date` from
`github.com/domonda/go-types/date`, is written as text unless you register a
formatter that turns it into a time:

```go
// dateCell writes a valid date as an Excel date and an invalid one as its text.
func dateCell(d date.Date) exceltable.Cell {
    t := d.MidnightUTC()
    if t.IsZero() && !d.IsZero() {
        return exceltable.StringCell(string(d))
    }
    return exceltable.DateTimeCell(t, "dd.mm.yyyy")
}

writer := exceltable.NewWriter[[]Invoice]().
    WithTypeFormatter(reflect.TypeFor[date.Date](), exceltable.CellFormatterFunc(
        func(ctx context.Context, view retable.View, row, col int) (exceltable.Cell, error) {
            return dateCell(view.Cell(row, col).(date.Date)), nil
        },
    )).
    WithTypeFormatter(reflect.TypeFor[date.NullableDate](), exceltable.CellFormatterFunc(
        func(ctx context.Context, view retable.View, row, col int) (exceltable.Cell, error) {
            nd := view.Cell(row, col).(date.NullableDate)
            if nd.IsNull() {
                // Let the writer's nil value apply
                return exceltable.Cell{}, errors.ErrUnsupported
            }
            return dateCell(nd.Date()), nil
        },
    ))
```

Returning `errors.ErrUnsupported` hands the cell to the next step, here the
null check, which writes the value of `WithNilValue`.

Times are written as the wall clock time in their own location, because
Excel dates have no time zone. To show all times in one zone, convert them
before writing, for example with a type formatter that calls
`t.In(location)`.

## Write enums as text

Number types with a `String` method, such as `time.Month` or an `int`-based
status enum, are written as numbers so Excel can calculate with them. To write
the text instead, register a formatter for the type. `StringCellFormatter`
turns a string formatter of the root package into a typed one:

```go
writer := exceltable.NewWriter[[]Row]().
    WithTypeFormatter(
        reflect.TypeFor[time.Month](),
        exceltable.StringCellFormatter(retable.SprintCellFormatter(false)),
    )
```

Register each enum type on its own. A formatter for the `fmt.Stringer`
interface would also catch `time.Time`, `time.Duration` and amount types,
which all have a `String` method, and turn dates and numbers into text.

## Change how durations are shown

A `time.Duration` is written as a fraction of a day with the format
`[h]:mm:ss`, so 30 hours shows as `30:00:00` and the cell can be summed. The
format rounds to whole seconds. To keep milliseconds:

```go
writer := exceltable.NewWriter[[]Row]().WithDurationFormat("[h]:mm:ss.000")
```

Excel cannot show a negative time, so a negative duration is written as the
text `-1:30:00`. `retable.StringParser.ParseDuration` reads that form back.

## Write null values as text

Nil pointers, nil interfaces and values whose `IsNull()` or `IsZero()` returns
true are written as empty cells. To mark them:

```go
writer := exceltable.NewWriter[[]Row]().WithNilValue("NULL")
```

A zero `time.Time` stays an empty cell. A nil `*time.Time` gets the nil value.
The zero of a number type with an `IsZero` method, such as an amount type,
is null too and gets the nil value instead of `0`; register a type formatter
for the type to write `0`.

## Change the header font

The header row is bold in the workbook's default font. To set the family and
size in points:

```go
writer := exceltable.NewWriter[[]Invoice]().
    WithHeaderRow(true).
    WithHeaderFont("Liberation Sans", 10)
```

## Write a view or a string table

Any `retable.View` can be written, for example a query result or a filtered
view. `WriteView` and `WriteViewSheet` name the sheet after the view's title:

```go
view := retable.NewStringsView("Query Result", rows, "Name", "Amount")
err := exceltable.NewWriter[any]().WithHeaderRow(true).WriteView(ctx, file, view)
```

A `[][]string` table uses its first row as column names. String cells stay
text: `"1.5"` is not converted to a number. To get numbers, parse the strings
in a column formatter, or write structs with typed fields instead.

## Verification

Read the file back with `exceltable` and compare it with what you wrote. With
a header row, the structs come back equal, apart from the limits listed in
[Reading the file back](reference-writer.md#reading-the-file-back):

```go
view, err := exceltable.ReadFirstSheet(bytes.NewReader(buf.Bytes()), false)
if err != nil {
    return err
}
got, err := retable.ViewToStructSlice[Invoice](view, &retable.DefaultStructFieldNaming, nil, nil, nil, nil)
```

Pass `&retable.DefaultStructFieldNaming`: with `nil`, columns are matched
against Go field names instead of the `col` titles in the header row.

To check cell types and number formats, open the file in Excel or LibreOffice:
a number cell is right-aligned and its format shows under Format Cells.

## Troubleshooting

**Every later write returns `exceltable: workbook unusable after a failed sheet`.**
A sheet failed after it was added, through a formatter error, a cancelled
context, an excelize error or a panic, which the error shows as
`exceltable: panic: <value>` followed by the stack trace. The sheet holds only part of its rows, so the
workbook refuses to write a truncated file. The error wraps the cause and names
the sheet, row and column. Fix the cause and start a new `Workbook`.

**`duplicate sheet name: "A_B" (existing sheet "a_b")` for names that looked different.**
Names are compared after sanitizing and ignoring case: `a/b` and `A?B`
become `a_b` and `A_B`, which Excel treats as the same name. Make the names unique before sanitizing, see
[When two sheet names collide](#when-two-sheet-names-collide).

**`table must be slice or array kind but is Invoice`.**
The writer was created for the row type instead of the table type. Use
`NewWriter[[]Invoice]()`, not `NewWriter[Invoice]()`. The same message for
another type, like `int`, comes from a `Writer[any]` that was handed a value
that is not a table.

**A type assertion panics in a formatter.**
A formatter registered for an interface or a kind sees values of many types.
Check the type and return `errors.ErrUnsupported` for the ones it does not
handle:

```go
amount, ok := view.Cell(row, col).(money.Amount)
if !ok {
    return exceltable.Cell{}, errors.ErrUnsupported
}
```

**`panic: ... not an interface type` from `WithInterfaceTypeFormatter`.**
Pass the interface type, such as `reflect.TypeFor[fmt.Stringer]()`, not the
type of a value that implements it. Use `WithTypeFormatter` for concrete types.

**An enum or amount shows as a number, not text.**
Number kinds come before `fmt.Stringer`. Register a type formatter, see
[Write enums as text](#write-enums-as-text).

**A date before 1900 shows as text.**
Excel dates start at 1900-01-01 and end in the year 9999. Times outside that
range are written as `2006-01-02T15:04:05` text that `retable.StringParser`
parses back.

**`exceltable: workbook has no sheets`.**
`WriteTo` was called before any `WriteSheet` succeeded. An Excel file needs at
least one sheet.

**`exceltable: workbook is closed`.**
`Close` was called before `WriteTo` or before another `WriteSheet`. `Close`
releases the sheet data, so call it last, for example with `defer wb.Close()`
right after `NewWorkbook`.

**`exceltable: sheet "Invoices" has 1048577 rows, more than Excel's 1048576`.**
An Excel sheet holds at most 1,048,576 rows, the header row included, and
16,384 columns. The size is checked before any cell is converted, so the
workbook stays usable and you can write the rows as several smaller sheets.

**Excel shows a large ID with the last digits changed.**
Excel holds numbers as doubles with 15 significant digits. Write IDs and
account numbers as text: use a `string` field, or a column formatter that
returns a `StringCell`.

## Related

- [Writer reference](reference-writer.md): every method, default and
  conversion rule
- [Why exceltable writes typed cells](explanation-typed-cells.md)
- [README: Working with Excel Files](../../README.md#working-with-excel-files)
