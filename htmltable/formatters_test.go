package htmltable

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/domonda/go-retable"
	"github.com/stretchr/testify/require"
)

func TestJSONCellFormatter_FormatCell(t *testing.T) {
	tests := []struct {
		name    string
		fmt     JSONCellFormatter
		view    retable.View
		wantStr string
		wantRaw bool
		wantErr bool
	}{
		{name: "empty nil", fmt: ``, view: retable.SingleCellView("", "", any(nil)), wantStr: ``, wantRaw: false, wantErr: false},
		{name: "empty string", fmt: ``, view: retable.SingleCellView("", "", ""), wantStr: ``, wantRaw: false, wantErr: false},
		{name: "empty nil pointer", fmt: ``, view: retable.SingleCellView("", "", (*int)(nil)), wantStr: `<pre>null</pre>`, wantRaw: true, wantErr: false},
		{name: "compact string JSON", fmt: ``, view: retable.SingleCellView("", "", `{"1": 1}`), wantStr: `<pre>{"1":1}</pre>`, wantRaw: true, wantErr: false},
		{name: "compact []byte JSON", fmt: ``, view: retable.SingleCellView("", "", []byte(`{"1": 1}`)), wantStr: `<pre>{"1":1}</pre>`, wantRaw: true, wantErr: false},
		{name: "compact RawMessage JSON", fmt: ``, view: retable.SingleCellView("", "", json.RawMessage(`{"1": 1}`)), wantStr: `<pre>{"1":1}</pre>`, wantRaw: true, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			str, raw, err := tt.fmt.FormatCell(context.Background(), tt.view, 0, 0)
			require.Equal(t, tt.wantErr, err != nil, "err result: %v", err)
			require.Equal(t, tt.wantStr, str, "str result")
			require.Equal(t, tt.wantRaw, raw, "raw result")
		})
	}
}
