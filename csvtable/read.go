package csvtable

import (
	"reflect"

	"github.com/domonda/go-retable"
	"github.com/ungerik/go-fs"
)

// ReadStringsToStructSlice converts parsed CSV rows into a slice of structs.
//
// When requiredCols are passed they are used to locate the row with the column
// titles via DetectTableBounds, so header and trailer lines around the table
// are not mistaken for column titles or data. Without requiredCols the first
// non-empty row is used as column titles.
func ReadStringsToStructSlice[T any](rows [][]string, naming *retable.StructFieldNaming, dstScanner retable.Scanner, parser retable.Parser, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) ([]T, error) {
	if len(requiredCols) > 0 {
		// Only cut the table out when there are titles to find it by,
		// guessing the column titles row without them would change
		// the result for callers that pass a table without header lines.
		if bounds := DetectTableBounds(rows, requiredCols...); bounds.Valid() {
			rows = bounds.Rows(rows)
		}
	}
	rows = RemoveEmptyRows(rows)
	return retable.ViewToStructSlice[T](
		retable.NewStringsView("", rows),
		naming,
		dstScanner,
		parser,
		srcFormatter,
		validate,
		requiredCols...,
	)
}

func ReadBytesWithFormatToStructSlice[T any](csvData []byte, format *Format, naming *retable.StructFieldNaming, dstScanner retable.Scanner, parser retable.Parser, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) ([]T, error) {
	rows, err := ParseWithFormat(csvData, format)
	if err != nil {
		return nil, err
	}
	return ReadStringsToStructSlice[T](rows, naming, dstScanner, parser, srcFormatter, validate, requiredCols...)
}

func ReadFileWithFormatToStructSlice[T any](csvFile fs.FileReader, format *Format, naming *retable.StructFieldNaming, dstScanner retable.Scanner, parser retable.Parser, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) ([]T, error) {
	data, err := csvFile.ReadAll()
	if err != nil {
		return nil, err
	}
	return ReadBytesWithFormatToStructSlice[T](data, format, naming, dstScanner, parser, srcFormatter, validate, requiredCols...)
}

func ReadBytesDetectFormatToStructSlice[T any](csvData []byte, detectConfig *FormatDetectionConfig, naming *retable.StructFieldNaming, dstScanner retable.Scanner, parser retable.Parser, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) ([]T, *Format, error) {
	rows, format, err := ParseDetectFormat(csvData, detectConfig)
	if err != nil {
		return nil, format, err
	}
	slice, err := ReadStringsToStructSlice[T](rows, naming, dstScanner, parser, srcFormatter, validate, requiredCols...)
	return slice, format, err
}

func ReadFileDetectFormatToStructSlice[T any](csvFile fs.FileReader, detectConfig *FormatDetectionConfig, naming *retable.StructFieldNaming, dstScanner retable.Scanner, parser retable.Parser, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) ([]T, *Format, error) {
	data, err := csvFile.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	return ReadBytesDetectFormatToStructSlice[T](data, detectConfig, naming, dstScanner, parser, srcFormatter, validate, requiredCols...)
}
