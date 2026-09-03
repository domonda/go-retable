package htmltable

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func ExampleWriter() {
	type Row struct {
		Status        json.RawMessage `db:"-"              col:"Status"`
		CompanyName   string          `db:"company_name"   col:"Company"`
		InternalNames []string        `db:"internal_names" col:"-"`
		CompanyID     uint64          `db:"company_id"     col:"Company ID"`
	}
	table := []Row{
		{Status: nil, CompanyName: "Company 1", InternalNames: []string{"Company 1a"}, CompanyID: 1},
		{Status: json.RawMessage(`{"ok":true}`), CompanyName: "Company 2", InternalNames: nil, CompanyID: 2},
	}

	NewWriter[[]Row]().
		WithHeaderRow(true).
		WithTypeFormatter(reflect.TypeFor[json.RawMessage](), JSONCellFormatter("")).
		Write(context.Background(), os.Stdout, table, "Table Title")

	// Output:
	// <table>
	//   <caption>Table Title</caption>
	//   <tr><th>Status</th><th>Company</th><th>Company ID</th></tr>
	//   <tr><td></td><td>Company 1</td><td>1</td></tr>
	//   <tr><td><pre>{"ok":true}</pre></td><td>Company 2</td><td>2</td></tr>
	// </table>
}

// TestWriter_WithTemplate checks that the configured templates actually reach the
// output. WithTemplate cloned the writer, assigned all three templates to the clone
// and then returned the receiver, so every custom template was silently discarded
// and the writer kept rendering the default table.
func TestWriter_WithTemplate(t *testing.T) {
	type Row struct {
		Name string
	}
	table := []Row{{Name: "a"}, {Name: "b"}}

	header := template.Must(template.New("header").Parse("<ul>\n"))
	row := template.Must(template.New("row").Parse(
		"{{range $cell := .RawCells}}  <li>{{$cell}}</li>\n{{end}}",
	))
	footer := template.Must(template.New("footer").Parse("</ul>"))

	var buf bytes.Buffer
	err := NewWriter[[]Row]().
		WithTemplate(header, row, footer).
		Write(context.Background(), &buf, table)
	require.NoError(t, err)
	require.Equal(t, "<ul>\n  <li>a</li>\n  <li>b</li>\n</ul>", buf.String())
}

// TestWriter_WithTemplateLeavesReceiverUnchanged checks the immutability contract the
// other With methods follow: the configured writer is a new value and the one it was
// derived from still renders what it did before. That contract is the reason
// WithTemplate clones at all, and returning the receiver broke both halves of it.
func TestWriter_WithTemplateLeavesReceiverUnchanged(t *testing.T) {
	type Row struct {
		Name string
	}
	table := []Row{{Name: "a"}}

	original := NewWriter[[]Row]()
	configured := original.WithTemplate(
		template.Must(template.New("header").Parse("<ul>\n")),
		template.Must(template.New("row").Parse("{{range $cell := .RawCells}}  <li>{{$cell}}</li>\n{{end}}")),
		template.Must(template.New("footer").Parse("</ul>")),
	)
	require.NotSame(t, original, configured)

	var originalBuf, configuredBuf bytes.Buffer
	require.NoError(t, original.Write(context.Background(), &originalBuf, table))
	require.NoError(t, configured.Write(context.Background(), &configuredBuf, table))

	require.Equal(t, "<table>\n  <tr><td>a</td></tr>\n</table>", originalBuf.String())
	require.Equal(t, "<ul>\n  <li>a</li>\n</ul>", configuredBuf.String())
}
