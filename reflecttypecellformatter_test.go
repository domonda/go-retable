package retable

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// nilCellFormatters returns one ReflectTypeCellFormatter per dispatch
// strategy, each configured to format a string cell with the passed
// formatter, so that every strategy can be checked against the same cell.
func nilCellFormatters(formatter CellFormatter) map[string]*ReflectTypeCellFormatter {
	return map[string]*ReflectTypeCellFormatter{
		"no formatter":        NewReflectTypeCellFormatter(),
		"type formatter":      NewReflectTypeCellFormatter().WithTypeFormatter(reflect.TypeFor[string](), formatter),
		"kind formatter":      NewReflectTypeCellFormatter().WithKindFormatter(reflect.String, formatter),
		"interface formatter": NewReflectTypeCellFormatter().WithInterfaceTypeFormatter(reflect.TypeFor[error](), formatter),
		"default formatter":   NewReflectTypeCellFormatter().WithDefaultFormatter(formatter),
	}
}

// TestReflectTypeCellFormatter_FormatCell_NilInterfaceCell covers a cell that
// holds a nil interface. Every dispatch strategy of ReflectTypeCellFormatter
// keys on the reflect.Type of the cell value, which a nil interface does not
// have, so reading its type used to panic on the invalid reflect.Value and
// take down whoever was writing the table.
//
// Reporting errors.ErrUnsupported instead hands the cell back to the caller
// unformatted, which is what lets a writer treat it as a null value. That also
// holds for a configured default formatter: it is a fallback for a value whose
// type matched nothing, not a handler for a cell that has no value to format.
func TestReflectTypeCellFormatter_FormatCell_NilInterfaceCell(t *testing.T) {
	formatter := CellFormatterFunc(func(context.Context, View, int, int) (string, bool, error) {
		return "FORMATTED", false, nil
	})
	// Column A holds a string so the formatters below are known to match
	// something, column B holds the nil interface under test.
	view := &AnyValuesView{
		Cols: []string{"A", "B"},
		Rows: [][]any{{"a", nil}},
	}

	for name, f := range nilCellFormatters(formatter) {
		t.Run(name, func(t *testing.T) {
			var (
				str string
				raw bool
				err error
			)
			require.NotPanics(t, func() {
				str, raw, err = f.FormatCell(context.Background(), view, 0, 1)
			}, "a nil interface cell must not panic")
			require.ErrorIs(t, err, errors.ErrUnsupported)
			require.Empty(t, str)
			require.False(t, raw)
		})
	}
}

// TestReflectTypeCellFormatter_FormatCell_TypedCell is the counterpart of the
// test above: it proves the formatters used there do fire for a cell that has
// a type, so the unsupported result for the nil cell cannot pass for the
// trivial reason that nothing was configured to format it.
func TestReflectTypeCellFormatter_FormatCell_TypedCell(t *testing.T) {
	formatter := CellFormatterFunc(func(context.Context, View, int, int) (string, bool, error) {
		return "FORMATTED", false, nil
	})
	view := &AnyValuesView{
		Cols: []string{"A", "B"},
		Rows: [][]any{{"a", nil}},
	}

	for name, f := range nilCellFormatters(formatter) {
		t.Run(name, func(t *testing.T) {
			str, _, err := f.FormatCell(context.Background(), view, 0, 0)
			if name == "no formatter" || name == "interface formatter" {
				// A string neither has a formatter registered
				// nor implements the error interface.
				require.ErrorIs(t, err, errors.ErrUnsupported)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "FORMATTED", str)
		})
	}
}
