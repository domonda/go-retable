package htmltable

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/domonda/go-retable"
)

// HTMLPreCellFormatter wraps cell values in HTML <pre> elements.
// The cell value is converted to a string using fmt.Sprint and HTML-escaped.
// This formatter returns raw HTML (the second return value is true).
//
// Output example: <pre>some text</pre>
//
// Usage:
//
//	writer := htmltable.NewWriter[[]Data]().
//	    WithColumnFormatter(3, htmltable.HTMLPreCellFormatter)
var HTMLPreCellFormatter retable.CellFormatterFunc = func(ctx context.Context, view retable.View, row, col int) (str string, raw bool, err error) {
	value := template.HTMLEscapeString(fmt.Sprint(view.Cell(row, col)))
	return "<pre>" + value + "</pre>", true, nil
}

// HTMLPreCodeCellFormatter wraps cell values in HTML <pre><code> elements.
// The cell value is converted to a string using fmt.Sprint and HTML-escaped.
// This formatter returns raw HTML (the second return value is true).
// Useful for displaying code snippets in tables.
//
// Output example: <pre><code>func main() {}</code></pre>
//
// Usage:
//
//	writer := htmltable.NewWriter[[]Snippet]().
//	    WithColumnFormatter(1, htmltable.HTMLPreCodeCellFormatter)
var HTMLPreCodeCellFormatter retable.CellFormatterFunc = func(ctx context.Context, view retable.View, row, col int) (str string, raw bool, err error) {
	value := template.HTMLEscapeString(fmt.Sprint(view.Cell(row, col)))
	return "<pre><code>" + value + "</code></pre>", true, nil
}

// ValueAsHTMLAnchorCellFormatter formats cell values as HTML anchor elements.
// The cell value is converted to a string using fmt.Sprint, HTML-escaped,
// and used as both the anchor's id attribute and inner text.
// This formatter returns raw HTML (the second return value is true).
//
// Output example: <a id='abc123'>abc123</a>
//
// Usage:
//
//	writer := htmltable.NewWriter[[]Item]().
//	    WithColumnFormatter(0, htmltable.ValueAsHTMLAnchorCellFormatter)
var ValueAsHTMLAnchorCellFormatter retable.CellFormatterFunc = func(ctx context.Context, view retable.View, row, col int) (str string, raw bool, err error) {
	value := template.HTMLEscapeString(fmt.Sprint(view.Cell(row, col)))
	return fmt.Sprintf("<a id='%[1]s'>%[1]s</a>", value), true, nil
}

var (
	_ retable.CellFormatter = JSONCellFormatter("")
	_ retable.CellFormatter = HTMLSpanClassCellFormatter("")
)

// JSONCellFormatter formats cell values as JSON wrapped in HTML <pre> elements.
// The formatter value (a string) controls JSON formatting:
//   - Non-empty string: Used as indentation prefix (e.g., "  " for 2 spaces)
//   - Empty string: JSON is compacted to a single line
//
// The formatter handles various input types:
//   - json.RawMessage: Used directly
//   - json.Marshaler: Marshaled via MarshalJSON
//   - []byte and string: Parsed as JSON
//   - Other types: Marshaled via json.Marshal
//
// Nil values and values that produce empty JSON return "", false, nil.
// This formatter returns raw HTML (the second return value is true).
//
// Usage:
//
//	// Compact JSON
//	writer := htmltable.NewWriter[[]Data]().
//	    WithColumnFormatter(2, htmltable.JSONCellFormatter(""))
//
//	// Indented JSON with 2 spaces
//	writer := htmltable.NewWriter[[]Data]().
//	    WithColumnFormatter(2, htmltable.JSONCellFormatter("  "))
type JSONCellFormatter string

// FormatCell implements the retable.CellFormatter interface.
// It formats the cell value as JSON wrapped in a <pre> element.
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

// HTMLSpanClassCellFormatter wraps cell values in HTML <span> elements with a CSS class.
// The formatter value (a string) is used as the class attribute.
// The cell value is converted to a string using fmt.Sprint and HTML-escaped.
// This formatter returns raw HTML (the second return value is true).
//
// Output example (with class="highlight"): <span class='highlight'>value</span>
//
// Usage:
//
//	// Add a CSS class to a column
//	writer := htmltable.NewWriter[[]Data]().
//	    WithColumnFormatter(1, htmltable.HTMLSpanClassCellFormatter("highlight"))
//
//	// Different classes for different columns
//	writer := htmltable.NewWriter[[]Data]().
//	    WithColumnFormatter(0, htmltable.HTMLSpanClassCellFormatter("id-column")).
//	    WithColumnFormatter(1, htmltable.HTMLSpanClassCellFormatter("name-column"))
type HTMLSpanClassCellFormatter string

// FormatCell implements the retable.CellFormatter interface.
// It wraps the cell value in a <span> element with the configured CSS class.
func (class HTMLSpanClassCellFormatter) FormatCell(ctx context.Context, view retable.View, row, col int) (str string, raw bool, err error) {
	text := template.HTMLEscapeString(fmt.Sprint(view.Cell(row, col)))
	return fmt.Sprintf("<span class='%s'>%s</span>", class, text), true, nil
}
