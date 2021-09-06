package htmltable

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/domonda/go-retable"
)

var (
	HTMLPreCellFormatter retable.CellFormatterFunc = func(ctx context.Context, cell *retable.Cell) (str string, raw bool, err error) {
		return fmt.Sprintf("<pre>%s</pre>", cell.Value.Interface()), true, nil
	}
	HTMLCodeCellFormatter retable.CellFormatterFunc = func(ctx context.Context, cell *retable.Cell) (str string, raw bool, err error) {
		return fmt.Sprintf("<code>%s</code>", cell.Value.Interface()), true, nil
	}

	_ retable.CellFormatter = JSONCellFormatter("")
)

type JSONCellFormatter string

func (indent JSONCellFormatter) FormatCell(ctx context.Context, cell *retable.Cell) (str string, raw bool, err error) {
	var src bytes.Buffer
	_, err = fmt.Fprintf(&src, "%s", cell.Value.Interface())
	if err != nil {
		return "", false, err
	}
	buf := bytes.NewBuffer([]byte("<pre>"))
	err = json.Indent(buf, src.Bytes(), "", string(indent))
	if err != nil {
		return "", false, err
	}
	buf.WriteString("</pre>")
	return buf.String(), true, nil
}
