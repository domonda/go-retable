package csvtable

import (
	"reflect"

	"github.com/domonda/go-retable"
	"github.com/ungerik/go-fs"
)

func ReadStringsToStructSlice[T any](rows [][]string, naming *retable.StructFieldNaming, dstScanner retable.Scanner, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) ([]T, error) {
	rows = RemoveEmptyRows(rows)
	return retable.ViewToStructSlice[T](
		retable.NewStringsView("", rows),
		naming,
		dstScanner,
		srcFormatter,
		validate,
		requiredCols...,
	)
}

func ReadBytesWithFormatToStructSlice[T any](csvData []byte, format *Format, naming *retable.StructFieldNaming, dstScanner retable.Scanner, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) ([]T, error) {
	rows, err := ParseWithFormat(csvData, format)
	if err != nil {
		return nil, err
	}
	return ReadStringsToStructSlice[T](rows, naming, dstScanner, srcFormatter, validate, requiredCols...)
}

func ReadFileWithFormatToStructSlice[T any](csvFile fs.FileReader, format *Format, naming *retable.StructFieldNaming, dstScanner retable.Scanner, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) ([]T, error) {
	data, err := csvFile.ReadAll()
	if err != nil {
		return nil, err
	}
	return ReadBytesWithFormatToStructSlice[T](data, format, naming, dstScanner, srcFormatter, validate, requiredCols...)
}

func ReadBytesDetectFormatToStructSlice[T any](csvData []byte, detectConfig *FormatDetectionConfig, naming *retable.StructFieldNaming, dstScanner retable.Scanner, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) ([]T, *Format, error) {
	rows, format, err := ParseDetectFormat(csvData, detectConfig)
	if err != nil {
		return nil, format, err
	}
	slice, err := ReadStringsToStructSlice[T](rows, naming, dstScanner, srcFormatter, validate, requiredCols...)
	return slice, format, err
}

func ReadFileDetectFormatToStructSlice[T any](csvFile fs.FileReader, detectConfig *FormatDetectionConfig, naming *retable.StructFieldNaming, dstScanner retable.Scanner, srcFormatter retable.Formatter, validate func(reflect.Value) error, requiredCols ...string) ([]T, *Format, error) {
	data, err := csvFile.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	return ReadBytesDetectFormatToStructSlice[T](data, detectConfig, naming, dstScanner, srcFormatter, validate, requiredCols...)
}
