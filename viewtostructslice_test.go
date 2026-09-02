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

	t.Run("the parser a Scanner receives cannot reach DefaultParser", func(t *testing.T) {
		// Checked by identity, not by wrecking the parser and looking
		// for damage. Setting its fields to nil proves nothing, because
		// a nil field falls back to the package defaults, so the
		// assertions afterwards pass whether or not DefaultParser was
		// the object handed over. Writing a non-nil sentinel would
		// prove it, but corrupts the package defaults for every later
		// test in this package when it fails.
		var received Parser
		recorder := ScannerFunc(func(_ reflect.Value, _ string, parser Parser) error {
			received = parser
			return errors.ErrUnsupported
		})
		_, err := ViewToStructSlice[countRow](countView(), &StructFieldNaming{}, recorder, nil, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, received, "the Scanner has to be reached for this to prove anything")

		require.NotSame(t, DefaultParser, received, "a Scanner must not be handed the shared parser")

		got, ok := received.(*StringParser)
		require.True(t, ok)
		def, ok := DefaultParser.(*StringParser)
		require.True(t, ok)

		// Not the same object is not enough: sharing a backing array
		// would let an in-place write reconfigure DefaultParser anyway.
		sameArray := func(x, y []string) bool { return len(x) > 0 && len(y) > 0 && &x[0] == &y[0] }
		require.False(t, sameArray(got.TrueStrings, def.TrueStrings), "TrueStrings must not alias")
		require.False(t, sameArray(got.NilStrings, def.NilStrings), "NilStrings must not alias")
		require.False(t, sameArray(got.TimeFormats, def.TimeFormats), "TimeFormats must not alias")
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

// validatable reports itself invalid for the zero value, which is the
// shape CallValidateMethod is meant for: a column whose type knows what
// a usable value looks like.
type validatableAge int

func (a validatableAge) Valid() bool { return a > 0 }

type validatableName string

func (n validatableName) Validate() error {
	if n == "" {
		return errors.New("name must not be empty")
	}
	return nil
}

// TestCallValidateMethod covers the ready-made validate function, which
// had no test. It dispatches on two different interfaces and has to stay
// silent for a type that implements neither.
func TestCallValidateMethod(t *testing.T) {
	t.Run("Valid() bool", func(t *testing.T) {
		require.NoError(t, CallValidateMethod(reflect.ValueOf(validatableAge(42))))
		require.ErrorContains(t, CallValidateMethod(reflect.ValueOf(validatableAge(0))), "is not valid")
	})

	t.Run("Validate() error", func(t *testing.T) {
		require.NoError(t, CallValidateMethod(reflect.ValueOf(validatableName("Erik"))))
		require.ErrorContains(t, CallValidateMethod(reflect.ValueOf(validatableName(""))), "name must not be empty")
	})

	t.Run("a type implementing neither is not an error", func(t *testing.T) {
		require.NoError(t, CallValidateMethod(reflect.ValueOf(42)))
		require.NoError(t, CallValidateMethod(reflect.ValueOf("")))
	})

	t.Run("an invalid reflect.Value is not an error", func(t *testing.T) {
		require.NoError(t, CallValidateMethod(reflect.Value{}))
	})

	// End to end: a row whose cell fails validation fails the conversion
	t.Run("through ViewToStructSlice", func(t *testing.T) {
		type Row struct {
			Name validatableName
			Age  validatableAge
		}
		ok := NewStringsView("", [][]string{{"Name", "Age"}, {"Erik", "42"}})
		rows, err := ViewToStructSlice[Row](ok, nil, nil, nil, nil, CallValidateMethod)
		require.NoError(t, err)
		require.Equal(t, []Row{{Name: "Erik", Age: 42}}, rows)

		bad := NewStringsView("", [][]string{{"Name", "Age"}, {"Erik", "0"}})
		_, err = ViewToStructSlice[Row](bad, nil, nil, nil, nil, CallValidateMethod)
		require.ErrorContains(t, err, "is not valid")
	})
}

// nameCount has one string and one parsed field so that a missing cell
// is distinguishable from an empty one.
type nameCount struct {
	Name  string
	Count int
}

// TestViewToStructSlice_RaggedRows covers the skip for a cell that has
// no value at all. A short line in a CSV file or a trailing empty
// column in a spreadsheet produces a row with fewer cells than the view
// has columns, and every view of this package reports those positions
// as an invalid reflect.Value. Those fields have to stay at their zero
// value, because failing the whole file over a missing optional column
// is the behaviour the callers of this package read files to avoid.
func TestViewToStructSlice_RaggedRows(t *testing.T) {
	view := &ReflectValuesView{
		Cols: []string{"Name", "Count"},
		Rows: [][]reflect.Value{
			{reflect.ValueOf("Erik"), reflect.ValueOf(7)},
			{reflect.ValueOf("Ann")}, // no Count cell at all
		},
	}

	rows, err := ViewToStructSlice[nameCount](view, &StructFieldNaming{}, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []nameCount{
		{Name: "Erik", Count: 7},
		{Name: "Ann", Count: 0},
	}, rows)
}
