package csvtable

import (
	"strings"
)

// SetRowsWithNonUniformColumnsNil set rows to nil that don't have the same field count as the majority of rows,
// so every rows is either nil or has the same number of fields.
//
// Single column rows don't take part in the majority vote unless no row has
// more than one column, because header and trailer lines of a table are usually
// single column rows that must not outvote the actual table rows even if there
// are more of them.
func SetRowsWithNonUniformColumnsNil(rows [][]string) [][]string {
	if len(rows) == 0 {
		return nil
	}

	result := make([][]string, len(rows))

	majority := majorityRowColumns(rows)
	for i, row := range rows {
		if len(row) == majority {
			result[i] = row
		}
	}

	return result
}

// majorityRowColumns returns the number of columns that the majority of rows
// has, or 0 when there is no row with a column. More columns win a tie.
//
// Single column rows don't take part in the vote unless no row has more than
// one column, because header and trailer lines of a table are usually single
// column rows that must not outvote the actual table rows even if there are
// more of them.
func majorityRowColumns(rows [][]string) int {
	// map from number of columns to number of rows with that many columns
	rowColumnsCount := make(map[int]int)
	for _, row := range rows {
		if rowColumns := len(row); rowColumns > 0 {
			rowColumnsCount[rowColumns]++
		}
	}
	if len(rowColumnsCount) > 1 {
		delete(rowColumnsCount, 1)
	}
	majority := 0
	highestRowCount := 0
	for rowColumns, rowCount := range rowColumnsCount {
		if rowCount > highestRowCount || (rowCount == highestRowCount && rowColumns > majority) {
			majority = rowColumns
			highestRowCount = rowCount
		}
	}
	return majority
}

// SetEmptyRowsNil sets rows to nil,
// where all columns are empty strings.
func SetEmptyRowsNil(rows [][]string) [][]string {
	if len(rows) == 0 {
		return nil
	}

	result := make([][]string, len(rows))
	for i, row := range rows {
		rowIsEmpty := true
		for _, field := range row {
			if field != "" {
				rowIsEmpty = false
				break
			}
		}
		if !rowIsEmpty {
			result[i] = row
		}
	}

	return result
}

// RemoveEmptyRows removes rows without columns,
// or rows where all columns are empty strings.
func RemoveEmptyRows(rows [][]string) [][]string {
	if len(rows) == 0 {
		return nil
	}
	var (
		hasEmptyRows bool
		nonEmptyRows [][]string
	)
	for i, row := range rows {
		rowIsEmpty := true
		for _, field := range row {
			if field != "" {
				rowIsEmpty = false
				break
			}
		}
		if rowIsEmpty {
			if !hasEmptyRows {
				if i > 0 {
					nonEmptyRows = append(nonEmptyRows, rows[:i]...)
				}
				hasEmptyRows = true
			}
		} else {
			if hasEmptyRows {
				nonEmptyRows = append(nonEmptyRows, row)
			}
		}
	}
	if !hasEmptyRows {
		// Nothing removed, return original rows
		return rows
	}
	return nonEmptyRows
}

// CompactSpacedStrings removes spaces if they are between every other character,
// meaning that every odd character index is a space.
func CompactSpacedStrings(rows [][]string) (numModified int) {
	for _, row := range rows {
		for col, field := range row {
			cleaned, modified := compactSpacedString(field)
			if modified {
				row[col] = cleaned
				numModified++
			}
		}
	}
	return numModified
}

// compactSpacedString removes spaces if they are between every other character,
// meaning that every odd character index is a space.
func compactSpacedString(str string) (cleaned string, modified bool) {
	// First check if every odd indexed rune is a space.
	// Count runes and not bytes because the loops here index runes,
	// else a multi-byte string would be compacted where the same
	// string with single-byte characters would not be. Counting in
	// this loop instead of up front keeps the early return for the
	// common string that is not spaced out.
	numSpaces := 0
	i := 0 // Don't use index from range over string because it counts bytes not UTF-8 runes
	for _, r := range str {
		if i&1 == 1 {
			if r != ' ' {
				return str, false
			}
			numSpaces++
		}
		i++
	}
	if i < 3 {
		return str, false
	}

	b := strings.Builder{}
	b.Grow(len(str) - numSpaces)
	i = 0
	for _, r := range str {
		if i&1 == 0 {
			b.WriteRune(r)
		}
		i++
	}
	return b.String(), true
}

// newlineReplacer replaces all newline variants with a single space.
// \r\n is listed first because Replacer matches the pairs in argument
// order, which keeps a Windows newline from becoming two spaces.
var newlineReplacer = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")

// ReplaceNewlineWithSpace replaces all newlines in all fields with a space.
func ReplaceNewlineWithSpace(rows [][]string) {
	for _, row := range rows {
		for col, field := range row {
			row[col] = newlineReplacer.Replace(field)
		}
	}
}

// ReplaceNewlineWithSpacefunc replaces all newlines in all fields with a space.
//
// Deprecated: use [ReplaceNewlineWithSpace], this alias only exists
// because the misspelled name is part of the published API.
func ReplaceNewlineWithSpacefunc(rows [][]string) {
	ReplaceNewlineWithSpace(rows)
}

// TrimSpace removes leading and trailing spaces from all fields.
func TrimSpace(rows [][]string) {
	for _, row := range rows {
		for col, field := range row {
			row[col] = strings.TrimSpace(field)
		}
	}
}
