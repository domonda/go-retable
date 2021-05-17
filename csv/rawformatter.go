package csv

import (
	"context"
	"strings"

	"github.com/domonda/go-retable"
)

type RawFormatter interface {
	RawCSV(ctx context.Context, cell *retable.ViewCell) (string, error)
}

type RawFormatterFunc func(ctx context.Context, cell *retable.ViewCell) (string, error)

func (f RawFormatterFunc) RawCSV(ctx context.Context, cell *retable.ViewCell) (string, error) {
	return f(ctx, cell)
}

func EscapeQuotes(val string) string {
	return strings.ReplaceAll(val, `"`, `""`)
}
