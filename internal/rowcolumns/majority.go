// Package rowcolumns has functions for the numbers of columns
// of rows of strings that the retable packages share.
package rowcolumns

// Majority returns the number of columns that the majority of rows
// has, or 0 when there is no row with a column. More columns win a tie.
//
// Single column rows don't take part in the vote unless no row has more than
// one column, because header and trailer lines of a table are usually single
// column rows that must not outvote the actual table rows even if there are
// more of them.
func Majority(rows [][]string) int {
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
