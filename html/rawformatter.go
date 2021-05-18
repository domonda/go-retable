package html

import (
	"context"

	"github.com/domonda/go-retable"
)

type RawFormatter interface {
	RawHTML(ctx context.Context, cell *retable.ViewCell) (string, error)
}

type RawFormatterFunc func(ctx context.Context, cell *retable.ViewCell) (string, error)

func (f RawFormatterFunc) RawHTML(ctx context.Context, cell *retable.ViewCell) (string, error) {
	return f(ctx, cell)
}
