package htmltable

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/domonda/go-retable"
)

var (
	HTMLPreCellFormatter retable.CellFormatterFunc = func(ctx context.Context, view retable.View, row, col int) (str string, raw bool, err error) {
		value := template.HTMLEscapeString(fmt.Sprint(view.AnyValue(row, col)))
		return "<pre>" + value + "</pre>", true, nil
	}

	HTMLPreCodeCellFormatter retable.CellFormatterFunc = func(ctx context.Context, view retable.View, row, col int) (str string, raw bool, err error) {
		value := template.HTMLEscapeString(fmt.Sprint(view.AnyValue(row, col)))
		return "<pre><code>" + value + "</code></pre>", true, nil
	}

	// ValueAsHTMLAnchorCellFormatter formats the cell value using fmt.Sprint,
	// escapes it for HTML and returns an HTML anchor element with the
	// value as id and inner text.
	ValueAsHTMLAnchorCellFormatter retable.CellFormatterFunc = func(ctx context.Context, view retable.View, row, col int) (str string, raw bool, err error) {
		value := template.HTMLEscapeString(fmt.Sprint(view.AnyValue(row, col)))
		return fmt.Sprintf("<a id='%[1]s'>%[1]s</a>", value), true, nil
	}

	_ retable.CellFormatter = JSONCellFormatter("")
	_ retable.CellFormatter = HTMLSpanClassCellFormatter("")
)

type JSONCellFormatter string

func (indent JSONCellFormatter) FormatCell(ctx context.Context, view retable.View, row, col int) (str string, raw bool, err error) {
	var src bytes.Buffer
	_, err = fmt.Fprintf(&src, "%s", view.AnyValue(row, col))
	if err != nil {
		return "", false, err
	}
	buf := bytes.NewBufferString("<pre>")
	err = json.Indent(buf, src.Bytes(), "", string(indent))
	if err != nil {
		return "", false, err
	}
	buf.WriteString("</pre>")
	return buf.String(), true, nil
}

// HTMLSpanClassCellFormatter formats the cell value within an HTML span element
// with the class of the underlying string value.
type HTMLSpanClassCellFormatter string

func (class HTMLSpanClassCellFormatter) FormatCell(ctx context.Context, view retable.View, row, col int) (str string, raw bool, err error) {
	text := template.HTMLEscapeString(fmt.Sprint(view.AnyValue(row, col)))
	return fmt.Sprintf("<span class='%s'>%s</span>", class, text), true, nil
}
