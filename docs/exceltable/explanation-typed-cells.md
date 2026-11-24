# Why exceltable writes typed cells

csvtable and htmltable turn every cell into a string, because CSV and HTML
only have strings. An `.xlsx` sheet has more: numbers, booleans and dates that
Excel can sort, filter, sum and chart. `exceltable.Writer` keeps that
information. This page explains the problems it solves, how it decides what
each Go value becomes, and what that costs.

For the API, see the [reference](reference-writer.md); for recipes, see
[How to write Excel workbooks](howto-write-workbooks.md).

## The problem

A sheet of text cells looks right and behaves wrong:

- `1234.5` stored as text is left-aligned, `SUM` skips it, and sorting puts
  `100` before `20`.
- A date stored as the text `05.11.2026` sorts by day of the month, can't be
  filtered by month, and reads as May 11 to someone used to US dates.
- A duration of `30h0m0s` can't be added to another one.

The user then has to run "Text to Columns" or retype formulas before the export
is useful. Typed cells avoid that work.

Typed cells bring their own failure modes, which the rest of this page is
about. Excel has no time zones and no negative times, holds 15 significant
digits, starts its calendar in 1900, and displays built-in formats according
to the reader's locale. A writer has to decide what to do in each of those
cases.

## A typed result for formatters

csvtable and htmltable formatters implement `retable.CellFormatter` and return
a string. exceltable formatters implement `exceltable.CellFormatter` and return
an `exceltable.Cell`, which is one of: empty, text, number with format code,
boolean, or date/time with format code.

```
retable.CellFormatter     (ctx, view, row, col) -> (string, raw bool, error)
exceltable.CellFormatter  (ctx, view, row, col) -> (Cell, error)
```

Both are registered the same way, by column, type, interface and kind, and both
decline a cell with `errors.ErrUnsupported`. The type, interface and kind lookup
is one implementation, `retable.ReflectTypeDispatcher`, used by the string
writers and by exceltable. A type implementing two registered interfaces is
therefore handled by the same formatter in all three writers: the interface
registered first.

`StringCellFormatter` turns an existing string formatter into a typed one that
writes text, so formatters written for CSV and HTML can be reused.

## Why Writer has a type parameter

`exceltable.Writer[T]` is generic over the table type, like
`csvtable.Writer[T]` and `htmltable.Writer[T]`. All three writers share the
same builder methods and differ only in the cell result, so code that writes
the same table as CSV, HTML and Excel reads the same way three times.

What `T` buys:

- **A checked argument.** `Write(ctx, dest, table T, sheetName)` accepts only
  the declared table type. A `Writer[[]Invoice]` stored in a package variable
  cannot be handed a `[]Booking` by mistake.
- **A writer per row type.** A column formatter is registered by column index,
  and index 3 is `Total` only for `Invoice`. Binding the writer to `[]Invoice`
  keeps that configuration with the type whose columns it describes. A workbook
  with an overview and detail sheets uses one writer per row type for this
  reason.

What `T` does not do: the writer never looks at `T`. It passes the table to a
`retable.Viewer`, whose `NewView(title string, table any)` is untyped, and
reads every cell from the resulting `retable.View`. `T` is declared as `any`,
so `NewWriter[Invoice]()` and `NewWriter[[]any]()` compile and fail on the
first `Write`, and `WriteView` ignores `T` completely. The type safety covers
the argument, not whether the table can be written.

Because the viewer decides at runtime, `Writer[any]` is a valid writer, not a
workaround: it opts out of the argument check and writes whatever table each
call passes. That suits code that is itself untyped, such as a function taking
a list of sheets as `any`, or a writer used only with views. It gives up the
per-type binding of column formatters, so configuration that should hold
across tables goes into type formatters instead. The
[reference](reference-writer.md#typed-t-or-any) has the comparison.

```
Write(ctx, dest, table T, name)
        |
        v
retable.Viewer.NewView(name, table any)   <- T ends here
        |
        v
retable.View  ->  Cell(row, col) for every cell  ->  exceltable.Cell
```

This is a known open question for all three writers, tracked in `TODOS.md`
under "Decide the Viewer model: generic or untyped": either drop `T` from the
writers and keep the untyped viewer, or make the viewer generic so `T` is
checked end to end. Until it is decided, exceltable follows the shape of the
other two writers, so that a decision applies to all three at once.

## The order of conversion

Each cell is converted by the first step that handles it:

```
cell(row, col)
  |- column formatter
  |- formatters for: exact type -> interfaces (registration order) -> kind
  |    then the pointed-to type of a non-nil pointer
  |    then the dynamic type of a non-nil interface value
  |- time.Time      -> date serial, or text outside 1900..9999
  |- time.Duration  -> fraction of a day, or text if negative
  |- null-like      -> nil value (empty cell by default)
  |- bool, string, integer, float kinds -> boolean, text, number
  |- fmt.Stringer   -> text
  |- []byte         -> text
  '- fmt.Sprint     -> text
```

Two choices in this order are worth explaining.

**Time and duration come before the null check.** A zero `time.Time` is
null-like, but it is also a time. It is written as an empty cell, not as the
nil value, so `WithNilValue("NULL")` does not put `NULL` into every unset
date. The null check is `retable.IsNullLike`, the same one csvtable and
htmltable use: besides nil pointers and `IsNull()`, it counts any value whose
`IsZero()` returns true. A number type with an `IsZero` method, like an
amount in cents, therefore writes its zero as the nil value, not as `0`;
register a type formatter for it to write `0`.

**Number kinds come before `fmt.Stringer`.** An amount type like
`type Amount float64` with a `String` method that formats `1,234.50` should
still be a number Excel can sum. The cost: an enum like `time.Month` is
written as `3` here and as `March` by csvtable and htmltable, which check
`fmt.Stringer` first because for them every result is text anyway. For
exceltable, the number is the more useful default, and a type formatter writes
the text where needed. The opposite default would turn every amount type with
a `String` method into text without any warning.

## Dates

Excel stores a date as a serial number: days since a day zero, with the time
of day as the fraction. exceltable uses the 1900 date system, the default of
Excel on Windows and macOS.

```
1900-01-01 00:00   ->      1
1900-02-28 12:00   ->     59.5
(1900-02-29)       ->     60     a day that never existed, see below
1900-03-01 00:00   ->     61
2026-07-01 12:00   ->  46204.5
```

**The 1900 leap year.** Excel counts a 29 February 1900, which never existed,
because Lotus 1-2-3 did, and Excel kept the error for compatibility. From
1900-03-01 on, day zero is effectively 1899-12-30; before it, day zero is
1899-12-31. exceltable follows Excel on both sides of the gap, so 1900-01-01
is serial 1 like in Excel. Simple arithmetic that uses 1899-12-30 for every
date writes the first two months of 1900 one day late.

**No time zones.** An Excel date is a wall clock reading. exceltable writes the
wall clock time of each `time.Time` in its own location: 12:00 in Vienna is
written as 12:00, not as the 10:00 or 11:00 it is in UTC. Converting to UTC
would show a time nobody in the source system ever saw. Converting to one
fixed zone would need a setting, and the value already says which zone it
belongs to. The cost is that the zone is lost: reading the file back gives
12:00 UTC. Convert times with `t.In(location)` before writing when the sheet
must show one zone.

**Out of range.** Excel dates end with the year 9999 and cannot be negative.
A time outside 1900 to 9999 is written as text in the form
`2006-01-02T15:04:05.999999999`, which `retable.StringParser` parses back into
the same time. The alternative, a number Excel displays as `#####`, would be
unreadable and would not round trip.

## Durations

A duration is a fraction of a day: 30 hours is 1.25. With the format
`[h]:mm:ss`, the brackets make Excel show total hours, `30:00:00`, instead of
the hour within the day, `06:00:00`. Such cells can be summed into a total
duration.

Excel shows `#####` for negative time values. A negative duration is therefore
written as text in the clock form `-1:30:00`, and `StringParser.ParseDuration`
accepts that form, so it reads back.

The format also sets the precision of the round trip: `[h]:mm:ss` shows whole
seconds, so 1.5s reads back as 2s. `[h]:mm:ss.000` keeps milliseconds.

## Numbers and their precision

Excel holds every number as an IEEE 754 double and works with 15 significant
digits. exceltable rounds every float cell to 15 significant digits before
writing it. Without the rounding, the library that writes the file would cut
off the digits beyond 15, and values would be stored slightly too low: one
hour, 1/24 of a day, would become `0.0416666666666666`, and Excel's `HOUR`
function would read 0:59:59 as 0. Rounded, it is `0.0416666666666667`, a
full hour.

For a date serial of a current date, 15 digits leave ten decimal places of a
day, about 9 µs. The rounding never moves a date into the next day: when it
would carry a time just before midnight past it, the serial keeps its value
within the day.

A `float32` is written as its shortest decimal, so `float32(19.99)` is stored
as `19.99`, not as the `19.989999771118164` that converting it to a float64
gives.

Integers are written exactly, but Excel rounds anything beyond 2^53 when it
opens the file. IDs and account numbers that long belong in text cells.

## Format codes are stored as given

Excel has built-in number formats, referenced by number. Excel displays some
of them, dates in particular, according to the reader's locale, so the same
file shows `11/16/2026` to one reader and `16.11.2026` to another. exceltable
stores every format code as a custom code, exactly as you pass it, and never
maps it onto a built-in format. `dd.mm.yyyy` shows `16.11.2026` for every
reader. The thousands and decimal separators in a code like `#,##0.00` still
follow the reader's locale; that is how Excel defines the characters `,` and
`.` in format codes.

## Sheet names

Excel limits sheet names to 31 characters, counted in UTF-16 code units,
forbids `\ / ? * [ ] :`, and forbids an apostrophe at either end. exceltable
sanitizes every name instead of failing on it, because names usually come from
data, like a cost center or customer name, and an export should not fail on a
colon in a customer name.

Two names that sanitize to the same name, ignoring case like Excel does, are an
error, `ErrDuplicateSheetName`. Silently appending the rows of the second sheet
to the first, or renaming it to `Overview (2)`, would produce a file whose
structure differs from what the code asked for. The caller knows which suffix
makes sense, so the writer leaves the choice to it. A caller that wants the
numbered names asks for them explicitly with `Workbook.UniqueSheetName`.

## A failed sheet makes the workbook unusable

Rows are streamed into a sheet one after another and are not kept in memory
as Go values; large sheets go to temporary files. This keeps memory flat for
large exports, but a streamed sheet cannot be rolled back. When a formatter
fails or the context is cancelled at row 5000 of 10000, the sheet exists with
5000 rows.

A file with half a sheet looks complete. Nobody counts the rows of an export
before forwarding it to the tax advisor. So after a failure, the `Workbook`
remembers the error and returns it from every later `WriteSheet`,
`WriteViewSheet` and `WriteTo`. The error says which sheet, row and column
failed and wraps the cause, so `errors.Is(err, context.Canceled)` still works.

A panic, like a formatter's failed type assertion or reflection over a nil
value, stops the sheet midway just like an error. The writer recovers it,
returns it as a `retable.PanicError` that keeps the stack trace, and fails the workbook the same
way. Sheets are written in HTTP handlers and job workers, where an unrecovered
panic takes down more than the one export.

Errors and panics before a sheet is added do not have this effect: a duplicate
name, a table the viewer can't handle, a table with more rows or columns than
an Excel sheet holds, or a context that was already done leave the workbook
unchanged and usable. The size is checked before any cell is converted.

`Writer.Write` and `Writer.WriteView` build the whole workbook before writing
its bytes to `dest`, so they write nothing on error. That is what makes it safe
to send an HTTP error response after a failed export.

## The round trip

go-retable treats round trips as a design rule (decided 2026-09-06 for all
writers): a table read into structs, written, and read again gives equal
structs. For exceltable, a sheet written with a header row and read back with
`exceltable.Read` and `retable.ViewToStructSlice` gives the same structs, with
three limits that come from Excel itself:

- times lose their zone,
- times and durations lose the fraction of a second their format hides,
- without a header row, the reader drops all-empty columns.

Each text fallback in this page exists for the round trip: out-of-range
times, negative durations and NaN are written as text that the parser reads
back, rather than as numbers that Excel can't display or that mean something
else.

## Trade-offs

| Choice | Gained | Given up |
| --- | --- | --- |
| Typed cells | sorting, filtering, sums in Excel | the output differs from csvtable/htmltable for `fmt.Stringer` number types |
| Wall clock time per value | the time the source system showed | the time zone; readers get UTC |
| Text for out-of-range values | readable cells that round trip | the cell is text, not a date or number, in Excel |
| Custom format codes only | the same display for every reader | Excel's locale-aware built-in date formats |
| Streaming rows | flat memory for large sheets | a failed sheet cannot be undone; the workbook must be rebuilt |
| Error on duplicate sheet names | no silently merged or renamed sheets | the caller picks the unique name |

## Related

- [Writer reference](reference-writer.md): conversion table, sheet name rules,
  error messages
- [How to write Excel workbooks](howto-write-workbooks.md)
