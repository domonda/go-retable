package csvtable

import (
	"io"
	"reflect"

	"github.com/domonda/go-retable"
)

// ReadStringsToStructSlice converts parsed CSV rows into a slice of structs.
//
// When requiredCols are passed they are used to locate the row with the column
// titles via DetectTableBounds, so header and trailer lines around the table
// are not mistaken for column titles or data. Without requiredCols the first
// non-empty row is used as column titles.
//
// ReadStringsToStructSlice never panics: a panic while converting the rows,
// including one from the passed dstScanner, parser, srcFormatter or validate,
// is returned as an error with the prefix "csvtable: panic".
func ReadStringsToStructSlice[T any](rows [][]string, naming *retable.StructFieldNaming, dstScanner retable.Scanner, parser retable.Parser, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) (slice []T, err error) {
	defer recoverParsePanic(&err)

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

// ReadBytesWithFormatToStructSlice parses csvData with the passed format
// using ParseWithFormat and converts the rows into a slice of structs
// using ReadStringsToStructSlice.
//
// ReadBytesWithFormatToStructSlice never panics: a panic while parsing or
// converting, including one from the passed dstScanner, parser, srcFormatter
// or validate, is returned as an error with the prefix "csvtable: panic".
func ReadBytesWithFormatToStructSlice[T any](csvData []byte, format *Format, naming *retable.StructFieldNaming, dstScanner retable.Scanner, parser retable.Parser, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) (slice []T, err error) {
	defer recoverParsePanic(&err)

	rows, err := ParseWithFormat(csvData, format)
	if err != nil {
		return nil, err
	}
	return ReadStringsToStructSlice[T](rows, naming, dstScanner, parser, srcFormatter, validate, requiredCols...)
}

// ReadFileWithFormatToStructSlice reads all data from csvReader and passes
// it to ReadBytesWithFormatToStructSlice.
//
// ReadFileWithFormatToStructSlice never panics: a panic while reading,
// parsing or converting, including one from the passed csvReader, dstScanner,
// parser, srcFormatter or validate, is returned as an error with the prefix
// "csvtable: panic".
func ReadFileWithFormatToStructSlice[T any](csvReader io.Reader, format *Format, naming *retable.StructFieldNaming, dstScanner retable.Scanner, parser retable.Parser, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) (slice []T, err error) {
	defer recoverParsePanic(&err)

	data, err := io.ReadAll(csvReader)
	if err != nil {
		return nil, err
	}
	return ReadBytesWithFormatToStructSlice[T](data, format, naming, dstScanner, parser, srcFormatter, validate, requiredCols...)
}

// ReadBytesDetectFormatToStructSlice parses csvData with ParseDetectFormat
// and converts the rows into a slice of structs using ReadStringsToStructSlice.
// The detected format is returned even when the conversion fails.
//
// ReadBytesDetectFormatToStructSlice never panics: a panic while parsing or
// converting, including one from the passed dstScanner, parser, srcFormatter
// or validate, is returned as an error with the prefix "csvtable: panic".
func ReadBytesDetectFormatToStructSlice[T any](csvData []byte, detectConfig *FormatDetectionConfig, naming *retable.StructFieldNaming, dstScanner retable.Scanner, parser retable.Parser, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) (slice []T, format *Format, err error) {
	defer recoverParsePanic(&err)

	rows, format, err := ParseDetectFormat(csvData, detectConfig)
	if err != nil {
		return nil, format, err
	}
	slice, err = ReadStringsToStructSlice[T](rows, naming, dstScanner, parser, srcFormatter, validate, requiredCols...)
	return slice, format, err
}

// ReadFileDetectFormatToStructSlice reads all data from csvReader and passes
// it to ReadBytesDetectFormatToStructSlice.
//
// ReadFileDetectFormatToStructSlice never panics: a panic while reading,
// parsing or converting, including one from the passed csvReader, dstScanner,
// parser, srcFormatter or validate, is returned as an error with the prefix
// "csvtable: panic".
func ReadFileDetectFormatToStructSlice[T any](csvReader io.Reader, detectConfig *FormatDetectionConfig, naming *retable.StructFieldNaming, dstScanner retable.Scanner, parser retable.Parser, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) (slice []T, format *Format, err error) {
	defer recoverParsePanic(&err)

	data, err := io.ReadAll(csvReader)
	if err != nil {
		return nil, nil, err
	}
	return ReadBytesDetectFormatToStructSlice[T](data, detectConfig, naming, dstScanner, parser, srcFormatter, validate, requiredCols...)
}
