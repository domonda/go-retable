package htmltable

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"

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
//   - Other types: Marshaled via json.Encoder
//
// Nil values and values that produce empty JSON return "", false, nil.
// This formatter returns raw HTML (the second return value is true),
// so the JSON is HTML-escaped before it's wrapped in the <pre> element.
// The escaping happens at the HTML level instead of leaving it to the
// JSON encoder, because json.Compact and json.Indent don't escape and
// the JSON of every other input type would pass through unescaped.
// Only the value marshaled here can be encoded without HTML escaping;
// JSON that arrives already marshaled keeps the escapes it carries.
// The indentation prefix is escaped along with the JSON, so an indent
// written as an HTML entity appears literally instead of as the entity.
//
// This formatter writes HTML and must only be used with an HTML writer.
// It returns raw output, which a non-HTML writer such as csvtable emits
// without its own quoting, so the markup and the entities would corrupt
// the row structure there.
//
// Output example: <pre>{"ok":true}</pre>
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
		// Marshal without HTML escaping because the JSON
		// is HTML-escaped as a whole further down
		var marshaled bytes.Buffer
		encoder := json.NewEncoder(&marshaled)
		encoder.SetEscapeHTML(false)
		err = encoder.Encode(val)
		if err != nil {
			return "", false, err
		}
		valJSON = marshaled.Bytes()
	}
	if len(valJSON) == 0 {
		return "", false, nil
	}
	var buf bytes.Buffer
	if indent != "" {
		err = json.Indent(&buf, valJSON, "", string(indent))
	} else {
		err = json.Compact(&buf, valJSON)
	}
	if err != nil {
		return "", false, err
	}
	// json.Indent copies trailing whitespace from its source while json.Compact
	// drops it, so trim it here to keep both of them in agreement. Without this
	// an indented cell ends with the newline json.Encoder.Encode appends, or
	// with whatever trailing space an already marshaled value carried, and
	// renders a blank line inside the <pre> element.
	trimmed := strings.TrimRight(buf.String(), " \t\r\n")
	return "<pre>" + preTextEscaper.Replace(trimmed) + "</pre>", true, nil
}

// preTextEscaper escapes the characters that are special in HTML text content.
// JSONCellFormatter emits the <pre> element itself, so HTML text content is where
// its JSON is meant to land, and these three are the ones that can end the text
// node early. A custom row template can put the value somewhere else, and there
// html/template re-escapes it contextually, which is what covers the quotes this
// escaper leaves alone. template.HTMLEscapeString is
// deliberately not used here: it also escapes the quotes, which are only
// special inside an attribute value, and JSON quotes every key and every
// string value, so it would rewrite `{"ok":true}` to `{&#34;ok&#34;:true}`
// without making the output any safer.
//
// That is why this file escapes with two different functions, which is a
// deliberate split and not an oversight: anything interpolated into an
// attribute, like the class of HTMLSpanClassCellFormatter or the id of
// ValueAsHTMLAnchorCellFormatter, needs template.HTMLEscapeString because a
// quote ends an attribute value. Text content only needs these three. The
// other formatters here escape short values where the quotes cost nothing,
// so they use the stricter escaper for uniformity with their attributes.
var preTextEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

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
// The class is escaped like the cell value because it's interpolated into a
// quoted attribute, where template.HTMLEscapeString is the right escaper:
// unlike the <pre> text of JSONCellFormatter, an attribute value is ended by
// a quote, so the quotes have to be escaped too.
func (class HTMLSpanClassCellFormatter) FormatCell(ctx context.Context, view retable.View, row, col int) (str string, raw bool, err error) {
	text := template.HTMLEscapeString(fmt.Sprint(view.Cell(row, col)))
	return fmt.Sprintf("<span class='%s'>%s</span>", template.HTMLEscapeString(string(class)), text), true, nil
}
