package htmltable

import "html/template"

var (
	HeaderTemplate = template.Must(template.New("header").Parse(
		"<table{{if .TableClass}} class='{{.TableClass}}'{{end}}>" +
			"{{if .Caption}}<caption>{{.Caption}}</caption>{{end}}",
	))

	RowTemplate = template.Must(template.New("row").Parse(
		"{{if .IsHeaderRow}}" +
			"<tr>{{range $cell := .RawCells}}<th>{{$cell}}</th>{{end}}</tr>" +
			"{{else}}" +
			"<tr>{{range $cell := .RawCells}}<td>{{$cell}}</td>{{end}}</tr>" +
			"{{end}}",
	))

	FooterTemplate = template.Must(template.New("footer").Parse(
		"</table>",
	))
)

type HeaderTemplateContext struct {
	TableClass string
	Caption    string
}

type RowTemplateContext struct {
	TableClass  string
	IsHeaderRow bool
	RowIndex    int
	RawCells    []template.HTML
}

type FooterTemplateContext struct {
	TableClass string
	Caption    string
}
