# exceltable Writer reference

`exceltable.Writer[T]` writes tables as sheets of an `.xlsx` workbook with typed
cells: numbers are Excel numbers, dates are Excel dates, booleans are Excel
booleans. A `Workbook` collects several sheets, each with its own row type.
Use it whenever an export has to be sorted, filtered or summed in Excel, which
a CSV or a sheet of text cells cannot offer.

This page lists the complete writer API, its defaults and the exact conversion
of every Go value. For tasks, see
[How to write Excel workbooks](howto-write-workbooks.md); for the reasons
behind the rules, see [Why exceltable writes typed cells](explanation-typed-cells.md).
The generated API docs are on
[pkg.go.dev](https://pkg.go.dev/github.com/domonda/go-retable/exceltable).

```sh
go get github.com/domonda/go-retable@v1.0.0-beta.2 github.com/domonda/go-retable/exceltable@v1.0.0-beta.2
```

`exceltable` v1.0.0-beta.2 uses root API added in v1.0.0-beta.2
(`retable.ReflectTypeDispatcher`), so upgrade both modules together.

## API

### Writer[T]

`Writer[T]` is an immutable builder like `csvtable.Writer` and
`htmltable.Writer`: every `With*` method returns a modified copy and leaves the
receiver unchanged, so a configured writer can be stored in a package variable
and shared. A `Writer` is safe for concurrent use.

#### The type parameter T

`T` is the type of the table you pass to `Write` and `WriteSheet`, the whole
table, not one row: `Writer[[]Invoice]`, not `Writer[Invoice]`.

| `T` | Works with the default viewer | Columns |
| --- | --- | --- |
| `[]Invoice` | yes | the exported fields of `Invoice`, titles from the `col` tag |
| `[]*Invoice` | yes | same; a nil element is a row of null cells |
| `[3]Invoice`, `*[]Invoice` | yes | same as the slice |
| `[][]string` | yes | the first row holds the column names |
| `any` | yes, for every table the viewer accepts, decided per call | those of the table passed |
| `Invoice` | compiles, `Write` fails: `table must be slice or array kind but is Invoice` | |
| `[]any` | compiles, `Write` fails: `row type must be a struct but is interface {}` | |
| `retable.View` | compiles, `Write` fails; use `WriteView`, or `WithTableViewer` with a viewer for views | |
| anything else | only with `WithTableViewer` and a viewer that accepts it | whatever the viewer returns |

`T` is used for exactly one thing: the parameter type of `Write` and
`WriteSheet`. The writer hands the table to a `retable.Viewer`, whose
`NewView(title string, table any)` turns it into a `retable.View`; every cell
is then read from that view. So `T` gives a compile-time check of the argument
(a `Writer[[]Invoice]` refuses a `[]Booking`), but `T` is not constrained, and
whether the default viewer can handle it is checked at runtime.

`WriteView` and `WriteViewSheet` take a `retable.View` and ignore `T`.

#### Typed T or any

`NewWriter[any]()` is a writer for every table type: `Write` and `WriteSheet`
accept any value, and the default viewer picks the columns from the dynamic
type of each table, so one writer can add a `[]Invoice` sheet, a `[]Booking`
sheet and a `[][]string` sheet to the same workbook. A table the viewer can't
handle, such as `42`, is an error from that call, at runtime instead of at
compile time; the workbook stays usable.

| Use a typed writer, `NewWriter[[]Invoice]()`, when | Use `NewWriter[any]()` when |
| --- | --- |
| the writer is for one row type, the common case | the writer is used only with `WriteView` and `WriteViewSheet` |
| it has column formatters, whose indices describe the columns of that row type | one function writes tables whose type it does not know, for example a list of sheets passed in as `any` |
| you want the compiler to reject a table of another type | the configuration is the same for all tables: type, interface and kind formatters, time and duration formats, nil value, header row |

Avoid `WithColumnFormatter` on a `Writer[any]` that writes tables of
different types: column 3 is a different field in each, and the formatter runs
on all of them. Type formatters are the per-value alternative that works across
tables; a formatter registered for an amount type formats that type in every
sheet.

| Function / method | Effect |
| --- | --- |
| `NewWriter[T any]() *Writer[T]` | Writer with the [defaults](#defaults). |
| `(w) Write(ctx, dest io.Writer, table T, sheetName string) error` | Writes `table` as the only sheet of a new workbook to `dest`. |
| `(w) WriteView(ctx, dest io.Writer, view retable.View) error` | Writes `view` as the only sheet of a new workbook to `dest`, named after `view.Title()`. |
| `(w) WriteSheet(ctx, wb *Workbook, table T, sheetName string) error` | Adds `table` as a sheet named `sheetName` to `wb`. |
| `(w) WriteViewSheet(ctx, wb *Workbook, view retable.View) error` | Adds `view` as a sheet to `wb`, named after `view.Title()`. |
| `(w) WithHeaderRow(headerRow bool)` | Writes the column names as a bold first row. |
| `(w) WithHeaderFont(family string, size float64)` | Font family and size in points of the header row, always bold. `""` or `0` keeps the workbook default. |
| `(w) WithTableViewer(viewer retable.Viewer)` | Uses `viewer` to turn a `T` into a view instead of `retable.SelectViewer`. |
| `(w) WithColumnFormatter(columnIndex int, f CellFormatter)` | Formatter for the zero-based column, tried before all others. `nil` removes it. |
| `(w) WithTypeFormatter(typ reflect.Type, f CellFormatter)` | Formatter for cells of the exact type `typ` and for non-nil pointers to it. `nil` removes it. |
| `(w) WithInterfaceTypeFormatter(typ reflect.Type, f CellFormatter)` | Formatter for cells whose type implements the interface `typ`. Interfaces are tried in registration order; registering one again replaces its formatter and keeps its position. `nil` removes it. Panics if `typ` is nil or not an interface type. |
| `(w) WithKindFormatter(kind reflect.Kind, f CellFormatter)` | Formatter for cells of the `reflect.Kind`, tried after types and interfaces. `nil` removes it. |
| `(w) WithTimeFormat(format string)` | Excel number format code for `time.Time` cells and for `DateTimeCell`s without their own format. |
| `(w) WithDurationFormat(format string)` | Excel number format code for `time.Duration` cells. |
| `(w) WithNilValue(nilValue string)` | Text written for null-like cells. `""` writes an empty cell. |

`Write` and `WriteView` write nothing to `dest` when a sheet fails: the
workbook is built completely before its bytes are written.

A nil `CellFormatterFunc` counts as `nil` too: passing one removes the
registration instead of storing a formatter that would panic when called.
csvtable and htmltable treat a nil formatter and a nil `…Func` the same way.

### Defaults

| Setting | Default |
| --- | --- |
| Header row | off |
| Header font | workbook default (Calibri 11), bold |
| Table viewer | `retable.SelectViewer`: `retable.StringsViewer` for `[][]string`, `retable.DefaultStructFieldNaming` for everything else |
| Time format | `dd.mm.yyyy hh:mm:ss` |
| Duration format | `[h]:mm:ss` (30 hours shows as `30:00:00`) |
| Nil value | `""`, an empty cell |
| Formatters | none |

With the default viewer, header titles of a struct slice come from the `col`
struct tag; an untagged field gets its name split at case changes
(`CostCenterNumber` becomes `Cost Center Number`), and `col:"-"` skips the
field. For `[][]string` the first row holds the column names; with
`WithHeaderRow(true)` it is written as the header row.

### Workbook

| Function / method | Effect |
| --- | --- |
| `NewWorkbook() *Workbook` | An empty workbook. |
| `(wb) WriteTo(dest io.Writer) (int64, error)` | Writes the workbook as `.xlsx` to `dest`. Can be called more than once, with sheets added in between. |
| `(wb) UniqueSheetName(name string) string` | The sanitized `name`, or, if a sheet of that name exists, `name` followed by ` (2)`, ` (3)` and so on, see [Sheet names](#sheet-names). Pass it to `WriteSheet` or as view title to `WriteViewSheet`. |
| `(wb) Close() error` | Releases the workbook, including the temporary files that large sheets are streamed to. Afterwards `WriteTo` and adding sheets return an error. |

Sheets appear in the order they were added. A `Workbook` is not safe for
concurrent use.

`WriteTo` returns an error when the workbook has no sheets, when a sheet
failed while it was written, or after `Close`, see [Errors](#errors).

### Cell

`Cell` is a typed cell value: empty, text, a number with an optional number
format, a boolean, or a date/time with a number format. A `CellFormatter`
returns one. The zero `Cell` is an empty cell. Cells are comparable with `==`,
so a formatter test can compare its result with the expected `Cell`.

| Constructor | Cell |
| --- | --- |
| `StringCell(s string) Cell` | Text. Never becomes a formula, even when `s` starts with `=`. Text longer than Excel's limit of 32767 UTF-16 code units is cut to that length when written. |
| `NumberCell(n float64, format string) Cell` | Number with the Excel number format code `format`, for example `#,##0.00` or `#,##0.00 [$EUR];-#,##0.00 [$EUR]`. `""` is Excel's General format. Stored rounded to 15 significant digits, the precision of an Excel number. NaN, +Inf and -Inf are written as the text `NaN`, `+Inf`, `-Inf`. |
| `BoolCell(b bool) Cell` | Excel boolean (`TRUE`/`FALSE`). |
| `DateTimeCell(t time.Time, format string) Cell` | Excel date serial of the wall clock time of `t` in its own location, displayed with `format`. `""` uses the writer's time format. A zero `t` is an empty cell. A `t` before year 1900 or after year 9999 is text in the form `2006-01-02T15:04:05.999999999`. |

Format codes are written exactly as given. They are never mapped onto one of
Excel's built-in formats, because Excel displays those according to the
reader's locale.

### CellFormatter

```go
type CellFormatter interface {
    FormatCell(ctx context.Context, view retable.View, row, col int) (Cell, error)
}

type CellFormatterFunc func(ctx context.Context, view retable.View, row, col int) (Cell, error)

func StringCellFormatter(f retable.CellFormatter) CellFormatter
```

`CellFormatter` is the typed counterpart of `retable.CellFormatter`, which
returns a string for csvtable and htmltable. `row` is the zero-based data row
of `view`, not counting the header row.

A formatter has three possible results:

| Return | Effect |
| --- | --- |
| a `Cell`, `nil` | The cell is written. |
| any `Cell`, `errors.ErrUnsupported` | The writer tries the next formatter, then the built-in conversion. |
| any `Cell`, any other error | The sheet is aborted and the workbook is unusable, see [Errors](#errors). |

A formatter registered for a type `T` also handles cells of type `*T` that are
not nil. It receives a `retable.DerefView`, so `view.Cell(row, col)` returns the
`T`, not the pointer. The same holds for a struct field of interface type, such
as `any`: the formatter of the dynamic type gets a view whose `Cell` returns
the dynamic value.

`StringCellFormatter` adapts a csvtable or htmltable formatter: its string
result becomes a `StringCell`, its `raw` flag is ignored, and its errors,
including `errors.ErrUnsupported`, pass through. A nil `f`, also a nil func,
returns nil.

### Constants and errors

| Name | Value |
| --- | --- |
| `ContentType` | `"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"`, the MIME type of `.xlsx`. |
| `ErrDuplicateSheetName` | Returned by `WriteSheet` and `WriteViewSheet` when the sanitized sheet name equals an existing one, ignoring case. |

## Conversion of cell values

Each cell goes through these steps in order. The first step that handles the
cell decides what is written.

| # | Step | Applies to | Written as |
| - | --- | --- | --- |
| 1 | Column formatter | the column of `WithColumnFormatter` | the formatter's `Cell` |
| 2 | Type, interface and kind formatters | the cell's type: exact type, then interfaces in registration order, then kind; then the same for the pointed-to type of a non-nil pointer; then for the dynamic type of a non-nil interface value | the formatter's `Cell` |
| 3 | `time.Time` | also `*time.Time` and interfaces holding one | date serial with the time format; zero time: empty cell; before 1900 or after 9999: text `2006-01-02T15:04:05.999999999` |
| 4 | `time.Duration` | also through pointers and interfaces | number of days (`d / 24h`) with the duration format; negative: text `-H:MM:SS[.fraction]` |
| 5 | Null-like | `retable.IsNullLike`: nil pointers, interfaces, slices, maps, channels and funcs, an interface holding a typed nil pointer, `struct{}{}`, values with `IsNull() bool` or `IsZero() bool` returning true (pointer receivers included), `driver.Valuer` returning nil, and pointers to any of these | the nil value text, an empty cell by default |
| 6 | `bool` kind | | Excel boolean |
| 6 | `string` kind | named string types too, even with a `String` method | text |
| 6 | Integer kinds | `int*`, `uint*`, `uintptr`, named types too | number, stored exactly |
| 6 | Float kinds | `float32`, `float64`, named types too | number with General format, rounded to 15 significant digits; a `float32` as its shortest decimal, so `float32(19.99)` is `19.99`, not `19.989999771118164`; NaN and ±Inf as text |
| 7 | `fmt.Stringer` | the value, then its address (a struct field whose `String` has a pointer receiver), then the dereferenced value | text |
| 8 | `[]byte` | | text |
| 9 | Everything else | | text of `fmt.Sprint` |

Notes on the table:

- Steps 3 to 9 look through pointers and interfaces, so a `*int` is a number
  and a `*time.Time` is a date.
- A zero `time.Time` is an empty cell even with `WithNilValue("NULL")`. A nil
  `*time.Time` is null and gets the nil value.
- A type with a numeric kind and a `String` method, such as `time.Month` or an
  amount type, is a number (step 6 comes before step 7). csvtable and htmltable
  write the `String` result, so `time.Month(3)` is `March` there and `3` here.
  Register a type or kind formatter to write the text instead.
- A number type with an `IsZero` method is null-like when it is zero (step 5
  comes before step 6), so its zero gets the nil value, not `0`. Register a
  type formatter to write `0`.
- Integers are stored exactly in the file. Excel and LibreOffice hold every
  number as an IEEE 754 double, so integers beyond 2^53 lose precision when the
  file is opened.

### Date serials

A `time.Time` becomes a serial number in Excel's 1900 date system, the number
of days since day zero including the time as a fraction:

| Time | Serial |
| --- | --- |
| `1900-01-01 00:00` | `1` |
| `1900-02-28 12:00` | `59.5` |
| `1900-03-01 00:00` | `61` (Excel counts a non-existent 1900-02-29 as 60) |
| `2024-02-29 00:00 UTC` | `45351` |
| `2026-07-01 12:00 Europe/Vienna` | `46204.5` (the wall clock, not UTC) |

The serial represents the wall clock time of the value in its own location,
because Excel dates have no time zone.

A duration is a fraction of a day: 30 hours is `1.25`, 90 minutes is `0.0625`.

Like every float cell, serials and durations are stored rounded to 15
significant digits, the precision of an Excel number. For current dates that
is about 9 µs (a five-digit day number leaves ten decimal places), and one
hour is stored as `0.0416666666666667`, a full hour for Excel's `HOUR`
function, not 0:59:59. A stored date serial never moves into the next day:
neither float64 arithmetic nor the rounding carries a time just before
midnight past it. Excel still rounds the *displayed* time
to the precision of the format, so `23:59:59.999` shows as the next day in a
format without a fraction of a second.

## Sheet names

`WriteSheet` and `Write` take a name; `WriteView` and `WriteViewSheet` use the
title of the view. Every name is sanitized for Excel before the sheet is added:

1. Surrounding whitespace is trimmed.
2. An empty name becomes `UNNAMED`.
3. `\ / ? * [ ] :` are replaced with `_`.
4. An apostrophe at the start or end is replaced with `_`. Apostrophes inside
   the name stay.
5. A name longer than 31 UTF-16 code units is cut to whole characters within
   30 units, followed by `…`. Most characters count as one unit, an emoji as two.

| Input | Sheet name |
| --- | --- |
| `"  Overview \t"` | `Overview` |
| `""` | `UNNAMED` |
| `"KST: Vertrieb"` | `KST_ Vertrieb` |
| `"a[b]"` | `a_b_` |
| `"'Q1'"` | `_Q1_` |
| `"O'Brien"` | `O'Brien` |
| 32 × `a` | 30 × `a` followed by `…` |

The sanitized name is then compared with the existing sheet names, ignoring
case like Excel does. A match returns `ErrDuplicateSheetName` and leaves the
workbook unchanged. Names that differ before sanitizing can collide after it:
`a/b` and `a?b` both become `a_b`, and two names that differ only after their
30th character get the same truncated name.

For names from data, like cost center numbers, `Workbook.UniqueSheetName(name)`
returns a name that does not collide, compared the same way. Without a
collision it is the sanitized name. With one it appends ` (2)`, ` (3)` and so on
to the trimmed name, `UNNAMED (2)` for an empty one, and cuts the name before the
number to whole characters so that the number always fits into the 31 units and
is never replaced by `…`. A sheet written with the returned name gets exactly
that name.

The returned name is not reserved. It is unique among the sheets the workbook
has at the time of the call, so write each sheet before asking for the next
name. Asking twice for `4711` without writing a sheet in between returns the
same name twice, and the second `WriteSheet` returns `ErrDuplicateSheetName`.

| Existing sheets | Input | `UniqueSheetName` |
| --- | --- | --- |
| none | `" a/b "` | `a_b` |
| `4711` | `"4711"` | `4711 (2)` |
| `Overview` | `"OVERVIEW"` | `OVERVIEW (2)` |
| `X`, `X (2)` | `"X"` | `X (3)` |
| `UNNAMED` | `""` | `UNNAMED (2)` |
| one written as 40 × `a` | 40 × `a` | 27 × `a` followed by ` (2)` |

## Errors

| Situation | Error | Workbook afterwards |
| --- | --- | --- |
| The viewer cannot create a view of `table`, for example a struct instead of a slice | the viewer's error | unchanged, usable |
| `ctx` is done before the sheet is added | `ctx.Err()` | unchanged, usable |
| Duplicate sheet name | wraps `ErrDuplicateSheetName`: `duplicate sheet name: "Overview" (existing sheet "OVERVIEW")` | unchanged, usable |
| More than 1,048,576 rows (header included) or 16,384 columns | `exceltable: sheet "Invoices" has 1048577 rows, more than Excel's 1048576` | unchanged, usable |
| A formatter error, a cancelled `ctx` or an excelize error while rows are written | `sheet "Invoices": row 1, column "Total": <cause>` | unusable |
| A panic before the sheet is added, for example of the viewer or of `view.Title()` | `exceltable: panic: <value>` followed by the stack trace | unchanged, usable |
| A panic after the sheet is added, for example of a formatter | `sheet "Invoices": exceltable: panic: <value>` followed by the stack trace | unusable |
| Any write to an unusable workbook, including `WriteTo` | `exceltable: workbook unusable after a failed sheet: sheet "Invoices": ...` | unusable |
| `WriteTo` without sheets | `exceltable: workbook has no sheets` | unchanged, usable |
| Any use after `Close` | `exceltable: workbook is closed` | closed |

Every error wraps its cause, so `errors.Is(err, context.Canceled)` and
`errors.Is(err, yourFormatterErr)` work, also on the errors of an unusable
workbook. A panic value that is an error is wrapped too. The stack trace of a
panic is part of the error message, and an unusable workbook returns it again
from every later call, so log these errors instead of showing them to a user.
A workbook becomes unusable because the failed sheet holds only part
of its rows and cannot be removed again; refusing every further write makes it
impossible to deliver that truncated file. See
[Why exceltable writes typed cells](explanation-typed-cells.md#a-failed-sheet-makes-the-workbook-unusable).

## Reading the file back

`exceltable.Read` and `exceltable.ReadFirstSheet` return every cell as a
string. A sheet written with a header row reads back into the same structs with
`retable.ViewToStructSlice` and `&retable.DefaultStructFieldNaming`, with these
limits:

- Times lose their time zone; they read back as UTC wall clock times.
- Times and durations lose any fraction of a second that their format does not
  show. The default `[h]:mm:ss` rounds 1.5s to 2s; `[h]:mm:ss.000` keeps
  milliseconds.
- Negative durations and times before 1900 are text that
  `retable.StringParser` parses back.
- Without a header row, the reader drops a column whose cells are all empty and
  the following columns shift.

## Examples

A struct slice with a header row, a date-only time format and a money column:

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

err := writer.Write(ctx, file, invoices, "Invoices")
```

A `time.Month` column written as month names instead of numbers, reusing a
string formatter from the root package:

```go
writer := exceltable.NewWriter[[]Row]().
    WithTypeFormatter(
        reflect.TypeFor[time.Month](),
        exceltable.StringCellFormatter(retable.SprintCellFormatter(false)),
    )
```

A view that is not a struct slice, for example a query result:

```go
view := retable.NewStringsView("Query Result", rows, "Name", "Amount")
err := exceltable.NewWriter[any]().WithHeaderRow(true).WriteView(ctx, file, view)
```

## Related

- [How to write Excel workbooks](howto-write-workbooks.md): several sheets,
  HTTP downloads, money and date types, custom formatters
- [Why exceltable writes typed cells](explanation-typed-cells.md): date
  serials, the conversion order, the unusable workbook
- [README: Working with Excel Files](../../README.md#working-with-excel-files)
