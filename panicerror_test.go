package retable

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// panicAndRecover panics with value and returns
// the PanicError created by the deferred recover.
func panicAndRecover(value any) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = NewPanicError("retable", p)
		}
	}()
	panic(value)
}

func TestPanicError(t *testing.T) {
	tests := []struct {
		name       string
		value      any
		wantMsg    string
		wantUnwrap error
	}{
		{name: "string", value: "boom", wantMsg: "retable: panic: boom"},
		{name: "int", value: 42, wantMsg: "retable: panic: 42"},
		{name: "error", value: io.ErrUnexpectedEOF, wantMsg: "retable: panic: unexpected EOF", wantUnwrap: io.ErrUnexpectedEOF},
		{name: "nil value", value: nil, wantMsg: "retable: panic: <nil>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewPanicError("retable", tt.value)
			require.Equal(t, tt.wantMsg, err.Error())
			require.Equal(t, tt.wantUnwrap, err.Unwrap())
			require.NotContains(t, err.Error(), "goroutine", "the message has no stack trace")
			require.Contains(t, string(err.Stack), "goroutine ")
			if tt.wantUnwrap != nil {
				require.ErrorIs(t, err, tt.wantUnwrap)
			}
		})
	}
}

// TestPanicError_StackOfThePanic covers that a PanicError created in the
// deferred function that recovered shows where the panic happened, and
// that it is found with errors.As through errors wrapping it.
func TestPanicError_StackOfThePanic(t *testing.T) {
	err := fmt.Errorf("sheet %q: %w", "Invoices", panicAndRecover(io.EOF))
	require.EqualError(t, err, `sheet "Invoices": retable: panic: EOF`)
	require.ErrorIs(t, err, io.EOF)

	var panicErr *PanicError
	require.True(t, errors.As(err, &panicErr))
	require.Equal(t, "retable", panicErr.Package)
	require.Equal(t, io.EOF, panicErr.Value)
	require.Contains(t, string(panicErr.Stack), "panic(", "stack trace of the panic")
	require.Contains(t, string(panicErr.Stack), "retable.panicAndRecover", "the frame that panicked")
}
