package retable

import (
	"context"
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
			wantValType: reflect.TypeOf(int(0)),
		},
		{
			name: "func(int) (string, error)",
			args: args{
				function:  func(arg int) (string, error) { return fmt.Sprint(arg), nil },
				rawResult: false,
			},
			wantValType: reflect.TypeOf(int(0)),
		},
		{
			name: "func(context.Context, int) string",
			args: args{
				function:  func(_ context.Context, arg int) string { return fmt.Sprint(arg) },
				rawResult: false,
			},
			wantValType: reflect.TypeOf(int(0)),
		},
		{
			name: "func(context.Context, int) (string, error)",
			args: args{
				function:  func(_ context.Context, arg int) (string, error) { return fmt.Sprint(arg), nil },
				rawResult: false,
			},
			wantValType: reflect.TypeOf(int(0)),
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
