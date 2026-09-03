# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## 2026-09-03

Released by merging to `main`. Not tagged: this repository carries no version
tags, so consumers resolve it as a pseudo-version of the `main` commit.

### Security

- `htmltable.JSONCellFormatter` HTML-escapes its JSON for every input type. The
  formatter returns raw HTML, so its output goes into the document verbatim, but
  only the `default` branch was escaped, and only as a side effect of
  `json.Marshal` escaping `&`, `<` and `>` by itself. `json.Compact` and
  `json.Indent` do not escape, so a cell arriving as `json.RawMessage`, `string`,
  `[]byte` or a `json.Marshaler` that does not escape passed straight through: a
  value containing `</pre><script>` closed the `<pre>` element early and the rest
  of it ran as markup. `json.RawMessage` is the type the package's own example
  formats, so the documented usage was the vulnerable one.

  The escaping now happens on the finished JSON, so it covers every input type
  instead of whichever one happened to route through `json.Marshal`.

- `htmltable.HTMLSpanClassCellFormatter` escapes its class. The class is
  interpolated into a quoted attribute and only the cell value was escaped, so a
  class containing a quote could end the attribute and add another one: a class
  of `x' onmouseover='alert(1)` produced a live event handler. The class comes
  from calling code rather than from the table data, so this was not reachable
  from cell values, but nothing pinned that.

### Changed

- Marshaling inside `htmltable.JSONCellFormatter` uses a `json.Encoder` with
  `SetEscapeHTML(false)`, because escaping twice would be wrong. For a value the
  formatter marshals itself, `&`, `<` and `>` are now readable: a browser renders
  `{"name":"Meier & Co"}` where it previously showed `{"name":"Meier \u0026 Co"}`.
  JSON that arrives already marshaled keeps whatever escapes it literally
  carries. A `MarshalJSON` built on `json.Marshal` escapes on its own, so that
  branch still renders `\u0026`; a `json.RawMessage`, `string` or `[]byte` holding
  hand written or database sourced JSON never went through such a marshaler and
  renders the `&` it actually contains.

  The escaping is scoped to the `<pre>` text context, where only `&`, `<` and `>`
  can end the text node, rather than using `template.HTMLEscapeString`, which
  also escapes the quotes that JSON puts around every key and every string value.
  **Output bytes are therefore unchanged for JSON containing none of `&`, `<` and
  `>`** and a fixture asserting `<pre>{"ok":true}</pre>` keeps passing. Where
  those three do appear the output changes, which is the point of the fix.

### Fixed

- `htmltable.Writer.WithTemplate` applies the templates it's given. It cloned the
  writer, assigned all three templates to the clone and then returned the
  receiver, so every custom template was silently discarded and the writer kept
  rendering the default table. It also broke the immutability contract the other
  `With` methods follow, because `w.WithTemplate(...) == w` was true. An audit of
  `htmltable/writer.go`, `csvtable/writer.go` and `structrowsviewer.go` found no
  other instance: every other `With` method that doesn't return a clone delegates
  to one that does.

  **A consumer already calling `WithTemplate` will see its HTML change**, because
  the templates it passed now take effect where before they were ignored.

  A nil argument leaves that template unchanged instead of being stored, which is
  new. Before this fix nil was harmless, since the clone holding it was thrown
  away; storing it would have turned `WithTemplate(nil, rowTmpl, nil)`, a
  reasonable way to replace one template, into a nil dereference inside
  `Execute` after the rows had already been written. Nil keeps rather than
  resets, because `HeaderTemplate`, `RowTemplate` and `FooterTemplate` are
  exported: resetting stays expressible by passing them, keeping would not be
  expressible at all.

  Custom templates are reachable for the first time, so it's worth being precise
  about what that does and doesn't change for escaping. Cells arrive as
  `template.HTML` values in `RawCells`, already escaped by the writer unless
  their formatter returned raw output, and `html/template` then applies its own
  contextual escaping on top. It re-escapes for attribute contexts and for
  executable script contexts, so a cell placed in `title='...'` has its quotes
  escaped and one placed in `<script>` is JS-encoded. It does not re-escape in
  ordinary element content, nor in `<template>`, `<noscript>` or
  `<script type="text/template">`, where content is emitted verbatim. That only
  matters for a formatter that returned raw output, which is the documented
  trusted-content escape hatch; a non-raw cell was already escaped by the writer
  and stays inert everywhere.

- The documentation for the `raw` result describes what it does. It said raw
  meant the string "can be used directly in the output format without escaping
  or sanitization", which reads as a claim about the string rather than as the
  instruction it is: raw switches the *writer's* escaping off, and the formatter
  inherits the job. That framing is what produced the `JSONCellFormatter` hole
  fixed above, where returning raw output and leaning on an incidental
  `json.Marshal` escape looked reasonable.

  Reworded at the definition and at every restatement of it, so the wrong model
  is not left canonical anywhere: the `CellFormatter` interface doc and its
  `FormatCell` method doc, `PrintfCellFormatter`, `SprintCellFormatter` and
  `CellFormatterFromFormatter` in `formatter.go`. The wording is now writer
  relative rather than HTML relative, because the interface serves `csvtable`
  too, where raw means skipping RFC 4180 quoting rather than HTML escaping.

- The `PrintfRawCellFormatter` documentation no longer claims its output
  "doesn't need sanitization". It interpolates the cell value into the format
  string without escaping and marks the result raw, so a value containing markup
  reaches the document as markup. It now carries the same "only use this for
  trusted content" warning as `Writer.WithRawColumn` and
  `WithTypeFormatterReflectRawFunc`. Behavior is unchanged: this is a deliberate
  escape hatch, and the documentation was the part that was wrong.

## 2026-09-02

Released by merging to `main`. Not tagged: this repository carries no version
tags, so consumers resolve it as a pseudo-version of the `main` commit.

### Fixed

- A numeric cell that does not fit its destination is reported instead of
  silently becoming a different number. `reflect.Value.Convert` applies Go's
  conversion rules, which truncate and wrap without complaining, so
  `int64(300)` into an `int8` was 44, `float64(1234.56)` into an `int` was 1234
  with the cents gone, `int64(-1)` into a `uint8` was 255 and a float too large
  for its destination became an infinity. The string side of `SmartAssign`
  already reported all of these, so which behavior a caller got depended only on
  whether the `View` held strings or numbers: the `csvtable` and `exceltable`
  readers produce string cells, while `sqltable.ScanRowsAsView`,
  `AnyValuesView` and `ReflectValuesView` produce the numbers themselves.

  **This is a breaking change** for a caller that relied on the narrowing. The
  errors wrap `strconv.ErrRange`, or `strconv.ErrSyntax` for a fraction assigned
  to an integer, so a caller can tell "did not fit" from any other assignment
  failure with `errors.Is`. The string side reports the same sentinels.
  Integer to float is deliberately still allowed: above 2^53 it loses precision
  but lands on the closest representable number rather than a wrapped one, and
  rejecting it would reject every ID stored in a `float64`. Float underflow is
  allowed for the same reason: `float32(1e-300)` is 0, which is the nearest
  representable value and not a wrapped one. An infinity the source already held
  is also still assigned, because that is what it says.

- A number written with a dot or comma thousands separator and an exponent
  parses correctly: `1.234.567e5` is 1.234567e11, not 1.234567e8. Only a *lone*
  dot or comma is a decimal separator; two or more are integer grouping. The
  exponent path applied the first half of that rule and not the second, so the
  silent factor of 1000 came back on the very path the previous fix below
  addressed. The rule now lives in one function that both paths call, because
  splitting it across two copies is what let them drift apart in the first
  place. `1.234.567` without an exponent was correct throughout, which is what
  made the gap invisible.

- A number written with a space or apostrophe thousands separator and an
  exponent parses correctly: `1 234e5` is 123400000, not 123400. The rule that
  only a dot or a comma can be the decimal separator was applied where a value
  ends but not where an exponent follows, so the same silent factor of 1000
  survived on that path.

- A nil `*StringParser` stored in a `Parser` interface no longer panics. It is
  not a nil interface, so `cmp.Or(parser, DefaultParser)` does not replace it
  and it reaches the accessors; they now treat a nil receiver like a parser with
  no field set.

- `StringParser.ParseFloat` reports both readings when a cell is rejected.
  Previously only the `strconv` error survived, so every rejected cell said
  `invalid syntax` and never said what the locale reading made of the
  separators. **The error text changed**: code matching on it should use
  `errors.Is` instead, against `strconv.ErrSyntax` for a malformed number and
  `strconv.ErrRange` for one that is out of range. Both still hold through the
  join; matching on either alone was never enough, and matching on the message
  no longer works.

- `SmartAssign` no longer crashes the process on a self referential pointer
  *source*. Bounding the destination walk left the source branch unbounded, and
  `src.Elem()` of a value that points at itself is the same value, so the
  recursion overflowed the stack. A stack overflow is fatal, so the deferred
  recover could not report it.

- A cell formatter built by `ReflectCellFormatterFunc` reports a cell whose type
  its function does not accept, instead of panicking. Guarding only the invalid
  cell was not enough: a formatter registered by kind or by interface receives
  defined types, and `reflect.Value.Call` panics on an argument that is not
  assignable. The panic escaped through `TryFormattersOrSprint` into the CSV and
  HTML writers.

- A code page encoding strips a leading UTF-8 byte order mark. Excel writes one
  in front of files it otherwise encodes in a code page, and a code page has no
  mark of its own, so the mark was decoded as text and put `ï»¿` in front of the
  first column title. That title then matched no struct field and the whole
  column stayed at its zero value for every row, with no error anywhere.

- `StrictNilStrings` no longer dereferences a nil `Parser`. A `Scanner` is
  documented as usable on its own, and only `SmartAssign` substitutes
  `DefaultParser`, so calling one directly panicked.

- `SingleCellView` reports the title it was passed. The title argument was never
  stored, so `Title()` returned the column name instead, which contradicted both
  the parameter documentation and the example on the function itself, where
  `SingleCellView("Count", "Total", 42)` is shown returning `"Count"`.
  `SingleColView`, which takes no title, still uses its column name for it.

- A cell formatter built by `ReflectCellFormatterFunc` reports a cell without a
  value as an unsupported operation instead of panicking. It passed the cell
  straight into `reflect.Value.Call`, which panics with `reflect: Call using
  zero Value argument` for a zero `reflect.Value`. A nil interface cell of an
  `AnyValuesView` produces exactly that, so the panic was reachable through a
  view of this package and escaped into the caller, because nothing on the
  formatter path recovers. `ReflectTypeCellFormatter.FormatCell` has had the
  same guard all along.

- `SmartAssign` no longer hangs or crashes on a self referential pointer type.
  `type SelfPtr *SelfPtr` is legal Go and its element type is itself, so the
  walk to the pointed-to type never ended: an empty cell spun forever and a
  non-empty one recursed until the stack overflowed, which is fatal and cannot
  be recovered. Such a destination is now reported as an unsupported operation.

- A lone space or apostrophe between digits is a thousands separator again, not
  a decimal separator, so `1 234` parses as 1234 instead of 1.234. The rule that
  a single separator is the decimal one resolves a real ambiguity for `.` and
  `,`, where `1,234` has two readings, but no locale writes a decimal fraction
  after a space or an apostrophe. The old reading was silent and never an error,
  and it hit exactly the amounts that carry no decimals, so a currency column
  mixed values a factor of 1000 apart while the amounts with decimals stayed
  correct. `csvtable` made this reachable for ordinary files because
  `sanitizeUTF8` rewrites the non-breaking space that Excel, SAP and French or
  Nordic exports use for grouping into a plain space before parsing. A space or
  apostrophe group that is not 3 digits long is now rejected rather than read as
  a fraction.

- `t`, `T`, `f` and `F` are accepted as booleans again. They are what
  `strconv.ParseBool` accepted before the conversions moved to the `Parser`, and
  what PostgreSQL writes for booleans in CSV exports, so a previously readable
  file failed to import entirely.

- A `StringParser` that leaves fields nil uses the defaults for them instead of
  accepting nothing. Every `SmartAssign` string conversion now goes through the
  `Parser`, so a parser built by a struct literal or unmarshalled from a
  configuration file that only sets `TimeFormats` silently stopped parsing
  numbers and booleans for a whole file. Set a field to an empty non-nil slice
  to really accept nothing.

- `NewStringParser` no longer shares its `TimeFormats` backing array with the
  package defaults and every other parser. Writing through a parser a caller
  owned reconfigured `DefaultParser` and every other live parser unsynchronised,
  which defeated the isolation that `ViewToStructSlice` documents for a `Scanner`
  that reconfigures the `Parser` it receives.

- A number that does not fit its destination is reported instead of silently
  truncated. The `Parser` always parses 64 bits while `reflect.Value.SetInt`,
  `SetUint` and `SetFloat` narrow to the destination width, so a cell reading
  `300` became `44` in an `int8` column and `1e39` became `+Inf` in a `float32`
  one, with no error. An infinity the source really spelled out is still
  assigned, because it represents what the cell said.

- A truncated UTF-32 file is reported instead of silently altered.
  `golang.org/x/text` decodes a partial trailing code unit to U+FFFD with no
  error, which `sanitizeUTF8` then turns into a space, so the last cell gained a
  character and the row count could change. UTF-16 has rejected an odd length
  all along.

- Format detection and a named encoding consume the same number of byte order
  marks. Detection split one mark off and then stripped a second, while a named
  encoding stripped one, so a file beginning with two marks parsed differently
  through `ParseDetectFormat` than through `ParseWithFormat` with the very
  `Format` detection had just reported, leaving an invisible U+FEFF at the start
  of the first cell.

- CSV files in UTF-32LE are detected and decoded as UTF-32LE instead of being
  silently decoded as UTF-16LE into NUL padded garbage without any error. The
  UTF-16LE byte order mark `FF FE` is a prefix of the UTF-32LE mark
  `FF FE 00 00`, and the shorter one used to be tested first, which made the
  UTF-32LE case unreachable. The marks are now matched longest first, which is
  what ICU, .NET and `file(1)` do. Ported from go-types PR 21.

  The two are not distinguishable from the bytes alone, because UTF-16LE text
  whose first character is U+0000 serializes to the same bytes. That ambiguity
  only exists while guessing: decoding with an encoding the caller named
  matches that encoding's mark first, so `FF FE` still means UTF-16LE for
  everyone who already knows the encoding, and a wrong mark is still an error.

- An encoding named `UTF-32LE` or `UTF-32BE` in `Format.Encoding` no longer
  turns a leading byte order mark into the first character of the first cell.
  `golang.org/x/text` decodes the mark to U+FEFF rather than removing it, and
  the UTF-16 encodings stripped it while the UTF-32 encodings did not. This
  was unreachable until the detection order above was fixed, because UTF-32LE
  was never detected, and it broke feeding the `Format` from
  `ParseDetectFormat` back into `ParseWithFormat`. This part is not in
  go-types PR 21 and is still present upstream.

- `SmartAssign` assigns the zero value for an empty source string instead of
  failing with `unsupported operation: assigning string "" to uint64`. An empty
  cell means "no value", and Excel and CSV files have empty cells for optional
  columns of every type, so reading one must not fail the whole file. The
  conversions for numbers, booleans and `time.Time` all parse the source string
  and fall through to the unsupported-operation error when the parse fails, so an
  empty cell was only survivable for string destinations. This mirrors the branch
  directly above it, which already assigns the zero value for a null source.
  Destinations that can hold the string itself keep the empty string rather than
  the zero value, which are `string` types, `any`, `[]byte`, `[]rune`, and
  pointers to those at any level, so a `*string` or `**string` column still gets
  a non-nil pointer to `""`.

  The zero value is only assigned to the destinations that the conversions below
  would parse the string into, which are the numeric kinds, `bool`, `time.Time`,
  and pointers to those. A destination that cannot hold a string of any content,
  such as a `chan`, `map`, `struct`, array, or an interface other than `any`, is
  a type mismatch rather than an empty cell and still reports an error. Assigning
  the zero value there would make the error depend on the data: a struct field
  wired to a column of the wrong type would parse every row with an empty cell
  and only fail on the first non-empty one, so a sparse column could hide the
  mismatch entirely.

  This surfaced through `exceltable`: short rows used to yield `nil` cells that
  `ViewToStructSlice` skipped, and now correctly yield `""`, which reached
  `SmartAssign` and turned every optional numeric spreadsheet column into an error.

- `SmartAssign` converts integers to string destinations as their decimal digits
  instead of the character with that code point. `reflect.Value.Convert` applies
  Go's `string(rune)` conversion, so `int(42)` silently became `"*"`, `int64(9)`
  became `"\t"` and `int(-1)` became `"�"`. Because that direct conversion ran
  first, the `srcFormatter`, `fmt.Stringer` and `fmt.Sprint` paths below it were
  unreachable for every integer source, which also made the documented example on
  `SmartAssign` itself return `"*"` where it claims `"#42"`. Integer sources are now
  excluded from the direct conversion for string destinations. `time.Month` and
  `time.Duration` sources reach their `String` method as a result, so they format as
  `"January"` and `"1h30m0s"` rather than as control characters.

- `SmartAssign` no longer assigns a value and then reports failure for the `bool`
  conversions. Converting `bool` to a numeric destination and a numeric destination
  to `bool` both wrote the result and then fell through to
  `unsupported operation: assigning bool true to int`, so `ViewToStructSlice`
  aborted the whole file on any boolean column while the destination had already
  been modified. Only the `string` sub-case returned successfully.

- `SmartAssign` no longer panics when converting a slice to a longer array. The
  guard meant to prevent this read `dst.Elem().Len()`, and `reflect.Value.Elem()`
  returns the zero `Value` for the nil destination pointer of a freshly allocated
  struct field, so `Len()` panicked with
  `reflect: call of reflect.Value.Len on zero Value` instead of returning the error.
  The array length is static and is now read from the destination type. The guard
  also only covered pointer-to-array destinations, so a plain array destination
  panicked inside `reflect.Value.Convert`; both forms are now checked.

- `SmartAssign` calls the `dstScanner` it accepts. The parameter was documented and
  threaded through `ViewToStructSlice` and the `csvtable` read functions, but
  `Scanner.ScanString` was never invoked, so a custom scanner was silently ignored.
  It is now tried for string sources, mirroring the `srcFormatter` case for string
  destinations, and falls through to the built-in parsing on
  `errors.ErrUnsupported`.

- `SmartAssign` parses duration strings into `time.Duration` destinations. The
  underlying `int64` kind meant only a plain number of nanoseconds was accepted and
  `"1h30m"` failed, although the `Parser` interface has had `ParseDuration` all
  along and the README advertised duration parsing. A string without a unit still
  parses as nanoseconds.

- `SmartAssign` and `ViewToStructSlice` take the `Parser` that is passed on to
  `Scanner.ScanString` as a parameter. It used to be a single `StringParser`
  shared by every call, whose exported `TrueStrings`, `FalseStrings`,
  `NilStrings` and `TimeFormats` fields a `Scanner` could reconfigure for all
  other concurrent conversions, which the documentation of `StringParser` shows
  how to do. Boolean strings, time formats and number locales are now
  configurable per call. `ViewToStructSlice` allocates a default `StringParser`
  once per view when it has a `Scanner` but no `Parser`, so no parser is
  allocated per cell.

- `SmartAssign` passes an empty source string to `dstScanner` before assigning
  the zero value. The scanner is the only extension point of `SmartAssign`, so
  it has to be asked first for a custom `Scanner` to be able to give an empty
  cell a different meaning than the zero value of the destination.

- `SmartAssign` recovers from panics in package `reflect` again. The deferred
  recover that the comment above it describes was commented out, so the edge cases
  it names escaped to the caller instead of being returned as errors.

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

- `Parser.IsNil(str string) bool`, which reports whether a source string
  represents a nil or null value. `StringParser` answers it from its
  `NilStrings` field, which existed with a default of `"", "nil", "<nil>",
  "null", "NULL"` and was documented as "used by higher-level scanning logic"
  but was read by nothing, while `SmartAssign` hardcoded a check for the empty
  string.

  The interface needed a method rather than a field because the parsing methods
  cannot express this: they return a value or an error, and "no value" is
  neither, so a failing `ParseInt` cannot be told apart from a string that means
  null. `IsNil` only classifies the source string; what that means for the
  destination stays with `SmartAssign` and the `Scanner`.

- `MultiScanner`, which combines `Scanner`s into a single `Scanner` that calls
  them in order until one of them handles the destination type. The `Scanner` documentation already asked
  unsupported types to be reported as `errors.ErrUnsupported` "allowing scanner
  chains", but nothing composed them and `SmartAssign` and `ViewToStructSlice`
  take a single `Scanner`, so a caller had to choose between an own `Scanner`
  and any provided one. The first `Scanner` that does not report
  `errors.ErrUnsupported` decides, so an earlier one overrides a later one and
  a real parsing error stops the chain instead of being retried.

- `StrictNilStrings`, a `Scanner` that reports an error when a source string
  meaning no value is assigned to a type that cannot represent the absence of a
  value, which are the numeric types, `bool`, `time.Time` and `time.Duration`.
  By default `SmartAssign` assigns the zero value to those, because an empty
  cell usually means "no value" and reading one must not fail a whole file. The
  cost is that the parsed data cannot tell an empty cell from a cell containing
  `0`, and that a struct field wired to a column of the wrong type keeps
  parsing every empty cell and only fails on the first non-empty one.

  Pointer destinations are not rejected, they keep the `nil` that `SmartAssign`
  already assigns for an empty string. So the fix for an error is to declare
  the field as a pointer, which states in the type which columns are optional
  and keeps an empty cell distinguishable from a parsed zero:

  ```go
  scanner := retable.MultiScanner(retable.StrictNilStrings, myScanner)
  rows, err := retable.ViewToStructSlice[Row](view, nil, scanner, nil, nil, nil)
  ```

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
- `StringParser.ParseFloat` falls back to a separator detecting parser when
  `strconv.ParseFloat` fails, so the number formats of other locales parse
  instead of erroring: `1,234.56` and `1.234,56` both become 1234.56,
  `1,234,567` and `1.234.567` become 1234567, spaces and apostrophes are
  recognized as thousands separators, surrounding whitespace is trimmed, and
  the trailing minus written by accounting and ERP exports is a negative sign.
  Digit groups before the decimal separator have to be 3 digits long, so a
  wrongly grouped string like `12.34,56` is rejected instead of being parsed
  into an arbitrary number, which is what a hand written last-separator-wins
  heuristic would do.
  `strconv.ParseFloat` is still tried first so that everything in Go's float
  literal syntax keeps parsing unchanged. When both readings fail the two errors
  are joined, so the message both quotes the string the caller passed in and says
  what the locale reading made of its separators. A lone comma
  or dot stays the decimal separator, so `1,234` parses as 1.234 and not as 1234,
  an ambiguity nothing in the string can resolve. All of this is documented on the
  method, along with the first tests for `ParseFloat`, which cover the strategies,
  the ambiguity and the strings it rejects.
- Types defined as `time.Time` or `time.Duration`, like `type Date time.Time`
  and `type Timeout time.Duration`, are parsed from strings by `SmartAssign`
  like the types they are defined as, and an empty cell assigns the zero value
  for them. Before, the destination had to be exactly `time.Time` or
  `time.Duration`: a string is not convertible to a defined type, so a `Date`
  column reported an unsupported operation for every cell, and a `Timeout`
  column fell through to the integer parsing, which accepted a plain number of
  nanoseconds but not `"5s"`. Such a column needed an own `Scanner`.

  A type defined as `time.Time` is recognized exactly, because only such a type
  is convertible to `time.Time`: a conversion between struct types needs
  identical underlying types and the unexported fields of `time.Time` can only
  be named by package `time`. A struct that embeds `time.Time` has a different
  underlying type and stays unsupported.

  A type defined as `time.Duration` cannot be recognized at all, because a
  defined type only keeps its underlying type, which is `int64`, so reflection
  cannot tell `type Timeout time.Duration` from `type Bytes int64`. Every
  defined `int64` type therefore parses duration strings, which costs a `"5m"`
  cell of a `Bytes` column being read as 5 minutes in nanoseconds instead of
  being reported as an error. The predeclared `int64` has no package path and
  is excluded, so an ordinary numeric column keeps rejecting duration strings,
  and all of these types still parse a plain number as nanoseconds.

- Tests for the `exceltable` package, which previously had none: its only test was
  commented out and referenced a fixture file that did not exist. The new tests
  build xlsx workbooks in memory and cover sparse rows, header trimming, empty
  row and column removal, empty and invalid sheets, raw versus formatted cell
  strings, and all four public read functions.
- Tests for `NewStringsView` header widening, whitespace trimming, caller-data
  immutability, and for `ViewWithTitle` cell pass-through.

### Removed

- The dependency on `github.com/domonda/go-types`, so the root `retable`
  package has no external dependencies at all again and `csvtable` needs
  `golang.org/x/text`, which moves from an indirect to a direct requirement.
  A program that imports `retable` and parses a float shrinks from 3,597,090
  to 2,807,490 bytes and loses all 6 indirect requirements that came with
  go-types: `invopop/jsonschema`, `wk8/go-ordered-map`, `bahlo/generic-list-go`,
  `buger/jsonparser`, `mailru/easyjson` and `gopkg.in/yaml.v3`. The Go linker
  cannot remove them, because `init` functions are never eliminated and
  `language.Code` is converted to an interface at package scope, which keeps
  its `JSONSchema` method and the whole schema type graph reachable. Only two
  packages were used and both are replaced without any change to the public
  API. The code is ported from go-types, which is MIT licensed with the same
  copyright holder as this repository.

  - `float.Parse` becomes the unexported `parseFloat` in `parsefloat.go`,
    ported verbatim together with its upstream test suite and with the
    `strutil.TrimSpace` that also trims the zero width space U+200B. It was
    verified against `float.Parse` over 114,337 inputs without a difference at
    the time of the port, before the separator fix listed under Fixed
    deliberately diverged it from upstream.
    Nothing in the parser needs `go-types/language`, which is what pulled in
    the JSON schema packages.
  - `charset.GetEncoding`, `charset.AutoDecode` and `charset.TrimBOM` become
    unexported functions in `csvtable/charset.go`, using the standard library
    and `golang.org/x/text` directly instead of the wrapper types of go-types.
    The accepted encoding names and the detection algorithm are unchanged,
    because the names are part of the `FormatDetectionConfig.Encodings` and
    `Format.Encoding` contract. Both implementations were compared over a
    corpus of byte order marks, encoded fixtures and random data, and over
    34 million fuzz executions, without a difference beyond the byte order
    mark fixes listed under Fixed.

### Changed

- `SprintlnView` and `SprintlnTable` no longer take an `io.Writer`. Both build
  their result into a `strings.Builder` and return it, so the parameter was
  never written to; its own documentation described it as "Ignored parameter
  (kept for signature compatibility)", and the `SprintlnView` example already
  showed the call without it. Use the `Fprintln` pair to write to a writer.

- `SmartAssign` assigns the zero value for every string that the `Parser`
  reports as nil, not only for the empty string. With the default
  `StringParser` a cell reading `NULL`, `null`, `nil` or `<nil>` now assigns
  `0` to an `int` and `nil` to an `*int`, where it used to fail with an
  unsupported operation error. Exports of database tables write those spellings
  for a null value, so failing on them meant a column that is optional in the
  database could not be read at all.

  Destinations that can hold the string itself are unaffected and keep the
  text, so a `string` or `*string` column still reads `"NULL"` as `"NULL"`.
  Only the source format knows whether such a cell is a null value or that
  text, which is why the strings are configured on the `Parser` through
  `StringParser.NilStrings` rather than hardcoded. A `Parser` with an empty
  non-nil `NilStrings` treats every string as a value, including the empty one,
  because the zero-value branch is gated entirely on `Parser.IsNil` and the
  empty string is not special-cased. Leaving the field nil uses the defaults.

- `SmartAssign`, `ViewToStructSlice` and the `csvtable` read functions take an
  additional `Parser` parameter after `dstScanner`. Passing `nil` selects
  `DefaultParser`. That is the recommended value, but it is **not** the previous
  behavior: there was no `Parser` on these paths before, and `DefaultParser`
  parses differently from the `strconv` calls it replaced. See the entries below
  for what changed: locale aware float parsing, `NULL`/`nil`/`<nil>` read as no
  value, `yes`/`no`/`on`/`off` read as booleans, and time parsed through
  `TimeFormats`. A caller that needs the old parsing has to pass a `Parser` that
  accepts only the Go literal formats.

- `SmartAssign` performs all of its string conversions through the passed
  `Parser` instead of calling `strconv.ParseBool`, `strconv.ParseInt`,
  `strconv.ParseUint`, `strconv.ParseFloat`, `ParseTime` and
  `time.ParseDuration` directly. The `Parser` used to reach only
  `Scanner.ScanString`, so a caller could configure how a `Scanner` parses but
  not how `SmartAssign` itself parses, and the two disagreed: `StringParser`
  recognized `1.234,56` while `SmartAssign` rejected it for the same cell.
  Integer, unsigned and duration parsing are unchanged because `StringParser`
  delegates them to the same functions, float parsing gains the locale handling,
  and time parsing now honors `StringParser.TimeFormats`, which the package-level
  `ParseTime` ignored. Boolean parsing gains `yes` and `no` with their case
  variants and keeps the `t`, `T`, `f` and `F` that `strconv.ParseBool` accepts,
  because `StringParser.TrueStrings` and `FalseStrings` define the strings now.

- A nil `Parser` passed to `SmartAssign` resolves to the new package-level
  `DefaultParser` variable instead of allocating a `StringParser` per call.
  `DefaultParser` is shared by all calls, so it must not be modified after
  initialization and must not be reconfigured by a `Scanner`.
  `ViewToStructSlice` keeps allocating a `StringParser` per call when it has a
  `Scanner` but no `Parser`, so a `Scanner` that reconfigures the `Parser` it
  receives still cannot affect a concurrent conversion.

- `csvtable` rejects the empty string as an encoding name instead of resolving
  it to `CodePage037`. The go-types implementation compared the name against
  the second result of `charmap.Charmap.ID()`, which is always the empty
  string, so an empty `FormatDetectionConfig.Encodings` entry silently decoded
  with the first code page of `golang.org/x/text`.

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
