package retable

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStructFieldNaming_Columns(t *testing.T) {
	type StructWithFloat struct {
		Float float64 `col:"float"`
	}
	tests := []struct {
		name   string
		naming *StructFieldNaming
		strct  any
		want   []string
	}{
		{
			name:   "empty struct, nil naming",
			naming: nil,
			strct:  struct{}{},
			want:   []string{},
		},
		{
			name:   "exported names, nil naming",
			naming: nil,
			strct: struct {
				Int  int
				Bool bool
			}{},
			want: []string{"Int", "Bool"},
		},
		{
			name:   "exported and private names, nil naming",
			naming: nil,
			strct: struct {
				Int    int
				Bool   bool
				hidden string
			}{},
			want: []string{"Int", "Bool"},
		},
		{
			name:   "mixed, nil naming",
			naming: nil,
			strct: struct {
				Int int
				StructWithFloat
				Struct struct {
					Sub bool
				}
				hidden string
			}{},
			want: []string{"Int", "Float", "Struct"},
		},

		{
			name:   "empty struct, DefaultStructFieldNaming",
			naming: &DefaultStructFieldNaming,
			strct:  struct{}{},
			want:   []string{},
		},
		{
			name:   "exported names, DefaultStructFieldNaming",
			naming: &DefaultStructFieldNaming,
			strct: struct {
				Int  int
				Bool bool `col:"boolean"`
			}{},
			want: []string{"Int", "boolean"},
		},
		{
			name:   "exported and private names, DefaultStructFieldNaming",
			naming: &DefaultStructFieldNaming,
			strct: struct {
				Int        int  `col:"Integer"`
				Bool       bool `col:"-"`
				hidden     string
				HelloWorld string
			}{},
			want: []string{"Integer", "Hello World"},
		},
		{
			name:   "mixed, DefaultStructFieldNaming",
			naming: &DefaultStructFieldNaming,
			strct: struct {
				hidden string `col:"-"`
				Int    int
				StructWithFloat
				Struct struct {
					Sub bool
				}
			}{},
			want: []string{"Int", "float", "Struct"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.naming.Columns(tt.strct)
			require.Equal(t, tt.want, got, "StructFieldNaming.Columns()")
		})
	}
}

func TestTrimEqualFold(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{a: "Name", b: "Name", want: true},
		{a: " name\t", b: "NAME", want: true},
		{a: "Straße", b: "STRASSE", want: false}, // EqualFold is simple folding, not ß → SS
		{a: "Name", b: "Names", want: false},
		{a: "First Name", b: "FirstName", want: false},
		{a: "", b: "  ", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.a+"|"+tt.b, func(t *testing.T) {
			require.Equal(t, tt.want, TrimEqualFold(tt.a, tt.b))
		})
	}
}

func TestTrimEqual(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{a: "Name", b: "Name", want: true},
		{a: " Name\t", b: "\nName ", want: true},
		{a: "Name", b: "NAME", want: false},
		{a: "First Name", b: "FirstName", want: false},
		{a: "", b: "  ", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.a+"|"+tt.b, func(t *testing.T) {
			require.Equal(t, tt.want, TrimEqual(tt.a, tt.b))
		})
	}
}

// TestStructFieldNaming_ColumnStructFieldValue_ClosestField covers that a
// column goes to the field whose title it matches the closest, not to the
// first field that MatchTitle accepts, and to the first of equally close ones.
func TestStructFieldNaming_ColumnStructFieldValue_ClosestField(t *testing.T) {
	type record struct {
		Upper   string `col:"NAME"`
		Spaced  string `col:" Name "`
		Exact   string `col:"Name"`
		Another string `col:"name"`
	}
	rec := record{Upper: "Upper", Spaced: "Spaced", Exact: "Exact", Another: "Another"}
	loose := &StructFieldNaming{Tag: "col", MatchTitle: TrimEqualFold}
	for column, wantField := range map[string]string{
		"Name":   "Exact",   // equal
		"Name\t": "Spaced",  // first of two equal without whitespace, before a field differing in case
		"nAME":   "Upper",   // first of four differing in case
		" name":  "Another", // equal without whitespace
	} {
		require.Equal(t, wantField, loose.ColumnStructFieldValue(reflect.ValueOf(rec), column).String(), "column %q", column)
	}
	require.False(t, loose.ColumnStructFieldValue(reflect.ValueOf(rec), "Names").IsValid())

	trimmed := loose.WithMatchTitle(TrimEqual)
	require.Equal(t, "Spaced", trimmed.ColumnStructFieldValue(reflect.ValueOf(rec), "Name\t").String())
	require.False(t, trimmed.ColumnStructFieldValue(reflect.ValueOf(rec), "nAME").IsValid(), "TrimEqual is case sensitive")
}

func TestStructFieldNaming_WithDisallowAmbiguousTitles(t *testing.T) {
	naming := DefaultStructFieldNaming.WithDisallowAmbiguousTitles(true)
	require.True(t, naming.DisallowAmbiguousTitles)
	require.False(t, DefaultStructFieldNaming.DisallowAmbiguousTitles, "DefaultStructFieldNaming must not be modified")
	require.False(t, naming.WithDisallowAmbiguousTitles(false).DisallowAmbiguousTitles)
}

func TestStructFieldNaming_WithPreferFirstColumn(t *testing.T) {
	naming := DefaultStructFieldNaming.WithPreferFirstColumn(true)
	require.True(t, naming.PreferFirstColumn)
	require.False(t, DefaultStructFieldNaming.PreferFirstColumn, "DefaultStructFieldNaming must not be modified")
	require.False(t, naming.WithPreferFirstColumn(false).PreferFirstColumn)
}

func TestStructFieldNaming_TitlesMatch(t *testing.T) {
	tests := []struct {
		name       string
		naming     *StructFieldNaming
		viewTitle  string
		fieldTitle string
		want       bool
	}{
		{name: "nil naming equal", naming: nil, viewTitle: "Name", fieldTitle: "Name", want: true},
		{name: "nil naming case differs", naming: nil, viewTitle: "name", fieldTitle: "Name", want: false},
		{name: "exact by default", naming: &StructFieldNaming{}, viewTitle: "Name ", fieldTitle: "Name", want: false},
		{name: "TrimEqualFold", naming: &StructFieldNaming{MatchTitle: TrimEqualFold}, viewTitle: " NAME ", fieldTitle: "Name", want: true},
		{
			name:       "custom MatchTitle gets the view title first",
			naming:     &StructFieldNaming{MatchTitle: func(viewTitle, fieldTitle string) bool { return viewTitle == "view" && fieldTitle == "field" }},
			viewTitle:  "view",
			fieldTitle: "field",
			want:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.naming.TitlesMatch(tt.viewTitle, tt.fieldTitle))
		})
	}
}

func TestStructFieldNaming_WithMatchTitle(t *testing.T) {
	naming := DefaultStructFieldNaming.WithMatchTitle(TrimEqualFold)
	require.True(t, naming.TitlesMatch("name", "Name"))
	require.Nil(t, DefaultStructFieldNaming.MatchTitle, "DefaultStructFieldNaming must not be modified")
	require.False(t, naming.WithMatchTitle(nil).TitlesMatch("name", "Name"))
}

// onlyTagged is the naming of domonda's CSV importers:
// only fields with a col tag are columns.
var onlyTagged = &StructFieldNaming{
	Tag:        "col",
	Ignore:     "-",
	Untagged:   func(string) string { return "" },
	MatchTitle: TrimEqualFold,
}

type embeddedAccountNumber struct {
	Number string `col:"Konto"`
}

type account struct {
	embeddedAccountNumber
	Name     string `col:"Bezeichnung"`
	Internal string `col:"-"`
	Untagged string
	hidden   string `col:"Hidden"` //nolint:unused
}

func TestStructFieldNaming_ColumnStructFieldValue_MatchTitle(t *testing.T) {
	tests := []struct {
		name      string
		naming    *StructFieldNaming
		column    string
		wantField string // empty for no field
	}{
		{name: "exact naming, exact title", naming: &StructFieldNaming{Tag: "col", Ignore: "-"}, column: "Bezeichnung", wantField: "Name"},
		{name: "exact naming, title with other case", naming: &StructFieldNaming{Tag: "col", Ignore: "-"}, column: "bezeichnung"},
		{name: "exact naming, title with spaces", naming: &StructFieldNaming{Tag: "col", Ignore: "-"}, column: " Bezeichnung"},
		{name: "loose, other case and spaces", naming: onlyTagged, column: " BEZEICHNUNG\t", wantField: "Name"},
		{name: "loose, field of embedded struct", naming: onlyTagged, column: "konto", wantField: "Number"},
		{name: "loose, ignored title with spaces", naming: onlyTagged, column: " - "},
		{name: "loose, empty title", naming: onlyTagged, column: "  "},
		{name: "untagged field is no column", naming: onlyTagged, column: "Untagged"},
		{name: "unexported field is no column", naming: onlyTagged, column: "Hidden"},
		{name: "loose with default untagged naming", naming: DefaultStructFieldNaming.WithMatchTitle(TrimEqualFold), column: "untagged", wantField: "Untagged"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := account{
				embeddedAccountNumber: embeddedAccountNumber{Number: "Number"},
				Name:                  "Name",
				Internal:              "Internal",
				Untagged:              "Untagged",
			}
			v := tt.naming.ColumnStructFieldValue(reflect.ValueOf(acc), tt.column)
			if tt.wantField == "" {
				require.False(t, v.IsValid(), "got field with value %v", v)
				return
			}
			require.True(t, v.IsValid())
			require.Equal(t, tt.wantField, v.String())
		})
	}
}

func TestStructFieldNaming_CheckUniqueColumns(t *testing.T) {
	type embeddedName struct {
		Name string `col:"name"`
	}
	tests := []struct {
		name    string
		naming  *StructFieldNaming
		strct   any
		wantErr string // empty for no error
	}{
		{name: "unique", naming: onlyTagged, strct: account{}},
		{
			name:   "equal titles, exact naming",
			naming: &StructFieldNaming{Tag: "col"},
			strct: struct {
				A string `col:"Name"`
				B string `col:"Name"`
			}{},
			wantErr: `duplicate column title: "Name" and "Name"`,
		},
		{
			name:   "titles differing in case, exact naming",
			naming: &StructFieldNaming{Tag: "col"},
			strct: struct {
				A string `col:"Name"`
				B string `col:"name"`
			}{},
		},
		{
			name:   "titles differing in case, loose naming",
			naming: onlyTagged,
			strct: struct {
				A string `col:"Name"`
				B string `col:"name"`
			}{},
			wantErr: `duplicate column title: "Name" and "name"`,
		},
		{
			name:   "title of embedded struct",
			naming: onlyTagged,
			strct: struct {
				embeddedName
				B string `col:" NAME"`
			}{},
			wantErr: `duplicate column title: "name" and " NAME"`,
		},
		{
			name:   "ignored fields are not columns",
			naming: onlyTagged,
			strct: struct {
				A string `col:"-"`
				B string `col:"-"`
				C string
				D string
			}{},
		},
		{
			name:   "asymmetric MatchTitle is checked both ways",
			naming: &StructFieldNaming{MatchTitle: func(viewTitle, fieldTitle string) bool { return viewTitle == "A" && fieldTitle == "B" }},
			strct: struct {
				B string
				A string
			}{},
			wantErr: `duplicate column title: "B" and "A"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.naming.CheckUniqueColumns(tt.strct)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrDuplicateColumnTitle)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// amounts has two columns that sameAlternative lets
// the same title of a file match.
type amounts struct {
	Amount string `col:"Betrag"`
	Total  string `col:"Total"`
}

// sameAlternative accepts the title "Summe" for the fields titled
// "Betrag" and "Total", which CheckUniqueColumns can't know,
// because it only compares the field titles with each other.
var sameAlternative = &StructFieldNaming{
	Tag:    "col",
	Ignore: "-",
	MatchTitle: func(viewTitle, fieldTitle string) bool {
		return TrimEqualFold(viewTitle, fieldTitle) || TrimEqualFold(viewTitle, "Summe")
	},
}

func TestStructFieldNaming_FindTitleRow(t *testing.T) {
	tests := []struct {
		name         string
		naming       *StructFieldNaming
		strct        any
		rows         [][]string
		wantTitleRow int
		wantColumns  []int
		wantErr      error
		wantErrText  string // optional
	}{
		{
			name:   "first row, struct order",
			naming: onlyTagged,
			strct:  account{},
			rows: [][]string{
				{"Konto", "Bezeichnung"},
				{"120", "Software"},
			},
			wantTitleRow: 0,
			wantColumns:  []int{0, 1},
		},
		{
			name:   "after preamble rows with fewer columns, any order, extra columns",
			naming: onlyTagged,
			strct:  &account{},
			rows: [][]string{
				{"Firma: 200670"},
				{},
				{"Überschrift: Sachkonten", "Konto"},
				{"Bezeichnung", "Kontoart", " KONTO ", ""},
				{"Software", "1", "120", ""},
			},
			wantTitleRow: 3,
			wantColumns:  []int{2, 0},
		},
		{
			name:   "equal cell wins over an earlier looser match",
			naming: onlyTagged,
			strct:  account{},
			rows: [][]string{
				{"konto", "Bezeichnung", "Konto"},
			},
			wantTitleRow: 0,
			wantColumns:  []int{2, 1},
		},
		{
			name:   "trimmed equal cell wins over one differing in case, equal over both",
			naming: onlyTagged,
			strct:  account{},
			rows: [][]string{
				{"KONTO", " Konto ", "bezeichnung", " Bezeichnung", "Bezeichnung"},
			},
			wantTitleRow: 0,
			wantColumns:  []int{1, 4},
		},
		{
			name:   "last of equally close cells wins",
			naming: onlyTagged,
			strct:  account{},
			rows: [][]string{
				{"konto", "Bezeichnung", "KONTO", "Konten", "kONTO", "bezeichnung"},
			},
			wantTitleRow: 0,
			wantColumns:  []int{4, 1},
		},
		{
			name:   "first of equally close cells wins with PreferFirstColumn",
			naming: onlyTagged.WithPreferFirstColumn(true),
			strct:  account{},
			rows: [][]string{
				{"konto", "Bezeichnung", "KONTO", "Konten", "kONTO", "bezeichnung"},
			},
			wantTitleRow: 0,
			wantColumns:  []int{0, 1},
		},
		{
			name:   "PreferFirstColumn does not prefer a looser match",
			naming: onlyTagged.WithPreferFirstColumn(true),
			strct:  account{},
			rows: [][]string{
				{"konto", "bezeichnung", "Konto", "Bezeichnung", "Konto"},
			},
			wantTitleRow: 0,
			wantColumns:  []int{2, 3},
		},
		{
			name:   "TrimEqual does not match other case",
			naming: onlyTagged.WithMatchTitle(TrimEqual),
			strct:  account{},
			rows: [][]string{
				{"KONTO", "Bezeichnung"},
				{" Konto ", "Bezeichnung\t"},
			},
			wantTitleRow: 1,
			wantColumns:  []int{0, 1},
		},
		{
			name:   "disallowed ambiguity, unambiguous title row",
			naming: onlyTagged.WithDisallowAmbiguousTitles(true),
			strct:  account{},
			rows: [][]string{
				{"Konto", "Konto"}, // not the title row, so not checked
				{" KONTO ", "Kontoart", "Bezeichnung"},
			},
			wantTitleRow: 1,
			wantColumns:  []int{0, 2},
		},
		{
			name:   "disallowed ambiguity, two cells match one title",
			naming: onlyTagged.WithDisallowAmbiguousTitles(true),
			strct:  account{},
			rows: [][]string{
				{"konto", "Bezeichnung", "Konto"},
			},
			wantTitleRow: -1,
			wantErr:      ErrAmbiguousColumnTitle,
			wantErrText:  `columns "konto" and "Konto" both match "Konto" of retable.account`,
		},
		{
			name:   "disallowed ambiguity, two equal cells with exact matching",
			naming: (&StructFieldNaming{Tag: "col", Ignore: "-", Untagged: UseTitle("-")}).WithDisallowAmbiguousTitles(true),
			strct:  account{},
			rows: [][]string{
				{"Konto", "Bezeichnung", "Konto"},
			},
			wantTitleRow: -1,
			wantErr:      ErrAmbiguousColumnTitle,
		},
		{
			name:   "one cell matching two titles is used for both",
			naming: sameAlternative,
			strct:  amounts{},
			rows: [][]string{
				{"Summe"},
			},
			wantTitleRow: 0,
			wantColumns:  []int{0, 0},
		},
		{
			name:   "an equal cell is not taken by the other title",
			naming: sameAlternative,
			strct:  amounts{},
			rows: [][]string{
				{"Summe", "Total"},
			},
			wantTitleRow: 0,
			wantColumns:  []int{0, 1},
		},
		{
			name:   "disallowed ambiguity, one cell matches two titles",
			naming: sameAlternative.WithDisallowAmbiguousTitles(true),
			strct:  amounts{},
			rows: [][]string{
				{"Summe"},
			},
			wantTitleRow: -1,
			wantErr:      ErrAmbiguousColumnTitle,
			wantErrText:  `column "Summe" matches both "Betrag" and "Total" of retable.amounts`,
		},
		{
			name:   "first row with all titles wins",
			naming: onlyTagged,
			strct:  account{},
			rows: [][]string{
				{"Konto", "Bezeichnung"},
				{"Konto", "Bezeichnung", "Extra"},
			},
			wantTitleRow: 0,
			wantColumns:  []int{0, 1},
		},
		{
			name:   "exact naming does not match other case",
			naming: &StructFieldNaming{Tag: "col", Ignore: "-", Untagged: func(string) string { return "" }},
			strct:  account{},
			rows: [][]string{
				{"konto", "Bezeichnung"},
				{"Konto", "Bezeichnung"},
			},
			wantTitleRow: 1,
			wantColumns:  []int{0, 1},
		},
		{
			name:   "missing title",
			naming: onlyTagged,
			strct:  account{},
			rows: [][]string{
				{"Konto", "Name"},
				{"120", "Software"},
			},
			wantTitleRow: -1,
			wantErr:      ErrTitleRowNotFound,
		},
		{
			name:         "no rows",
			naming:       onlyTagged,
			strct:        account{},
			wantTitleRow: -1,
			wantErr:      ErrTitleRowNotFound,
		},
		{
			name:         "no columns",
			naming:       onlyTagged,
			strct:        struct{ Untagged string }{},
			rows:         [][]string{{"Untagged"}},
			wantTitleRow: -1,
			wantErr:      ErrNoColumns,
		},
		{
			name:   "duplicate columns",
			naming: onlyTagged,
			strct: struct {
				A string `col:"Konto"`
				B string `col:"konto"`
			}{},
			rows:         [][]string{{"Konto"}},
			wantTitleRow: -1,
			wantErr:      ErrDuplicateColumnTitle,
		},
		{
			name:   "nil naming uses field names",
			naming: nil,
			strct: struct {
				A string
				B int
			}{},
			rows:         [][]string{{"x"}, {"B", "A"}},
			wantTitleRow: 1,
			wantColumns:  []int{1, 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			titleRow, columns, err := tt.naming.FindTitleRow(tt.rows, tt.strct)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.ErrorContains(t, err, tt.wantErrText)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantTitleRow, titleRow)
			require.Equal(t, tt.wantColumns, columns)
		})
	}
}

// TestStructFieldNaming_FindTitleRow_FilteredView checks the documented way
// of reading the rows after the title row: only one cell per title is read,
// also when another column matches the title equally closely, the last
// of them by default and the first one with PreferFirstColumn.
func TestStructFieldNaming_FindTitleRow_FilteredView(t *testing.T) {
	rows := [][]string{
		{"Firma: 200670"},
		{"Bezeichnung", "Kontoart", " KONTO ", "konto"},
		{"Software", "1", "120", "999"},
		{"Patente", "1", "100", ""},
	}
	for name, tt := range map[string]struct {
		naming      *StructFieldNaming
		wantNumbers [2]string
	}{
		// The empty cell is read, the other column is not part of the view
		"last column by default":              {naming: onlyTagged, wantNumbers: [2]string{"999", ""}},
		"first column with PreferFirstColumn": {naming: onlyTagged.WithPreferFirstColumn(true), wantNumbers: [2]string{"120", "100"}},
	} {
		t.Run(name, func(t *testing.T) {
			titleRow, columns, err := tt.naming.FindTitleRow(rows, account{})
			require.NoError(t, err)
			view := &FilteredView{
				Source:        NewStringsView("", rows[titleRow+1:], rows[titleRow]...),
				ColumnMapping: columns,
			}
			accounts, err := ViewToStructSlice[account](view, tt.naming, nil, nil, nil, nil, "Konto", "Bezeichnung")
			require.NoError(t, err)
			require.Equal(t, []account{
				{embeddedAccountNumber: embeddedAccountNumber{Number: tt.wantNumbers[0]}, Name: "Software"},
				{embeddedAccountNumber: embeddedAccountNumber{Number: tt.wantNumbers[1]}, Name: "Patente"},
			}, accounts)
		})
	}
}
