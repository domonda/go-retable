package csv

import (
	"context"
	"strings"

	"github.com/domonda/go-retable"
)

type Formatter interface {
	FormatCSV(ctx context.Context, row, col int, view retable.View) (string, error)
}

type FormatterFunc func(ctx context.Context, row, col int, view retable.View) (string, error)

func (f FormatterFunc) FormatCSV(ctx context.Context, row, col int, view retable.View) (string, error) {
	return f(ctx, row, col, view)
}

func EscapeQuotes(val string) string {
	return strings.ReplaceAll(val, `"`, `""`)
}
