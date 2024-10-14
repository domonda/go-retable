package csvtable

import (
	"strings"
)

// SetRowsWithNonUniformColumnsNil set rows to nil that don't have the same field count as the majority of rows,
// so every rows is either nil or has the same number of fields.
func SetRowsWithNonUniformColumnsNil(rows [][]string) [][]string {
	if len(rows) == 0 {
		return nil
	}

	result := make([][]string, len(rows))

	// map from number of columns to number of rows with that column
	rowColumnsCount := make(map[int]int)
	for _, row := range rows {
		if rowColumns := len(row); rowColumns > 1 {
			rowColumnsCount[rowColumns]++
		}
	}
	majorityRowColumns := 0
	highestRowCount := 0
	for rowColumns, rowCount := range rowColumnsCount {
		if rowCount > highestRowCount || (rowCount == highestRowCount && rowColumns > majorityRowColumns) {
			majorityRowColumns = rowColumns
			highestRowCount = rowCount
		}
	}
	for i, row := range rows {
		if len(row) == majorityRowColumns {
			result[i] = row
		}
	}

	return result
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
	if len(str) < 3 {
		return str, false
	}

	// First check if every odd indexed rune is a space.
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

func ReplaceNewlineWithSpacefunc(rows [][]string) {
	for _, row := range rows {
		for col, field := range row {
			row[col] = strings.ReplaceAll(field, "\n", " ")
		}
	}
}
