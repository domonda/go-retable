package rowcolumns

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMajority(t *testing.T) {
	tests := []struct {
		name string
		rows [][]string
		want int
	}{
		{name: "no rows", rows: nil, want: 0},
		{name: "only empty rows", rows: [][]string{{}, nil}, want: 0},
		{name: "one column count", rows: [][]string{{"a", "b"}, {"c", "d"}}, want: 2},
		{name: "most rows win", rows: [][]string{{"a", "b", "c"}, {"a", "b"}, {"c", "d"}}, want: 2},
		{name: "more columns win a tie", rows: [][]string{{"a", "b", "c"}, {"a", "b"}}, want: 3},
		{name: "single column rows don't outvote the table", rows: [][]string{{"x"}, {"y"}, {"z"}, {"a", "b"}}, want: 2},
		{name: "only single column rows", rows: [][]string{{"x"}, {}, {"y"}}, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, Majority(tt.rows))
		})
	}
}
