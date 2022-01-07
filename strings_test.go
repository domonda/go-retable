package retable

import (
	"context"
	"reflect"
	"testing"
)

func TestStrings(t *testing.T) {
	type args struct {
		table        interface{}
		addHeaderRow bool
		formatters   *TypeFormatters
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
				table:        [][]string{},
				addHeaderRow: false,
			},
			wantRows: nil,
		},
		{
			name: "empty []struct{}",
			args: args{
				table:        []struct{}{},
				addHeaderRow: false,
			},
			wantRows: nil,
		},
		{
			name: `[][]string{{"Hello", "World", "!"}} no header`,
			args: args{
				table:        [][]string{{"Hello", "World", "!"}},
				addHeaderRow: false,
			},
			wantRows: nil,
		},
		{
			name: `[][]string{{"Hello", "World", "!"}} no header`,
			args: args{
				table:        [][]string{{"Hello", "World", "!"}},
				addHeaderRow: true,
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
				addHeaderRow: true,
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
			gotRows, err := Strings(context.Background(), tt.args.table, tt.args.addHeaderRow, tt.args.formatters)
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
