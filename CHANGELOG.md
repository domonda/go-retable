# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased] - 2026-08-31

### Fixed

- `SmartAssign` assigns the zero value for an empty source string instead of
  failing with `unsupported operation: assigning string "" to uint64`. An empty
  cell means "no value", and Excel and CSV files have empty cells for optional
  columns of every type, so reading one must not fail the whole file. The
  conversions for numbers, booleans and `time.Time` all parse the source string
  and fall through to the unsupported-operation error when the parse fails, so an
  empty cell was only survivable for string destinations. This mirrors the branch
  directly above it, which already assigns the zero value for a null source.
  String destinations are unaffected and still receive the empty string.

  This surfaced through `exceltable`: short rows used to yield `nil` cells that
  `ViewToStructSlice` skipped, and now correctly yield `""`, which reached
  `SmartAssign` and turned every optional numeric spreadsheet column into an error.

- `csvtable` no longer aborts a whole file with `can't handle CSV field` when a
  field's quoting is not one of a handful of hard-coded shapes. `readLines` split
  every line on the separator and then classified the fragments by their leading
  and trailing quote-run counts, and the combinations `(1,2)`, `(2,1)`, `(2,3)` and
  `(3,2)` were simply missing from that table. A JSON object inside a quoted field
  hits this: after splitting on the comma, `"{""orderTransactionId"":""019b...142""`
  ends with an escaped quote pair. Field handling is now decided by quote parity,
  an odd leading quote run opens a quoted field and an odd total quote count means
  it is not closed again within the field, which covers every combination.
- An unterminated quote in `csvtable` no longer silently swallows every row up to
  the next line whose first field ends with a quote. It reported no error and could
  lose an unbounded number of rows. The unclosed field now keeps its literal text
  and the following rows are parsed normally.
- The closing part of a quoted field split by both a separator and a newline is
  found again. The search stopped at the first field of a line, so a field split
  both ways was never rejoined.
- The outer quotes of a field are only removed when the field really ends with a
  quote. `"a"x` became `a"`, silently dropping the last character.
- A carriage return inside a quoted field survives line splitting. `splitLines`
  trimmed newline characters from both ends of every line, so `A;"first\n\rsecond";B`
  returned `first\nsecond`. `\n\r` line endings are now detected and split by, so
  only the end of a line has to be trimmed.
- `csvtable` separator detection no longer lets the content of a quoted field vote
  on the structure of the file, and scores candidates by how uniform the resulting
  column count is instead of by how often they occur. A semicolon-separated file
  whose text columns contain commas was detected as comma-separated, and every row
  came out garbage without an error. Line endings are picked by count for the same
  reason, so a single `\r\n` inside a quoted field no longer switches an otherwise
  `\n` file to Windows line endings.
- `csvtable.ParseDetectFormat` returns a `Format` that passes its own `Validate()`
  for empty input, instead of one with an empty separator that callers could not
  reuse. A quote or a control character other than tab is rejected as a separator,
  both in a `sep=` header line and in `Format.Validate`.
- A `sep=` header line no longer leaks a carriage return into the last field of
  every line, and `ParseWithFormat` and `ParseDetectFormat` agree on the same bytes.
- `csvtable.SetRowsWithNonUniformColumnsNil` no longer sets every row of a single
  column table to `nil`. Single column rows are excluded from the majority vote so
  that header and trailer lines cannot outvote the table rows, which left a table
  that only has single column rows with no majority at all. They now count when no
  row has more than one column.
- The `csvtable` writer quotes a field containing a quote instead of only doubling
  its quotes. The unquoted form was readable by `ParseWithFormat` but rejected by
  `encoding/csv` with `bare " in non-quoted-field`.
- Column formatters are no longer applied to the column titles of a `csvtable`
  header row, and a nil interface cell reports `errors.ErrUnsupported` instead of
  panicking on an invalid `reflect.Value`.
- `ViewWithTitle` no longer panics when a cell value is not a pointer or interface.
  Its `ReflectCell` called `reflect.Value.Elem()` on every cell, which panics with
  `reflect: call of reflect.Value.Elem on string Value` for the string cells of
  `StringsView` and every `exceltable` sheet. It now passes cell values through
  unchanged, consistent with its `Cell` method. Use `DerefView` to dereference
  pointer or interface cells.
- Reading a sheet whose rows have trailing empty cells no longer panics in
  formatters and no longer renders `<nil>`. `exceltable` returned `nil` from `Cell`
  and an invalid `reflect.Value` from `ReflectCell` for cells past the end of a
  short row, which made `ReflectTypeCellFormatter` panic on `reflect.Value.Type`
  and made `ViewToStructSlice` skip those cells without running its `validate`
  func. Excel and `excelize.File.GetRows` omit trailing empty cells, so short rows
  are the normal case, not an edge case.
- `exceltable.Read` no longer aborts the whole workbook when one sheet is empty.
  It now skips empty sheets, as its documentation and `ReadLocalFile` always
  promised. A single blank sheet, which is the Excel default, previously made
  `Read` return `ErrEmptySheet` and no views at all.
- `NewStringsView` no longer trims the passed `cols` slice or the header row in
  place. Trimming the caller's data made `StringsViewer` non-reusable, because
  `StringsViewer.NewView` passes its own `Cols` field. Column names are now
  trimmed into a newly allocated slice.
- Column names read from an Excel sheet are whitespace-trimmed, like in every
  other `View` constructor. A header cell containing `"  Name  "` broke lookup by
  column name, for example the required-column check of `ViewToStructSlice`.

### Added

- `csvtable.DetectTableBounds` and `csvtable.TableBounds` locate the actual table
  within rows that also contain header and trailer lines, which exports put around
  it as a title, a reporting period or a total. The table width is the column count
  of the majority of rows, the column titles are the row of that width matching the
  most expected titles, and the data is the rows of that width after it.
  `ReadStringsToStructSlice` uses it when `requiredCols` are passed, so a statement
  starting with `Kontoauszug Nr. 4` no longer takes that line as its column titles
  and reports every required column as missing. Without `requiredCols` the first
  non-empty row is still used, so existing callers are unaffected.
- `|` is a `csvtable` separator candidate, alongside `,`, `;` and tab.
- `csvtable.ReplaceNewlineWithSpace`, replacing all newline variants in one pass so
  a Windows newline does not become two spaces. The misspelled
  `ReplaceNewlineWithSpacefunc` remains as a deprecated alias, because it is part of
  the published API.
- A csvtable chapter in the README covering import into structs, parsing, format
  detection, table bounds, row cleanup, writing, and how malformed CSV is handled
  including the known limits.
- `StringsView` implements `ReflectCellView` natively. `AsReflectCellView` now
  returns the view itself instead of allocating a wrapper on every call, and
  `ReflectCell` mirrors `Cell` for sparse rows.
- Tests for the `exceltable` package, which previously had none: its only test was
  commented out and referenced a fixture file that did not exist. The new tests
  build xlsx workbooks in memory and cover sparse rows, header trimming, empty
  row and column removal, empty and invalid sheets, raw versus formatted cell
  strings, and all four public read functions.
- Tests for `NewStringsView` header widening, whitespace trimming, caller-data
  immutability, and for `ViewWithTitle` cell pass-through.

### Changed

- `NewStringsView` widens a header row that is shorter than the widest data row,
  so cells past the end of the header row are reachable through `Cell` and
  `ReflectCell`. Explicitly passed `cols` are never widened, because they state
  the columns of the view.
- `exceltable` builds a `retable.StringsView` instead of its own view type. This
  removes a duplicate implementation and is what fixes the sparse-cell and
  header-trimming behavior above.
- The four `exceltable` read functions share `readFirstSheet` and `readAllSheets`
  helpers, so opening, closing, and empty-sheet handling have one implementation
  each instead of four.
- Documentation of empty row and column removal now matches the behavior: every
  completely empty row and column is removed, including ones between non-empty
  ones, so the data is compacted and row indices do not correspond to the original
  Excel row numbers. The previous text claimed removal only from the edges of the
  data range.
- **Minimum Go version raised from 1.24.3 to 1.26** for both the root module and
  the `exceltable` submodule.
- Dependencies updated: `excelize` v2.10.0 to v2.11.0, `testify` v1.11.1 to
  v1.12.1, and the `golang.org/x` modules.
