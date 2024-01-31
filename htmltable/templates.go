package htmltable

import "html/template"

var (
	HeaderTemplate = template.Must(template.New("header").Parse(
		"<table{{if .TableClass}} class='{{.TableClass}}'{{end}}>\n" +
			"{{if .Caption}}  <caption>{{.Caption}}</caption>\n{{end}}",
	))

	RowTemplate = template.Must(template.New("row").Parse("" +
		"{{if .IsHeaderRow}}" +
		"  <tr>{{range $cell := .RawCells}}<th>{{$cell}}</th>{{end}}</tr>\n" +
		"{{else}}" +
		"  <tr>{{range $cell := .RawCells}}<td>{{$cell}}</td>{{end}}</tr>\n" +
		"{{end}}",
	))

	FooterTemplate = template.Must(template.New("footer").Parse(
		"</table>",
	))
)

type TemplateContext struct {
	TableClass string
	Caption    string
}

type RowTemplateContext struct {
	TemplateContext

	IsHeaderRow bool
	RowIndex    int
	RawCells    []template.HTML
}
