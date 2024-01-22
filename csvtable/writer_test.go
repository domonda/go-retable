package csvtable

import (
	"bytes"
	"context"
	"testing"

	"github.com/domonda/go-retable"
)

func TestWriter_WriteView(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		writer   *Writer[any]
		view     retable.View
		wantDest string
		wantErr  bool
	}{
		{
			name:     "empty view",
			writer:   NewWriter[any](),
			view:     &retable.AnyValuesView{},
			wantDest: ``,
		},
		{
			name: "simple",
			writer: NewWriter[any]().
				WithHeaderRow(true),
			view: &retable.AnyValuesView{
				Cols: []string{"A", "B", "C"},
				Rows: [][]any{
					{1, "Hello", nil},
					{2, "world!", new(float64)},
				},
			},
			wantDest: "" +
				`A;B;C` + "\r\n" +
				`1;Hello;` + "\r\n" +
				`2;world!;0` + "\r\n",
		},
		{
			name: "simple no header",
			writer: NewWriter[any]().
				WithHeaderRow(true).
				WithHeaderRow(false),
			view: &retable.AnyValuesView{
				Cols: []string{"A", "B", "C"},
				Rows: [][]any{
					{1, "Hello", nil},
					{2, "world!", new(float64)},
				},
			},
			wantDest: "" +
				`1;Hello;` + "\r\n" +
				`2;world!;0` + "\r\n",
		},
		{
			name: "simple padded align left",
			writer: NewWriter[any]().
				WithHeaderRow(true).
				WithDelimiter('|').
				WithPadding(AlignLeft),
			view: &retable.AnyValuesView{
				Cols: []string{"A", "B", "Blah"},
				Rows: [][]any{
					{1, "Hello", nil},
					{123, "world!", new(float64)},
				},
			},
			wantDest: "" +
				`A  |B     |Blah` + "\r\n" +
				`1  |Hello |    ` + "\r\n" +
				`123|world!|0   ` + "\r\n",
		},
		{
			name: "simple padded align center",
			writer: NewWriter[any]().
				WithHeaderRow(true).
				WithDelimiter('|').
				WithPadding(AlignCenter),
			view: &retable.AnyValuesView{
				Cols: []string{"A", "B", "Blah"},
				Rows: [][]any{
					{1, "Hello", nil},
					{123, "world!", new(float64)},
				},
			},
			wantDest: "" +
				` A |  B   |Blah` + "\r\n" +
				` 1 |Hello |    ` + "\r\n" +
				`123|world!| 0  ` + "\r\n",
		},
		{
			name: "simple padded align right",
			writer: NewWriter[any]().
				WithHeaderRow(true).
				WithDelimiter('|').
				WithPadding(AlignRight),
			view: &retable.AnyValuesView{
				Cols: []string{"A", "B", "Blah"},
				Rows: [][]any{
					{1, "Hello", nil},
					{123, "world!", new(float64)},
				},
			},
			wantDest: "" +
				`  A|     B|Blah` + "\r\n" +
				`  1| Hello|    ` + "\r\n" +
				`123|world!|   0` + "\r\n",
		},
		{
			name: "command and quoted fields",
			writer: NewWriter[any]().
				WithHeaderRow(true).
				WithDelimiter(',').
				WithQuoteAllFields(true),
			view: &retable.AnyValuesView{
				Cols: []string{" A ", "B", "C"},
				Rows: [][]any{
					{1, "Hello", nil},
					{2, "world!", new(float64)},
				},
			},
			wantDest: "" +
				`" A ","B","C"` + "\r\n" +
				`"1","Hello",""` + "\r\n" +
				`"2","world!","0"` + "\r\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dest bytes.Buffer
			if err := tt.writer.WriteView(ctx, &dest, tt.view); (err != nil) != tt.wantErr {
				t.Errorf("Writer.WriteView() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotDest := dest.String(); gotDest != tt.wantDest {
				t.Errorf("Writer.WriteView() wrote:\n%s\nbut want:\n%s", gotDest, tt.wantDest)
			}
		})
	}
}
