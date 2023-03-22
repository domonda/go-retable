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

type Writer struct {
	tableClass       string
	viewer           retable.Viewer
	columnFormatters map[int]retable.CellFormatter
	typeFormatters   *retable.TypeFormatters
	nilValue         template.HTML
	headerTemplate   *template.Template
	rowTemplate      *template.Template
	footerTemplate   *template.Template
}

func NewWriter() *Writer {
	return &Writer{
		tableClass:       "",
		columnFormatters: make(map[int]retable.CellFormatter),
		typeFormatters:   nil, // OK to use nil retable.TypeFormatters
		nilValue:         "",
		headerTemplate:   HeaderTemplate,
		rowTemplate:      RowTemplate,
		footerTemplate:   FooterTemplate,
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

func (w *Writer) WithTableViewer(viewer retable.Viewer) *Writer {
	mod := w.clone()
	mod.viewer = viewer
	return mod
}

// WithColumnFormatter returns a new writer with the passed formatter registered for columnIndex.
// If nil is passed as formatter, then a previous registered column formatter is removed.
func (w *Writer) WithColumnFormatter(columnIndex int, formatter retable.CellFormatter) *Writer {
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

// WithColumnFormatterFunc returns a new writer with the passed formatterFunc registered for columnIndex.
// If nil is passed as formatterFunc, then a previous registered column formatter is removed.
func (w *Writer) WithColumnFormatterFunc(columnIndex int, formatterFunc retable.CellFormatterFunc) *Writer {
	return w.WithColumnFormatter(columnIndex, formatterFunc)
}

// WithRawColumn returns a new writer that interprets the collumn
// with columnIndex as raw HTML strings.
func (w *Writer) WithRawColumn(columnIndex int) *Writer {
	return w.WithColumnFormatter(columnIndex, retable.SprintRawCellFormatter())
}

func (w *Writer) WithTypeFormatters(formatter *retable.TypeFormatters) *Writer {
	mod := w.clone()
	mod.typeFormatters = formatter
	return mod
}

func (w *Writer) WithTypeFormatter(typ reflect.Type, fmt retable.CellFormatter) *Writer {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithTypeFormatterFunc(typ reflect.Type, fmt retable.CellFormatterFunc) *Writer {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithTypeFormatterReflectFunc(function interface{}) *Writer {
	fmt, typ, err := retable.ReflectCellFormatterFunc(function, false)
	if err != nil {
		panic(err)
	}
	return w.WithTypeFormatter(typ, fmt)
}

func (w *Writer) WithTypeFormatterReflectRawFunc(function interface{}) *Writer {
	fmt, typ, err := retable.ReflectCellFormatterFunc(function, true)
	if err != nil {
		panic(err)
	}
	return w.WithTypeFormatter(typ, fmt)
}

func (w *Writer) WithInterfaceTypeFormatter(typ reflect.Type, fmt retable.CellFormatter) *Writer {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithInterfaceTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithInterfaceTypeFormatterFunc(typ reflect.Type, fmt retable.CellFormatterFunc) *Writer {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithInterfaceTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer) WithKindFormatter(kind reflect.Kind, fmt retable.CellFormatter) *Writer {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithKindFormatter(kind, fmt)
	return mod
}

func (w *Writer) WithKindFormatterFunc(kind reflect.Kind, fmt retable.CellFormatterFunc) *Writer {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithKindFormatter(kind, fmt)
	return mod
}

func (w *Writer) WithNilValue(nilValue template.HTML) *Writer {
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

func (w *Writer) NilValue() template.HTML {
	return w.nilValue
}

// Write calls WriteView with the result of Viewer.NewView(table)
// using the writer's viewer if not nil or else retable.DefaultViewer.
func (w *Writer) Write(ctx context.Context, dest io.Writer, table any, writeHeaderRow bool, caption ...string) error {
	viewer := w.viewer
	if viewer == nil {
		var err error
		viewer, err = retable.SelectViewer(table)
		if err != nil {
			return err
		}
	}
	view, err := viewer.NewView(table)
	if err != nil {
		return err
	}
	return w.WriteView(ctx, dest, view, writeHeaderRow, caption...)
}

func (w *Writer) WriteView(ctx context.Context, dest io.Writer, view retable.View, writeHeaderRow bool, caption ...string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var (
		columns   = view.Columns()
		templData = &RowTemplateContext{
			TemplateContext: TemplateContext{
				TableClass: w.tableClass,
				Caption:    strings.Join(caption, " "),
			},
			RawCells: make([]template.HTML, len(columns)), // will be reused with updated elements
		}
	)

	err := w.headerTemplate.Execute(dest, templData.TemplateContext)
	if err != nil {
		return err
	}

	if writeHeaderRow {
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

	// cell will be reused with updated Row and Col fields
	cell := retable.Cell{View: view}

	for row, numRows := 0, view.NumRows(); row < numRows; row++ {
		rowVals, err := view.ReflectRow(row)
		if err != nil {
			return err
		}

		cell.Row = row
		for col, val := range rowVals {
			cell.Col = col
			cell.Value = val

			if colFormatter, ok := w.columnFormatters[col]; ok {
				str, isRaw, err := colFormatter.FormatCell(ctx, &cell)
				if err != nil && !errors.Is(err, retable.ErrNotSupported) {
					return err
				}
				if err == nil {
					if !isRaw {
						str = template.HTMLEscapeString(str)
					}
					templData.RawCells[col] = template.HTML(str) //#nosec G203
					continue
				}
			}

			str, isRaw, err := w.typeFormatters.FormatCell(ctx, &cell)
			if err != nil {
				if !errors.Is(err, retable.ErrNotSupported) {
					return err
				}
				// In case of retable.ErrNotSupported
				// use fallback method of formatting
				if retable.ValueIsNil(val) {
					templData.RawCells[col] = w.nilValue
					continue
				}
				if val.Kind() == reflect.Ptr {
					val = val.Elem()
				}
				str, isRaw = fmt.Sprint(val.Interface()), false
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
