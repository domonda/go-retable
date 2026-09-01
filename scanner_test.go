package retable

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// scannerFor returns a Scanner that handles exactly dstType by setting
// the passed value, and reports errors.ErrUnsupported for every other
// type, which is the contract a Scanner has to follow to be chainable.
func scannerFor(dstType reflect.Type, set any) ScannerFunc {
	return func(dest reflect.Value, str string, _ Parser) error {
		if dest.Type() != dstType {
			return errors.ErrUnsupported
		}
		dest.Set(reflect.ValueOf(set))
		return nil
	}
}

func TestScanners(t *testing.T) {
	failing := ScannerFunc(func(reflect.Value, string, Parser) error {
		return fmt.Errorf("scanner failed")
	})
	unsupported := ScannerFunc(func(reflect.Value, string, Parser) error {
		return errors.ErrUnsupported
	})

	t.Run("empty chain reports unsupported so SmartAssign continues", func(t *testing.T) {
		var dst int
		err := Scanners{}.ScanString(reflect.ValueOf(&dst).Elem(), "42", DefaultParser)
		require.ErrorIs(t, err, errors.ErrUnsupported)
	})

	t.Run("unsupported falls through to the next scanner", func(t *testing.T) {
		var dst int
		chain := Scanners{unsupported, scannerFor(reflect.TypeFor[int](), 7)}
		require.NoError(t, chain.ScanString(reflect.ValueOf(&dst).Elem(), "42", DefaultParser))
		require.Equal(t, 7, dst)
	})

	t.Run("the first scanner that handles the type wins", func(t *testing.T) {
		var dst int
		chain := Scanners{
			scannerFor(reflect.TypeFor[int](), 1),
			scannerFor(reflect.TypeFor[int](), 2),
		}
		require.NoError(t, chain.ScanString(reflect.ValueOf(&dst).Elem(), "42", DefaultParser))
		require.Equal(t, 1, dst, "an earlier scanner overrides a later one")
	})

	// A parsing failure is not the same as an unsupported type, so it
	// must not be retried by the next scanner with a different result.
	t.Run("a real error stops the chain", func(t *testing.T) {
		var dst int
		chain := Scanners{failing, scannerFor(reflect.TypeFor[int](), 7)}
		err := chain.ScanString(reflect.ValueOf(&dst).Elem(), "42", DefaultParser)
		require.ErrorContains(t, err, "scanner failed")
		require.Zero(t, dst)
	})
}

// TestStrictEmptyStrings covers which destinations an empty string is
// rejected for. The point of rejecting them is that their zero value is
// indistinguishable from a parsed value, so an empty cell would silently
// become a 0 that nothing downstream can tell apart from a real 0.
func TestStrictEmptyStrings(t *testing.T) {
	scanner := Scanners{StrictEmptyStrings}

	rejected := []struct {
		name string
		dst  any
	}{
		{name: "int", dst: new(int)},
		{name: "int64", dst: new(int64)},
		{name: "uint8", dst: new(uint8)},
		{name: "float64", dst: new(float64)},
		{name: "bool", dst: new(bool)},
		{name: "time.Time", dst: new(time.Time)},
		{name: "time.Duration", dst: new(time.Duration)},
	}
	for _, tt := range rejected {
		t.Run("rejects empty string for "+tt.name, func(t *testing.T) {
			dst := reflect.ValueOf(tt.dst).Elem()
			err := SmartAssign(dst, reflect.ValueOf(""), scanner, nil, nil)
			require.ErrorContains(t, err, "cannot assign an empty string to")
			require.NotErrorIs(t, err, errors.ErrUnsupported, "the error must stop SmartAssign")
		})
	}

	// A pointer already states that the column is optional, so an empty
	// cell stays the nil that SmartAssign assigns for it.
	t.Run("keeps nil for a pointer destination", func(t *testing.T) {
		dst := new(*int)
		*dst = new(int)
		**dst = 5
		require.NoError(t, SmartAssign(reflect.ValueOf(dst).Elem(), reflect.ValueOf(""), scanner, nil, nil))
		require.Nil(t, *dst)
	})

	t.Run("keeps nil for a pointer to pointer destination", func(t *testing.T) {
		dst := new(**int)
		require.NoError(t, SmartAssign(reflect.ValueOf(dst).Elem(), reflect.ValueOf(""), scanner, nil, nil))
		require.Nil(t, *dst)
	})

	// Destinations that can hold the empty string itself are unaffected,
	// there is nothing ambiguous about assigning "" to them.
	accepted := []struct {
		name string
		dst  any
		want any
	}{
		{name: "string", dst: new(string), want: ""},
		{name: "any", dst: new(any), want: any("")},
		{name: "[]byte", dst: new([]byte), want: []byte("")},
	}
	for _, tt := range accepted {
		t.Run("accepts empty string for "+tt.name, func(t *testing.T) {
			dst := reflect.ValueOf(tt.dst).Elem()
			require.NoError(t, SmartAssign(dst, reflect.ValueOf(""), scanner, nil, nil))
			require.Equal(t, tt.want, dst.Interface())
		})
	}

	t.Run("does not affect non-empty strings", func(t *testing.T) {
		var i int
		require.NoError(t, SmartAssign(reflect.ValueOf(&i).Elem(), reflect.ValueOf("42"), scanner, nil, nil))
		require.Equal(t, 42, i)
	})

	// Without the scanner the empty cell is the zero value, which is the
	// behavior StrictEmptyStrings exists to opt out of.
	t.Run("without it an empty string is the zero value", func(t *testing.T) {
		var i int
		require.NoError(t, SmartAssign(reflect.ValueOf(&i).Elem(), reflect.ValueOf(""), nil, nil, nil))
		require.Equal(t, 0, i)
	})
}

// TestStrictEmptyStringsInViewToStructSlice is the case the Scanner is
// meant for: an optional numeric column declared as a value type is a
// wiring mistake that should be reported, and declaring it as a pointer
// is the fix.
func TestStrictEmptyStringsInViewToStructSlice(t *testing.T) {
	view := NewStringsView("", [][]string{
		{"Name", "Amount"},
		{"a", "1"},
		{"b", ""},
	})
	scanner := Scanners{StrictEmptyStrings}

	type RequiredAmount struct {
		Name   string
		Amount int
	}
	_, err := ViewToStructSlice[RequiredAmount](view, nil, scanner, nil, nil, nil)
	require.ErrorContains(t, err, "cannot assign an empty string to int")

	type OptionalAmount struct {
		Name   string
		Amount *int
	}
	rows, err := ViewToStructSlice[OptionalAmount](view, nil, scanner, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.NotNil(t, rows[0].Amount)
	require.Equal(t, 1, *rows[0].Amount)
	require.Nil(t, rows[1].Amount, "the empty cell stays distinguishable from 0")
}
