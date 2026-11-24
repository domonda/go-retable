package retable

import (
	"strings"

	"github.com/domonda/go-retable/internal/rowcolumns"
)

// TableBounds locates the actual table within rows that can also contain
// header lines before and trailer lines after it, as produced by exports
// that put a title, a period, or a total around the table.
//
// Use StructFieldNaming.DetectTableBounds to find them
// and Rows to cut the table out.
type TableBounds struct {
	// TitleRow is the index of the row with the column titles,
	// or -1 when no table was found.
	TitleRow int

	// EndRow is the index after the last data row.
	EndRow int

	// NumColumns is the number of columns of the table.
	NumColumns int
}

// Valid reports whether the bounds describe a table.
//
// The zero value is not valid: a detected table always has an EndRow after
// its TitleRow, because the column titles row is part of it.
func (b TableBounds) Valid() bool {
	return b.TitleRow >= 0 && b.EndRow > b.TitleRow
}

// Rows returns the column titles row followed by the data rows,
// which is the layout expected by NewStringsView,
// or nil when the bounds are not valid.
func (b TableBounds) Rows(rows [][]string) [][]string {
	if !b.Valid() || b.EndRow > len(rows) {
		return nil
	}
	return rows[b.TitleRow:b.EndRow]
}

// DetectTableBounds locates the table within rows of raw cells, as parsed
// from a CSV or read from a spreadsheet, that can also contain header lines
// before and trailer lines after the table.
//
// The number of columns of the table is the one that the majority of rows
// has. Single column rows don't take part in that vote unless no row has
// more than one column, because header and trailer lines are usually
// single column rows.
//
// The column titles row is the row with that number of columns that matches
// the most expectedTitles according to ColumnTitlesMatch, so TitleMatcher
// decides how loosely, the same way as for ViewToStructSlice. The cells are
// compared without their surrounding whitespace, because NewStringsView
// trims the column titles of the view made from the rows. A row that
// matches none of expectedTitles is never used as titles, so passing the
// titles of the target struct makes the detection reliable.
// Without expectedTitles the first row of the table's column count whose
// cells are all non-empty is used, which can't tell a header line with as
// many columns as the table from the real column titles.
//
// The data rows are the rows after the column titles that have the table's
// column count, ending at the first row with a different count. Empty rows in
// between are skipped instead of ending the table, because a field containing
// a newline leaves empty rows behind while it is joined together again.
//
// A trailer row that has as many columns as the table, like a total, can't be
// told apart from a data row and is therefore included. Validate the scanned
// values to reject it.
//
// To require every column title of a struct instead of the best match,
// regardless of the number of columns of the rows, use FindTitleRow.
//
// This method is safe to call with a nil receiver,
// which compares the titles for equality.
//
// Example:
//
//	rows := [][]string{
//	    {"Kontoauszug Nr. 4"},
//	    {"DATUM", "Text", "Betrag"},
//	    {"02.01.2026", "Miete", "-500,00"},
//	    {"Erstellt am 06.01.2026"},
//	}
//	naming := DefaultStructFieldNaming.WithTitleMatcher(TrimEqualFold)
//	bounds := naming.DetectTableBounds(rows, "Datum", "Betrag")
//	// bounds == TableBounds{TitleRow: 1, EndRow: 3, NumColumns: 3}
//	view := NewStringsView("", bounds.Rows(rows))
func (n *StructFieldNaming) DetectTableBounds(rows [][]string, expectedTitles ...string) TableBounds {
	numColumns := rowcolumns.Majority(rows)
	if numColumns == 0 {
		return TableBounds{TitleRow: -1}
	}

	titleRow := -1
	if len(expectedTitles) == 0 {
		for row, cells := range rows {
			if len(cells) == numColumns && !hasBlankCell(cells) {
				titleRow = row
				break
			}
		}
	} else {
		mostMatches := 0
		for row, cells := range rows {
			if len(cells) != numColumns {
				continue
			}
			if matches := n.countTitlesMatchingCells(cells, expectedTitles); matches > mostMatches {
				mostMatches, titleRow = matches, row
				if matches == len(expectedTitles) {
					break
				}
			}
		}
	}
	if titleRow < 0 {
		return TableBounds{TitleRow: -1}
	}

	endRow := titleRow + 1
	for row := titleRow + 1; row < len(rows); row++ {
		switch {
		case len(rows[row]) == 0:
			// Empty rows are left behind by joining a field
			// containing a newline and don't end the table
		case len(rows[row]) != numColumns:
			return TableBounds{TitleRow: titleRow, EndRow: endRow, NumColumns: numColumns}
		default:
			endRow = row + 1
		}
	}
	return TableBounds{TitleRow: titleRow, EndRow: endRow, NumColumns: numColumns}
}

// hasBlankCell reports whether a cell of row is empty or only whitespace.
func hasBlankCell(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) == "" {
			return true
		}
	}
	return false
}

// countTitlesMatchingCells returns how many of titles match a cell of row
// without its surrounding whitespace according to ColumnTitlesMatch.
func (n *StructFieldNaming) countTitlesMatchingCells(row, titles []string) (matches int) {
	for _, title := range titles {
		for _, cell := range row {
			if n.ColumnTitlesMatch(strings.TrimSpace(cell), title) {
				matches++
				break
			}
		}
	}
	return matches
}
