package retable

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// CellFormatter is an interface for formatting view cells as strings.
type CellFormatter interface {
	// FormatCell formats the view cell at a row/col position as string
	// or returns a wrapped errors.ErrUnsupported error if
	// it doesn't support formatting the value of the cell.
	// The raw result indicates if the returned string
	// is in the raw format of the table format and can be
	// used as is or if it has to be sanitized in some way.
	FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error)
}

// CellFormatterFunc implements CellFormatter for a function.
type CellFormatterFunc func(ctx context.Context, view View, row, col int) (str string, raw bool, err error)

func (f CellFormatterFunc) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	return f(ctx, view, row, col)
}

// PrintfCellFormatter implements CellFormatter by calling
// fmt.Sprintf with this type's string value as format.
type PrintfCellFormatter string

func (format PrintfCellFormatter) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	return fmt.Sprintf(string(format), view.AnyValue(row, col)), false, nil
}

// PrintfRawCellFormatter implements CellFormatter by calling
// fmt.Sprintf with this type's string value as format.
// The result will be indicated to be a raw value.
type PrintfRawCellFormatter string

func (format PrintfRawCellFormatter) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	return fmt.Sprintf(string(format), view.AnyValue(row, col)), true, nil
}

// SprintCellFormatter returns a CellFormatter
// that formats a cell's value using fmt.Sprint
// and returns the result together with the rawResult argument.
func SprintCellFormatter(rawResult bool) CellFormatter {
	return CellFormatterFunc(func(ctx context.Context, view View, row, col int) (string, bool, error) {
		return fmt.Sprint(view.AnyValue(row, col)), rawResult, nil
	})
}

// UnsupportedCellFormatter is a CellFormatter that always returns errors.ErrUnsupported.
type UnsupportedCellFormatter struct{}

func (UnsupportedCellFormatter) Format(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	return "", false, errors.ErrUnsupported
}

// SprintRawCellFormatter returns a CellFormatter
// that tries the passed formatters in order
// until they return no error or a non errors.ErrUnsupported error.
// If all formatters return errors.ErrUnsupported
// then fmt.Sprint is used as fallback or
// an empty string returned for nil.
// In case of the fallback the raw bool is always false.
func TryFormattersOrSprint(formatters ...CellFormatter) CellFormatter {
	return CellFormatterFunc(func(ctx context.Context, view View, row, col int) (string, bool, error) {
		for _, f := range formatters {
			str, raw, err := f.FormatCell(ctx, view, row, col)
			if !errors.Is(err, errors.ErrUnsupported) {
				return str, raw, err
			}
		}

		// Fallback for no formatters passed or when
		// all formatters returned errors.ErrUnsupported
		v := view.ReflectValue(row, col)
		if IsNullLike(v) {
			return "", false, nil
		}
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		return fmt.Sprint(v.Interface()), false, nil
	})
}

// RawCellString implements CellFormatter by returning
// the underlying string as raw value.
type RawCellString string

func (rawStr RawCellString) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	return string(rawStr), true, nil
}

// LayoutFormatter formats any type that implements
// interface{ Format(string) string } like time.Time
// by calling the Format method
// with the string value of LayoutFormatter.
type LayoutFormatter string

func (f LayoutFormatter) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	formatter, ok := view.AnyValue(row, col).(interface{ Format(string) string })
	if !ok {
		return "", false, fmt.Errorf("%T does not implement interface{ Format(string) string }", view.AnyValue(row, col))
	}
	return formatter.Format(string(f)), false, nil
}

// StringIfTrue formats bool cells by
// returning the underlying string as non-raw value
// for true and an empty string as non-raw value for false.
type StringIfTrue string

func (f StringIfTrue) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	if view.ReflectValue(row, col).Bool() {
		return string(f), false, nil
	}
	return "", false, nil
}

// RawStringIfTrue formats bool cells by
// returning the underlying string as raw value
// for true and an empty string as raw value for false.
type RawStringIfTrue string

func (f RawStringIfTrue) FormatCell(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
	if view.ReflectValue(row, col).Bool() {
		return string(f), true, nil
	}
	return "", true, nil
}

// ReflectCellFormatterFunc uses reflection to convert the passed function
// into a CellFormatterFunc.
// The function can have zero to two arguments and one or two results.
// In case of two arguments the first argument must be of type context.Context.
// The first result must be of type string and the optional second result of type error.
// The returned CellFormatterFunc will return the passed rawResult argument
// as raw result value.
func ReflectCellFormatterFunc(function any, rawResult bool) (formatter CellFormatterFunc, valType reflect.Type, err error) {
	// Check if function is really a function
	fv := reflect.ValueOf(function)
	if !fv.IsValid() {
		return nil, nil, errors.New("nil function")
	}
	ft := fv.Type()
	if ft.Kind() != reflect.Func {
		return nil, nil, fmt.Errorf("expected function but got %T", function)
	}

	// Check results of function
	if ft.NumOut() == 0 {
		return nil, nil, errors.New("function needs result")
	}
	if ft.NumOut() > 2 {
		return nil, nil, errors.New("function must not have more than 2 results")
	}
	if ft.Out(0).Kind() != reflect.String {
		return nil, nil, fmt.Errorf("function result must be a string kind, but is %s", ft.Out(0))
	}
	errIndex := -1
	if ft.NumOut() == 2 {
		if ft.Out(1) != typeOfError {
			return nil, nil, fmt.Errorf("second function result must be error, but is %s", ft.Out(1))
		}
		errIndex = 1
	}

	// Check arguments of function
	var (
		ctxIndex = -1
		valIndex = -1
	)
	for i := 0; i < ft.NumIn(); i++ {
		switch ft.In(i) {
		case typeOfContext:
			if ctxIndex != -1 {
				return nil, nil, errors.New("second context.Context argument not allowed")
			}
			ctxIndex = i
		default:
			if valIndex != -1 {
				return nil, nil, errors.New("too many arguments")
			}
			valIndex = i
			valType = ft.In(i)
		}
	}

	formatter = func(ctx context.Context, view View, row, col int) (str string, raw bool, err error) {
		if err = ctx.Err(); err != nil {
			return "", false, err
		}
		args := make([]reflect.Value, ft.NumIn())
		if ctxIndex != -1 {
			args[ctxIndex] = reflect.ValueOf(ctx)
		}
		if valIndex != -1 {
			args[valIndex] = view.ReflectValue(row, col)
		}
		res := fv.Call(args)
		if errIndex != -1 && !res[errIndex].IsNil() {
			return "", false, res[errIndex].Interface().(error)
		}
		return res[0].String(), rawResult, nil
	}

	return formatter, valType, nil
}
