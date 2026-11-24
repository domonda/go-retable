package retable

import (
	"errors"
	"reflect"
	"strings"
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

func TestViewToStructSlice_TitleMatcher(t *testing.T) {
	type person struct {
		Name string `col:"Name"`
		Age  int    `col:"Age"`
	}
	view := NewStringsView("", [][]string{{" name ", "AGE"}, {"Alice", "30"}})
	tests := []struct {
		name         string
		naming       *StructFieldNaming
		requiredCols []string
		want         []person
		wantErr      string
	}{
		{
			name:   "exact matching assigns no column",
			naming: &StructFieldNaming{Tag: "col"},
			want:   []person{{}},
		},
		{
			name:         "exact matching misses required column",
			naming:       &StructFieldNaming{Tag: "col"},
			requiredCols: []string{"Name"},
			wantErr:      `required column "Name" not found in View columns`,
		},
		{
			name:   "TrimEqualFold assigns all columns",
			naming: &StructFieldNaming{Tag: "col", TitleMatcher: TrimEqualFold},
			want:   []person{{Name: "Alice", Age: 30}},
		},
		{
			name:         "TrimEqualFold finds required columns in view and struct",
			naming:       &StructFieldNaming{Tag: "col", TitleMatcher: TrimEqualFold},
			requiredCols: []string{"NAME", "age"},
			want:         []person{{Name: "Alice", Age: 30}},
		},
		{
			name:         "TrimEqualFold missing required column",
			naming:       &StructFieldNaming{Tag: "col", TitleMatcher: TrimEqualFold},
			requiredCols: []string{"City"},
			wantErr:      `required column "City" not found in View columns`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := ViewToStructSlice[person](view, tt.naming, nil, nil, nil, nil, tt.requiredCols...)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, rows)
		})
	}
}

// TestViewToStructSlice_TitleMatcherDuplicateFieldTitles covers that two fields
// whose titles match each other under TitleMatcher are an error and not a
// silently wrong struct: every column matching both titles would be assigned
// to the first field and the second one would never be filled.
func TestViewToStructSlice_TitleMatcherDuplicateFieldTitles(t *testing.T) {
	type record struct {
		ID string `col:"ID"`
		Id string `col:"Id"`
	}
	view := NewStringsView("", [][]string{{"first", "second"}}, "ID", "Id")
	exact := &StructFieldNaming{Tag: "col"}

	// The titles differ for exact matching, which is unchanged
	records, err := ViewToStructSlice[record](view, exact, nil, nil, nil, nil, "ID", "Id")
	require.NoError(t, err)
	require.Equal(t, []record{{ID: "first", Id: "second"}}, records)

	loose := exact.WithTitleMatcher(TrimEqualFold)
	records, err = ViewToStructSlice[record](view, loose, nil, nil, nil, nil)
	require.ErrorIs(t, err, ErrDuplicateColumnTitle)
	require.ErrorContains(t, err, `"ID" and "Id" of retable.record`)
	require.Nil(t, records)

	// Also for a slice of struct pointers and for a view without rows,
	// because the struct and the naming are wrong whatever the view holds
	pointers, err := ViewToStructSlice[*record](NewStringsView("", nil, "ID"), loose, nil, nil, nil, nil)
	require.ErrorIs(t, err, ErrDuplicateColumnTitle)
	require.Nil(t, pointers)
}

// TestViewToStructSlice_EqualFieldTitlesExactMatching pins that equal field
// titles stay accepted without TitleMatcher, where the first field is filled,
// so that the duplicate check does not change existing callers.
func TestViewToStructSlice_EqualFieldTitlesExactMatching(t *testing.T) {
	type record struct {
		A string `col:"Name"`
		B string `col:"Name"`
	}
	view := NewStringsView("", [][]string{{"x"}}, "Name")
	records, err := ViewToStructSlice[record](view, &StructFieldNaming{Tag: "col"}, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []record{{A: "x"}}, records)
}

// TestViewToStructSlice_SeveralColumnsMatchOneField covers which of several
// columns matching one struct field fills it: the closest match whatever the
// column order, and of equally close columns the cell with content.
// A wrong choice here imports the value of another column without any error.
func TestViewToStructSlice_SeveralColumnsMatchOneField(t *testing.T) {
	type booking struct {
		Amount string `col:"Betrag"`
		Text   string `col:"Text"`
	}
	exact := &StructFieldNaming{Tag: "col"}
	tests := []struct {
		name   string
		naming *StructFieldNaming
		cols   []string
		rows   [][]string
		want   []booking
	}{
		{
			name:   "equal name wins over looser matches before and after it",
			naming: exact.WithTitleMatcher(TrimEqualFold),
			cols:   []string{"BETRAG", " Betrag ", "Betrag", "betrag", "Text"},
			rows:   [][]string{{"upper", "spaced", "equal", "lower", "t"}, {"upper", "spaced", "", "lower", "t"}},
			// The empty cell of the equal column is not replaced by a looser match
			want: []booking{{Amount: "equal", Text: "t"}, {Amount: "", Text: "t"}},
		},
		{
			name:   "equal without whitespace wins over differing in case",
			naming: exact.WithTitleMatcher(TrimEqualFold),
			cols:   []string{"BETRAG", "Betrag ", "betrag"},
			rows:   [][]string{{"upper", "spaced", "lower"}},
			want:   []booking{{Amount: "spaced"}},
		},
		{
			name: "a match differing in case wins over one only TitleMatcher accepts",
			naming: exact.WithTitleMatcher(func(viewTitle, fieldTitle string) bool {
				return TrimEqualFold(viewTitle, fieldTitle) || (fieldTitle == "Betrag" && viewTitle == "Umsatz")
			}),
			cols: []string{"Umsatz", "BETRAG", "Umsatz"},
			rows: [][]string{{"alternative", "upper", "alternative"}},
			want: []booking{{Amount: "upper"}},
		},
		{
			name:   "equally close columns: the cell with content wins, the last one of several",
			naming: exact.WithTitleMatcher(TrimEqualFold),
			cols:   []string{"BETRAG", "betrag", "Text"},
			rows: [][]string{
				{"first", "", "content then empty"},
				{"", "second", "empty then content"},
				{"first", "second", "both"},
				{"", " \t", "none"},
				{"first", " \t", "content then whitespace"},
			},
			want: []booking{
				{Amount: "first", Text: "content then empty"},
				{Amount: "second", Text: "empty then content"},
				{Amount: "second", Text: "both"},
				{Amount: " \t", Text: "none"},
				{Amount: "first", Text: "content then whitespace"},
			},
		},
		{
			name:   "equally close columns with PreferFirstColumn: the first cell with content",
			naming: exact.WithTitleMatcher(TrimEqualFold).WithPreferFirstColumn(true),
			cols:   []string{"BETRAG", "betrag", "Text", "bETRAG"},
			rows: [][]string{
				{"first", "", "content then empty", ""},
				{"", "second", "empty then content", "third"},
				{"first", "second", "all", "third"},
				{"", " \t", "none", " "},
			},
			want: []booking{
				{Amount: "first", Text: "content then empty"},
				{Amount: "second", Text: "empty then content"},
				{Amount: "first", Text: "all"},
				{Amount: "", Text: "none"},
			},
		},
		{
			name:   "PreferFirstColumn does not prefer a looser match",
			naming: exact.WithTitleMatcher(TrimEqualFold).WithPreferFirstColumn(true),
			cols:   []string{"BETRAG", "Betrag", "betrag", "Betrag"},
			rows:   [][]string{{"upper", "equal 1", "lower", "equal 2"}, {"upper", "", "lower", "equal 2"}},
			want:   []booking{{Amount: "equal 1"}, {Amount: "equal 2"}},
		},
		{
			name:   "columns of the same name without TitleMatcher and with PreferFirstColumn",
			naming: exact.WithPreferFirstColumn(true),
			cols:   []string{"Betrag", "Text", "Betrag"},
			rows:   [][]string{{"first", "both", "second"}, {"", "empty then content", "second"}},
			want:   []booking{{Amount: "first", Text: "both"}, {Amount: "second", Text: "empty then content"}},
		},
		{
			name:   "columns of the same name without TitleMatcher",
			naming: exact,
			cols:   []string{"Betrag", "Text", "Betrag"},
			rows: [][]string{
				{"first", "content then empty", ""},
				{"", "empty then content", "second"},
				{"first", "both", "second"},
			},
			want: []booking{
				{Amount: "first", Text: "content then empty"},
				{Amount: "second", Text: "empty then content"},
				{Amount: "second", Text: "both"},
			},
		},
		{
			name:   "TrimEqual ignores a column differing in case",
			naming: exact.WithTitleMatcher(TrimEqual),
			cols:   []string{" Betrag", "BETRAG", "Text"},
			rows:   [][]string{{"spaced", "upper", "t"}},
			want:   []booking{{Amount: "spaced", Text: "t"}},
		},
		{
			name:   "without TitleMatcher only the equal name matches",
			naming: exact,
			cols:   []string{" Betrag", "BETRAG", "Betrag"},
			rows:   [][]string{{"spaced", "upper", "equal"}},
			want:   []booking{{Amount: "equal"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not NewStringsView, which trims the column names
			view := &StringsView{Cols: tt.cols, Rows: tt.rows}
			got, err := ViewToStructSlice[booking](view, tt.naming, nil, nil, nil, nil)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestViewToStructSlice_EmptyCellsOfEquallyCloseColumns covers what counts
// as a cell without content for columns of the same name: null-like values
// of any type, not only empty strings.
func TestViewToStructSlice_EmptyCellsOfEquallyCloseColumns(t *testing.T) {
	type record struct {
		Count *int
	}
	one := 1
	view := &AnyValuesView{Cols: []string{"Count", "Count"}, Rows: [][]any{
		{&one, (*int)(nil)},
		{nil, &one},
		{(*int)(nil), nil},
	}}
	got, err := ViewToStructSlice[record](view, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []record{{Count: &one}, {Count: &one}, {Count: nil}}, got)
}

// TestViewToStructSlice_ValidatesTheAssignedCellOnly covers that only the
// cell a field is assigned from is converted and validated, so that a cell
// of another column of the same name that loses can't fail the import.
func TestViewToStructSlice_ValidatesTheAssignedCellOnly(t *testing.T) {
	type record struct {
		Count int `col:"Count"`
	}
	view := &StringsView{Cols: []string{"Count", "COUNT", "Count"}, Rows: [][]string{{"not a number", "neither", "3"}}}
	naming := &StructFieldNaming{Tag: "col", TitleMatcher: TrimEqualFold}
	validated := 0
	got, err := ViewToStructSlice[record](view, naming, nil, nil, nil, func(reflect.Value) error {
		validated++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []record{{Count: 3}}, got)
	require.Equal(t, 1, validated)
}

// TestViewToStructSlice_DisallowAmbiguousTitles covers that a naming with
// DisallowAmbiguousTitles rejects every view whose columns can be assigned
// to the fields in more than one way, instead of choosing by the rules.
func TestViewToStructSlice_DisallowAmbiguousTitles(t *testing.T) {
	type booking struct {
		Amount string `col:"Betrag"`
		Text   string `col:"Text"`
	}
	exact := (&StructFieldNaming{Tag: "col"}).WithDisallowAmbiguousTitles(true)
	loose := exact.WithTitleMatcher(TrimEqualFold)
	tests := []struct {
		name    string
		naming  *StructFieldNaming
		cols    []string
		wantErr string // empty for no error
	}{
		{name: "unambiguous", naming: loose, cols: []string{"BETRAG", "Valuta", " text "}},
		{name: "columns differing in case", naming: loose, cols: []string{"Betrag", "Text", "BETRAG"}, wantErr: `columns "Betrag" and "BETRAG" both match "Betrag" of retable.booking`},
		{name: "columns of the same name", naming: loose, cols: []string{"Text", "Text"}, wantErr: `columns "Text" and "Text" both match "Text" of retable.booking`},
		{name: "columns of the same name without TitleMatcher", naming: exact, cols: []string{"Betrag", "Betrag"}, wantErr: `columns "Betrag" and "Betrag" both match "Betrag" of retable.booking`},
		{name: "other case is another column without TitleMatcher", naming: exact, cols: []string{"Betrag", "BETRAG"}},
		{name: "other case is another column for TrimEqual", naming: exact.WithTitleMatcher(TrimEqual), cols: []string{"Betrag", "BETRAG"}},
		{name: "columns differing in whitespace for TrimEqual", naming: exact.WithTitleMatcher(TrimEqual), cols: []string{"Betrag", " Betrag"}, wantErr: `columns "Betrag" and " Betrag" both match "Betrag"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := make([]string, len(tt.cols))
			view := &StringsView{Cols: tt.cols, Rows: [][]string{row}}
			got, err := ViewToStructSlice[booking](view, tt.naming, nil, nil, nil, nil)
			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Len(t, got, 1)
				return
			}
			require.ErrorIs(t, err, ErrAmbiguousColumnTitle)
			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, got)

			// Without the option the same view is read by the rules
			_, err = ViewToStructSlice[booking](view, tt.naming.WithDisallowAmbiguousTitles(false), nil, nil, nil, nil)
			require.NoError(t, err)
		})
	}

	t.Run("one column matches two fields", func(t *testing.T) {
		view := NewStringsView("", [][]string{{"12"}}, "Summe")
		got, err := ViewToStructSlice[amounts](view, sameAlternative, nil, nil, nil, nil)
		require.NoError(t, err)
		require.Equal(t, []amounts{{Amount: "12"}}, got, "the first field gets it by default")

		pointers, err := ViewToStructSlice[*amounts](view, sameAlternative.WithDisallowAmbiguousTitles(true), nil, nil, nil, nil)
		require.ErrorIs(t, err, ErrAmbiguousColumnTitle)
		require.ErrorContains(t, err, `column "Summe" matches both "Betrag" and "Total" of retable.amounts`)
		require.Nil(t, pointers)
	})
}

// TestViewToStructSlice_RequiredColumnMatchesTheField covers that a required
// column has to match the title of a struct field itself. A TitleMatcher that
// is no equivalence relation, here a view title containing the field title,
// can match a view column to a field whose title the required column does
// not match, which must not count as the required field.
func TestViewToStructSlice_RequiredColumnMatchesTheField(t *testing.T) {
	type amount struct {
		Net string `col:"Net"`
	}
	naming := &StructFieldNaming{Tag: "col", TitleMatcher: strings.Contains}
	view := NewStringsView("", [][]string{{"5"}}, "Net Amount")

	_, err := ViewToStructSlice[amount](view, naming, nil, nil, nil, nil, "Amount")
	require.EqualError(t, err, `required column "Amount" not found as struct field`)

	rows, err := ViewToStructSlice[amount](view, naming, nil, nil, nil, nil, "Net")
	require.NoError(t, err)
	require.Equal(t, []amount{{Net: "5"}}, rows)
}

// panickingIsZero is a cell value whose IsZero method panics.
type panickingIsZero struct{}

func (panickingIsZero) IsZero() bool { panic("IsZero must not be called") }

// TestViewToStructSlice_SingleColumnCellMethodsNotCalled covers that the cell
// of a field with one matching column is assigned without calling methods of
// the cell value to find out if it has content, which is only needed to
// choose between several columns and which v1.0.0-beta.3 never did.
func TestViewToStructSlice_SingleColumnCellMethodsNotCalled(t *testing.T) {
	type row struct {
		A any
	}
	view := &AnyValuesView{Cols: []string{"A"}, Rows: [][]any{{panickingIsZero{}}}}
	rows, err := ViewToStructSlice[row](view, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []row{{A: panickingIsZero{}}}, rows)
}

// embeddedString is a non struct type embedded in a row struct.
type embeddedString string

// TestViewToStructSlice_EmbeddedNonStructType covers that an embedded type
// that is no struct, which has no column title, is skipped instead of
// panicking while the fields are searched for the columns of the view.
func TestViewToStructSlice_EmbeddedNonStructType(t *testing.T) {
	type row struct {
		Name string
		embeddedString
	}
	view := NewStringsView("", [][]string{{"x", "y"}}, "Name", "Other")
	for name, naming := range map[string]*StructFieldNaming{
		"nil naming":             nil,
		"TrimEqualFold matching": {TitleMatcher: TrimEqualFold}, // also checks the titles with CheckUniqueColumns
	} {
		t.Run(name, func(t *testing.T) {
			rows, err := ViewToStructSlice[row](view, naming, nil, nil, nil, nil)
			require.NoError(t, err)
			require.Equal(t, []row{{Name: "x"}}, rows)
		})
	}
}
