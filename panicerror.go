package retable

import (
	"fmt"
	"runtime/debug"
)

// PanicError is a recovered panic returned as an error.
//
// SmartAssign, and with it ViewToStructSlice, recovers panics of package
// reflect and of the Scanner, Parser and Formatter passed to it, but not
// of the validate function of ViewToStructSlice. The parse and read
// functions of csvtable and the write functions of exceltable recover
// any panic inside them, also of a validate function or a CellFormatter.
// They return a PanicError instead, so that one broken cell can't take
// the whole program down.
//
// The message is short and has no stack trace, so it can be shown to
// a user. The stack trace that locates the bug is in Stack. Get it with
// errors.As to log it:
//
//	rows, err := csvtable.ReadBytesWithFormatToStructSlice[Row](data, format, naming, nil, nil, nil, nil)
//	var panicErr *retable.PanicError
//	if errors.As(err, &panicErr) {
//	    log.Printf("%v\n%s", err, panicErr.Stack)
//	}
type PanicError struct {
	// Package is the name of the package that recovered the panic,
	// like "csvtable", which starts the error message.
	Package string

	// Value is the value that panic was called with.
	Value any

	// Stack is the stack trace of the panicking goroutine
	// as formatted by runtime/debug.Stack.
	Stack []byte
}

// NewPanicError returns a PanicError for the value recovered from a panic
// by the named package, with the stack trace of the current goroutine.
//
// Call it in the deferred function that called recover, where the stack
// of the panic is not yet unwound, otherwise Stack does not show
// where the panic happened.
//
// Example:
//
//	func Convert(row []string) (err error) {
//	    defer func() {
//	        if p := recover(); p != nil {
//	            err = retable.NewPanicError("mypackage", p)
//	        }
//	    }()
//	    ...
//	}
func NewPanicError(packageName string, value any) *PanicError {
	return &PanicError{
		Package: packageName,
		Value:   value,
		Stack:   debug.Stack(),
	}
}

// Error returns the package name followed by ": panic: " and the panic
// value, like "csvtable: panic: runtime error: index out of range".
// It does not include the stack trace.
func (e *PanicError) Error() string {
	return fmt.Sprintf("%s: panic: %v", e.Package, e.Value)
}

// Unwrap returns the panic value if it is an error, so that
// errors.Is and errors.As find it, or nil otherwise.
func (e *PanicError) Unwrap() error {
	err, _ := e.Value.(error)
	return err
}
