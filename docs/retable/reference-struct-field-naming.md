# StructFieldNaming reference

`retable.StructFieldNaming` decides which struct fields are table columns and
what their titles are. Writers use it to turn a struct slice into a table, and
readers use it to find the struct field for each column of a table. The
readers are `ViewToStructSlice`, the `csvtable` `Read*ToStructSlice` functions
and `FindTitleRow`. Set `MatchTitle` when the titles come from files typed by
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
    MatchTitle func(viewTitle, fieldTitle string) bool
}
```

| Field | Zero value | Effect |
| --- | --- | --- |
| `Tag` | `""`: no field counts as tagged | The struct tag whose value is the column title. Only the part before the first comma counts, so `csv:"name,omitempty"` gives `name`. An empty tag value, as in `csv:""` or `csv:",omitempty"`, counts as untagged. |
| `Ignore` | `""` | A column title that removes the field from the table. `"-"` follows the `json:"-"` convention. A field whose title is `""` is always ignored. |
| `Untagged` | `nil`: the field name is the title | Called with the field name of every field without a usable `Tag` value. Return `Ignore`'s value or `""` to leave untagged fields out. |
| `MatchTitle` | `nil`: titles must be equal | Reports whether a title read from a table matches the title of a struct field. See [Title matching](#title-matching). |

A nil `*StructFieldNaming` is valid for every method except `NewView` and the
`With*` methods. It uses every exported field with its field name as the
title, and compares titles for equality.

### Default namings

| Variable | `Tag` | `Ignore` | `Untagged` | `MatchTitle` |
| --- | --- | --- | --- | --- |
| `DefaultStructFieldNaming` | `col` | `-` | `SpacePascalCase`: `DueDate` becomes `Due Date` | `nil` |
| `DefaultStructFieldNamingIgnoreUntagged` | `col` | `-` | `UseTitle("-")`: untagged fields are ignored | `nil` |

Both are package variables, so don't modify them. Derive a copy instead:

```go
naming := retable.DefaultStructFieldNamingIgnoreUntagged.WithMatchTitle(retable.TrimEqualFold)
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
| `TitlesMatch(viewTitle, fieldTitle string) bool` | yes | `MatchTitle(viewTitle, fieldTitle)`, or `viewTitle == fieldTitle` when `MatchTitle` is nil. |
| `ColumnStructFieldValue(structVal reflect.Value, column string) reflect.Value` | yes | The field of `structVal`, a struct or pointer to struct, whose title matches `column` according to `TitlesMatch`. The first matching field wins, searching embedded structs at their position. Returns an invalid `reflect.Value` when `column` is ignored or no field matches. Panics if `structVal` is not a struct or pointer to struct. |
| `CheckUniqueColumns(strct any) error` | yes | An error wrapping `ErrDuplicateColumnTitle` if two columns of `strct` match each other, nil otherwise. See [Duplicate titles](#duplicate-titles). Panics if `strct` is not a struct or pointer to struct. |
| `FindTitleRow(rows [][]string, strct any) (titleRow int, columns []int, err error)` | yes | The index of the first row containing every column title of `strct`, and the cell index of each title in it. See [FindTitleRow](#findtitlerow). Panics if `strct` is not a struct or pointer to struct. |
| `NewView(title string, table any) (View, error)` | panics | A `StructRowsView` of `table`, a slice or array of structs or struct pointers. Implements `retable.Viewer`. |
| `WithTag(tag string) *StructFieldNaming` | panics | A copy with `Tag` set. |
| `WithIgnore(ignore string) *StructFieldNaming` | panics | A copy with `Ignore` set. |
| `WithMatchTitle(match func(viewTitle, fieldTitle string) bool) *StructFieldNaming` | panics | A copy with `MatchTitle` set. `nil` restores exact matching. |
| `String() string` | yes | `StructFieldNaming{Tag: "col", Ignore: "-"}`. `Untagged` and `MatchTitle` are not shown. |

### Functions for the function fields

| Function | Use as | Result |
| --- | --- | --- |
| `TrimEqualFold(a, b string) bool` | `MatchTitle` | `strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))`. Unicode whitespace is trimmed and case is ignored, using Unicode simple case folding. `csvtable.DetectTableBounds` compares titles the same way. |
| `SpacePascalCase(name string) string` | `Untagged` | A space before every upper case letter that follows a lower case one, and for every `_`: `FirstName` becomes `First Name`, `UserID` becomes `User ID`, `HTTPServer` stays `HTTPServer`. |
| `SpaceGoCase(name string) string` | `Untagged` | `FirstName` becomes `First Name`, `HTTPServer` becomes `HTTP Server`, `APIKey` becomes `API Key`. |
| `UseTitle(columnTitle string) func(string) string` | `Untagged` | Always `columnTitle`. `UseTitle("-")` with `Ignore: "-"` leaves untagged fields out. |

### Errors

| Variable | Returned by | Message |
| --- | --- | --- |
| `ErrNoColumns` | `FindTitleRow` for a struct without columns | `struct has no columns: main.Empty` |
| `ErrDuplicateColumnTitle` | `CheckUniqueColumns`, `FindTitleRow` | `duplicate column title: "Name" and "name " of main.Account` |
| `ErrTitleRowNotFound` | `FindTitleRow` when no row has every title | `no row contains all column titles ["Konto" "IBAN"] of main.Account` |

The returned errors wrap these variables, so test them with `errors.Is`.

## Title matching

Every lookup of a column by its title goes through `TitlesMatch`:

| Caller | Compares |
| --- | --- |
| `ColumnStructFieldValue` | each field title with the requested column |
| `ViewToStructSlice` | each view column title with the field titles, to find the field to assign |
| `ViewToStructSlice` `requiredCols` | each required title with the view column titles, then the matching view column with the field titles |
| `csvtable.ReadStringsToStructSlice` and the other `Read*ToStructSlice` | through `ViewToStructSlice` |
| `CheckUniqueColumns` | each pair of field titles, in both argument orders |
| `FindTitleRow` | each cell of a row with each field title |

The first argument of `MatchTitle` is the title read from the table, the
second the title of the struct field. A custom `MatchTitle` does not have to be
symmetric, so it can, for example, accept a column titled `Konto-Nr.` for the
field titled `Konto` but not the other way round.

With the default exact matching, a column title read from a file must equal
the field title byte for byte. `retable.NewStringsView` trims the surrounding
whitespace of column titles, which helps with `" Konto"`, but not with `KONTO`
or `konto`. Those need `TrimEqualFold`:

| Title in the table | Field title | `nil` `MatchTitle` | `TrimEqualFold` |
| --- | --- | --- | --- |
| `Konto` | `Konto` | match | match |
| `" Konto\t"` | `Konto` | no match | match |
| `KONTO` | `Konto` | no match | match |
| `Konten` | `Konto` | no match | no match |
| `STRASSE` | `Straße` | no match | no match, because `ß` has no simple case folding to `SS` |

### More than one column matches a field

`ViewToStructSlice` assigns every view column whose title matches a field, so
when two columns match, the last one wins. A file with both `Konto` and
`konto` columns fills the field from `konto` with `TrimEqualFold`. Use
`FindTitleRow` with a `FilteredView`, as shown below, to read only the first
matching cell of the title row.

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

loose := exact.WithMatchTitle(retable.TrimEqualFold)
err := loose.CheckUniqueColumns(Account{})
// duplicate column title: "Number" and "number" of main.Account
// errors.Is(err, retable.ErrDuplicateColumnTitle) == true
```

The struct and the naming are both fixed at compile time, so a duplicate is a
programming error. Check it in a test, not on every import.

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
- The first cell matching a title is used.

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
naming := retable.DefaultStructFieldNamingIgnoreUntagged.WithMatchTitle(retable.TrimEqualFold)

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
should be found with `csvtable.DetectTableBounds` instead.

`FindTitleRow` only finds the start of the table. Rows after the data, like a
total line, become rows of the view. Cut them off, or reject them with the
`validate` function of `ViewToStructSlice`.

### FindTitleRow compared with DetectTableBounds

| | `FindTitleRow` | `csvtable.DetectTableBounds` |
| --- | --- | --- |
| Input | rows and a struct | rows and any number of expected titles |
| Title row | first row with every title | row with the most titles among the rows with the majority column count |
| Missing title | `ErrTitleRowNotFound` | still found, if it has more titles than any other row |
| Rows of different lengths | no effect | only rows of the majority column count are considered |
| Title comparison | `TitlesMatch` of the naming | always `TrimEqualFold` |
| End of the table | not detected | first row with a different column count |
| Result | row index and cell index of each title | `TableBounds` with title row, end row and column count |

## Related

- [How to import tables typed by people](howto-import-tables-typed-by-people.md):
  the tasks this API is built for, with troubleshooting
- [csvtable reference](../csvtable/reference.md): `DetectTableBounds` and the
  `Read*ToStructSlice` functions
- [README: Struct Field Naming](../../README.md#struct-field-naming): the short
  introduction
