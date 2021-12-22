package retable

import (
	"reflect"
	"testing"
)

func TestStringColumnWidths(t *testing.T) {
	type args struct {
		rows    [][]string
		numCols int
	}
	tests := []struct {
		name string
		args args
		want []int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringColumnWidths(tt.args.rows, tt.args.numCols); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StringColumnWidths() = %v, want %v", got, tt.want)
			}
		})
	}
}
