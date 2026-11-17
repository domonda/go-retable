# How to read and write CSV files

Recipes for the tasks that come up with CSV files from and for other systems:
importing an export into structs, controlling the format detection, cleaning
up rows, and writing files that Excel opens correctly. Each recipe stands on
its own.

## Prerequisites

- Go 1.26 or later.
- The root module, which contains `csvtable`:

  ```sh
  go get github.com/domonda/go-retable
  ```

- The recipes import these packages plus the standard library packages they
  name:

  ```go
  import (
      "github.com/domonda/go-retable"
      "github.com/domonda/go-retable/csvtable"
  )
  ```

The complete API, the detection rules and every default are in the
[reference](reference.md).

## Import a CSV file into structs

The input below is a bank statement with a title line, a period line, blank
lines and a trailer, exported with `;` separators and Windows line endings.
You don't have to tell the parser any of that.

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

    bookings, format, err := csvtable.ReadBytesDetectFormatToStructSlice[Booking](
        statement,
        nil, // default format detection
        &retable.StructFieldNaming{Tag: "csv"},
        nil, nil, nil, nil, // scanner, parser, formatter, validate
        "Datum", "Text", "Betrag", // required columns
    )
    if err != nil {
        panic(err)
    }
    fmt.Printf("%s, separator %q\n", format.Encoding, format.Separator)
    for _, b := range bookings {
        fmt.Printf("%+v\n", b)
    }
}
```

Output:

```
UTF-8, separator ";"
{Datum:01.01.2025 Text:Miete Betrag:-500,00}
{Datum:02.01.2025 Text:Gehalt, Januar Betrag:2000,00}
```

Three things happened without configuration:

1. `;` was picked as the separator over the comma inside `"Gehalt, Januar"`.
2. `Datum;Text;Betrag` was found as the column titles, not
   `Kontoauszug Nr. 4`. The required column titles make this work.
3. The title, period and trailer lines were dropped.

For a file, pass an `io.Reader` to `ReadFileDetectFormatToStructSlice`
instead. Typed fields work too: `Betrag float64` reads `-500,00` as `-500`.
When the column titles come with stray spaces or in another case, see
[How to import tables typed by people](../retable/howto-import-tables-typed-by-people.md).

## Parse rows without structs

```go
rows, format, err := csvtable.ParseDetectFormat(csvBytes, nil)
if err != nil {
    return err
}
rows = csvtable.RemoveEmptyRows(rows)
view := retable.NewStringsView("Data", rows) // first row holds the titles
```

`ParseDetectFormat` returns one row per line, including an empty row for each
blank line and for each line joined into a multi-line field, so a row index is
the line number minus one. Only a `sep=` first line is removed, which shifts
the rows by one. Keep the rows unfiltered while you report errors to the
sender of the file:

```go
bounds := csvtable.DetectTableBounds(rows, "Datum", "Betrag")
if !bounds.Valid() {
    return errors.New("no table with the columns Datum and Betrag")
}
for i := bounds.EndRow; i < len(rows); i++ {
    if len(rows[i]) > 1 {
        log.Printf("line %d: %d fields after the end of the table", i+1, len(rows[i]))
    }
}
```

## Restrict the encodings that are tried

Without a byte order mark, the encoding is guessed by decoding with every
candidate and counting test characters like `ä`, `€` and `д`. When you know the
senders, restrict the candidates and the test characters to their languages:

```go
config := &csvtable.FormatDetectionConfig{
    Encodings:     []string{"UTF-8", "Windows 1252"},
    EncodingTests: []string{"ä", "ö", "ü", "Ä", "Ö", "Ü", "ß", "€"},
}
rows, format, err := csvtable.ParseDetectFormat(csvBytes, config)
```

Encoding names are written with a space, `Windows 1252` and `ISO 8859-1`, not
with a hyphen. The [reference](reference.md#encoding-names) lists the accepted
names.

## Parse every file of a sender with the same format

Detection guesses from the data. A file with few rows, or without any
non-ASCII character, can be guessed differently from the sender's other files.
Detect the format once from a typical file, store it, and parse later files
with it:

```go
_, format, err := csvtable.ParseDetectFormat(typicalFile, nil)
if err != nil {
    return err
}
stored, err := json.Marshal(format) // {"encoding":"UTF-8","separator":";","newline":"\r\n"}

// Later
var format csvtable.Format
if err := json.Unmarshal(stored, &format); err != nil {
    return err
}
rows, err := csvtable.ParseWithFormat(nextFile, &format)
```

`ReadBytesWithFormatToStructSlice` and `ReadFileWithFormatToStructSlice` take
the stored format the same way.

## Clean up parsed rows

The cleanup functions change the rows in place or return new slices:

```go
rows, _, err := csvtable.ParseDetectFormat(csvBytes, nil)
if err != nil {
    return err
}
csvtable.TrimSpace(rows)               // " 4711 " becomes "4711"
csvtable.ReplaceNewlineWithSpace(rows) // a multi-line address becomes one line
rows = csvtable.RemoveEmptyRows(rows)  // drop blank lines last, after trimming
```

Call `RemoveEmptyRows` after `TrimSpace`, because a row of spaces only counts
as empty once it is trimmed.

For CSV extracted from a PDF, where text comes out as `S h i n e r g y`, add
`csvtable.CompactSpacedStrings(rows)`, which rewrites it as `Shinergy`. Only
call it for such files: it rewrites every value whose every second character
is a space, so a regular value like `A B` becomes `AB` too.

## Write CSV that Excel opens correctly

Excel opens a double-clicked `.csv` file in the legacy code page of the system
unless the file starts with a UTF-8 byte order mark, so `Müller` turns into
`MÃ¼ller`. It also splits the fields at the list separator of the system
locale: `;` in most of Europe, `,` in the US. Write both to match your readers:

```go
type Invoice struct {
    Number string    `col:"Rechnung"`
    Date   time.Time `col:"Datum"`
    Total  float64   `col:"Betrag"`
}

writer := csvtable.NewWriter[[]Invoice]().
    WithHeaderRow(true).
    WithUTF8BOM(true). // Excel reads UTF-8
    WithDelimiter(';'). // the default, matching a German Excel
    WithTypeFormatter(reflect.TypeFor[time.Time](), retable.LayoutFormatter("02.01.2006"))

err := writer.Write(ctx, file, invoices)
```

Every `With*` method returns a new writer, so keep the returned value. Store a
configured writer in a package variable and share it.

## Write CSV in another encoding

For a system that only reads Windows 1252, encode the output instead of
writing a byte order mark:

```go
import "golang.org/x/text/encoding/charmap"

writer := csvtable.NewWriter[[]Invoice]().
    WithHeaderRow(true).
    WithEncoder(charmap.Windows1252.NewEncoder())
err := writer.Write(ctx, file, invoices)
```

The encoder converts every row, so `Müller €` is written as the Windows 1252
bytes `4D FC 6C 6C 65 72 20 80`. A character that does not exist in the target
encoding, like `д` in Windows 1252, makes the write fail. An encoder of
`golang.org/x/text` is not safe for concurrent use, so create this writer per
request, not as a shared variable.

Don't combine `WithEncoder` with `WithUTF8BOM(true)`: writing then returns
`csvtable.ErrUTF8BOMWithEncoder`, because the mark would declare the encoded
bytes to be UTF-8.

## Serve CSV as an HTTP download

```go
func serveInvoices(w http.ResponseWriter, r *http.Request) {
    invoices, err := loadInvoices(r.Context())
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "text/csv; charset=utf-8")
    w.Header().Set("Content-Disposition", `attachment; filename="invoices.csv"`)
    err = invoiceWriter.Write(r.Context(), w, invoices)
    if err != nil {
        log.Printf("writing invoices.csv: %v", err)
    }
}
```

Rows are written one by one, so once the first row is out, the status is sent
and an error can only be logged; the client gets a truncated file. When a
formatter can fail, write into a `bytes.Buffer` first and copy it to the
response only on success, or use `WithPadding`, which formats every cell
before it writes anything.

## Format cells

Register formatters by column, type or kind. The first one that handles a
cell wins, in this order: column, exact type, interface, kind.

```go
writer := csvtable.NewWriter[[]Invoice]().
    WithHeaderRow(true).
    // Two decimals with a decimal comma for every float64
    WithKindFormatterFunc(reflect.Float64,
        func(ctx context.Context, view retable.View, row, col int) (string, bool, error) {
            s := strconv.FormatFloat(view.Cell(row, col).(float64), 'f', 2, 64)
            return strings.Replace(s, ".", ",", 1), false, nil
        },
    ).
    // Column 0 in upper case, not applied to the header row
    WithColumnFormatterFunc(0,
        func(ctx context.Context, view retable.View, row, col int) (string, bool, error) {
            return strings.ToUpper(view.Cell(row, col).(string)), false, nil
        },
    )
```

Return `false` as the second result so that the writer still quotes the
string where needed. Return `errors.ErrUnsupported` to leave a cell to the
next formatter. A nil pointer or a zero time that no formatter handles is
written as the nil value, `""` by default, set with `WithNilValue("NULL")`.
A formatter registered for a type also gets its zero values:
`retable.LayoutFormatter("02.01.2006")` writes a zero `time.Time` as
`01.01.0001`, so use a pointer field for an optional date.

## Align columns for people

```go
csvtable.NewWriter[[]Product]().
    WithHeaderRow(true).
    WithDelimiter('|').
    WithPadding(csvtable.AlignLeft).
    Write(ctx, os.Stdout, products)
// SKU|Name  |Notes
// A-1|Widget|"He said ""hi"""
```

`AlignRight` and `AlignCenter` work the same way. Padded files are for reading
in a terminal or a test; the spaces become part of the values when parsed.

## Verification

Read back what you wrote, in a test:

```go
var buf bytes.Buffer
if err := writer.Write(ctx, &buf, invoices); err != nil {
    t.Fatal(err)
}
rows, format, err := csvtable.ParseDetectFormat(buf.Bytes(), nil)
if err != nil {
    t.Fatal(err)
}
// The byte order mark is detected and removed: format.Encoding == "UTF-8"
// rows[0] holds the header row, rows[1] the first invoice
```

To check the file in Excel, open it by double-click, not through an import
dialog, because that is what your users do.

## Troubleshooting

**Excel shows `MÃ¼ller` instead of `Müller`.** The file is UTF-8 without a
byte order mark. Add `WithUTF8BOM(true)`.

**Excel shows every row in the first column.** The delimiter differs from the
list separator of the reader's locale. Use `;` for German or Austrian Excel and
`,` for US Excel.

**`csvtable: UTF-8 byte order mark can't be combined with an encoder`
(`ErrUTF8BOMWithEncoder`).** The writer has both `WithUTF8BOM(true)` and
`WithEncoder`. Drop the mark, or write the mark of the target encoding to the
destination yourself before calling the writer.

**Parsed values contain spaces where umlauts were, like `M ller`.** The file
was decoded with the wrong encoding, and the bytes that were invalid in it
became spaces. This happens with `ParseWithFormat` and a format whose encoding
is wrong, or when detection found no test character to decide by. Parse with
the sender's real encoding, or restrict the candidates as shown
[above](#restrict-the-encodings-that-are-tried).

**`encoding not found: "ISO-8859-1"`.** Write encoding names with a space:
`ISO 8859-1`, `Windows 1252`.

**The wrong separator was detected.** A single-column file with a `;` inside
some of its values, or a file of only a few rows, can be split at the wrong
character. Parse files of a known sender with their stored format, see
[above](#parse-every-file-of-a-sender-with-the-same-format).

**`separator ';' in header line is different from format.Separator ','`.**
The file starts with a `sep=;` line, which declares another separator than
the format you passed. Pass the declared one, or use `ParseDetectFormat`,
which reads it.

**Every struct is empty and there is no error.** No required columns were
passed, so the first non-empty line, like a report title, was taken as the
column titles and no column matched a field. Pass the titles you need as
`requiredCols`.

**`required column "Datum" not found in View columns`.** The column titles in
the file differ in case or spelling from the field titles. See
[How to import tables typed by people](../retable/howto-import-tables-typed-by-people.md#troubleshooting).

**A total line like `Summe;;1500,00` is imported as a row.** It has as many
fields as the table, so it can't be told apart from a data row. Reject it with
the `validate` function or remove it from the result.

**An error starting with `csvtable: panic:`.** Something panicked while
parsing or converting, often the `validate` function you passed. The stack
trace in the error shows where. Report a panic inside `csvtable` itself as a
bug.

**A `\r` inside a quoted field is missing.** A `\r` directly before a line
break inside a quoted field is lost, see the
[known limits](explanation-malformed-csv.md#known-limits).

## Related

- [csvtable reference](reference.md): every function, option and default
- [How csvtable reads malformed CSV](explanation-malformed-csv.md): why
  detection and parsing work the way they do
- [How to import tables typed by people](../retable/howto-import-tables-typed-by-people.md)
- [README: Parsers and Scanners](../../README.md#parsers-and-scanners):
  number, date and empty-cell parsing
