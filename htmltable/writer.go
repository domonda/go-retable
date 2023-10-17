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

type Writer[T any] struct {
	tableClass       string
	viewer           retable.Viewer
	columnFormatters map[int]retable.CellFormatter
	typeFormatters   *retable.TypeFormatters
	nilValue         template.HTML
	headerTemplate   *template.Template
	rowTemplate      *template.Template
	footerTemplate   *template.Template
}

func NewWriter[T any]() *Writer[T] {
	return &Writer[T]{
		tableClass:       "",
		columnFormatters: make(map[int]retable.CellFormatter),
		typeFormatters:   nil, // OK to use nil retable.TypeFormatters
		nilValue:         "",
		headerTemplate:   HeaderTemplate,
		rowTemplate:      RowTemplate,
		footerTemplate:   FooterTemplate,
	}
}

// Write calls WriteView with the result of Viewer.NewView(table)
// using the writer's viewer if not nil or else retable.DefaultViewer.
func (w *Writer[T]) Write(ctx context.Context, dest io.Writer, table T, writeHeaderRow bool, caption ...string) error {
	viewer := w.viewer
	if viewer == nil {
		var err error
		viewer, err = retable.SelectViewer(table)
		if err != nil {
			return err
		}
	}
	return w.WriteWithViewer(ctx, dest, viewer, table, writeHeaderRow, caption...)
}

func (w *Writer[T]) WriteWithViewer(ctx context.Context, dest io.Writer, viewer retable.Viewer, table T, writeHeaderRow bool, caption ...string) error {
	view, err := viewer.NewView(table)
	if err != nil {
		return err
	}
	return w.WriteView(ctx, dest, view, writeHeaderRow, caption...)
}

func (w *Writer[T]) WriteView(ctx context.Context, dest io.Writer, view retable.View, writeHeaderRow bool, caption ...string) error {
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
				if err != nil && !errors.Is(err, errors.ErrUnsupported) {
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
				if !errors.Is(err, errors.ErrUnsupported) {
					return err
				}
				// In case of errors.ErrUnsupported
				// use fallback method of formatting
				if retable.ValueIsNil(val) {
					templData.RawCells[col] = w.nilValue
					continue
				}
				if val.Kind() == reflect.Pointer {
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

func (w *Writer[T]) clone() *Writer[T] {
	c := new(Writer[T])
	*c = *w
	return c
}

func (w *Writer[T]) WithTableClass(tableClass string) *Writer[T] {
	mod := w.clone()
	mod.tableClass = tableClass
	return mod
}

func (w *Writer[T]) WithTableViewer(viewer retable.Viewer) *Writer[T] {
	mod := w.clone()
	mod.viewer = viewer
	return mod
}

// WithColumnFormatter returns a new writer with the passed formatter registered for columnIndex.
// If nil is passed as formatter, then a previous registered column formatter is removed.
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

// WithColumnFormatterFunc returns a new writer with the passed formatterFunc registered for columnIndex.
// If nil is passed as formatterFunc, then a previous registered column formatter is removed.
func (w *Writer[T]) WithColumnFormatterFunc(columnIndex int, formatterFunc retable.CellFormatterFunc) *Writer[T] {
	return w.WithColumnFormatter(columnIndex, formatterFunc)
}

// WithRawColumn returns a new writer that interprets the collumn
// with columnIndex as raw HTML strings.
func (w *Writer[T]) WithRawColumn(columnIndex int) *Writer[T] {
	return w.WithColumnFormatter(columnIndex, retable.SprintRawCellFormatter())
}

func (w *Writer[T]) WithTypeFormatters(formatter *retable.TypeFormatters) *Writer[T] {
	mod := w.clone()
	mod.typeFormatters = formatter
	return mod
}

func (w *Writer[T]) WithTypeFormatter(typ reflect.Type, fmt retable.CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer[T]) WithTypeFormatterFunc(typ reflect.Type, fmt retable.CellFormatterFunc) *Writer[T] {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer[T]) WithTypeFormatterReflectFunc(function interface{}) *Writer[T] {
	fmt, typ, err := retable.ReflectCellFormatterFunc(function, false)
	if err != nil {
		panic(err)
	}
	return w.WithTypeFormatter(typ, fmt)
}

func (w *Writer[T]) WithTypeFormatterReflectRawFunc(function interface{}) *Writer[T] {
	fmt, typ, err := retable.ReflectCellFormatterFunc(function, true)
	if err != nil {
		panic(err)
	}
	return w.WithTypeFormatter(typ, fmt)
}

func (w *Writer[T]) WithInterfaceTypeFormatter(typ reflect.Type, fmt retable.CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithInterfaceTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer[T]) WithInterfaceTypeFormatterFunc(typ reflect.Type, fmt retable.CellFormatterFunc) *Writer[T] {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithInterfaceTypeFormatter(typ, fmt)
	return mod
}

func (w *Writer[T]) WithKindFormatter(kind reflect.Kind, fmt retable.CellFormatter) *Writer[T] {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithKindFormatter(kind, fmt)
	return mod
}

func (w *Writer[T]) WithKindFormatterFunc(kind reflect.Kind, fmt retable.CellFormatterFunc) *Writer[T] {
	mod := w.clone()
	mod.typeFormatters = w.typeFormatters.WithKindFormatter(kind, fmt)
	return mod
}

func (w *Writer[T]) WithNilValue(nilValue template.HTML) *Writer[T] {
	mod := w.clone()
	mod.nilValue = nilValue
	return mod
}

func (w *Writer[T]) WithTemplate(tableTemplate, rowTemplate, footerTemplate *template.Template) *Writer[T] {
	mod := w.clone()
	mod.headerTemplate = tableTemplate
	mod.rowTemplate = rowTemplate
	mod.footerTemplate = footerTemplate
	return w
}

func (w *Writer[T]) TableClass() string {
	return w.tableClass
}

func (w *Writer[T]) NilValue() template.HTML {
	return w.nilValue
}
