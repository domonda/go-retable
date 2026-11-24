# StructFieldNaming reference

`retable.StructFieldNaming` decides which struct fields are table columns and
what their titles are. Writers use it to turn a struct slice into a table, and
readers use it to find the struct field for each column of a table. The
readers are `ViewToStructSlice`, the `csvtable` `Read*ToStructSlice` functions
and `FindTitleRow`. Set `TitleMatcher` when the titles come from files typed by
people, so that `" KONTO "` still finds the field titled `Konto`.

This page lists the complete API, its defaults and its errors. For tasks, see
[How to import tables typed by people](howto-import-tables-typed-by-people.md).
The generated API docs are on
[pkg.go.dev](https://pkg.go.dev/github.com/domonda/go-retable#StructFieldNaming).

## API

### Fields

```go
type StructFieldNaming struct {
    Tag      string
    Ignore   string
    Untagged func(fieldName string) (column string)
    TitleMatcher func(viewTitle, fieldTitle string) bool
    DisallowAmbiguousTitles bool
    PreferFirstColumn bool
}
```

| Field | Zero value | Effect |
| --- | --- | --- |
| `Tag` | `""`: no field counts as tagged | The struct tag whose value is the column title. Only the part before the first comma counts, so `csv:"name,omitempty"` gives `name`. An empty tag value, as in `csv:""` or `csv:",omitempty"`, counts as untagged. |
| `Ignore` | `""` | A column title that removes the field from the table. `"-"` follows the `json:"-"` convention. A field whose title is `""` is always ignored. |
| `Untagged` | `nil`: the field name is the title | Called with the field name of every field without a usable `Tag` value. Return `Ignore`'s value or `""` to leave untagged fields out. |
| `TitleMatcher` | `nil`: titles must be equal | Reports whether a title read from a table matches the title of a struct field, which sets how loose the matching is: `nil`, `TrimEqual`, `TrimEqualFold` or your own function. See [Title matching](#title-matching). |
| `DisallowAmbiguousTitles` | `false`: the closest match wins | Makes it an error, `ErrAmbiguousColumnTitle`, when more than one column matches a field or one column matches more than one field. See [More than one column matches a field](#more-than-one-column-matches-a-field). |
| `PreferFirstColumn` | `false`: the last column wins | Decides between columns that match a field equally closely, like two of the same name: the first one wins instead of the last. |

A nil `*StructFieldNaming` is valid for every method except `NewView` and the
`With*` methods. It uses every exported field with its field name as the
title, and compares titles for equality.

### Default namings

| Variable | `Tag` | `Ignore` | `Untagged` | `TitleMatcher` |
| --- | --- | --- | --- | --- |
| `DefaultStructFieldNaming` | `col` | `-` | `SpacePascalCase`: `DueDate` becomes `Due Date` | `nil` |
| `DefaultStructFieldNamingIgnoreUntagged` | `col` | `-` | `UseTitle("-")`: untagged fields are ignored | `nil` |

Both are package variables, so don't modify them. Derive a copy instead:

```go
naming := retable.DefaultStructFieldNamingIgnoreUntagged.WithTitleMatcher(retable.TrimEqualFold)
```

`retable.SelectViewer` returns `&DefaultStructFieldNaming` for every table
that is not a `[][]string`, which is why writers use the `col` tag by default.
`ViewToStructSlice` with a nil naming does not use the default. It matches
columns to field names, without the `col` tag.

### Methods

| Method | Nil receiver | Returns |
| --- | --- | --- |
| `Columns(strct any) []string` | yes | The titles of all columns of `strct`, a struct or pointer to struct, in field order. Fields of anonymously embedded structs are inlined at their position. Panics for any other type. |
| `StructFieldColumn(field reflect.StructField) string` | yes | The title of one field, before `Ignore` is applied. `""` for unexported and anonymous fields. |
| `IsIgnored(column string) bool` | yes | Whether `column` is `""` or equals `Ignore`. |
| `ColumnTitlesMatch(viewTitle, fieldTitle string) bool` | yes | `TitleMatcher(viewTitle, fieldTitle)`, or `viewTitle == fieldTitle` when `TitleMatcher` is nil. |
| `ColumnStructFieldValue(structVal reflect.Value, column string) reflect.Value` | yes | The field of `structVal`, a struct or pointer to struct, whose title matches `column` according to `ColumnTitlesMatch`. If several fields match, the [closest match](#more-than-one-column-matches-a-field) wins, and the first field of equally close ones, searching embedded structs at their position. Returns an invalid `reflect.Value` when `column` is ignored or no field matches. Panics if `structVal` is not a struct or pointer to struct. |
| `CheckUniqueColumns(strct any) error` | yes | An error wrapping `ErrDuplicateColumnTitle` if two columns of `strct` match each other, nil otherwise. See [Duplicate titles](#duplicate-titles). Panics if `strct` is not a struct or pointer to struct. |
| `DetectTableBounds(rows [][]string, expectedTitles ...string) TableBounds` | yes | The table within rows that also hold header and trailer lines: the row matching the most `expectedTitles` among the rows of the majority column count, and the end of the table. See [FindTitleRow compared with DetectTableBounds](#findtitlerow-compared-with-detecttablebounds). |
| `FindTitleRow(rows [][]string, strct any) (titleRow int, columns []int, err error)` | yes | The index of the first row containing every column title of `strct`, and the cell index of each title in it. See [FindTitleRow](#findtitlerow). Panics if `strct` is not a struct or pointer to struct. |
| `NewView(title string, table any) (View, error)` | panics | A `StructRowsView` of `table`, a slice or array of structs or struct pointers. Implements `retable.Viewer`. |
| `WithTag(tag string) *StructFieldNaming` | panics | A copy with `Tag` set. |
| `WithIgnore(ignore string) *StructFieldNaming` | panics | A copy with `Ignore` set. |
| `WithTitleMatcher(match func(viewTitle, fieldTitle string) bool) *StructFieldNaming` | panics | A copy with `TitleMatcher` set. `nil` restores exact matching. |
| `WithDisallowAmbiguousTitles(disallow bool) *StructFieldNaming` | panics | A copy with `DisallowAmbiguousTitles` set. |
| `WithPreferFirstColumn(preferFirst bool) *StructFieldNaming` | panics | A copy with `PreferFirstColumn` set. |
| `String() string` | yes | `StructFieldNaming{Tag: "col", Ignore: "-"}`. `Untagged` and `TitleMatcher` are not shown. |

`StructRowsViewer` embeds a `StructFieldNaming` and has its own `WithTag`,
`WithIgnore`, `WithTitleMatcher`, `WithDisallowAmbiguousTitles` and
`WithPreferFirstColumn`, which return a `*StructRowsViewer`.

### Functions for the function fields

| Function | Use as | Result |
| --- | --- | --- |
| `TrimEqual(a, b string) bool` | `TitleMatcher` | `strings.TrimSpace(a) == strings.TrimSpace(b)`. Unicode whitespace is trimmed, case counts. |
| `TrimEqualFold(a, b string) bool` | `TitleMatcher` | `strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))`. Unicode whitespace is trimmed and case is ignored, using Unicode simple case folding. `csvtable.DetectTableBounds` compares titles the same way. |
| `SpacePascalCase(name string) string` | `Untagged` | A space before every upper case letter that follows a lower case one, and for every `_`: `FirstName` becomes `First Name`, `UserID` becomes `User ID`, `HTTPServer` stays `HTTPServer`. |
| `SpaceGoCase(name string) string` | `Untagged` | `FirstName` becomes `First Name`, `HTTPServer` becomes `HTTP Server`, `APIKey` becomes `API Key`. |
| `UseTitle(columnTitle string) func(string) string` | `Untagged` | Always `columnTitle`. `UseTitle("-")` with `Ignore: "-"` leaves untagged fields out. |

### Errors

| Variable | Returned by | Message |
| --- | --- | --- |
| `ErrNoColumns` | `FindTitleRow` for a struct without columns | `struct has no columns: main.Empty` |
| `ErrDuplicateColumnTitle` | `CheckUniqueColumns`, `FindTitleRow`, `ViewToStructSlice` with a `TitleMatcher` | `duplicate column title: "Name" and "name " of main.Account` |
| `ErrTitleRowNotFound` | `FindTitleRow` when no row has every title | `no row contains all column titles ["Konto" "IBAN"] of main.Account` |
| `ErrAmbiguousColumnTitle` | `ViewToStructSlice`, `FindTitleRow` with `DisallowAmbiguousTitles` | `ambiguous column title: columns "Konto" and "KONTO" both match "Konto" of main.Account`, or `ambiguous column title: column "Summe" matches both "Betrag" and "Total" of main.Booking` |

The returned errors wrap these variables, so test them with `errors.Is`.

## Title matching

Every lookup of a column by its title goes through `ColumnTitlesMatch`:

| Caller | Compares |
| --- | --- |
| `ColumnStructFieldValue` | each field title with the requested column |
| `ViewToStructSlice` | each view column title with the field titles, to find the field to assign |
| `ViewToStructSlice` `requiredCols` | each required title with the view column titles, then the matching view column with the field titles |
| `csvtable.ReadStringsToStructSlice` and the other `Read*ToStructSlice` | through `ViewToStructSlice` |
| `CheckUniqueColumns` | each pair of field titles, in both argument orders |
| `FindTitleRow` | each cell of a row with each field title |

The first argument of `TitleMatcher` is the title read from the table, the
second the title of the struct field. A custom `TitleMatcher` does not have to be
symmetric, so it can, for example, accept a column titled `Konto-Nr.` for the
field titled `Konto` but not the other way round.

`TitleMatcher` sets how loose the matching is. With the default exact matching,
a column title read from a file must equal the field title byte for byte.
`TrimEqual` ignores the whitespace around the titles, `TrimEqualFold` also
their case:

| Title in the table | Field title | `nil` `TitleMatcher` | `TrimEqual` | `TrimEqualFold` |
| --- | --- | --- | --- | --- |
| `Konto` | `Konto` | match | match | match |
| `" Konto\t"` | `Konto` | no match | match | match |
| `KONTO` | `Konto` | no match | no match | match |
| `Konten` | `Konto` | no match | no match | no match |
| `STRASSE` | `Straße` | no match | no match | no match, because `ß` has no simple case folding to `SS` |

`retable.NewStringsView` trims the surrounding whitespace of column titles, and
the `csvtable` `Read*ToStructSlice` functions use it. A file title `" Konto"`
therefore matches the field title `Konto` there even with exact matching.

### More than one column matches a field

A file can have several columns that match the title of one field: `Konto`
and `KONTO` with `TrimEqualFold`, two columns both named `Konto`, or a title
and an alternative that a custom `TitleMatcher` accepts. `ViewToStructSlice`
then assigns the field once per row, from one of them:

1. The closest match wins, whatever the order of the columns:

   | Rank | The column title | Example for the field title `Konto` |
   | --- | --- | --- |
   | 1 | equals the field title | `Konto` |
   | 2 | equals it without surrounding whitespace | `" Konto"` |
   | 3 | equals it without surrounding whitespace and case | `KONTO` |
   | 4 | is only accepted by `TitleMatcher` | `Konto-Nr.` with a custom function |

   `TitleMatcher` still decides whether titles match at all. With `TrimEqual`
   there is no rank 3, with `nil` only rank 1. A column of a looser rank is
   not read at all, also in the rows where the closer column is empty.

2. Of several equally close columns, the cell with content wins over an empty
   one, row by row. A cell is empty when it is null-like (see `IsNullLike`) or
   a string of only whitespace. If more than one of them has content, the last
   column wins. If none has, the last column is assigned. With
   `PreferFirstColumn` it is the first column in both cases.

Only the cell that wins is converted and validated.

| Columns | Row | `Konto` field with `TrimEqualFold` | and `PreferFirstColumn` |
| --- | --- | --- | --- |
| `KONTO`, `Konto` | `120`, `999` | `999`: the equal title wins | `999` |
| `KONTO`, `Konto` | `120`, empty | empty: the looser column is not read | empty |
| `Konto`, `Konto` | `120`, empty | `120`: the cell with content wins | `120` |
| `Konto`, `Konto` | empty, `999` | `999`: the cell with content wins | `999` |
| `Konto`, `Konto` | `120`, `999` | `999`: the last of two cells with content | `120`: the first of them |

A column that matches several fields is assigned to the field it matches the
closest, and to the first field of equally close ones. `TrimEqual` and
`TrimEqualFold` can't cause that for a struct that passes
[`CheckUniqueColumns`](#duplicate-titles), a custom `TitleMatcher` can, by
accepting the same alternative title for two fields.

`FindTitleRow` applies the same ranks to the cells of the title row and takes
the last of several equally close cells, or the first one with
`PreferFirstColumn`. It returns one cell per title, so the rule about cells
with content does not apply to a `FilteredView` made from its result: the
field is read from that one column, also where its cell is empty.

Set `DisallowAmbiguousTitles` to get an error instead of a choice:

```go
naming := retable.DefaultStructFieldNamingIgnoreUntagged.
    WithTitleMatcher(retable.TrimEqualFold).
    WithDisallowAmbiguousTitles(true)

_, err := retable.ViewToStructSlice[Account](view, naming, nil, nil, nil, nil)
// ambiguous column title: columns "KONTO" and "Konto" both match "Konto" of main.Account
// errors.Is(err, retable.ErrAmbiguousColumnTitle) == true
```

`ViewToStructSlice` and the `csvtable` `Read*ToStructSlice` functions then
return `ErrAmbiguousColumnTitle` before the first row when more than one
column matches a field, of whatever rank, or one column matches more than one
field. `FindTitleRow` returns it for a title row with more than one cell for a
title or one cell for more than one title. It works with every `TitleMatcher`,
also `nil`, where two columns of the same name are the only ambiguity.

### Duplicate titles

Loose matching can make two fields match the same column. With `TrimEqualFold`
the titles `Name` and `name` both match a column `NAME`, and only the first
field would ever be filled. `CheckUniqueColumns` finds such a pair:

```go
type Account struct {
    Number string `col:"Number"`
    Code   string `col:"number"`
}

exact := &retable.DefaultStructFieldNamingIgnoreUntagged
exact.CheckUniqueColumns(Account{}) // nil, "Number" != "number"

loose := exact.WithTitleMatcher(retable.TrimEqualFold)
err := loose.CheckUniqueColumns(Account{})
// duplicate column title: "Number" and "number" of main.Account
// errors.Is(err, retable.ErrDuplicateColumnTitle) == true
```

`FindTitleRow` runs this check before it searches the rows. `ViewToStructSlice`,
and with it the `csvtable` `Read*ToStructSlice` functions, runs it before the
first row when the naming has a `TitleMatcher`, and returns the error. Without a
`TitleMatcher` it accepts equal field titles and fills the first field.

The struct and the naming are both fixed at compile time, so a duplicate is a
programming error. Check it in a test, so that it is found before an import
fails.

## FindTitleRow

```go
func (n *StructFieldNaming) FindTitleRow(rows [][]string, strct any) (titleRow int, columns []int, err error)
```

`FindTitleRow` finds the column titles row in raw rows, such as rows parsed
from a CSV file or formatted from an Excel sheet. The title row is the first
row that contains a cell matching every column title of `strct`:

- The cells can be in any order.
- The row can have more cells than `strct` has columns.
- Rows can have different numbers of cells.
- Of several cells matching a title, the closest match is used, and the last
  of equally close ones, or the first with `PreferFirstColumn`. See
  [More than one column matches a field](#more-than-one-column-matches-a-field).

`columns[i]` is the index of the cell holding the title `n.Columns(strct)[i]`.
Pass `columns` as the `ColumnMapping` of a `FilteredView` to get a view with
exactly the columns of `strct`, in field order:

```go
type Account struct {
    Number string `col:"Konto"`
    Name   string `col:"Bezeichnung"`
}

rows := [][]string{
    {"Kontenplan 2026"},
    {"Bezeichnung", "Kontoart", " KONTO "},
    {"Software", "1", "120"},
    {"Bank", "2", "2800"},
}
naming := retable.DefaultStructFieldNamingIgnoreUntagged.WithTitleMatcher(retable.TrimEqualFold)

titleRow, columns, err := naming.FindTitleRow(rows, Account{})
// titleRow == 1, columns == []int{2, 0}

view := &retable.FilteredView{
    Source:        retable.NewStringsView("", rows[titleRow+1:], rows[titleRow]...),
    ColumnMapping: columns,
}
accounts, err := retable.ViewToStructSlice[Account](view, naming, nil, nil, nil, nil)
// []Account{{Number: "120", Name: "Software"}, {Number: "2800", Name: "Bank"}}
```

Before searching the rows it returns `ErrNoColumns` for a struct without
columns, and `ErrDuplicateColumnTitle` if two columns match each other, as
`CheckUniqueColumns` would. Every column of `strct` is required. A column that
may be missing from the file belongs in a struct used only for reading, or
should be found with `DetectTableBounds` instead.

`FindTitleRow` only finds the start of the table. Rows after the data, like a
total line, become rows of the view. Cut them off, or reject them with the
`validate` function of `ViewToStructSlice`.

### FindTitleRow compared with DetectTableBounds

| | `FindTitleRow` | `DetectTableBounds` |
| --- | --- | --- |
| Input | rows and a struct | rows and any number of expected titles |
| Title row | first row with every title | row with the most titles among the rows with the majority column count |
| Missing title | `ErrTitleRowNotFound` | still found, if it has more titles than any other row |
| Rows of different lengths | no effect | only rows of the majority column count are considered |
| Title comparison | `ColumnTitlesMatch` of the naming, on the cells as they are | `ColumnTitlesMatch` of the naming, on the trimmed cells |
| Several cells match a title | the closest match, then `PreferFirstColumn` | does not choose, `ViewToStructSlice` does afterwards |
| End of the table | not detected | first row with a different column count |
| Result | row index and cell index of each title | `TableBounds` with title row, end row and column count |

Both are methods of the naming and work on rows of any format, so a CSV file
and a spreadsheet find their titles with the same rules. The `csvtable`
`Read*ToStructSlice` functions call `naming.DetectTableBounds` with their
`requiredCols`. `csvtable.DetectTableBounds` is the method of a naming with
`TrimEqualFold`.

```go
bounds := naming.DetectTableBounds(rows, "Konto", "Bezeichnung")
if !bounds.Valid() {
    return errors.New("no table with the columns Konto and Bezeichnung")
}
view := retable.NewStringsView("", bounds.Rows(rows))
accounts, err := retable.ViewToStructSlice[Account](view, naming, nil, nil, nil, nil, "Konto", "Bezeichnung")
```

## Related

- [How to import tables typed by people](howto-import-tables-typed-by-people.md):
  the tasks this API is built for, with troubleshooting
- [csvtable reference](../csvtable/reference.md): `DetectTableBounds` and the
  `Read*ToStructSlice` functions
- [README: Struct Field Naming](../../README.md#struct-field-naming): the short
  introduction
