package retable

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// countRow has a non string field on purpose: SmartAssign assigns a string
// source to a string destination by direct conversion before it ever asks a
// Scanner, so only a destination that needs parsing reaches dstScanner.
type countRow struct {
	Count int
}

func countView() View {
	return NewStringsView("", [][]string{{"Count"}, {"7"}})
}

// recordingScanner returns a Scanner that records every Parser it is called
// with and leaves the assignment to SmartAssign by reporting an unsupported
// destination.
func recordingScanner(received *[]Parser) Scanner {
	return ScannerFunc(func(dest reflect.Value, str string, parser Parser) error {
		*received = append(*received, parser)
		return errors.ErrUnsupported
	})
}

// TestViewToStructSlice_ScannerGetsOwnParser covers why ViewToStructSlice
// allocates a Parser when a Scanner is passed without one. A Scanner receives
// the Parser and may reconfigure it for the file it is reading. Handing it the
// shared DefaultParser would let one conversion change how every other
// conversion in the process parses, including ones running concurrently.
func TestViewToStructSlice_ScannerGetsOwnParser(t *testing.T) {
	t.Run("a scanner never receives the shared DefaultParser", func(t *testing.T) {
		var received []Parser
		_, err := ViewToStructSlice[countRow](countView(), &StructFieldNaming{}, recordingScanner(&received), nil, nil, nil)
		require.NoError(t, err)
		require.Len(t, received, 1, "the scanner has to be called for the int column")
		require.NotSame(t, DefaultParser, received[0])
	})

	t.Run("reconfiguring the received parser leaves DefaultParser intact", func(t *testing.T) {
		// A Scanner that strips the parser of everything it knows.
		// Had it been handed DefaultParser, every later conversion in
		// the process would stop recognizing booleans and nil strings.
		wrecker := ScannerFunc(func(dest reflect.Value, str string, parser Parser) error {
			if p, ok := parser.(*StringParser); ok {
				p.TrueStrings = nil
				p.FalseStrings = nil
				p.NilStrings = nil
				p.TimeFormats = nil
			}
			return errors.ErrUnsupported
		})
		_, err := ViewToStructSlice[countRow](countView(), &StructFieldNaming{}, wrecker, nil, nil, nil)
		require.NoError(t, err)

		b, err := DefaultParser.ParseBool("true")
		require.NoError(t, err, "DefaultParser must still know its boolean strings")
		require.True(t, b)
		require.True(t, DefaultParser.IsNil(""), "DefaultParser must still know its nil strings")
	})

	t.Run("each call gets its own parser", func(t *testing.T) {
		var first, second []Parser
		_, err := ViewToStructSlice[countRow](countView(), &StructFieldNaming{}, recordingScanner(&first), nil, nil, nil)
		require.NoError(t, err)
		_, err = ViewToStructSlice[countRow](countView(), &StructFieldNaming{}, recordingScanner(&second), nil, nil, nil)
		require.NoError(t, err)
		require.NotSame(t, first[0], second[0])
	})

	t.Run("an explicitly passed parser is handed to the scanner unchanged", func(t *testing.T) {
		// The caller has to be able to configure parsing for its source
		// format, which only works if its Parser reaches the Scanner.
		parser := NewStringParser()
		var received []Parser
		_, err := ViewToStructSlice[countRow](countView(), &StructFieldNaming{}, recordingScanner(&received), parser, nil, nil)
		require.NoError(t, err)
		require.Same(t, parser, received[0])
	})
}

// TestViewToStructSlice_PassedParserParsesCells covers the passed Parser being
// used for the built-in conversions and not only for a Scanner, which is what
// makes a view written in another locale or language readable at all.
func TestViewToStructSlice_PassedParserParsesCells(t *testing.T) {
	type paidRow struct {
		Paid bool
	}
	view := NewStringsView("", [][]string{{"Paid"}, {"ja"}})
	naming := &StructFieldNaming{}

	// Without a Parser the default English boolean strings are used,
	// and a cell they do not know must be reported instead of silently
	// becoming the zero value of the field.
	_, err := ViewToStructSlice[paidRow](view, naming, nil, nil, nil, nil)
	require.ErrorIs(t, err, errors.ErrUnsupported)

	german := NewStringParser()
	german.TrueStrings = []string{"ja"}
	german.FalseStrings = []string{"nein"}

	rows, err := ViewToStructSlice[paidRow](view, naming, nil, german, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []paidRow{{Paid: true}}, rows)
}

// TestViewToStructSlice_RequiredColumns covers the requiredCols check, which
// exists so that a view missing a column fails once up front instead of
// silently producing structs with a zero valued field in every row.
func TestViewToStructSlice_RequiredColumns(t *testing.T) {
	type person struct {
		Name string
	}
	view := NewStringsView("", [][]string{{"Name", "Age"}, {"Alice", "30"}})
	naming := &StructFieldNaming{}

	t.Run("column missing in the view", func(t *testing.T) {
		_, err := ViewToStructSlice[person](view, naming, nil, nil, nil, nil, "Name", "City")
		require.ErrorContains(t, err, `required column "City" not found in View columns`)
	})

	t.Run("column missing as struct field", func(t *testing.T) {
		// "Age" is a column of the view but not a field of person,
		// so the data of that column would be dropped without notice.
		_, err := ViewToStructSlice[person](view, naming, nil, nil, nil, nil, "Age")
		require.ErrorContains(t, err, `required column "Age" not found as struct field`)
	})

	t.Run("column present in both", func(t *testing.T) {
		rows, err := ViewToStructSlice[person](view, naming, nil, nil, nil, nil, "Name")
		require.NoError(t, err)
		require.Equal(t, []person{{Name: "Alice"}}, rows)
	})

	t.Run("pointer element type is checked against the struct behind it", func(t *testing.T) {
		rows, err := ViewToStructSlice[*person](view, naming, nil, nil, nil, nil, "Name")
		require.NoError(t, err)
		require.Equal(t, []*person{{Name: "Alice"}}, rows)
	})
}

// TestViewToStructSlice_NonStructElementType pins the type check that reports
// an element type the function cannot fill, because reflection would otherwise
// only fail deep inside the assignment loop.
func TestViewToStructSlice_NonStructElementType(t *testing.T) {
	view := NewStringsView("", [][]string{{"Name"}, {"Alice"}})

	_, err := ViewToStructSlice[string](view, &StructFieldNaming{}, nil, nil, nil, nil)
	require.ErrorContains(t, err, "is not a struct or pointer to struct")

	_, err = ViewToStructSlice[*string](view, &StructFieldNaming{}, nil, nil, nil, nil)
	require.ErrorContains(t, err, "is not a struct or pointer to struct")
}
