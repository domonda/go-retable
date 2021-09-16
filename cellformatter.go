package retable

import (
	"context"
	"errors"
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

// LayoutFormatter formats any type that implements
// interface{ Format(string) string } like time.Time
// by calling the Format method
// with the string value of LayoutFormatter.
type LayoutFormatter string

func (f LayoutFormatter) FormatCell(ctx context.Context, cell *Cell) (str string, raw bool, err error) {
	formatter, ok := cell.Value.Interface().(interface{ Format(string) string })
	if !ok {
		return "", false, fmt.Errorf("%s does not implement interface{ Format(string) string }", cell.Value.Type())
	}
	return formatter.Format(string(f)), false, nil
}

func ReflectCellFormatterFunc(function interface{}, rawResult bool) (formatter CellFormatterFunc, valType reflect.Type, err error) {
	fv := reflect.ValueOf(function)
	if !fv.IsValid() {
		return nil, nil, errors.New("nil function")
	}
	ft := fv.Type()
	if ft.Kind() != reflect.Func {
		return nil, nil, fmt.Errorf("expected function but got %T", function)
	}
	if ft.NumOut() == 0 {
		return nil, nil, errors.New("function needs result")
	}
	if ft.NumOut() > 2 {
		return nil, nil, errors.New("function must not have more than 2 results")
	}
	if ft.Out(0).Kind() != reflect.String {
		return nil, nil, fmt.Errorf("function result must be a string kind, but is %s", ft.Out(0))
	}
	if ft.NumOut() == 2 && ft.Out(1) != typeOfError {
		return nil, nil, fmt.Errorf("second function result must be error, but is %s", ft.Out(1))
	}
	var (
		ctxIndex  = -1
		valIndex  = -1
		cellIndex = -1
		errIndex  = -1
	)
	if ft.NumOut() == 2 {
		errIndex = 1
	}
	for i := 0; i < ft.NumIn(); i++ {
		switch ft.In(i) {
		case typeOfContext:
			if ctxIndex != -1 {
				return nil, nil, errors.New("second context.Context argument not allowed")
			}
			ctxIndex = i
		case typeOfCellPtr:
			if cellIndex != -1 {
				return nil, nil, errors.New("second retable.Cell argument not allowed")
			}
			cellIndex = i
		default:
			if valIndex != -1 {
				return nil, nil, errors.New("too many arguments")
			}
			valIndex = i
			valType = ft.In(i)
		}
	}
	if valIndex == -1 {
		return nil, nil, errors.New("no cell value argument")
	}

	formatter = func(ctx context.Context, cell *Cell) (str string, raw bool, err error) {
		args := make([]reflect.Value, ft.NumIn())
		if ctxIndex != -1 {
			args[ctxIndex] = reflect.ValueOf(ctx)
		}
		if cellIndex != -1 {
			args[cellIndex] = reflect.ValueOf(cell)
		}
		args[valIndex] = cell.Value
		res := fv.Call(args)
		if errIndex != -1 && !res[errIndex].IsNil() {
			return "", false, res[errIndex].Interface().(error)
		}
		return res[0].String(), rawResult, nil
	}

	return formatter, valType, nil
}
