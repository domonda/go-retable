package csvtable

import "strings"

// TableBounds locates the actual table within rows that can also contain
// header lines before and trailer lines after it, as produced by exports
// that put a title, a period, or a total around the table.
//
// Use DetectTableBounds to find them and Rows to cut the table out.
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
// which is the layout expected by retable.NewStringsView,
// or nil when the bounds are not valid.
func (b TableBounds) Rows(rows [][]string) [][]string {
	if !b.Valid() || b.EndRow > len(rows) {
		return nil
	}
	return rows[b.TitleRow:b.EndRow]
}

// DetectTableBounds locates the table within rows that can also contain
// header lines before and trailer lines after it.
//
// The number of columns of the table is the one that the majority of rows has,
// see SetRowsWithNonUniformColumnsNil for how single column rows are treated.
//
// The column titles row is the row with that number of columns that matches
// the most expectedTitles, compared without leading and trailing whitespace
// and without case. A row that matches none of them is never used as titles,
// so passing the titles of the target struct makes the detection reliable.
// Without expectedTitles the first row of the table's column count whose
// fields are all non-empty is used, which can't tell a header line with as
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
func DetectTableBounds(rows [][]string, expectedTitles ...string) TableBounds {
	numColumns := majorityRowColumns(rows)
	if numColumns == 0 {
		return TableBounds{TitleRow: -1}
	}

	titleRow := -1
	if len(expectedTitles) == 0 {
		for row, fields := range rows {
			if len(fields) == numColumns && !hasEmptyField(fields) {
				titleRow = row
				break
			}
		}
	} else {
		mostMatches := 0
		for row, fields := range rows {
			if len(fields) != numColumns {
				continue
			}
			if matches := countMatchingTitles(fields, expectedTitles); matches > mostMatches {
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

func hasEmptyField(row []string) bool {
	for _, field := range row {
		if strings.TrimSpace(field) == "" {
			return true
		}
	}
	return false
}

func countMatchingTitles(row, expectedTitles []string) (matches int) {
	for _, title := range expectedTitles {
		for _, field := range row {
			if strings.EqualFold(strings.TrimSpace(field), strings.TrimSpace(title)) {
				matches++
				break
			}
		}
	}
	return matches
}
