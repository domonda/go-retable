# How to import tables typed by people

Read a CSV export or an Excel sheet into a struct slice when the file was made
by a person or a foreign system: the column titles carry stray spaces and any
case, lines sit above the titles, the columns come in any order, and a total
line follows the data. Each recipe stands on its own.

## Prerequisites

- Go 1.26 or later.
- `github.com/domonda/go-retable` at a version after v1.0.0-beta.3, which added
  `TitleMatcher` and `FindTitleRow`. The Excel recipe also needs
  `github.com/domonda/go-retable/exceltable` at the same version.
- A struct with a `col` tag on every field to import:

  ```go
  type Booking struct {
      Date   string  `col:"Datum"`
      Text   string  `col:"Buchungstext"`
      Amount float64 `col:"Betrag"`
  }
  ```

The complete API is in the
[StructFieldNaming reference](reference-struct-field-naming.md).

## Match column titles loosely

Readers compare the column titles of a file with the field titles exactly, so
`DATUM` does not fill the field titled `Datum`. Set `TitleMatcher` to
`retable.TrimEqualFold` to ignore surrounding whitespace and case:

```go
naming := retable.DefaultStructFieldNamingIgnoreUntagged.WithTitleMatcher(retable.TrimEqualFold)
```

`DefaultStructFieldNamingIgnoreUntagged` uses the `col` tag and leaves untagged
fields out, so a helper field without a tag is never mistaken for a column.
Pass `naming` to every function below.

To ignore only the whitespace around a title and keep the case, use
`retable.TrimEqual` instead. Without a `TitleMatcher` the titles must be equal.

## Import a CSV export with lines around the table

1. Pass the titles of the columns you need as `requiredCols`. `csvtable` uses
   them to find the title row between the lines above it, and to find where
   the table ends:

   ```go
   export := []byte("Kontoauszug Nr. 4\r\n" +
       "Zeitraum;01.01.2026;31.01.2026\r\n" +
       "\r\n" +
       "DATUM; Buchungstext ;Betrag;Valuta\r\n" +
       "02.01.2026;Miete;-500,00;02.01.2026\r\n" +
       "05.01.2026;\"Gehalt, Januar\";2.000,00;05.01.2026\r\n" +
       "\r\n" +
       "Erstellt am 06.01.2026\r\n")

   bookings, format, err := csvtable.ReadBytesDetectFormatToStructSlice[Booking](
       export,
       nil,    // default format detection
       naming, // loose title matching
       nil, nil, nil, nil, // scanner, parser, formatter, validate
       "Datum", "Buchungstext", "Betrag", // required columns
   )
   if err != nil {
       return err
   }
   ```

   The result is:

   ```go
   []Booking{
       {Date: "02.01.2026", Text: "Miete", Amount: -500},
       {Date: "05.01.2026", Text: "Gehalt, Januar", Amount: 2000},
   }
   ```

   The separator `;` and the line ending `\r\n` were detected. The title row
   was found below the two header lines, the `Valuta` column was skipped, and
   the trailer line was dropped. The default parser read the German numbers
   `-500,00` and `2.000,00`.

2. For a file, use `ReadFileDetectFormatToStructSlice` with an `io.Reader`
   instead of the bytes.

The required titles do the work here. Without them `csvtable` takes the first
non-empty line as the titles. How the table is found is described in the
[csvtable reference](../csvtable/reference.md#finding-the-table).

## Import an Excel sheet whose titles are not in the first row

`exceltable` reads the first non-empty row of a sheet as the column titles.
When a title like `Kontenplan 2026` sits above the real titles, turn the sheet
back into rows and find the title row yourself:

1. Read the sheet and format it as rows, including its first row:

   ```go
   sheet, err := exceltable.ReadFirstSheet(file, false)
   if err != nil {
       return err
   }
   rows, err := retable.FormatViewAsStrings(ctx, sheet, nil, retable.OptionAddHeaderRow)
   if err != nil {
       return err
   }
   // [["Kontenplan 2026" "" ""]
   //  ["Bezeichnung" "Kontoart" " KONTO "]
   //  ["Software" "1" "120"]
   //  ["Bank" "2" "2800"]
   //  ["Summe" "" ""]]
   ```

   `exceltable` has already removed empty rows and columns. With `false` the
   cells hold the values as Excel displays them with their number formats.
   Pass `true` for the raw values, like `1234.5` for a cell displayed as
   `1,234.50`.

2. Find the row with every column title of the struct:

   ```go
   type Account struct {
       Number string `col:"Konto"`
       Name   string `col:"Bezeichnung"`
   }

   titleRow, columns, err := naming.FindTitleRow(rows, Account{})
   if err != nil {
       return err
   }
   // titleRow == 1, columns == []int{2, 0}: "Konto" is in cell 2, "Bezeichnung" in cell 0
   ```

3. End the data at the first row without an account number, so that the
   `Summe` line is not imported:

   ```go
   data := rows[titleRow+1:]
   for i, row := range data {
       if strings.TrimSpace(row[columns[0]]) == "" {
           data = data[:i]
           break
       }
   }
   ```

   `FormatViewAsStrings` returns rows of equal length, so `row[columns[0]]`
   is always in range. With rows from `csvtable`, check the length first.

4. Map the cells to the struct and convert:

   ```go
   view := &retable.FilteredView{
       Source:        retable.NewStringsView("", data, rows[titleRow]...),
       ColumnMapping: columns, // only the cells of the struct's columns
   }
   accounts, err := retable.ViewToStructSlice[Account](view, naming, nil, nil, nil, nil)
   // []Account{{Number: "120", Name: "Software"}, {Number: "2800", Name: "Bank"}}
   ```

The same steps work for CSV rows from `csvtable.ParseDetectFormat`, and they
are the better choice there when every column must be present: `FindTitleRow`
fails when one title is missing, while `csvtable` takes the row with the most
titles.

## Accept other titles for the same column

Exports of the same data name a column differently, `Betrag` in one and
`Umsatz` in the next. Write a `TitleMatcher` that also accepts the alternatives:

```go
alternatives := map[string][]string{
    "Betrag": {"Umsatz", "Betrag EUR"},
}
naming := retable.DefaultStructFieldNamingIgnoreUntagged.WithTitleMatcher(
    func(viewTitle, fieldTitle string) bool {
        if retable.TrimEqualFold(viewTitle, fieldTitle) {
            return true
        }
        for _, alt := range alternatives[fieldTitle] {
            if retable.TrimEqualFold(viewTitle, alt) {
                return true
            }
        }
        return false
    },
)
```

The first argument is the title from the file, the second the title of the
field. Keep `requiredCols` as the field titles, `"Betrag"`, not `"Umsatz"`.

A file that has both `Betrag` and `Umsatz` fills the field from `Betrag`: the
field's own title wins over an alternative, see
[Decide what happens when two columns match a field](#decide-what-happens-when-two-columns-match-a-field).
Don't accept the same alternative for two fields. The column would fill only
the first of them.

`csvtable` looks for the title row with the same `TitleMatcher`, so a row is
found by its alternative titles too, as with `FindTitleRow`.

## Decide what happens when two columns match a field

A file can have a `Betrag` and a `BETRAG` column, or two columns named
`Betrag`. By default the import still works, by fixed rules:

- The closest title wins: equal to the field title, then equal without
  surrounding whitespace, then equal without case, then a title only your
  `TitleMatcher` accepts. The order of the columns does not matter.
- Of two equally close columns, like two named `Betrag`, each row takes the
  cell that has content. If both have content, the last column wins. Add
  `WithPreferFirstColumn(true)` to the naming to let the first one win.

To refuse such a file instead, because nobody should guess which column holds
the amounts, add `WithDisallowAmbiguousTitles(true)`:

```go
naming := retable.DefaultStructFieldNamingIgnoreUntagged.
    WithTitleMatcher(retable.TrimEqualFold).
    WithDisallowAmbiguousTitles(true)

bookings, _, err := csvtable.ReadBytesDetectFormatToStructSlice[Booking](
    export, nil, naming, nil, nil, nil, nil, "Datum", "Buchungstext", "Betrag")
if errors.Is(err, retable.ErrAmbiguousColumnTitle) {
    // ambiguous column title: columns "Betrag" and "BETRAG" both match "Betrag" of main.Booking
    return fmt.Errorf("the file has two columns for the same value: %w", err)
}
```

The error names both columns, so it can be shown to the person who made the
file. `FindTitleRow` returns the same error for the title row.

## Make sure no two fields match the same column

With loose matching, fields titled `Name` and `name` would both match a column
`NAME`, and only the first one would ever be filled. Check every import struct
once, in a test:

```go
func TestImportStructs(t *testing.T) {
    for _, strct := range []any{Booking{}, Account{}} {
        if err := naming.CheckUniqueColumns(strct); err != nil {
            t.Error(err)
        }
    }
}
```

`FindTitleRow` runs the same check and returns `ErrDuplicateColumnTitle`
before it looks at the rows. So do `ViewToStructSlice` and the `csvtable`
`Read*ToStructSlice` functions for a naming with a `TitleMatcher`, so an import
with such a struct fails instead of leaving the second field empty. The test
finds it before a file does.

## Verification

Run the import with a sample file in a test and compare the result with the
structs you expect:

```go
func TestImportBookings(t *testing.T) {
    export, err := os.ReadFile("testdata/kontoauszug.csv")
    if err != nil {
        t.Fatal(err)
    }
    bookings, _, err := csvtable.ReadBytesDetectFormatToStructSlice[Booking](
        export, nil, naming, nil, nil, nil, nil, "Datum", "Buchungstext", "Betrag")
    if err != nil {
        t.Fatal(err)
    }
    if len(bookings) != 2 || bookings[1].Amount != 2000 {
        t.Errorf("unexpected bookings: %+v", bookings)
    }
}
```

Check the number of rows too: a missing title row shows up as empty structs,
not as an error (see below).

## Troubleshooting

**`required column "Datum" not found in View columns`, but the file has a
column `DATUM`.** The naming compares titles exactly. Set `TitleMatcher` to
`retable.TrimEqualFold`. Surrounding spaces alone are not the cause:
`NewStringsView` trims them from every column title.

**`required column "Betrag" not found as struct field`.** The file has the
column, but no field of the struct has that title. Check the `col` tag. With
`DefaultStructFieldNamingIgnoreUntagged` an untagged field is not a column,
even if its name equals the title.

**Every struct is empty and there is no error.** No `requiredCols` were passed,
so `csvtable` took the first non-empty line, like `Kontoauszug Nr. 4`, as the
column titles. No title matched a field, and unmatched columns are skipped
without an error. Pass the titles you need as `requiredCols`.

**`no row contains all column titles ["Konto" "IBAN"] of main.Account`
(`ErrTitleRowNotFound`).** One title is missing from the file or spelled
differently, like `Konto-Nr.`. Compare the titles in the error with the file.
Then fix the tag, [accept the other title](#accept-other-titles-for-the-same-column),
or move a column the file may lack to a struct used only for reading.

**`duplicate column title: "Name" and "name " of main.Account`
(`ErrDuplicateColumnTitle`).** Two fields match each other with the naming's
`TitleMatcher`. Rename one of the tags.

**`struct has no columns: main.Account` (`ErrNoColumns`).** No field has a
`col` tag. `DefaultStructFieldNamingIgnoreUntagged` ignores untagged fields.

**A field is filled from the wrong column.** Two columns of the file match its
title, like `KONTO` and `konto`. Neither equals the field title `Konto`, so
they are equally close and each row takes the cell with content, the last one
if both have it, or the first one with `WithPreferFirstColumn(true)`. Rename
the tag to the title the file uses, so that this column wins as the equal
one, or refuse such files with
[`WithDisallowAmbiguousTitles(true)`](#decide-what-happens-when-two-columns-match-a-field).

**`ambiguous column title: columns "Betrag" and "BETRAG" both match "Betrag"
of main.Booking` (`ErrAmbiguousColumnTitle`).** The naming has
`DisallowAmbiguousTitles` set and the file has two columns for one field.
Remove one of the columns from the file, or drop the option to let the closest
title win.

**The total line is imported as a row.** `FindTitleRow` finds the start of the
table, not its end. Cut the data as in step 3 of the Excel recipe. `csvtable`
ends the table at the first row with a different number of cells, so a total
line with as many cells as the table, like `Summe;;2500,00`, is imported too.
Reject it with a `validate` function or remove it from the result.

**`expected struct or pointer to struct instead of []main.Account` (panic).**
`FindTitleRow`, `Columns` and `CheckUniqueColumns` take a struct value like
`Account{}`, not a slice.

**A number like `1.234,56` fails to parse, or reads as 1.234.** See
[Parsers and Scanners](../../README.md#parsers-and-scanners) for the number
formats the default parser accepts and how to configure it.

## Related

- [StructFieldNaming reference](reference-struct-field-naming.md): every
  method, the matching rules and the errors
- [How column titles find their struct fields](explanation-title-matching.md):
  why the closest title wins and when to use which title row finder
- [PanicError reference](reference-panic-error.md): the error an import
  returns for a recovered panic
- [csvtable reference](../csvtable/reference.md): format detection, table
  bounds and the read functions
- [How to read and write CSV files](../csvtable/howto-read-and-write-csv.md)
- [How to write Excel workbooks](../exceltable/howto-write-workbooks.md)
