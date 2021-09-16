package retable

import (
	"reflect"
	"testing"
)

func TestReflectCellFormatterFunc(t *testing.T) {
	type args struct {
		function  interface{}
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
				function: func(int) string { return "" },
			},
			wantValType: reflect.TypeOf(int(0)),
		},
		{
			name: "func(int) (string, error)",
			args: args{
				function: func(int) (string, error) { return "", nil },
			},
			wantValType: reflect.TypeOf(int(0)),
		},
		{
			name: "func(context.Context, int) string",
			args: args{
				function: func(int) string { return "" },
			},
			wantValType: reflect.TypeOf(int(0)),
		},
		{
			name: "func(context.Context, int) (string, error)",
			args: args{
				function: func(int) (string, error) { return "", nil },
			},
			wantValType: reflect.TypeOf(int(0)),
		},
		{
			name: "func(context.Context, int, *Cell) string",
			args: args{
				function: func(int) string { return "" },
			},
			wantValType: reflect.TypeOf(int(0)),
		},
		{
			name: "func(context.Context, int, *Cell) (string, error)",
			args: args{
				function: func(int) (string, error) { return "", nil },
			},
			wantValType: reflect.TypeOf(int(0)),
		},

		// Invalid
		{
			name: "nil func",
			args: args{
				function: nil,
			},
			wantErr: true,
		},
		{
			name: "func(int)",
			args: args{
				function: func(int) {},
			},
			wantErr: true,
		},
		{
			name: "func(context.Context, int, *Cell) (error, string)",
			args: args{
				function: func(int) (error, string) { return nil, "" },
			},
			wantErr: true,
		},
		{
			name: "func(context.Context, int, *Cell) (error, string)",
			args: args{
				function: func(int) (error, string) { return nil, "" },
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFormatter, gotValType, err := ReflectCellFormatterFunc(tt.args.function, tt.args.rawResult)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReflectCellFormatterFunc() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if gotFormatter == nil {
				t.Error("ReflectCellFormatterFunc() gotFormatter = <nil>")
			}
			_, gotRaw, err := gotFormatter(nil, &Cell{Value: reflect.ValueOf(0)})
			if gotRaw != tt.args.rawResult {
				t.Errorf("gotFormatter() raw = %v, want %v", gotRaw, tt.args.rawResult)
			}
			if err != nil {
				t.Errorf("gotFormatter() returned %v", err)
			}
			if gotValType != tt.wantValType {
				t.Errorf("ReflectCellFormatterFunc() gotValType = %v, want %v", gotValType, tt.wantValType)
			}
		})
	}
}
