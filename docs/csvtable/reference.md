# csvtable reference

`github.com/domonda/go-retable/csvtable` parses CSV files as real systems
export them, with the encoding, separator and line ending detected, and
converts the rows into struct slices. Its `Writer[T]` writes struct slices
and views as CSV that reads back both here and in `encoding/csv`, as long as
the [UTF-8 byte order mark](#options) is off, which `encoding/csv` does not
remove. Use it for
bank statements, accounting exports and spreadsheet downloads, where the input
does not follow RFC 4180 (the CSV specification) and the output has to open
in Excel.

This page lists the complete API, its defaults and its errors. For tasks, see
[How to read and write CSV files](howto-read-and-write-csv.md). For why
malformed files parse the way they do, see
[How csvtable reads malformed CSV](explanation-malformed-csv.md). The
generated API docs are on
[pkg.go.dev](https://pkg.go.dev/github.com/domonda/go-retable/csvtable).

```sh
go get github.com/domonda/go-retable
```

`csvtable` is a package of the root module, so it has no version of its own.

## Parsing

| Function | Result |
| --- | --- |
| `ParseDetectFormat(csv []byte, config *FormatDetectionConfig) (rows [][]string, format *Format, err error)` | Detects encoding, line ending and separator, then parses. A nil `config` means `NewDefaultFormatDetectionConfig()`. |
| `ParseWithFormat(csv []byte, format *Format) (rows [][]string, err error)` | Parses with a known format. Returns `format.Validate()`'s error for an invalid format. |

Both decode the data to UTF-8, remove a byte order mark, and return one row
per line of the file:

- **A row can be empty** (`len(row) == 0`). A blank line is an empty row, and
  so is every line that was joined into a field containing a newline. The data
  of a file ending with a newline ends with an empty row. Empty rows keep row
  indices equal to line numbers, which is what an error report to the sender
  of the file needs. `RemoveEmptyRows` removes them.
- **Rows can have different lengths.** Nothing is padded.
- **A malformed field is returned as its literal text** instead of failing
  the file, see [How csvtable reads malformed CSV](explanation-malformed-csv.md).
- **Undecodable bytes become spaces.** Every byte that is not valid UTF-8 after
  decoding, every U+FFFD replacement character and every no-break space
  (U+00A0) is replaced with a plain space.
- **A `sep=X` first line**, which Excel writes, declares the separator and is
  removed from the rows. It may be quoted (`"sep=;"`) and written as `SEP=`.
  `ParseWithFormat` returns an error when it declares a different separator
  than `format.Separator`:
  `separator ';' in header line is different from format.Separator ','`.

Neither function panics. A panic while parsing is returned as an error, see
[Panics](#panics).

### Format

```go
type Format struct {
    Encoding  string `json:"encoding"`
    Separator string `json:"separator"`
    Newline   string `json:"newline"`
}
```

| Field | Valid values |
| --- | --- |
| `Encoding` | A name accepted by the [encoding lookup](#encoding-names), like `UTF-8`, `UTF-16LE`, `ISO 8859-1`, `Windows 1252`, `Macintosh`. |
| `Separator` | Exactly one byte that is not `"`, not DEL and not a control character, except tab. |
| `Newline` | `"\n"`, `"\r\n"` or `"\n\r"`. A lone `"\r"` is not supported. |

| Function | Result |
| --- | --- |
| `NewFormat(separator string) *Format` | `&Format{Encoding: "UTF-8", Separator: separator, Newline: "\r\n"}` |
| `(*Format).Validate() error` | nil for a valid format. Safe to call on a nil `*Format`, which returns the error `<nil> csv.Format`. Other errors: `missing csv.Format.Encoding`, `missing csv.Format.Separator`, `invalid csv.Format.Separator: "\""`, `missing csv.Format.Newline`, `invalid csv.Format.Newline: "\r"`. |
| `EscapeQuotes(val string) string` | `val` with every `"` doubled, as RFC 4180 escapes quotes in a quoted field. |

`Validate` does not look up the encoding name. An unknown name fails in
`ParseWithFormat` with `encoding not found: "Latin-1"`.

The `Format` returned by `ParseDetectFormat` always passes `Validate`, also
for empty input, which returns `UTF-8`, `,` and `\n`. Store it as JSON to parse
the next file of the same sender with `ParseWithFormat`, or pass its separator
to `Writer.WithDelimiter`.

### FormatDetectionConfig

```go
type FormatDetectionConfig struct {
    Encodings     []string `json:"encodings"`
    EncodingTests []string `json:"encodingTests"`
}
```

| Field | Default from `NewDefaultFormatDetectionConfig()` |
| --- | --- |
| `Encodings` | `UTF-8`, `UTF-16LE`, `ISO 8859-1`, `Windows 1252`, `Macintosh`, in this order |
| `EncodingTests` | `ä Ä ö Ö ü Ü ß § € д Д ъ Ъ б Б л Л и И ж` |

Detection works in three steps:

1. **Encoding.** A byte order mark decides alone: UTF-8, UTF-16LE, UTF-16BE,
   UTF-32LE or UTF-32BE, also when that encoding is not in `Encodings`. The
   marks are tested longest first, so `FF FE 00 00` is UTF-32LE, not UTF-16LE
   followed by a NUL character. Without a mark, the data is decoded with every
   encoding of `Encodings`, and the one whose decoded text contains the most
   occurrences of `EncodingTests` wins. A tie goes to the earlier encoding.
   When no test string occurs at all, the data is used as UTF-8. Because of the
   tie rule, `Müller` written in Windows 1252 is reported as `ISO 8859-1`:
   both decode it to the same text, and ISO 8859-1 comes first.
2. **Line ending.** `\r\n`, `\n\r` and bare `\n` are counted outside of quoted
   fields, and the most frequent one wins. On a tie the wider one wins, and
   `\r\n` beats `\n\r`. Without any newline the result is `\n`.
3. **Separator.** A `sep=X` first line wins. Otherwise `,` `;` tab and `|` are
   scored by how many records the candidate splits into the same number of
   columns, not by how often it occurs. Records are counted outside of quoted
   fields. A candidate that leaves some records unsplit only counts if it
   splits at least 2 records and at least one in five of all records. A tie
   goes to the candidate with more columns, then to the earlier candidate in
   the order above. Without a scoring candidate the result is `,`.

### Encoding names

`Format.Encoding` and `FormatDetectionConfig.Encodings` accept:

- `UTF-8`, `UTF-16LE`, `UTF-16BE`, `UTF-32LE`, `UTF-32BE`, in any case. A
  missing hyphen is added, so `UTF8` and `utf16le` work.
- The display names of `golang.org/x/text/encoding/charmap`, in any case, like
  `ISO 8859-1`, `ISO 8859-15`, `Windows 1252`, `Windows 1250`, `Macintosh`,
  `IBM Code Page 850` or `KOI8-R`. They are written with a space, not a
  hyphen: `ISO-8859-1` is not found.
- `ISO-8859-6E`, `ISO-8859-6I`, `ISO-8859-8E` and `ISO-8859-8I`.

A code page decoder removes a leading UTF-8 byte order mark first, because
Excel writes one in front of files in other encodings too.

## Reading into structs

| Function | Input |
| --- | --- |
| `ReadStringsToStructSlice[T any](rows [][]string, naming, dstScanner, parser, srcFormatter, validate, requiredCols...) ([]T, error)` | rows from `ParseDetectFormat` or `ParseWithFormat` |
| `ReadBytesWithFormatToStructSlice[T any](csvData []byte, format *Format, naming, ...) ([]T, error)` | bytes in a known format |
| `ReadFileWithFormatToStructSlice[T any](csvReader io.Reader, format *Format, naming, ...) ([]T, error)` | a reader in a known format |
| `ReadBytesDetectFormatToStructSlice[T any](csvData []byte, detectConfig *FormatDetectionConfig, naming, ...) ([]T, *Format, error)` | bytes in an unknown format |
| `ReadFileDetectFormatToStructSlice[T any](csvReader io.Reader, detectConfig *FormatDetectionConfig, naming, ...) ([]T, *Format, error)` | a reader in an unknown format |

All five take the same trailing parameters:

| Parameter | Type | nil means |
| --- | --- | --- |
| `naming` | `*retable.StructFieldNaming` | columns match field names exactly, without a tag |
| `dstScanner` | `retable.Scanner` | only the built-in conversions of `retable.SmartAssign` |
| `parser` | `retable.Parser` | the default `retable.StringParser` |
| `srcFormatter` | `retable.Formatter` | no custom conversion of source values to strings |
| `validate` | `func(reflect.Value) error` | no validation. Called with every assigned field; `retable.CallValidateMethod` calls its `Validate() error` or `Valid() bool` method. |
| `requiredCols` | `...string` | no required columns, and the first non-empty row holds the titles |

`T` is a struct or a pointer to a struct. The `Detect` variants also return
the detected format, even when the conversion fails afterwards. The `File`
variants read the whole reader into memory first.

`ReadStringsToStructSlice`, which the others call, does this:

1. With `requiredCols`, it cuts the table out of the rows with
   `naming.DetectTableBounds(rows, requiredCols...)`, so the title row is
   found with the same title matching that assigns the columns in step 3. If
   no table is found, it keeps all rows.
2. It removes empty rows with `RemoveEmptyRows`.
3. It calls `retable.ViewToStructSlice` with a `retable.StringsView` whose
   first row holds the column titles. That function matches each column title
   to a struct field with `naming.ColumnTitlesMatch`, so `naming.TitleMatcher` decides
   whether `DATUM` matches `Datum`. When several columns match one field, the
   closest title wins, or `naming.DisallowAmbiguousTitles` makes it an error,
   see the
   [StructFieldNaming reference](../retable/reference-struct-field-naming.md#more-than-one-column-matches-a-field). It returns an error when a required title
   is not a column or not a field:
   `required column "Datum" not found in View columns`,
   `required column "Datum" not found as struct field`.

Columns without a matching field are skipped. A file whose title row was not
found therefore gives one empty struct per row, without an error. Pass
`requiredCols` to prevent that. The title rules are in the
[StructFieldNaming reference](../retable/reference-struct-field-naming.md).

## Finding the table

```go
// package retable
func (n *StructFieldNaming) DetectTableBounds(rows [][]string, expectedTitles ...string) TableBounds

// package csvtable
type TableBounds = retable.TableBounds
func DetectTableBounds(rows [][]string, expectedTitles ...string) TableBounds
```

The detection lives in the root package, so rows of any format find their
table the same way: CSV rows, the rows of a spreadsheet, or your own. The
method compares titles the way its naming does, with `naming.TitleMatcher`.
`csvtable.DetectTableBounds` is that method of a naming with
`retable.TrimEqualFold`, for when you have no naming at hand.

| Member | Meaning |
| --- | --- |
| `TableBounds.TitleRow int` | Index of the column titles row, `-1` when no table was found |
| `TableBounds.EndRow int` | Index after the last data row |
| `TableBounds.NumColumns int` | Column count of the table |
| `TableBounds.Valid() bool` | Whether a table was found. The zero value is not valid. |
| `TableBounds.Rows(rows [][]string) [][]string` | `rows[TitleRow:EndRow]`: the titles row followed by the data rows, the layout `retable.NewStringsView` expects. nil when the bounds are not valid or `EndRow > len(rows)`. |

How it decides:

1. The table width is the column count the majority of rows have. Rows with
   one column only vote when no row has more, so a stack of title lines does
   not outvote the data. A tie goes to the wider count.
2. The title row is the row of that width matching the most `expectedTitles`.
   The cells are trimmed, like `retable.NewStringsView` trims column titles,
   and compared with `naming.ColumnTitlesMatch`: exactly, or as the naming's
   `TitleMatcher` says. `csvtable.DetectTableBounds` compares with
   `retable.TrimEqualFold`: trimmed and without case. The first
   row matching all of them wins. A row matching none is never used.
   Without `expectedTitles` it is the first row of that width without an
   empty field.
3. The data is the rows of that width after the titles, up to the first row of
   a different width. Empty rows in between are skipped, because joining a
   field that contains a newline leaves empty rows behind.

Known limits:

- Without `expectedTitles`, a header line as wide as the table is taken as the
  column titles.
- A trailer row as wide as the table, like `Summe;;1500,00`, cannot be told
  apart from a data row and is included.

`retable.StructFieldNaming.FindTitleRow` is the alternative that requires every
title of a struct and ignores the column count, compared in the
[StructFieldNaming reference](../retable/reference-struct-field-naming.md#findtitlerow-compared-with-detecttablebounds).

## Cleaning up rows

All of these take rows straight from the parser.

| Function | Effect |
| --- | --- |
| `RemoveEmptyRows(rows [][]string) [][]string` | Drops rows without fields and rows whose fields are all `""`. Returns `rows` itself when there is nothing to drop, else a new slice. A field of spaces is not empty. |
| `SetRowsWithNonUniformColumnsNil(rows [][]string) [][]string` | A new slice in which every row without the majority column count is nil, so indices stay aligned. Majority as in `DetectTableBounds`. |
| `SetEmptyRowsNil(rows [][]string) [][]string` | A new slice in which every row whose fields are all `""` is nil. |
| `TrimSpace(rows [][]string)` | In place. Trims leading and trailing white space from every field. |
| `ReplaceNewlineWithSpace(rows [][]string)` | In place. Replaces every `\r\n`, `\n` and `\r` in every field with one space. |
| `CompactSpacedStrings(rows [][]string) (numModified int)` | In place. Rewrites a field like `S h i n e r g y`, where every second character is a space, as `Shinergy`, and returns the number of changed fields. Fields shorter than 3 characters are kept, but a regular 3-character value like `A B` becomes `AB`, so only use it for CSV extracted from PDF files. |
| `ReplaceNewlineWithSpacefunc(rows [][]string)` | Deprecated alias of `ReplaceNewlineWithSpace`. |

## Writer

```go
func NewWriter[T any]() *Writer[T]
```

`Writer[T]` is an immutable builder: every `With*` method returns a modified
copy and leaves the receiver unchanged, so a configured writer can be shared.
`T` is the type of the whole table passed to `Write`, like `[]Invoice`.

### Write methods

| Method | Writes |
| --- | --- |
| `Write(ctx context.Context, dest io.Writer, table T) error` | `table` through the viewer set by `WithTableViewer`, or else `retable.SelectViewer(table)`: `[][]string` with its first row as titles, struct slices with `retable.DefaultStructFieldNaming`. |
| `WriteWithViewer(ctx, dest, viewer retable.Viewer, table T) error` | `table` through `viewer`. |
| `WriteView(ctx, dest, view retable.View) error` | `view`. `T` is ignored. |
| `ViewStrings(ctx, view retable.View) ([][]string, error)` | Nothing. Returns the formatted and escaped cells, with the header row first when enabled. Ignores padding, line ending, encoder and byte order mark. |

Without padding, the output is written row by row, so an error in a later row
leaves the earlier rows in `dest`. With padding, all cells are formatted
before the first byte is written. `ctx` is checked before every cell.

### Options

| Option | Default | Setter | Getter |
| --- | --- | --- | --- |
| Delimiter | `;` | `WithDelimiter(rune)` | `Delimiter() rune` |
| Line ending | `\r\n` | `WithNewLine(string)` | `NewLine() string` |
| Header row | off | `WithHeaderRow(bool)` | |
| Quote escaping | `""` | `WithEscapeQuotes(string)` | `EscapeQuotes() string` |
| Nil value | `""` | `WithNilValue(string)` | `NilValue() string` |
| Quote all fields | off | `WithQuoteAllFields(bool)` | `QuoteAllFields() bool` |
| Quote empty fields | off | `WithQuoteEmptyFields(bool)` | `QuoteEmptyFields() bool` |
| Padding | `NoPadding` | `WithPadding(Padding)` | |
| Output encoder | nil | `WithEncoder(Encoder)` | `Encoder() Encoder` |
| UTF-8 byte order mark | off | `WithUTF8BOM(bool)` | `UTF8BOM() bool` |
| Table viewer | nil: `retable.SelectViewer` | `WithTableViewer(retable.Viewer)` | |

**Quoting.** A formatted field is quoted when it contains the delimiter, a
`\n` or a `"`, or always with `WithQuoteAllFields(true)`. Every `"` inside is
replaced with the quote escaping, `""` by default, or `\"` with
`WithEscapeQuotes("\\\"")`. Every `\r` is removed from a field before. An
empty field is written as `""` with `WithQuoteEmptyFields(true)`. A formatter
that returns `isRaw` true bypasses all of this: its string is written as is.

**Header row.** `WithHeaderRow(true)` writes the column titles of the view as
the first row. Column formatters are not applied to it.

**Padding.** `AlignLeft`, `AlignRight` and `AlignCenter` pad every field with
spaces to the width of its column, counted in runes, for output read by
people. A padded field no longer reads back as the same value.

**Encoder.** The encoder converts each finished row, including its line
ending, before it is written:

```go
type Encoder interface {
    Bytes([]byte) ([]byte, error)
}
type EncoderFunc func([]byte) ([]byte, error)
func PassthroughEncoder() Encoder // returns the data unchanged
```

A `*encoding.Encoder` of `golang.org/x/text`, like
`charmap.Windows1252.NewEncoder()`, implements `Encoder`. It is not safe for
concurrent use, so give each goroutine its own writer with its own encoder.

**UTF-8 byte order mark.** With `WithUTF8BOM(true)`, the output of `Write`,
`WriteWithViewer` and `WriteView` starts with the bytes `"\xEF\xBB\xBF"`, so
that Excel opens the file as UTF-8 instead of the legacy code page of the
system. The mark is written once, in front of the first row, with and without
padding. A table without rows and without header row is written as the mark
alone. When writing fails or panics before the first row, `dest` stays
empty, as it would without the mark. Combined with an encoder, the write methods return
`ErrUTF8BOMWithEncoder` and write nothing, because the encoded output is not
UTF-8. That includes `PassthroughEncoder()`: any non-nil encoder is rejected.
A cancelled `ctx` returns its error before the mark is written, also for a
table without rows.

The parse and read functions of this package remove the mark. Other readers
may not: `encoding/csv` returns it as part of the first field,
`"\ufeffNumber"`, and fails with `bare " in non-quoted-field` when the first
field is quoted, as with `WithQuoteAllFields(true)`. Strip the mark from the
data before passing it to such a reader. The mark is written by every
`WriteView` call, so write a file with one call.

### Formatters

Each cell is formatted by the first of these that handles it:

1. The column formatter of its column index.
2. The type formatters: exact type, then interface types in registration
   order, then kind.
3. The nil value, for a cell that `retable.IsNullLike` reports, like a nil
   pointer or a zero `time.Time`.
4. `fmt.Sprint` of the value, dereferenced once if it is a pointer.

A formatter that returns `errors.ErrUnsupported` passes the cell on to the
next step. Any other error stops the write.

| Method | Registers |
| --- | --- |
| `WithColumnFormatter(columnIndex int, formatter retable.CellFormatter)` | a formatter for one column |
| `WithColumnFormatterFunc(columnIndex int, formatterFunc retable.CellFormatterFunc)` | the same for a function |
| `WithTypeFormatters(formatter *retable.ReflectTypeCellFormatter)` | all type formatters at once, replacing the registered ones |
| `WithTypeFormatter(typ reflect.Type, formatter retable.CellFormatter)` | a formatter for the exact type `typ` and pointers to it |
| `WithTypeFormatterFunc(typ reflect.Type, formatterFunc retable.CellFormatterFunc)` | the same for a function |
| `WithInterfaceTypeFormatter(typ reflect.Type, formatter retable.CellFormatter)` | a formatter for types implementing the interface `typ`. Panics if `typ` is nil or not an interface. |
| `WithInterfaceTypeFormatterFunc(typ reflect.Type, formatterFunc retable.CellFormatterFunc)` | the same for a function |
| `WithKindFormatter(kind reflect.Kind, formatter retable.CellFormatter)` | a formatter for a kind, like `reflect.Float64` |
| `WithKindFormatterFunc(kind reflect.Kind, formatterFunc retable.CellFormatterFunc)` | the same for a function |
| `WithTypeFormatterReflectFunc(function any)` | a type formatter from a function like `func(time.Time) string`, see `retable.ReflectCellFormatterFunc`. Panics if the function has the wrong signature. |
| `WithTypeFormatterReflectRawFunc(function any)` | the same, with the result written raw, without quoting |

Passing a nil formatter, or a nil function, removes the formatter registered
for that column, type or kind.

## Constants and errors

| Name | Value or message |
| --- | --- |
| `ErrUTF8BOMWithEncoder` | `csvtable: UTF-8 byte order mark can't be combined with an encoder` |
| `NoPadding`, `AlignLeft`, `AlignRight`, `AlignCenter` | values of `type Padding int`, `0` to `3` |

### Panics

The parse and read functions never panic. A panic inside them, also one of the
`csvReader` or of the `validate` function you pass, is returned as a
`*retable.PanicError` with the message `csvtable: panic: <value>`. A panic
value that is an error is wrapped, so `errors.Is` and `errors.As` find it. A
panic of the `dstScanner`, `parser` or `srcFormatter` is returned the same way
by `retable.SmartAssign`, with the message `retable: panic: <value>`.

The message has no stack trace. The stack trace of the panic is in the `Stack`
field:

```go
var panicErr *retable.PanicError
if errors.As(err, &panicErr) {
    log.Printf("%v\n%s", err, panicErr.Stack)
}
```

| `PanicError` member | Meaning |
| --- | --- |
| `Package string` | The package that recovered the panic: `csvtable`, `retable` or `exceltable` |
| `Value any` | The value `panic` was called with |
| `Stack []byte` | The stack trace of the panic, as `runtime/debug.Stack` formats it |
| `Error() string` | `<Package>: panic: <Value>` |
| `Unwrap() error` | `Value` if it is an error, else nil |

`Writer` does not recover panics. A panic of a formatter or a viewer reaches
the caller.

## Examples

Parse a file, then read the bank statement table out of it:

```go
statement := []byte("Kontoauszug Nr. 4\r\n" +
    "Zeitraum;01.01.2025;31.01.2025\r\n" +
    "\r\n" +
    "Datum;Text;Betrag\r\n" +
    "01.01.2025;Miete;-500,00\r\n" +
    "02.01.2025;\"Gehalt, Januar\";2000,00\r\n" +
    "\r\n" +
    "Erstellt am 03.01.2025\r\n")

rows, format, err := csvtable.ParseDetectFormat(statement, nil)
// format: &Format{Encoding: "UTF-8", Separator: ";", Newline: "\r\n"}

bounds := csvtable.DetectTableBounds(rows, "Datum", "Betrag")
// TableBounds{TitleRow: 3, EndRow: 6, NumColumns: 3}
view := retable.NewStringsView("Bookings", bounds.Rows(rows))
```

Write CSV for Excel:

```go
type Invoice struct {
    Number string  `col:"Nummer"`
    Total  float64 `col:"Betrag"`
    Note   *string `col:"Notiz"`
}

note := "bar"
invoices := []Invoice{{"R-1", 1234.5, nil}, {"R-2", 99, &note}}

writer := csvtable.NewWriter[[]Invoice]().
    WithHeaderRow(true).
    WithUTF8BOM(true)
err := writer.Write(ctx, dest, invoices)
// "\xEF\xBB\xBFNummer;Betrag;Notiz\r\nR-1;1234.5;\r\nR-2;99;bar\r\n"
```

Write a padded table for a log or a test:

```go
type Product struct {
    SKU   string `col:"SKU"`
    Name  string `col:"Name"`
    Notes string `col:"Notes"`
}
products := []Product{{"A-1", "Widget", `He said "hi"`}}

csvtable.NewWriter[[]Product]().
    WithHeaderRow(true).
    WithDelimiter('|').
    WithPadding(csvtable.AlignLeft).
    Write(ctx, os.Stdout, products)
// SKU|Name  |Notes
// A-1|Widget|"He said ""hi"""
```

## Related

- [How to read and write CSV files](howto-read-and-write-csv.md): recipes and
  troubleshooting
- [How csvtable reads malformed CSV](explanation-malformed-csv.md): the
  parsing design and its limits
- [StructFieldNaming reference](../retable/reference-struct-field-naming.md):
  how column titles find struct fields
- [How to import tables typed by people](../retable/howto-import-tables-typed-by-people.md)
- [PanicError reference](../retable/reference-panic-error.md): the recovered
  panic error of every package
