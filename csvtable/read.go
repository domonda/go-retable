package csvtable

import (
	"reflect"

	"github.com/domonda/go-retable"
)

func AsStructSlice[T any](rows [][]string, naming *retable.StructFieldNaming, requiredCols []string, dstScanner retable.Scanner, srcFormatter retable.Formatter, validate func(reflect.Value) error) ([]T, error) {
	rows = RemoveEmptyRows(rows)
	return retable.ViewToStructSlice[T](
		retable.NewStringsView("", rows),
		naming,
		requiredCols,
		dstScanner,
		srcFormatter,
		validate,
	)
}

func ReadWithFormatAsStructSlice[T any](csv []byte, format *Format, naming *retable.StructFieldNaming, requiredCols []string, dstScanner retable.Scanner, srcFormatter retable.Formatter, validate func(reflect.Value) error) ([]T, error) {
	rows, err := ParseWithFormat(csv, format)
	if err != nil {
		return nil, err
	}
	rows = RemoveEmptyRows(rows)
	return retable.ViewToStructSlice[T](
		retable.NewStringsView("", rows),
		naming,
		requiredCols,
		dstScanner,
		srcFormatter,
		validate,
	)
}

func ReadDetectFormatAsStructSlice[T any](csv []byte, configOrNil *FormatDetectionConfig, naming *retable.StructFieldNaming, requiredCols []string, dstScanner retable.Scanner, srcFormatter retable.Formatter, validate func(reflect.Value) error) ([]T, error) {
	rows, _, err := ParseDetectFormat(csv, configOrNil)
	if err != nil {
		return nil, err
	}
	rows = RemoveEmptyRows(rows)
	return retable.ViewToStructSlice[T](
		retable.NewStringsView("", rows),
		naming,
		requiredCols,
		dstScanner,
		srcFormatter,
		validate,
	)
}
