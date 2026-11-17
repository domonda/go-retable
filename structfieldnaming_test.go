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

func TestStructFieldNaming_FindTitleRow(t *testing.T) {
	tests := []struct {
		name         string
		naming       *StructFieldNaming
		strct        any
		rows         [][]string
		wantTitleRow int
		wantColumns  []int
		wantErr      error
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
			name:   "first matching cell of a title wins",
			naming: onlyTagged,
			strct:  account{},
			rows: [][]string{
				{"konto", "Bezeichnung", "Konto"},
			},
			wantTitleRow: 0,
			wantColumns:  []int{0, 1},
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
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantTitleRow, titleRow)
			require.Equal(t, tt.wantColumns, columns)
		})
	}
}

// TestStructFieldNaming_FindTitleRow_FilteredView checks the documented way
// of reading the rows after the title row: only the first cell matching a
// title is read, also when a later column has the same title.
func TestStructFieldNaming_FindTitleRow_FilteredView(t *testing.T) {
	rows := [][]string{
		{"Firma: 200670"},
		{"Bezeichnung", "Kontoart", " KONTO ", "konto"},
		{"Software", "1", "120", "999"},
		{"Patente", "1", "100", "999"},
	}
	titleRow, columns, err := onlyTagged.FindTitleRow(rows, account{})
	require.NoError(t, err)
	view := &FilteredView{
		Source:        NewStringsView("", rows[titleRow+1:], rows[titleRow]...),
		ColumnMapping: columns,
	}
	accounts, err := ViewToStructSlice[account](view, onlyTagged, nil, nil, nil, nil, "Konto", "Bezeichnung")
	require.NoError(t, err)
	require.Equal(t, []account{
		{embeddedAccountNumber: embeddedAccountNumber{Number: "120"}, Name: "Software"},
		{embeddedAccountNumber: embeddedAccountNumber{Number: "100"}, Name: "Patente"},
	}, accounts)
}
