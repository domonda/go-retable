package retable

import (
	"context"
	"fmt"
	"reflect"
)

type Cell struct {
	View  View
	Row   int
	Col   int
	Value reflect.Value
}

// CellFormatter is an interface for formatting reflected values as strings.
type CellFormatter interface {
	// FormatCell formats a cell as string
	// or returns a wrapped ErrNotSupported error if
	// it doesn't support formatting the value of the cell.
	// The raw result indicates if the returned string
	// is in the raw format of the table format and can be
	// used as is or if it has to be sanitized in some way.
	FormatCell(ctx context.Context, cell *Cell) (str string, raw bool, err error)
}

// CellFormatterFunc implements ValueFormatter for a function.
type CellFormatterFunc func(ctx context.Context, cell *Cell) (str string, raw bool, err error)

func (f CellFormatterFunc) FormatCell(ctx context.Context, cell *Cell) (str string, raw bool, err error) {
	return f(ctx, cell)
}

// PrintfCellFormatter implements ValueFormatter by calling
// fmt.Sprintf with this type's string value as format.
type PrintfCellFormatter string

func (format PrintfCellFormatter) FormatCell(ctx context.Context, cell *Cell) (str string, raw bool, err error) {
	return fmt.Sprintf(string(format), cell.Value.Interface()), false, nil
}

// PrintfRawCellFormatter implements ValueFormatter by calling
// fmt.Sprintf with this type's string value as format.
// The result will be indicated to be a raw value.
type PrintfRawCellFormatter string

func (format PrintfRawCellFormatter) FormatCell(ctx context.Context, cell *Cell) (str string, raw bool, err error) {
	return fmt.Sprintf(string(format), cell.Value.Interface()), true, nil
}

// RawCellString implements ValueFormatter by returning
// the underlying string as raw value.
type RawCellString string

func (rawStr RawCellString) FormatCell(ctx context.Context, cell *Cell) (str string, raw bool, err error) {
	return string(rawStr), true, nil
}
