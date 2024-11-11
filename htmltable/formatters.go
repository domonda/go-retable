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
		value := template.HTMLEscapeString(fmt.Sprint(view.Cell(row, col)))
		return "<pre>" + value + "</pre>", true, nil
	}

	HTMLPreCodeCellFormatter retable.CellFormatterFunc = func(ctx context.Context, view retable.View, row, col int) (str string, raw bool, err error) {
		value := template.HTMLEscapeString(fmt.Sprint(view.Cell(row, col)))
		return "<pre><code>" + value + "</code></pre>", true, nil
	}

	// ValueAsHTMLAnchorCellFormatter formats the cell value using fmt.Sprint,
	// escapes it for HTML and returns an HTML anchor element with the
	// value as id and inner text.
	ValueAsHTMLAnchorCellFormatter retable.CellFormatterFunc = func(ctx context.Context, view retable.View, row, col int) (str string, raw bool, err error) {
		value := template.HTMLEscapeString(fmt.Sprint(view.Cell(row, col)))
		return fmt.Sprintf("<a id='%[1]s'>%[1]s</a>", value), true, nil
	}

	_ retable.CellFormatter = JSONCellFormatter("")
	_ retable.CellFormatter = HTMLSpanClassCellFormatter("")
)

// JSONCellFormatter formats the cell value as JSON in a <pre> HTML element.
// The string value of the formatter is used as indentation for the JSON
// or if empty, then the JSON is compacted.
// Any value that formats to an empty string (like nil)
// will not be interpreted as JSON and "", false, nil will be returned.
type JSONCellFormatter string

func (indent JSONCellFormatter) FormatCell(ctx context.Context, view retable.View, row, col int) (str string, raw bool, err error) {
	val := view.Cell(row, col)
	if val == nil {
		return "", false, nil
	}
	var valJSON []byte
	switch x := val.(type) {
	case json.RawMessage:
		valJSON = x
	case json.Marshaler:
		valJSON, err = x.MarshalJSON()
		if err != nil {
			return "", false, err
		}
	case []byte:
		valJSON = x
	case string:
		valJSON = []byte(x)
	default:
		valJSON, err = json.Marshal(val)
		if err != nil {
			return "", false, err
		}
	}
	if len(valJSON) == 0 {
		return "", false, nil
	}
	buf := bytes.NewBufferString("<pre>")
	if indent != "" {
		err = json.Indent(buf, valJSON, "", string(indent))
	} else {
		err = json.Compact(buf, valJSON)
	}
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
	text := template.HTMLEscapeString(fmt.Sprint(view.Cell(row, col)))
	return fmt.Sprintf("<span class='%s'>%s</span>", class, text), true, nil
}
