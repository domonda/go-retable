package retable

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSprintFormatter(t *testing.T) {
	var f SprintFormatter

	str, err := f.Format(reflect.ValueOf(42))
	require.NoError(t, err)
	require.Equal(t, "42", str, "a number renders as its digits, not as a rune")

	str, err = f.Format(reflect.ValueOf("text"))
	require.NoError(t, err)
	require.Equal(t, "text", str)

	str, err = f.Format(reflect.ValueOf(true))
	require.NoError(t, err)
	require.Equal(t, "true", str)
}

// UnsupportedFormatter is the "I handle nothing" formatter, which is
// what makes a chain fall through to the next one rather than stop.
func TestUnsupportedFormatter(t *testing.T) {
	var f UnsupportedFormatter

	str, err := f.Format(reflect.ValueOf(42))
	require.ErrorIs(t, err, errors.ErrUnsupported,
		"the error has to be ErrUnsupported so a chain continues past it")
	require.Empty(t, str)
}

func TestFormatterFunc(t *testing.T) {
	var f Formatter = FormatterFunc(func(v reflect.Value) (string, error) {
		return "got:" + v.String(), nil
	})

	str, err := f.Format(reflect.ValueOf("x"))
	require.NoError(t, err)
	require.Equal(t, "got:x", str)
}

// The two adapters convert between the value-shaped Formatter and the
// position-shaped CellFormatter, which is what lets a formatter written
// for one interface be used where the other is expected.
func TestCellFormatterFromFormatter(t *testing.T) {
	view := NewStringsView("", [][]string{{"A"}, {"value"}})

	t.Run("the cell value reaches the wrapped formatter", func(t *testing.T) {
		cf := CellFormatterFromFormatter(FormatterFunc(func(v reflect.Value) (string, error) {
			return "seen:" + v.String(), nil
		}), false)

		str, raw, err := cf.FormatCell(context.Background(), view, 0, 0)
		require.NoError(t, err)
		require.Equal(t, "seen:value", str)
		require.False(t, raw)
	})

	t.Run("rawResult is passed through", func(t *testing.T) {
		cf := CellFormatterFromFormatter(SprintFormatter{}, true)

		_, raw, err := cf.FormatCell(context.Background(), view, 0, 0)
		require.NoError(t, err)
		require.True(t, raw, "a raw formatter's output must stay marked raw")
	})

	t.Run("the wrapped error is passed through", func(t *testing.T) {
		cf := CellFormatterFromFormatter(UnsupportedFormatter{}, false)

		_, _, err := cf.FormatCell(context.Background(), view, 0, 0)
		require.ErrorIs(t, err, errors.ErrUnsupported)
	})
}

func TestFormatterFromCellFormatter(t *testing.T) {
	t.Run("the value reaches the wrapped cell formatter", func(t *testing.T) {
		f := FormatterFromCellFormatter(CellFormatterFunc(
			func(_ context.Context, view View, row, col int) (string, bool, error) {
				return "cell:" + view.Cell(row, col).(string), false, nil
			},
		))

		str, err := f.Format(reflect.ValueOf("value"))
		require.NoError(t, err)
		require.Equal(t, "cell:value", str)
	})

	t.Run("the wrapped error is passed through", func(t *testing.T) {
		f := FormatterFromCellFormatter(UnsupportedCellFormatter{})

		_, err := f.Format(reflect.ValueOf("value"))
		require.ErrorIs(t, err, errors.ErrUnsupported)
	})

	// Round trip: a Formatter wrapped as a CellFormatter and back has
	// to still format the same value, or the two adapters disagree
	// about which position holds the value.
	t.Run("round trip through both adapters", func(t *testing.T) {
		f := FormatterFromCellFormatter(CellFormatterFromFormatter(SprintFormatter{}, false))

		str, err := f.Format(reflect.ValueOf(42))
		require.NoError(t, err)
		require.Equal(t, "42", str)
	})
}
