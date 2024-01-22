package retable

import (
	"context"
	"reflect"
	"testing"
)

func TestFormatTableAsStrings(t *testing.T) {
	type args struct {
		table      any
		options    []Option
		formatters *ReflectTypeCellFormatter
	}
	tests := []struct {
		name     string
		args     args
		wantRows [][]string
		wantErr  bool
	}{
		{
			name: "empty [][]string",
			args: args{
				table: [][]string{},
			},
			wantRows: nil,
		},
		{
			name: "empty []struct{}",
			args: args{
				table: []struct{}{},
			},
			wantRows: nil,
		},
		{
			name: `[][]string{{"Hello", "World", "!"}} no header`,
			args: args{
				table: [][]string{{"Hello", "World", "!"}},
			},
			wantRows: nil,
		},
		{
			name: `[][]string{{"Hello", "World", "!"}} no header`,
			args: args{
				table:   [][]string{{"Hello", "World", "!"}},
				options: []Option{OptionAddHeaderRow},
			},
			wantRows: [][]string{{"Hello", "World", "!"}},
		},
		{
			name: `multiline with header`,
			args: args{
				table: [][]string{
					{"Hello", "World", "!"},
					{"A", "B", "C"},
					{"First col only"},
				},
				options: []Option{OptionAddHeaderRow},
			},
			wantRows: [][]string{
				{"Hello", "World", "!"},
				{"A", "B", "C"},
				{"First col only", "", ""},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRows, err := FormatTableAsStrings(context.Background(), tt.args.table, tt.args.formatters, tt.args.options...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Strings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRows, tt.wantRows) {
				t.Errorf("Strings() = %v, want %v", gotRows, tt.wantRows)
			}
		})
	}
}
