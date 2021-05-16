package csv

import (
	"bytes"
	"context"
	"testing"

	"github.com/domonda/go-retable"
)

func TestWriter_Write(t *testing.T) {
	type fields struct {
		formatter        retable.TypeFormatters
		writeHeaderRow   bool
		quoteAllFields   bool
		quoteEmptyFields bool
		delimiter        rune
		newLine          string
		charset          retable.Charset
	}
	tests := []struct {
		name     string
		fields   fields
		view     retable.View
		wantDest string
		wantErr  bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Writer{
				formatter:        tt.fields.formatter,
				writeHeaderRow:   tt.fields.writeHeaderRow,
				quoteAllFields:   tt.fields.quoteAllFields,
				quoteEmptyFields: tt.fields.quoteEmptyFields,
				delimiter:        tt.fields.delimiter,
				newLine:          tt.fields.newLine,
				charset:          tt.fields.charset,
			}
			dest := &bytes.Buffer{}
			if err := w.Write(context.Background(), dest, tt.view); (err != nil) != tt.wantErr {
				t.Errorf("Writer.Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotDest := dest.String(); gotDest != tt.wantDest {
				t.Errorf("Writer.Write() = %v, want %v", gotDest, tt.wantDest)
			}
		})
	}
}
