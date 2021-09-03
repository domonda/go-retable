package htmltable

import (
	"context"
	"html/template"

	"github.com/domonda/go-retable"
)

var (
	_ RawFormatter = RawFormatterFunc(nil)
	_ RawFormatter = Raw("")
)

type RawFormatter interface {
	RawHTML(ctx context.Context, cell *retable.ViewCell) (template.HTML, error)
}

type RawFormatterFunc func(ctx context.Context, cell *retable.ViewCell) (template.HTML, error)

func (f RawFormatterFunc) RawHTML(ctx context.Context, cell *retable.ViewCell) (template.HTML, error) {
	return f(ctx, cell)
}

type Raw string

func (r Raw) RawHTML(ctx context.Context, cell *retable.ViewCell) (template.HTML, error) {
	return template.HTML(r), nil
}
