package retable

import (
	"context"
	"fmt"
	"reflect"
)

// ValueFormatter is an interface for formatting reflected values.
type ValueFormatter interface {
	FormatValue(ctx context.Context, val reflect.Value, cell *ViewCell) (string, error)
}

// ValueFormatterFunc implements ValueFormatter for a function.
type ValueFormatterFunc func(ctx context.Context, val reflect.Value, cell *ViewCell) (string, error)

func (f ValueFormatterFunc) FormatValue(ctx context.Context, val reflect.Value, cell *ViewCell) (string, error) {
	return f(ctx, val, cell)
}

// PrintfValueFormatter implements ValueFormatter by calling
// fmt.Sprintf with this type's string value as format.
type PrintfValueFormatter string

func (f PrintfValueFormatter) FormatValue(ctx context.Context, val reflect.Value, cell *ViewCell) (string, error) {
	return fmt.Sprintf(string(f), val.Interface()), nil
}
