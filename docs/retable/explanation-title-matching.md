# How column titles find their struct fields

A table read from a file has column titles, a struct has field titles, and
something has to decide which column fills which field. For files that a
program wrote, equal strings are enough. For files that people typed or that
foreign systems exported, they are not: the titles carry stray spaces and
any case, the same column appears twice, and a title or a total line sits
around the table. This page explains the rules go-retable uses to match
titles in such files, and why it uses those rules and not simpler ones.

The rules themselves, with every method and error, are in the
[StructFieldNaming reference](reference-struct-field-naming.md). The tasks
they serve are in
[How to import tables typed by people](howto-import-tables-typed-by-people.md).

## The problem

Take a bank export whose column for the booking date is titled `DATUM `, with
a trailing space, and a struct field tagged `col:"Datum"`. Exact matching
finds no field for the column. Without `requiredCols`, `ViewToStructSlice`
skips columns it has no field for, so the import succeeds with every date
empty, and nothing reports it.

Making the matching loose fixes that file and creates new failure modes:

- A file with a `Datum` and a `DATUM` column now has two candidates for one
  field. Which one wins must not depend on accidents like the column order of
  this month's export.
- Two fields titled `Name` and `name` are distinct with exact matching, but
  both match a column `NAME` with loose matching. Only one of them can be
  filled.
- A file with lines above the table has to be searched for its title row, and
  that search has to compare titles the same way the assignment does
  afterwards. Otherwise the search finds a row whose titles then match no
  field.

## The approach

### Loose matching is a function you choose

`StructFieldNaming.TitleMatcher` is a function that reports whether a title
read from a table matches the title of a struct field. `nil` keeps the exact
matching, `TrimEqual` ignores surrounding whitespace, `TrimEqualFold` also
case. Any other function can accept alternative titles, like `Umsatz` for the
field `Betrag`.

Every lookup by title goes through `ColumnTitlesMatch`, which calls
`TitleMatcher`: `ViewToStructSlice` and its `requiredCols`, the `csvtable`
`Read*ToStructSlice` functions, `ColumnStructFieldValue`, `FindTitleRow`,
`DetectTableBounds` and `CheckUniqueColumns`. One function decides what
matches for the search of the title row and for the assignment of the
columns, so the two can't disagree.

The function takes the table title first and the field title second, so it
does not have to be symmetric. A function that accepts `Konto-Nr.` for the
field `Konto` does not have to accept `Konto` for a field `Konto-Nr.`.

### The closest match wins

When several columns match one field, they are ranked by how close their
title is to the field title:

```
rank 1  equal                                   "Konto"
rank 2  equal without surrounding whitespace    " Konto"
rank 3  equal without whitespace and case       "KONTO"
rank 4  only accepted by TitleMatcher           "Konto-Nr."
```

`TitleMatcher` decides whether a title matches at all, and the rank only
orders the titles it accepted. With `TrimEqual` there is no rank 3, and with
exact matching every match is rank 1.

The ranks make the result independent of the column order. A field title
that fits a column exactly is never taken over by a looser match, so adding
`TrimEqualFold` to a naming never changes which column fills a field in a
file that also has the exact title.

The rank is decided once per table, from the titles, not per row. A rank 3
column is not read in the rows where the rank 1 column is empty. The titles
tell which column is meant. The content of a single row does not.

### Equally close columns are decided row by row

Two columns of the same name, or `KONTO` and `konto` for the field `Konto`,
have the same rank. Some exports fill one of them in some rows and the other
in other rows. For these columns the decision is made per row: the cell with
content wins over an empty one, where empty means null-like or only
whitespace. If several cells have content, the last column wins, or the first
one with `PreferFirstColumn`.

Only the winning cell is converted and validated. An invalid value in a
column that lost does not fail the import, because the import never used it.

### Duplicates are caught before the first row

`CheckUniqueColumns` reports two fields whose titles match each other with the
naming's `TitleMatcher`. `ViewToStructSlice` runs it before the first row for
a naming with a `TitleMatcher`, and `FindTitleRow` runs it always. A struct
that would leave a field empty for every file fails on the first file, with
an error that names both fields.

The struct and the naming are both fixed when the program is compiled, so the
check belongs in a test, where it fails before any customer file does.

### Two ways to find the title row

`FindTitleRow` and `DetectTableBounds` both find the column titles in rows
that also hold other lines, and both compare with the naming's
`TitleMatcher`. They answer different questions:

```
FindTitleRow(rows, Account{})          DetectTableBounds(rows, "Konto", "Name")

  is every title of the struct           which rows form the table, and
  in some row?                           which of them holds the titles?

  first row with all titles,             majority column count first,
  any column count                       then the row of that count with
                                         the most titles
  -> titleRow, cell index per title      -> TitleRow, EndRow, NumColumns
  -> ErrTitleRowNotFound if one          -> still found with a title
     title is missing                       missing
```

`FindTitleRow` rejects a file that lacks a column, and returns the cell of
each title for a `FilteredView`. `DetectTableBounds` reads a file that lacks a
column, with that field empty, and also finds where the table ends. The
`csvtable` readers use `DetectTableBounds`, because CSV exports put lines with
fewer cells around the table, and the column count tells those lines apart
from the table.

Both are methods of `StructFieldNaming` in the root package and take
`[][]string`, so rows from a CSV file, a spreadsheet or any other source find
their table with the same rules.

## Trade-offs

**Exact matching stays the default.** Loose matching would read more files
without configuration, but it would also change which field a column fills
in existing programs, for example for a struct with fields titled `Name` and
`name`. Programs that read files typed by people have to opt in with one
`WithTitleMatcher` call.

**The last column wins by default.** Before ranks existed, `ViewToStructSlice`
assigned every matching column in turn, so the last one won. Keeping the last
column as the default reads those tables the same way, except that an empty
cell no longer overwrites a filled one. `PreferFirstColumn` exists for exports
that put the column that matters first.

**A rule decides instead of an error.** A file with two candidate columns is
read, not rejected, because the rules give a predictable result and most such
files are harmless. Importers for which a wrong guess costs more than a
rejected file set `DisallowAmbiguousTitles` and get `ErrAmbiguousColumnTitle`,
whose message names the columns, so it can be shown to the person who made
the file.

**Equal field titles pass without a `TitleMatcher`.** `ViewToStructSlice` only
runs `CheckUniqueColumns` for a naming with a `TitleMatcher`. Exact namings
with two fields of the same title were accepted before, filling the first
field, and still are. `FindTitleRow` is new and checks every naming.

**The rank ignores the row content.** A per-row choice across ranks would fill
a field from `KONTO` where `Konto` is empty, but a field would then be read
from different columns in different rows, chosen by content. That is harder to
explain to the person who made the file than one column per field.

**`csvtable.DetectTableBounds` keeps its own matching.** The function existed
before the naming could do the detection, and compared titles trimmed and
without case. It now calls the method of a naming with `TrimEqualFold`, so its
callers see no change. The `Read*ToStructSlice` functions instead use the
naming passed to them. An exact naming therefore no longer finds a title row
that matches only without case, and a title row that only a custom
`TitleMatcher` accepts is found now. A file that was read without an error
before is read the same.

## What it replaced

**Every matching column in turn.** In v1.0.0-beta.3 and before, `ViewToStructSlice`
looked up the field for every cell and assigned every column that matched it,
in column order. The last column won, even with an empty cell, and every cell
was converted, so an invalid value in a column that lost still failed the
import. The result depended on the column order of the export.

**A field lookup per cell.** Deciding the columns of every field once per
table, before the first row, lets the ranks compare all columns of a field and
lets the duplicate and ambiguity checks fail before any row is read. It also
makes `ViewToStructSlice` about four times faster for a table of 1,000 rows
and 20 columns.

**Title row detection only in `csvtable`.** The detection of the table between
header and trailer lines used to be a `csvtable` function that always compared
trimmed and without case. It moved to the root package as a method of the
naming, so that spreadsheet rows can use it and so that it compares titles
like the assignment does.

## Related

- [StructFieldNaming reference](reference-struct-field-naming.md): every
  method, the ranks in a table, and the errors
- [How to import tables typed by people](howto-import-tables-typed-by-people.md):
  recipes for CSV exports and Excel sheets
- [csvtable reference: Finding the table](../csvtable/reference.md#finding-the-table):
  how the `Read*ToStructSlice` functions use `DetectTableBounds`
- [How csvtable reads malformed CSV](../csvtable/explanation-malformed-csv.md):
  the parsing that produces the rows
