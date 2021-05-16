package csv

import (
	"context"
	"reflect"

	"github.com/domonda/go-retable"
)

type Formatter interface {
	FormatCSV(ctx context.Context, val reflect.Value, row, col int, view retable.View) (string, error)
}

type FormatterFunc func(ctx context.Context, val reflect.Value, row, col int, view retable.View) (string, error)

func (f FormatterFunc) FormatCSV(ctx context.Context, val reflect.Value, row, col int, view retable.View) (string, error) {
	return f(ctx, val, row, col, view)
}
