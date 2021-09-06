package csv

import (
	"bytes"
	"context"
	"testing"

	"github.com/domonda/go-retable"
)

func TestWriter_WriteView(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name           string
		writer         *Writer
		view           retable.View
		writeHeaderRow bool
		wantDest       string
		wantErr        bool
	}{
		{
			name:           "empty view",
			writer:         NewWriter(),
			view:           &retable.MockView{},
			writeHeaderRow: false,
			wantDest:       ``,
		},
		{
			name:   "simple",
			writer: NewWriter(),
			view: &retable.MockView{
				Cols: []string{"A", "B", "C"},
				Rows: [][]interface{}{
					{1, "Hello", nil},
					{2, "world!", new(float64)},
				},
			},
			writeHeaderRow: true,
			wantDest: "" +
				`A;B;C` + "\r\n" +
				`1;Hello;` + "\r\n" +
				`2;world!;0` + "\r\n",
		},
		// {
		// 	name: "simple padded",
		// 	writer: NewWriter().
		// 		WithDelimiter('|').
		// 		WithFieldPadding(true),
		// 	view: &retable.MockView{
		// 		Cols: []string{"A", "B", "Blah"},
		// 		Rows: [][]interface{}{
		// 			{1, "Hello", nil},
		// 			{123, "world!", new(float64)},
		// 		},
		// 	},
		// 	writeHeaderRow: true,
		// 	wantDest: "" +
		// 		`A  |B     |Blah` + "\r\n" +
		// 		`1  |Hello |    ` + "\r\n" +
		// 		`123|world!|0   ` + "\r\n",
		// },
		{
			name: "command and quoted fields",
			writer: NewWriter().
				WithDelimiter(',').
				WithQuoteAllFields(true),
			view: &retable.MockView{
				Cols: []string{" A ", "B", "C"},
				Rows: [][]interface{}{
					{1, "Hello", nil},
					{2, "world!", new(float64)},
				},
			},
			writeHeaderRow: true,
			wantDest: "" +
				`" A ","B","C"` + "\r\n" +
				`"1","Hello",""` + "\r\n" +
				`"2","world!","0"` + "\r\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dest bytes.Buffer
			if err := tt.writer.WriteView(ctx, &dest, tt.view, tt.writeHeaderRow); (err != nil) != tt.wantErr {
				t.Errorf("Writer.WriteView() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotDest := dest.String(); gotDest != tt.wantDest {
				t.Errorf("Writer.WriteView() wrote:\n%s\nbut want:\n%s", gotDest, tt.wantDest)
			}
		})
	}
}
