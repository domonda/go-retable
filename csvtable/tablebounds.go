package csvtable

import (
	"github.com/domonda/go-retable"
)

// TableBounds locates the actual table within rows that can also contain
// header lines before and trailer lines after it, as produced by exports
// that put a title, a period, or a total around the table.
//
// It is the type of the root package, where the detection lives
// so that every table format can use it.
type TableBounds = retable.TableBounds

// trimEqualFoldTitles compares column titles
// without surrounding whitespace and case.
var trimEqualFoldTitles = &retable.StructFieldNaming{TitleMatcher: retable.TrimEqualFold}

// DetectTableBounds locates the table within rows that can also contain
// header lines before and trailer lines after it, comparing expectedTitles
// with the cells without leading and trailing whitespace and without case.
//
// It calls retable.StructFieldNaming.DetectTableBounds with
// retable.TrimEqualFold as TitleMatcher, see there for how the table
// is found. Call that method of your own naming to compare the titles
// the way it does, which is what the Read*ToStructSlice functions do.
func DetectTableBounds(rows [][]string, expectedTitles ...string) TableBounds {
	return trimEqualFoldTitles.DetectTableBounds(rows, expectedTitles...)
}
