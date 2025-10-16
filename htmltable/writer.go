// Package htmltable provides functionality for writing tables as HTML.
// It supports customizable formatting, HTML templating, and automatic
// HTML escaping for safe output.
//
// The package is built around the Writer type which converts table data
// into HTML table elements with support for:
//   - Custom CSS classes
//   - Column-specific formatters
//   - Type-based formatters
//   - Raw HTML output where needed
//   - Customizable templates
//   - Header rows
//
// Example usage:
//
//	type Person struct {
//	    Name string
//	    Age  int
//	}
//	data := []Person{{"Alice", 30}, {"Bob", 25}}
//
//	writer := htmltable.NewWriter[[]Person]().
//	    WithHeaderRow(true).
//	    WithTableClass("my-table")
//
//	err := writer.Write(ctx, os.Stdout, data, "People")
package htmltable

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"reflect"
	"strings"

	"github.com/domonda/go-retable"
)

// Writer writes table data as HTML table elements.
// It is generic over the table type T and provides extensive
// customization options through its With* methods.
//
// Writer is immutable after creation - all With* methods return
// a new Writer instance with the modified configuration.
//
// HTML Escaping:
// By default, all cell values are HTML-escaped for safety.
// Formatters can return raw HTML by setting the raw return value to true.
type Writer[T any] struct {
	tableClass       string
	viewer           retable.Viewer
	columnFormatters map[int]retable.CellFormatter
	typeFormatters   *retable.ReflectTypeCellFormatter
	nilValue         template.HTML
	headerRow        bool
	headerTemplate   *template.Template
	rowTemplate      *template.Template
	footerTemplate   *template.Template
}

// NewWriter creates a new HTML table writer for type T.
// The writer is initialized with default templates and no formatters.
//
// Default configuration:
//   - No table class
//   - No viewer (uses retable.SelectViewer at write time)
//   - No custom formatters
//   - Empty string for nil values
//   - No header row
//   - Standard HTML table templates
//
// Use the With* methods to customize the writer configuration.
func NewWriter[T any]() *Writer[T] {
	return &Writer[T]{
		tableClass:       "",
		viewer:           nil,
		columnFormatters: make(map[int]retable.CellFormatter),
		typeFormatters:   nil, // OK to use nil retable.TypeFormatters
		nilValue:         "",
		headerRow:        false,
		headerTemplate:   HeaderTemplate,
		rowTemplate:      RowTemplate,
		footerTemplate:   FooterTemplate,
	}
}

// Write writes the table data as HTML to the destination writer.
// It uses the writer's configured viewer if set, otherwise calls
// retable.SelectViewer to choose an appropriate viewer for the table type.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - dest: Where to write the HTML output
//   - table: The table data to write
//   - caption: Optional caption strings that will be joined and used as table caption
//
// Returns an error if the viewer cannot create a view or if writing fails.
//
// Example:
//
//	writer := htmltable.NewWriter[[]Person]()
//	err := writer.Write(ctx, &buf, people, "Employee List")
func (w *Writer[T]) Write(ctx context.Context, dest io.Writer, table T, caption ...string) error {
	viewer := w.viewer
	if viewer == nil {
		var err error
		viewer, err = retable.SelectViewer(table)
		if err != nil {
			return err
		}
	}
	return w.WriteWithViewer(ctx, dest, viewer, table, caption...)
}

// WriteWithViewer writes the table data as HTML using the specified viewer.
// This method allows overriding the writer's configured viewer for a single write operation.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - dest: Where to write the HTML output
//   - viewer: The viewer to use for creating the table view
//   - table: The table data to write
//   - caption: Optional caption strings that will be joined and used as table caption
//
// Returns an error if the viewer cannot create a view or if writing fails.
func (w *Writer[T]) WriteWithViewer(ctx context.Context, dest io.Writer, viewer retable.Viewer, table T, caption ...string) error {
	view, err := viewer.NewView(strings.Join(caption, " "), table)
	if err != nil {
		return err
	}
	return w.WriteView(ctx, dest, view)
}

// WriteView writes a table view as HTML to the destination writer.
// This is the core writing method that handles formatting and HTML generation.
//
// The method processes each cell through the following formatter cascade:
//  1. Column-specific formatters (if configured for the column)
//  2. Type-based formatters (if configured for the cell type)
//  3. Fallback to fmt.Sprint of the cell value
//
// All non-raw formatted values are HTML-escaped for safety.
// The context is checked for cancellation before writing begins.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - dest: Where to write the HTML output
//   - view: The table view to write
//
// Returns an error if context is cancelled, formatting fails, or writing fails.
func (w *Writer[T]) WriteView(ctx context.Context, dest io.Writer, view retable.View) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var (
		columns   = view.Columns()
		numCols   = len(columns)
		templData = &RowTemplateContext{
			TemplateContext: TemplateContext{
				TableClass: w.tableClass,
				Caption:    view.Title(),
			},
			RawCells: make([]template.HTML, numCols),
		}
		reflectView = retable.AsReflectCellView(view)
	)

	err := w.headerTemplate.Execute(dest, templData.TemplateContext)
	if err != nil {
		return err
	}

	if w.headerRow {
		templData.IsHeaderRow = true
		for i := range columns {
			templData.RawCells[i] = template.HTML(template.HTMLEscapeString(columns[i])) //#nosec G203
		}
		err = w.rowTemplate.Execute(dest, templData)
		if err != nil {
			return err
		}
		templData.IsHeaderRow = false
		templData.RowIndex++
	}

	for row, numRows := 0, view.NumRows(); row < numRows; row++ {
		for col := 0; col < numCols; col++ {

			if colFormatter, ok := w.columnFormatters[col]; ok {
				str, isRaw, err := colFormatter.FormatCell(ctx, view, row, col)
				if err != nil && !errors.Is(err, errors.ErrUnsupported) {
					return err
				}
				if err == nil {
					if !isRaw {
						str = template.HTMLEscapeString(str)
					}
					templData.RawCells[col] = template.HTML(str) //#nosec G203
					continue                                     // next column cell
				}
			}

			str, isRaw, err := w.typeFormatters.FormatCell(ctx, view, row, col)
			if err != nil {
				if !errors.Is(err, errors.ErrUnsupported) {
					return err
				}
				// In case of errors.ErrUnsupported
				// use fallback method of formatting
				v := reflectView.ReflectCell(row, col)
				if retable.IsNullLike(v) {
					templData.RawCells[col] = w.nilValue
					continue // next column cell
				}
				if v.Kind() == reflect.Pointer {
					v = v.Elem()
				}
				str, isRaw = fmt.Sprint(v.Interface()), false
			}

			if !isRaw {
				str = template.HTMLEscapeString(str)
			}
			templData.RawCells[col] = template.HTML(str) //#nosec G203
		}

		err = w.rowTemplate.Execute(dest, templData)
		if err != nil {
			return err
		}

		templData.RowIndex++
	}

	return w.footerTemplate.Execute(dest, templData.TemplateContext)
}

func (w *Writer[T]) clone() *Writer[T] {
	c := new(Writer[T])
	*c = *w
	return c
}

// WithHeaderRow returns a new writer with header row configuration.
// When enabled, the first row will be rendered using <th> elements instead of <td>.
//
// Example:
//
//	writer := htmltable.NewWriter[[]Person]().WithHeaderRow(true)
//	// Produces: <tr><th>Name</th><th>Age</th></tr>
func (w *Writer[T]) WithHeaderRow(headerRow bool) *Writer[T] {
	mod := w.clone()
	mod.headerRow = headerRow
	return mod
}

// WithTableClass returns a new writer with the specified CSS class for the table element.
// The class will be rendered as: <table class='tableClass'>
//
// Example:
//
//	writer := htmltable.NewWriter[[]Person]().WithTableClass("table table-striped")
func (w *Writer[T]) WithTableClass(tableClass string) *Writer[T] {
	mod := w.clone()
	mod.tableClass = tableClass
	return mod
}

// WithTableViewer returns a new writer with the specified viewer.
// The viewer is responsible for converting the table data into a retable.View.
// If not set, retable.SelectViewer will be called automatically.
//
// Example:
//
//	viewer := &myCustomViewer{}
//	writer := htmltable.NewWriter[MyTable]().WithTableViewer(viewer)
func (w *Writer[T]) WithTableViewer(viewer retable.Viewer) *Writer[T] {
	mod := w.clone()
	mod.viewer = viewer
	return mod
}

// WithColumnFormatter returns a new writer with the formatter registered for the specified column.
// Column formatters take precedence over type formatters in the formatting cascade.
// If nil is passed as formatter, any previously registered formatter for this column is removed.
//
// Parameters:
//   - columnIndex: Zero-based column index
//   - formatter: The formatter to use for this column, or nil to remove
//
// Example:
//
//	// Format the second column as a percentage
//	writer := htmltable.NewWriter[[]Stats]().
//	    WithColumnFormatter(1, percentFormatter)
func (w *Writer[T]) WithColumnFormatter(columnIndex int, formatter retable.CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.columnFormatters = make(map[int]retable.CellFormatter)
	for key, val := range w.columnFormatters {
		mod.columnFormatters[key] = val
	}
	if formatter != nil {
		mod.columnFormatters[columnIndex] = formatter
	} else {
		delete(mod.columnFormatters, columnIndex)
	}
	return mod
}

// WithColumnFormatterFunc returns a new writer with the formatter function registered for the specified column.
// This is a convenience wrapper around WithColumnFormatter that accepts a function.
// If nil is passed as formatterFunc, any previously registered formatter for this column is removed.
//
// Parameters:
//   - columnIndex: Zero-based column index
//   - formatterFunc: The formatter function to use for this column, or nil to remove
func (w *Writer[T]) WithColumnFormatterFunc(columnIndex int, formatterFunc retable.CellFormatterFunc) *Writer[T] {
	return w.WithColumnFormatter(columnIndex, formatterFunc)
}

// WithRawColumn returns a new writer that interprets the specified column as raw HTML strings.
// Values in this column will not be HTML-escaped, allowing HTML markup to be rendered directly.
//
// Warning: Only use this for trusted content to avoid XSS vulnerabilities.
//
// Parameters:
//   - columnIndex: Zero-based column index
//
// Example:
//
//	// Column 2 contains HTML links
//	writer := htmltable.NewWriter[[]Row]().WithRawColumn(2)
func (w *Writer[T]) WithRawColumn(columnIndex int) *Writer[T] {
	return w.WithColumnFormatter(columnIndex, retable.SprintCellFormatter(true))
}

// WithTypeFormatters returns a new writer with the specified type formatter set.
// This replaces all existing type-based formatters.
//
// Parameters:
//   - formatter: The ReflectTypeCellFormatter to use, or nil to clear all type formatters
func (w *Writer[T]) WithTypeFormatters(formatter *retable.ReflectTypeCellFormatter) *Writer[T] {
	mod := w.clone()
	mod.typeFormatters = formatter
	return mod
}

// WithTypeFormatter returns a new writer with a formatter registered for the specified type.
// Type formatters are used when no column formatter is configured for a cell.
//
// Parameters:
//   - typ: The reflect.Type to format
//   - fmt: The formatter to use for this type
//
// Example:
//
//	// Format all time.Time values
//	timeType := reflect.TypeOf(time.Time{})
//	writer := htmltable.NewWriter[[]Event]().
//	    WithTypeFormatter(timeType, timeFormatter)
func (w *Writer[T]) WithTypeFormatter(typ reflect.Type, fmt retable.CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithTypeFormatter(typ, fmt)
	return mod
}

// WithTypeFormatterFunc returns a new writer with a formatter function registered for the specified type.
// This is a convenience wrapper around WithTypeFormatter that accepts a function.
//
// Parameters:
//   - typ: The reflect.Type to format
//   - fmt: The formatter function to use for this type
func (w *Writer[T]) WithTypeFormatterFunc(typ reflect.Type, fmt retable.CellFormatterFunc) *Writer[T] {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithTypeFormatter(typ, fmt)
	return mod
}

// WithTypeFormatterReflectFunc returns a new writer with a type formatter derived from a function.
// The function signature is used to determine the type to format.
// The function should accept the type to format and return a string.
// Output is HTML-escaped unless WithTypeFormatterReflectRawFunc is used.
//
// Parameters:
//   - function: A function that formats values of a specific type
//
// Example:
//
//	// Function: func(t time.Time) string { return t.Format("2006-01-02") }
//	writer := htmltable.NewWriter[[]Event]().
//	    WithTypeFormatterReflectFunc(formatTime)
//
// Panics if the function signature is invalid.
func (w *Writer[T]) WithTypeFormatterReflectFunc(function any) *Writer[T] {
	fmt, typ, err := retable.ReflectCellFormatterFunc(function, false)
	if err != nil {
		panic(err)
	}
	return w.WithTypeFormatter(typ, fmt)
}

// WithTypeFormatterReflectRawFunc returns a new writer with a type formatter that outputs raw HTML.
// Similar to WithTypeFormatterReflectFunc but the returned string is not HTML-escaped.
//
// Warning: Only use this for trusted content to avoid XSS vulnerabilities.
//
// Parameters:
//   - function: A function that formats values of a specific type and returns HTML
//
// Example:
//
//	// Function: func(u url.URL) string { return fmt.Sprintf("<a href='%s'>%s</a>", u, u) }
//	writer := htmltable.NewWriter[[]Link]().
//	    WithTypeFormatterReflectRawFunc(formatURL)
//
// Panics if the function signature is invalid.
func (w *Writer[T]) WithTypeFormatterReflectRawFunc(function any) *Writer[T] {
	fmt, typ, err := retable.ReflectCellFormatterFunc(function, true)
	if err != nil {
		panic(err)
	}
	return w.WithTypeFormatter(typ, fmt)
}

// WithInterfaceTypeFormatter returns a new writer with a formatter for types implementing an interface.
// The formatter will be used for any cell value that implements the specified interface type.
//
// Parameters:
//   - typ: The interface type (must be an interface)
//   - fmt: The formatter to use for types implementing this interface
//
// Example:
//
//	// Format all fmt.Stringer implementations
//	stringerType := reflect.TypeOf((*fmt.Stringer)(nil)).Elem()
//	writer := htmltable.NewWriter[[]any]().
//	    WithInterfaceTypeFormatter(stringerType, stringerFormatter)
func (w *Writer[T]) WithInterfaceTypeFormatter(typ reflect.Type, fmt retable.CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithInterfaceTypeFormatter(typ, fmt)
	return mod
}

// WithInterfaceTypeFormatterFunc returns a new writer with a formatter function for types implementing an interface.
// This is a convenience wrapper around WithInterfaceTypeFormatter that accepts a function.
//
// Parameters:
//   - typ: The interface type (must be an interface)
//   - fmt: The formatter function to use for types implementing this interface
func (w *Writer[T]) WithInterfaceTypeFormatterFunc(typ reflect.Type, fmt retable.CellFormatterFunc) *Writer[T] {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithInterfaceTypeFormatter(typ, fmt)
	return mod
}

// WithKindFormatter returns a new writer with a formatter for a specific reflect.Kind.
// Kind formatters are the most generic and are used as a last resort before fmt.Sprint.
//
// Parameters:
//   - kind: The reflect.Kind to format
//   - fmt: The formatter to use for this kind
//
// Example:
//
//	// Format all float types with 2 decimal places
//	writer := htmltable.NewWriter[[]Stats]().
//	    WithKindFormatter(reflect.Float64, floatFormatter)
func (w *Writer[T]) WithKindFormatter(kind reflect.Kind, fmt retable.CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithKindFormatter(kind, fmt)
	return mod
}

// WithKindFormatterFunc returns a new writer with a formatter function for a specific reflect.Kind.
// This is a convenience wrapper around WithKindFormatter that accepts a function.
//
// Parameters:
//   - kind: The reflect.Kind to format
//   - fmt: The formatter function to use for this kind
func (w *Writer[T]) WithKindFormatterFunc(kind reflect.Kind, fmt retable.CellFormatterFunc) *Writer[T] {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithKindFormatter(kind, fmt)
	return mod
}

// WithNilValue returns a new writer with the specified HTML to use for nil/null values.
// By default, nil values are rendered as empty strings.
//
// Parameters:
//   - nilValue: The HTML to render for nil values
//
// Example:
//
//	writer := htmltable.NewWriter[[]Person]().
//	    WithNilValue(template.HTML("<em>N/A</em>"))
func (w *Writer[T]) WithNilValue(nilValue template.HTML) *Writer[T] {
	mod := w.clone()
	mod.nilValue = nilValue
	return mod
}

// WithTemplate returns a new writer with custom templates for rendering the HTML table.
// This allows complete control over the HTML structure.
//
// Parameters:
//   - tableTemplate: Template for the opening table tag and optional caption
//   - rowTemplate: Template for rendering each row
//   - footerTemplate: Template for the closing table tag
//
// The templates receive TemplateContext and RowTemplateContext respectively.
// See templates.go for the default templates and context structures.
//
// Example:
//
//	headerTmpl := template.Must(template.New("header").Parse("<table><thead>"))
//	rowTmpl := template.Must(template.New("row").Parse("<tr>...</tr>"))
//	footerTmpl := template.Must(template.New("footer").Parse("</thead></table>"))
//	writer := htmltable.NewWriter[[]Data]().
//	    WithTemplate(headerTmpl, rowTmpl, footerTmpl)
func (w *Writer[T]) WithTemplate(tableTemplate, rowTemplate, footerTemplate *template.Template) *Writer[T] {
	mod := w.clone()
	mod.headerTemplate = tableTemplate
	mod.rowTemplate = rowTemplate
	mod.footerTemplate = footerTemplate
	return w
}

// TableClass returns the CSS class configured for the table element.
// Returns an empty string if no class is configured.
func (w *Writer[T]) TableClass() string {
	return w.tableClass
}

// NilValue returns the HTML configured to be rendered for nil/null values.
// Returns an empty template.HTML if not configured.
func (w *Writer[T]) NilValue() template.HTML {
	return w.nilValue
}
