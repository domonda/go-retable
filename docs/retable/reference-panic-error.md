# PanicError reference

`retable.PanicError` is the error for a recovered panic. The import and export
functions of go-retable recover a panic in their own code, in package
`reflect` or in a function you pass to them, and return a `*PanicError`
instead. One broken cell or formatter then fails one import, not the whole
program. The message is one short line that can be shown to a user, and the
stack trace that locates the bug is kept in a field for your log.

This page lists the type, which functions return it, and how to create one in
your own code. The generated API docs are on
[pkg.go.dev](https://pkg.go.dev/github.com/domonda/go-retable#PanicError).

## API

### Type

```go
type PanicError struct {
    Package string
    Value   any
    Stack   []byte
}
```

| Member | Meaning |
| --- | --- |
| `Package string` | The name of the package that recovered the panic, like `csvtable`. It starts the error message. |
| `Value any` | The value `panic` was called with. |
| `Stack []byte` | The stack trace of the panicking goroutine, as `runtime/debug.Stack` formats it. |
| `Error() string` | `<Package>: panic: <Value>`, formatted with `%v`, without the stack trace. |
| `Unwrap() error` | `Value` if it is an `error`, otherwise `nil`. `errors.Is` and `errors.As` therefore find an error passed to `panic`, and a `runtime.Error` like an index out of range. |

The methods have pointer receivers. Use `*retable.PanicError` with
`errors.As`.

### NewPanicError

```go
func NewPanicError(packageName string, value any) *PanicError
```

Returns a `PanicError` with `Package` set to `packageName`, `Value` set to
`value` and `Stack` set to `debug.Stack()` of the current goroutine.

Call it in the deferred function that called `recover`. The stack of the panic
is not yet unwound there, so `Stack` shows the line that panicked. Called
anywhere else, `Stack` shows where `NewPanicError` was called instead.

## Who returns it

| Function | Recovers a panic of | `Package` | Error returned |
| --- | --- | --- | --- |
| `retable.SmartAssign` | package `reflect`, the `dstScanner`, the `parser` and the `srcFormatter` | `retable` | `errors.Join` of any error so far and the `*PanicError` |
| `retable.ViewToStructSlice` | the same, because it assigns every cell with `SmartAssign`, and the `IsNull`, `IsZero` and `Value` methods of the cells when several columns match one field and it looks for the cell with content | `retable` | the error of `SmartAssign`, or the `*PanicError` |
| `csvtable` `ParseDetectFormat`, `ParseWithFormat`, `ReadStringsToStructSlice` and the four `Read*ToStructSlice` variants | anything inside them, also the `validate` function you pass | `csvtable` | the `*PanicError`, replacing any other error |
| `exceltable` `Writer` methods `Write`, `WriteView`, `WriteSheet` and `WriteViewSheet` | anything while writing, like a `CellFormatter` or the `Viewer` | `exceltable` | the `*PanicError`, or an error wrapping it when the panic happened after the sheet was added |

Not recovered:

- A panic of the `validate` function of `retable.ViewToStructSlice` reaches
  the caller. Only the `csvtable` read functions, which call
  `ViewToStructSlice`, recover it.
- The `csvtable` `Writer` does not recover panics. A panic of a formatter or a
  viewer reaches the caller.
- A panic in a goroutine you start inside a callback crashes the program, as
  every unrecovered panic in a goroutine does.

How a panic while writing a sheet affects the `Workbook` is described in the
[exceltable Writer reference](../exceltable/reference-writer.md#errors).

## Messages

| Panic | Message |
| --- | --- |
| `panic(errBroken)` in a `Scanner` passed to `ViewToStructSlice` | `retable: panic: broken` |
| An index out of range inside a `csvtable` read function | `csvtable: panic: runtime error: index out of range [3] with length 1` |
| `panic("bad format")` in an `exceltable` `CellFormatter` after the sheet `Invoices` was added | `sheet "Invoices": exceltable: panic: bad format` |

Until v1.0.0-beta.3 the message ended with the stack trace on the following
lines. It is now one line, so an error that wraps it, like
`row 2: csvtable: panic: ...`, stays on one line too.

## Examples

### Log the stack trace of a failed import

The message tells the user that the file could not be read. The stack trace
belongs in your log:

```go
bookings, _, err := csvtable.ReadBytesDetectFormatToStructSlice[Booking](
    data, nil, naming, nil, nil, nil, nil, "Datum", "Betrag")
if err != nil {
    var panicErr *retable.PanicError
    if errors.As(err, &panicErr) {
        log.Printf("bug while importing: %v\n%s", err, panicErr.Stack)
    }
    return fmt.Errorf("the file could not be imported: %w", err)
}
```

### Find the error passed to panic

`Unwrap` returns the panic value when it is an error, so `errors.Is` sees
through the `PanicError`:

```go
var errBroken = errors.New("broken")

type brokenScanner struct{}

func (brokenScanner) ScanString(dst reflect.Value, str string, parser retable.Parser) error {
    panic(errBroken)
}

type Row struct {
    A int
}

view := retable.NewStringsView("", [][]string{{"1"}}, "A")
_, err := retable.ViewToStructSlice[Row](view, nil, brokenScanner{}, nil, nil, nil)
// err.Error() == "retable: panic: broken"
// errors.Is(err, errBroken) == true
```

### Return a PanicError from your own function

Use `NewPanicError` in your own conversion code, so that your callers handle
its panics the same way as those of go-retable:

```go
func convert(row []string) (err error) {
    defer func() {
        if p := recover(); p != nil {
            err = retable.NewPanicError("mypackage", p)
        }
    }()
    _ = row[3] // panics for a short row
    return nil
}

err := convert([]string{"a"})
// err.Error() == "mypackage: panic: runtime error: index out of range [3] with length 1"
```

`err` must be a named result, because only a deferred function can set the
result of a function that panicked.

## Related

- [csvtable reference: Panics](../csvtable/reference.md#panics): what the
  parse and read functions recover
- [exceltable Writer reference](../exceltable/reference-writer.md#errors): panics
  while writing a sheet and the state of the `Workbook` afterwards
- [How csvtable reads malformed CSV](../csvtable/explanation-malformed-csv.md):
  why the parse functions recover panics at all
- [README: Recovered panics](../../README.md#recovered-panics): the short
  introduction
