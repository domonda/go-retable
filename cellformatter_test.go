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
