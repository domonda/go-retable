package retable

import (
	"context"
	"errors"
	"io"
	"reflect"
)

type ValueWriter interface {
	WriteValue(ctx context.Context, dest io.Writer, val reflect.Value, row, col int, view View) error
}

type ValueWriterFunc func(ctx context.Context, dest io.Writer, val reflect.Value, row, col int, view View) error

func (f ValueWriterFunc) WriteValue(ctx context.Context, dest io.Writer, val reflect.Value, row, col int, view View) error {
	return f(ctx, dest, val, row, col, view)
}

type ValueWriters struct {
	TypeWriters      map[reflect.Type]ValueWriter
	InterfaceWriters map[reflect.Type]ValueWriter
	KindWriters      map[reflect.Kind]ValueWriter
}

func (w *ValueWriters) WriteValue(ctx context.Context, dest io.Writer, val reflect.Value, row, col int, view View) error {
	if tw, ok := w.TypeWriters[val.Type()]; ok {
		err := tw.WriteValue(ctx, dest, val, row, col, view)
		if err != nil && !errors.Is(err, ErrNotSupported) {
			return err
		}
	}
	for it, iw := range w.InterfaceWriters {
		if val.Type().Implements(it) {
			err := iw.WriteValue(ctx, dest, val, row, col, view)
			if err != nil && !errors.Is(err, ErrNotSupported) {
				return err
			}
		}
	}
	if kw, ok := w.KindWriters[val.Kind()]; ok {
		err := kw.WriteValue(ctx, dest, val, row, col, view)
		if err != nil && !errors.Is(err, ErrNotSupported) {
			return err
		}
	}
	return ErrNotSupported
}
