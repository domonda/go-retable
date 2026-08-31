# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased] - 2026-08-31

### Fixed

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
