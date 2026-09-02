package retable

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReflectCellFormatterFunc(t *testing.T) {
	type args struct {
		function  any
		rawResult bool
	}
	tests := []struct {
		name        string
		args        args
		wantValType reflect.Type
		wantErr     bool
	}{
		{
			name: "func(int) string",
			args: args{
				function:  func(arg int) string { return fmt.Sprint(arg) },
				rawResult: false,
			},
			wantValType: reflect.TypeFor[int](),
		},
		{
			name: "func(int) (string, error)",
			args: args{
				function:  func(arg int) (string, error) { return fmt.Sprint(arg), nil },
				rawResult: false,
			},
			wantValType: reflect.TypeFor[int](),
		},
		{
			name: "func(context.Context, int) string",
			args: args{
				function:  func(_ context.Context, arg int) string { return fmt.Sprint(arg) },
				rawResult: false,
			},
			wantValType: reflect.TypeFor[int](),
		},
		{
			name: "func(context.Context, int) (string, error)",
			args: args{
				function:  func(_ context.Context, arg int) (string, error) { return fmt.Sprint(arg), nil },
				rawResult: false,
			},
			wantValType: reflect.TypeFor[int](),
		},
		{
			name: "func() string",
			args: args{
				function:  func() string { return "666" },
				rawResult: false,
			},
			wantValType: nil,
		},
		{
			name: "func() (string, error)",
			args: args{
				function:  func() (string, error) { return "666", nil },
				rawResult: false,
			},
			wantValType: nil,
		},

		// Invalid
		{
			name: "nil func",
			args: args{
				function:  nil,
				rawResult: false,
			},
			wantErr: true,
		},
		{
			name: "func(int)",
			args: args{
				function:  func(int) {},
				rawResult: false,
			},
			wantErr: true,
		},
		{
			name: "func(int) (error, string)",
			args: args{
				function:  func(int) (error, string) { return nil, "" },
				rawResult: false,
			},
			wantErr: true,
		},
		{
			name: "func(context.Context, int) (error, string)",
			args: args{
				function:  func(context.Context, int) (error, string) { return nil, "" },
				rawResult: false,
			},
			wantErr: true,
		},
		{
			name: "func(context.Context, int) (string, bool, error)",
			args: args{
				function:  func(context.Context, int) (string, bool, error) { return "", false, nil },
				rawResult: false,
			},
			wantErr: true,
		},
	}
	view1int := &AnyValuesView{Cols: []string{"Col A"}, Rows: [][]any{{666}}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFormatter, gotValType, err := ReflectCellFormatterFunc(tt.args.function, tt.args.rawResult)
			require.Equalf(t, tt.wantErr, err != nil, "ReflectCellFormatterFunc() error = %v, wantErr %v", err, tt.wantErr)
			if err != nil {
				return
			}
			require.NotNil(t, gotFormatter, "ReflectCellFormatterFunc() gotFormatter = <nil>")
			require.Equalf(t, tt.wantValType, gotValType, "ReflectCellFormatterFunc() gotValType = %v, want %v", gotValType, tt.wantValType)

			gotStr, gotRaw, err := gotFormatter(context.Background(), view1int, 0, 0)
			require.Equalf(t, tt.args.rawResult, gotRaw, "ReflectCellFormatterFunc() raw = %v, want %v", gotRaw, tt.args.rawResult)
			require.Equalf(t, "666", gotStr, "ReflectCellFormatterFunc() str = %v, want %v", gotStr, "666")
			require.NoErrorf(t, err, "gotFormatter() error = %v", err)
		})
	}
}

// TestReflectCellFormatterFuncInvalidCell covers that a cell with no
// value is reported as unsupported instead of panicking.
//
// The formatter passes the cell into reflect.Value.Call, which panics
// with "reflect: Call using zero Value argument" for a zero Value. A
// nil interface cell of an AnyValuesView produces exactly that, so this
// was reachable through a view of this package, and the panic escaped
// into the caller because nothing in the formatter path recovers.
// ReflectTypeCellFormatter.FormatCell has had the same guard all along.
func TestReflectCellFormatterFuncInvalidCell(t *testing.T) {
	formatter, valType, err := ReflectCellFormatterFunc(
		func(s string) (string, error) { return "fn:" + s, nil },
		false,
	)
	require.NoError(t, err)
	require.Equal(t, reflect.TypeFor[string](), valType)

	t.Run("nil interface cell", func(t *testing.T) {
		view := &AnyValuesView{Cols: []string{"a"}, Rows: [][]any{{nil}}}
		require.False(t, AsReflectCellView(view).ReflectCell(0, 0).IsValid())

		str, raw, err := formatter(context.Background(), view, 0, 0)
		require.ErrorIs(t, err, errors.ErrUnsupported, "an alternative formatter has to get a chance")
		require.Empty(t, str)
		require.False(t, raw)
	})

	t.Run("out of range cell", func(t *testing.T) {
		view := NewStringsView("", [][]string{{"only a header"}})
		require.Zero(t, view.NumRows())

		_, _, err := formatter(context.Background(), view, 0, 0)
		require.ErrorIs(t, err, errors.ErrUnsupported)
	})

	// A cell that does have a value is still formatted
	t.Run("valid cell", func(t *testing.T) {
		view := NewStringsView("", [][]string{{"col"}, {"x"}})
		str, _, err := formatter(context.Background(), view, 0, 0)
		require.NoError(t, err)
		require.Equal(t, "fn:x", str)
	})
}

// The small string-typed formatters are the ones a caller reaches for
// to format a single column, and each carries the raw flag that decides
// whether the writer escapes its output. Getting that flag wrong is
// silent: it either double-escapes visible output or injects unescaped
// markup into a table.
func TestStringCellFormatters(t *testing.T) {
	view := NewStringsView("", [][]string{{"A"}, {"value"}})
	ctx := context.Background()

	t.Run("PrintfCellFormatter formats and is not raw", func(t *testing.T) {
		str, raw, err := PrintfCellFormatter("<%s>").FormatCell(ctx, view, 0, 0)
		require.NoError(t, err)
		require.Equal(t, "<value>", str)
		require.False(t, raw, "the output has to be escaped by the writer")
	})

	t.Run("PrintfRawCellFormatter formats and is raw", func(t *testing.T) {
		str, raw, err := PrintfRawCellFormatter("<b>%s</b>").FormatCell(ctx, view, 0, 0)
		require.NoError(t, err)
		require.Equal(t, "<b>value</b>", str)
		require.True(t, raw, "the markup must reach the output unescaped")
	})

	t.Run("SprintCellFormatter passes its raw flag through", func(t *testing.T) {
		for _, wantRaw := range []bool{false, true} {
			str, raw, err := SprintCellFormatter(wantRaw).FormatCell(ctx, view, 0, 0)
			require.NoError(t, err)
			require.Equal(t, "value", str)
			require.Equal(t, wantRaw, raw)
		}
	})

	t.Run("RawCellString ignores the cell and is raw", func(t *testing.T) {
		str, raw, err := RawCellString("&nbsp;").FormatCell(ctx, view, 0, 0)
		require.NoError(t, err)
		require.Equal(t, "&nbsp;", str, "the constant is the output, whatever the cell holds")
		require.True(t, raw)
	})

	t.Run("UnsupportedCellFormatter reports unsupported so a chain continues", func(t *testing.T) {
		str, raw, err := UnsupportedCellFormatter{}.FormatCell(ctx, view, 0, 0)
		require.ErrorIs(t, err, errors.ErrUnsupported)
		require.Empty(t, str)
		require.False(t, raw)
	})
}

// SprintCellFormatter renders a number as its digits. reflect based
// conversion of an integer to a string would produce the character with
// that code point instead, which is the mistake this package has hit
// before, so a numeric cell is worth pinning.
func TestSprintCellFormatterOnNumbers(t *testing.T) {
	view := &AnyValuesView{Cols: []string{"N"}, Rows: [][]any{{42}}}

	str, _, err := SprintCellFormatter(false).FormatCell(context.Background(), view, 0, 0)
	require.NoError(t, err)
	require.Equal(t, "42", str)
}

// LayoutFormatter formats a cell that knows how to format itself, which
// is how a time or a decimal column takes a layout string.
func TestLayoutFormatter(t *testing.T) {
	ctx := context.Background()

	t.Run("a cell implementing Format is given the layout", func(t *testing.T) {
		view := &AnyValuesView{Cols: []string{"T"}, Rows: [][]any{{layoutCell{}}}}

		str, raw, err := LayoutFormatter("2006-01-02").FormatCell(ctx, view, 0, 0)
		require.NoError(t, err)
		require.Equal(t, "formatted with 2006-01-02", str)
		require.False(t, raw)
	})

	t.Run("a cell that cannot format itself is an error, not a panic", func(t *testing.T) {
		view := NewStringsView("", [][]string{{"A"}, {"plain"}})

		_, _, err := LayoutFormatter("2006-01-02").FormatCell(ctx, view, 0, 0)
		require.ErrorContains(t, err, "does not implement")
	})
}

type layoutCell struct{}

func (layoutCell) Format(layout string) string { return "formatted with " + layout }

// StringIfTrue and RawStringIfTrue render a boolean column as a mark,
// like a checkmark for true and nothing for false.
func TestStringIfTrue(t *testing.T) {
	ctx := context.Background()
	view := &AnyValuesView{Cols: []string{"Active"}, Rows: [][]any{{true}, {false}}}

	t.Run("StringIfTrue", func(t *testing.T) {
		str, raw, err := StringIfTrue("yes").FormatCell(ctx, view, 0, 0)
		require.NoError(t, err)
		require.Equal(t, "yes", str)
		require.False(t, raw)

		str, _, err = StringIfTrue("yes").FormatCell(ctx, view, 1, 0)
		require.NoError(t, err)
		require.Empty(t, str, "false renders as nothing, not as the text")
	})

	t.Run("RawStringIfTrue is raw for both outcomes", func(t *testing.T) {
		str, raw, err := RawStringIfTrue("&check;").FormatCell(ctx, view, 0, 0)
		require.NoError(t, err)
		require.Equal(t, "&check;", str)
		require.True(t, raw)

		str, raw, err = RawStringIfTrue("&check;").FormatCell(ctx, view, 1, 0)
		require.NoError(t, err)
		require.Empty(t, str)
		require.True(t, raw, "the empty result is raw too, so it is not escaped into something visible")
	})
}

// TestReflectCellFormatterFuncWrongCellType covers that a cell whose
// type the function does not accept is reported instead of panicking.
// The invalid-cell guard was not enough: a formatter registered by kind
// or by interface receives defined types, and reflect.Value.Call panics
// on an argument that is not assignable, which escapes through
// TryFormattersOrSprint into the CSV and HTML writers.
func TestReflectCellFormatterFuncWrongCellType(t *testing.T) {
	type definedInt int

	formatter, _, err := ReflectCellFormatterFunc(func(int) string { return "formatted" }, false)
	require.NoError(t, err)

	byKind := new(ReflectTypeCellFormatter).WithKindFormatter(reflect.Int, formatter)
	view := &AnyValuesView{Cols: []string{"c"}, Rows: [][]any{{definedInt(7)}}}

	require.NotPanics(t, func() {
		str, _, err := byKind.FormatCell(context.Background(), view, 0, 0)
		require.ErrorIs(t, err, errors.ErrUnsupported, "an alternative formatter has to get its chance")
		require.Empty(t, str)
	})

	// The exact type the function accepts is still formatted
	plain := &AnyValuesView{Cols: []string{"c"}, Rows: [][]any{{7}}}
	str, _, err := byKind.FormatCell(context.Background(), plain, 0, 0)
	require.NoError(t, err)
	require.Equal(t, "formatted", str)
}
