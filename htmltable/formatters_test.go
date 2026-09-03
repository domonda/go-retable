package htmltable

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
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
		{name: "indented map JSON", fmt: `  `, view: retable.SingleCellView("", "", map[string]int{"1": 1}), wantStr: "<pre>{\n  \"1\": 1\n}</pre>", wantRaw: true, wantErr: false},
		{name: "indented string JSON", fmt: `  `, view: retable.SingleCellView("", "", `{"1": 1}`), wantStr: "<pre>{\n  \"1\": 1\n}</pre>", wantRaw: true, wantErr: false},
		{name: "indented []byte JSON", fmt: `  `, view: retable.SingleCellView("", "", []byte(`{"1": 1}`)), wantStr: "<pre>{\n  \"1\": 1\n}</pre>", wantRaw: true, wantErr: false},
		{name: "indented RawMessage JSON", fmt: `  `, view: retable.SingleCellView("", "", json.RawMessage(`{"1": 1}`)), wantStr: "<pre>{\n  \"1\": 1\n}</pre>", wantRaw: true, wantErr: false},
		{name: "indented json.Marshaler JSON", fmt: `  `, view: retable.SingleCellView("", "", jsonMarshalerString("x")), wantStr: `<pre>"x"</pre>`, wantRaw: true, wantErr: false},
		{name: "empty []byte", fmt: ``, view: retable.SingleCellView("", "", []byte{}), wantStr: ``, wantRaw: false, wantErr: false},
		{name: "empty RawMessage", fmt: ``, view: retable.SingleCellView("", "", json.RawMessage(nil)), wantStr: ``, wantRaw: false, wantErr: false},
		{name: "empty json.Marshaler result", fmt: ``, view: retable.SingleCellView("", "", jsonMarshalerEmpty{}), wantStr: ``, wantRaw: false, wantErr: false},
		{name: "malformed string compacted", fmt: ``, view: retable.SingleCellView("", "", `{"1":`), wantStr: ``, wantRaw: false, wantErr: true},
		{name: "malformed []byte compacted", fmt: ``, view: retable.SingleCellView("", "", []byte(`{"1":`)), wantStr: ``, wantRaw: false, wantErr: true},
		{name: "malformed RawMessage compacted", fmt: ``, view: retable.SingleCellView("", "", json.RawMessage(`{"1":`)), wantStr: ``, wantRaw: false, wantErr: true},
		{name: "malformed string indented", fmt: `  `, view: retable.SingleCellView("", "", `{"1":`), wantStr: ``, wantRaw: false, wantErr: true},
		{name: "malformed []byte indented", fmt: `  `, view: retable.SingleCellView("", "", []byte(`{"1":`)), wantStr: ``, wantRaw: false, wantErr: true},
		{name: "malformed RawMessage indented", fmt: `  `, view: retable.SingleCellView("", "", json.RawMessage(`{"1":`)), wantStr: ``, wantRaw: false, wantErr: true},
		{name: "unmarshalable channel", fmt: ``, view: retable.SingleCellView("", "", make(chan int)), wantStr: ``, wantRaw: false, wantErr: true},
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

// jsonMarshalerString marshals itself as a JSON string to exercise the
// json.Marshaler branch of JSONCellFormatter. It deliberately does not
// HTML-escape, like any MarshalJSON implementation is free not to,
// so the formatter can't rely on its input being escaped already.
type jsonMarshalerString string

func (s jsonMarshalerString) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(string(s))
	if err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// TestJSONCellFormatter_EscapesHTMLForEveryInputType guards against HTML injection.
// JSONCellFormatter returns raw HTML, so its output is written into the document
// verbatim. json.Compact and json.Indent don't escape HTML, and json.Marshal only
// escaped the values reaching its own branch, so a cell value containing "</pre>"
// used to close the <pre> element early and let the rest of the value run as markup.
func TestJSONCellFormatter_EscapesHTMLForEveryInputType(t *testing.T) {
	const payload = `</pre><script>alert(1)</script>`
	document := `{"v":"` + payload + `"}`
	values := map[string]any{
		"string":          document,
		"[]byte":          []byte(document),
		"json.RawMessage": json.RawMessage(document),
		"json.Marshaler":  jsonMarshalerString(payload),
		"other type":      map[string]string{"v": payload},
	}
	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			view := retable.SingleCellView("", "", value)
			str, raw, err := JSONCellFormatter(``).FormatCell(context.Background(), view, 0, 0)
			require.NoError(t, err)
			require.True(t, raw)
			require.True(t, strings.HasPrefix(str, "<pre>"), "got %q", str)
			require.True(t, strings.HasSuffix(str, "</pre>"), "got %q", str)
			// No markup of the payload may survive between the <pre> tags
			inner := strings.TrimSuffix(strings.TrimPrefix(str, "<pre>"), "</pre>")
			require.NotContains(t, inner, "<")
			require.NotContains(t, inner, ">")
		})
	}
}

// TestJSONCellFormatter_KeepsAmpersandReadable documents that the escaping happens
// at the HTML level, not via the HTML escaping of json.Marshal. Both are safe, but
// json.Marshal would turn "Meier & Co" into the literal text "Meier \u0026 Co",
// which a browser renders as the backslash escape instead of the ampersand.
func TestJSONCellFormatter_KeepsAmpersandReadable(t *testing.T) {
	view := retable.SingleCellView("", "", map[string]string{"name": "Meier & Co"})
	str, _, err := JSONCellFormatter(``).FormatCell(context.Background(), view, 0, 0)
	require.NoError(t, err)
	require.Equal(t, `<pre>{"name":"Meier &amp; Co"}</pre>`, str)
	require.NotContains(t, str, `\u0026`)
}

// jsonMarshalerEmpty returns no JSON at all, which is what the
// len(valJSON) == 0 check of JSONCellFormatter has to survive:
// json.Compact would report a syntax error on empty input.
type jsonMarshalerEmpty struct{}

func (jsonMarshalerEmpty) MarshalJSON() ([]byte, error) { return nil, nil }

var errMarshalJSON = errors.New("MarshalJSON failed")

type jsonMarshalerError struct{}

func (jsonMarshalerError) MarshalJSON() ([]byte, error) { return nil, errMarshalJSON }

// TestJSONCellFormatter_MarshalerErrorReachesCaller checks that a MarshalJSON error
// is passed through unwrapped instead of being turned into an empty cell, so the
// caller can tell a value that failed to marshal from one that has no JSON.
func TestJSONCellFormatter_MarshalerErrorReachesCaller(t *testing.T) {
	view := retable.SingleCellView("", "", jsonMarshalerError{})
	str, raw, err := JSONCellFormatter(``).FormatCell(context.Background(), view, 0, 0)
	require.ErrorIs(t, err, errMarshalJSON)
	require.Equal(t, ``, str)
	require.False(t, raw)
}

// TestJSONCellFormatter_MalformedJSONAbortsWrite checks the caller facing behavior
// of the formatter errors: Writer stops at the offending cell and reports the error
// instead of emitting a table with a broken or silently empty cell. The header is
// already flushed when the row fails, so what a caller gets on error is a truncated
// fragment, not a usable document: the error must not be ignored.
func TestJSONCellFormatter_MalformedJSONAbortsWrite(t *testing.T) {
	type Row struct {
		Data string
	}
	var buf bytes.Buffer
	err := NewWriter[[]Row]().
		WithColumnFormatter(0, JSONCellFormatter(``)).
		Write(context.Background(), &buf, []Row{{Data: `{"1":`}})
	var syntaxErr *json.SyntaxError
	require.ErrorAs(t, err, &syntaxErr)
	require.Equal(t, "<table>\n", buf.String(), "no cell content and no closing tag may reach the caller")
}

// jsonMarshalerEscaping is the ordinary way to implement MarshalJSON: delegate to
// json.Marshal. json.Marshal HTML-escapes by itself, and the formatter passes an
// already marshaled value through untouched, so this branch keeps the escape.
type jsonMarshalerEscaping struct{ Name string }

func (m jsonMarshalerEscaping) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct{ Name string }{m.Name})
}

// TestJSONCellFormatter_MarshalerKeepsItsOwnEscaping pins the limit of the
// readability fix. Only the branch that marshals the value itself can choose not
// to escape; JSON that arrives already marshaled keeps whatever escapes it carries,
// so a MarshalJSON built on json.Marshal still renders \u0026 instead of the
// ampersand. Safe either way, but it is why the CHANGELOG scopes that claim.
func TestJSONCellFormatter_MarshalerKeepsItsOwnEscaping(t *testing.T) {
	view := retable.SingleCellView("", "", jsonMarshalerEscaping{"Meier & Co"})
	str, _, err := JSONCellFormatter(``).FormatCell(context.Background(), view, 0, 0)
	require.NoError(t, err)
	require.Equal(t, `<pre>{"Name":"Meier \u0026 Co"}</pre>`, str)
}

// TestRawFormatters_EscapeUntrustedValues covers the other formatters in this file
// that also return raw HTML. Their output goes into the document verbatim, so every
// value they interpolate has to be escaped, in the attribute as well as in the text.
// HTMLSpanClassCellFormatter used to interpolate its class unescaped, which let a
// class like `x' onmouseover='alert(1)` add a live event handler to the span.
func TestRawFormatters_EscapeUntrustedValues(t *testing.T) {
	const markup = `</pre><script>alert(1)</script>`
	const breakOut = `x' onmouseover='alert(1)`

	tests := []struct {
		name      string
		formatter retable.CellFormatter
		cell      any
		wantStr   string
	}{
		{name: "pre escapes markup", formatter: HTMLPreCellFormatter, cell: markup,
			wantStr: `<pre>&lt;/pre&gt;&lt;script&gt;alert(1)&lt;/script&gt;</pre>`},
		{name: "pre code escapes markup", formatter: HTMLPreCodeCellFormatter, cell: markup,
			wantStr: `<pre><code>&lt;/pre&gt;&lt;script&gt;alert(1)&lt;/script&gt;</code></pre>`},
		{name: "anchor escapes the quote that would end its attribute", formatter: ValueAsHTMLAnchorCellFormatter, cell: breakOut,
			wantStr: `<a id='x&#39; onmouseover=&#39;alert(1)'>x&#39; onmouseover=&#39;alert(1)</a>`},
		{name: "span escapes the cell value", formatter: HTMLSpanClassCellFormatter("ok"), cell: markup,
			wantStr: `<span class='ok'>&lt;/pre&gt;&lt;script&gt;alert(1)&lt;/script&gt;</span>`},
		{name: "span escapes the class attribute", formatter: HTMLSpanClassCellFormatter(breakOut), cell: "value",
			wantStr: `<span class='x&#39; onmouseover=&#39;alert(1)'>value</span>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := retable.SingleCellView("", "", tt.cell)
			str, raw, err := tt.formatter.FormatCell(context.Background(), view, 0, 0)
			require.NoError(t, err)
			require.True(t, raw)
			require.Equal(t, tt.wantStr, str)
			// Nothing the caller supplied may still be able to end an attribute
			require.NotContains(t, str, `onmouseover='`)
		})
	}
}

// TestJSONCellFormatter_IndentForkIsEscapedToo covers the json.Indent fork, which
// carries surface the compact fork doesn't: json.Indent writes the indentation
// prefix into the output verbatim, so a hostile indent used to reach the document
// as markup. Both the cell value and the indent are escaped now.
func TestJSONCellFormatter_IndentForkIsEscapedToo(t *testing.T) {
	const payload = `</pre><script>alert(1)</script>`

	t.Run("hostile cell value", func(t *testing.T) {
		view := retable.SingleCellView("", "", json.RawMessage(`{"v":"`+payload+`"}`))
		str, _, err := JSONCellFormatter(`  `).FormatCell(context.Background(), view, 0, 0)
		require.NoError(t, err)
		inner := strings.TrimSuffix(strings.TrimPrefix(str, "<pre>"), "</pre>")
		require.NotContains(t, inner, "<")
		require.NotContains(t, inner, ">")
	})

	t.Run("hostile indent", func(t *testing.T) {
		view := retable.SingleCellView("", "", json.RawMessage(`{"a":1}`))
		str, _, err := JSONCellFormatter(payload).FormatCell(context.Background(), view, 0, 0)
		require.NoError(t, err)
		inner := strings.TrimSuffix(strings.TrimPrefix(str, "<pre>"), "</pre>")
		require.NotContains(t, inner, "<")
		require.NotContains(t, inner, ">")
	})
}

// TestJSONCellFormatter_NoTrailingBlankLine pins that both forks agree about
// trailing whitespace. json.Indent copies it from its source while json.Compact
// drops it, so an indented cell used to end with the newline json.Encoder.Encode
// appends, or with whatever an already marshaled value carried, and rendered a
// blank line before the closing tag.
func TestJSONCellFormatter_NoTrailingBlankLine(t *testing.T) {
	values := map[string]any{
		"marshaled here":  map[string]int{"a": 1},
		"json.RawMessage": json.RawMessage("{\"a\":1}\n"),
		"string":          "{\"a\":1}\n",
		"[]byte":          []byte("{\"a\":1}\n"),
	}
	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			view := retable.SingleCellView("", "", value)
			str, _, err := JSONCellFormatter(`  `).FormatCell(context.Background(), view, 0, 0)
			require.NoError(t, err)
			require.Equal(t, "<pre>{\n  \"a\": 1\n}</pre>", str)
		})
	}
}
