package htmltable

import (
	"context"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"reflect"
	"strings"

	"github.com/domonda/go-retable"
)

type Writer struct {
	tableClass     string
	formatter      *retable.TypeFormatters
	nilValue       string
	headerTemplate *template.Template
	rowTemplate    *template.Template
	footerTemplate *template.Template
}

func NewWriter() *Writer {
	return &Writer{
		tableClass:     "",
		formatter:      nil, // OK to use nil retable.TypeFormatters
		nilValue:       "",
		headerTemplate: HeaderTemplate,
		rowTemplate:    RowTemplate,
		footerTemplate: FooterTemplate,
	}
}

func (w *Writer) clone() *Writer {
	c := new(Writer)
	*c = *w
	return c
}

func (w *Writer) WithTableClass(tableClass string) *Writer {
	mod := w.clone()
	mod.tableClass = tableClass
	return mod
}

func (w *Writer) WithTypeFormatters(formatter *retable.TypeFormatters) *Writer {
	mod := w.clone()
	mod.formatter = formatter
	return mod
}

func (w *Writer) WithTypeFormatter(typ reflect.Type, fmt retable.ValueFormatter) *Writer {
	mod := w.clone()
	mod.formatter = w.formatter.WithTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithTypeFormatterFunc(typ reflect.Type, fmt retable.ValueFormatterFunc) *Writer {
	mod := w.clone()
	mod.formatter = w.formatter.WithTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithInterfaceTypeFormatter(typ reflect.Type, fmt retable.ValueFormatter) *Writer {
	mod := w.clone()
	mod.formatter = w.formatter.WithInterfaceTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithInterfaceTypeFormatterFunc(typ reflect.Type, fmt retable.ValueFormatterFunc) *Writer {
	mod := w.clone()
	mod.formatter = w.formatter.WithInterfaceTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithKindFormatter(kind reflect.Kind, fmt retable.ValueFormatter) *Writer {
	mod := w.clone()
	mod.formatter = w.formatter.WithKindFormatter(kind, fmt)
	return mod
}

func (w *Writer) WithKindFormatterFunc(kind reflect.Kind, fmt retable.ValueFormatterFunc) *Writer {
	mod := w.clone()
	mod.formatter = w.formatter.WithKindFormatter(kind, fmt)
	return mod
}

func (w *Writer) WithNilValue(nilValue string) *Writer {
	mod := w.clone()
	mod.nilValue = nilValue
	return mod
}

func (w *Writer) WithTemplate(tableTemplate, rowTemplate, footerTemplate *template.Template) *Writer {
	mod := w.clone()
	mod.headerTemplate = tableTemplate
	mod.rowTemplate = rowTemplate
	mod.footerTemplate = footerTemplate
	return w
}

func (w *Writer) TableClass() string {
	return w.tableClass
}

func (w *Writer) NilValue() string {
	return w.nilValue
}

// Write calls WriteView with the result of retable.DefaultViewer.NewView(table)
func (w *Writer) Write(ctx context.Context, dest io.Writer, table interface{}, writeHeaderRow bool, caption ...string) error {
	view, err := retable.DefaultViewer.NewView(table)
	if err != nil {
		return err
	}
	return w.WriteView(ctx, dest, view, writeHeaderRow, caption...)
}

func (w *Writer) WriteView(ctx context.Context, dest io.Writer, view retable.View, writeHeaderRow bool, caption ...string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Write table header
	captionStr := strings.Join(caption, " ")
	err := w.headerTemplate.Execute(dest, HeaderTemplateContext{TableClass: w.tableClass, Caption: captionStr})
	if err != nil {
		return err
	}

	columns := view.Columns()

	// Write rows

	// rowData will be reused per row with updated data
	rowData := &RowTemplateContext{
		TableClass: w.tableClass,
		RawCells:   make([]template.HTML, len(columns)), // will be reused with updated elements
	}

	if writeHeaderRow {
		rowData.IsHeaderRow = true
		for i := range columns {
			rowData.RawCells[i] = template.HTML(html.EscapeString(columns[i]))
		}
		err = w.rowTemplate.Execute(dest, rowData)
		if err != nil {
			return err
		}
		rowData.IsHeaderRow = false
		rowData.RowIndex++
	}

	// cell will be reused with updated Row and Col fields
	cell := retable.ViewCell{View: view}

	for row, numRows := 0, view.NumRows(); row < numRows; row++ {
		rowVals, err := view.ReflectRow(row)
		if err != nil {
			return err
		}

		cell.Row = row
		for col, val := range rowVals {
			cell.Col = col

			rawFormatter, _ := val.Interface().(RawFormatter)
			if rawFormatter == nil && val.CanAddr() {
				rawFormatter, _ = val.Addr().Interface().(RawFormatter)
			}
			if rawFormatter != nil {
				raw, err := rawFormatter.RawHTML(ctx, &cell)
				if err != nil {
					return err
				}
				rowData.RawCells[col] = raw
				continue
			}

			// No RawFormatter, try retable.TypeFormatters
			str, err := w.formatter.FormatValue(ctx, val, &cell)
			if err != nil {
				if !errors.Is(err, retable.ErrNotSupported) {
					return err
				}
				// In case of retable.ErrNotSupported
				// fall back on nilValue or fmt.Sprint
				switch {
				case isNil(val):
					str = w.nilValue
				case val.Kind() == reflect.Ptr:
					str = fmt.Sprint(val.Elem().Interface())
				default:
					str = fmt.Sprint(val.Interface())
				}
			}
			rowData.RawCells[col] = template.HTML(html.EscapeString(str))
		}

		err = w.rowTemplate.Execute(dest, rowData)
		if err != nil {
			return err
		}

		rowData.RowIndex++
	}

	// Write table footer
	return w.footerTemplate.Execute(dest, FooterTemplateContext{TableClass: w.tableClass, Caption: captionStr})
}

func isNil(val reflect.Value) bool {
	switch val.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return val.IsNil()
	default:
		return false
	}
}
