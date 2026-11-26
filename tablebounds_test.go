package retable

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStructFieldNaming_DetectTableBounds covers that the title row of a
// table within header and trailer lines is found with the title matching of
// the naming, which is what makes CSV, spreadsheet and any other rows find
// their titles the same way as ViewToStructSlice assigns them afterwards.
func TestStructFieldNaming_DetectTableBounds(t *testing.T) {
	rows := [][]string{
		{"Kontoauszug Nr. 4"},
		{"Zeitraum", "01.01.2026", "31.01.2026"},
		{},
		{"DATUM", " Text ", "Betrag"},
		{"02.01.2026", "Miete", "-500,00"},
		{"05.01.2026", "Gehalt", "2.000,00"},
		{},
		{"Erstellt am 06.01.2026"},
	}
	alternative := func(viewTitle, fieldTitle string) bool {
		return TrimEqualFold(viewTitle, fieldTitle) || (fieldTitle == "Valuta" && viewTitle == "DATUM")
	}
	table := TableBounds{TitleRow: 3, EndRow: 6, NumColumns: 3}
	// The header line "Zeitraum" has as many cells as the table,
	// and the empty row after it does not end a table
	firstRowOfColumnCount := TableBounds{TitleRow: 1, EndRow: 6, NumColumns: 3}
	notFound := TableBounds{TitleRow: -1}

	tests := []struct {
		name           string
		naming         *StructFieldNaming
		expectedTitles []string
		want           TableBounds
	}{
		{name: "nil naming, equal titles", naming: nil, expectedTitles: []string{"Betrag"}, want: table},
		{name: "nil naming, title of other case", naming: nil, expectedTitles: []string{"Datum"}, want: notFound},
		{name: "exact, cells are compared trimmed like NewStringsView trims them", naming: &StructFieldNaming{}, expectedTitles: []string{"Text"}, want: table},
		{name: "exact, title of other case", naming: &StructFieldNaming{}, expectedTitles: []string{"Datum"}, want: notFound},
		{name: "exact, the row with the most titles", naming: &StructFieldNaming{}, expectedTitles: []string{"Datum", "Zeitraum", "Text", "Betrag"}, want: table},
		{name: "TrimEqual is case sensitive", naming: &StructFieldNaming{TitleMatcher: TrimEqual}, expectedTitles: []string{"Datum"}, want: notFound},
		{name: "TrimEqualFold", naming: &StructFieldNaming{TitleMatcher: TrimEqualFold}, expectedTitles: []string{"Datum", "text", "BETRAG"}, want: table},
		{name: "a title only the TitleMatcher accepts", naming: &StructFieldNaming{TitleMatcher: alternative}, expectedTitles: []string{"Valuta"}, want: table},
		{name: "no title of the table", naming: &StructFieldNaming{TitleMatcher: TrimEqualFold}, expectedTitles: []string{"IBAN"}, want: notFound},
		{name: "without titles the first row of the column count without empty cell", naming: nil, want: firstRowOfColumnCount},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bounds := tt.naming.DetectTableBounds(rows, tt.expectedTitles...)
			require.Equal(t, tt.want, bounds)
			if tt.want == table {
				require.True(t, bounds.Valid())
				require.Equal(t, rows[3:6], bounds.Rows(rows))
			}
		})
	}

	// Without titles a header line with as many cells as the table
	// is no title row if a cell is empty or only whitespace
	headerWithEmptyCell := [][]string{{"Export", " ", ""}, {"Datum", "Text", "Betrag"}, {"1", "2", "3"}}
	require.Equal(t, 1, (*StructFieldNaming)(nil).DetectTableBounds(headerWithEmptyCell).TitleRow)

	require.False(t, (*StructFieldNaming)(nil).DetectTableBounds(nil).Valid(), "no rows means no table")
	require.False(t, TableBounds{}.Valid(), "the zero value is not a table")
	require.Nil(t, TableBounds{TitleRow: 0, EndRow: 9, NumColumns: 1}.Rows(rows), "bounds of other rows")
}

// TestStructFieldNaming_DetectTableBounds_ViewToStructSlice covers the
// bounds together with the conversion they are made for, with rows that
// could as well come from a spreadsheet: both use the same naming.
func TestStructFieldNaming_DetectTableBounds_ViewToStructSlice(t *testing.T) {
	type account struct {
		Number string `col:"Konto"`
		Name   string `col:"Bezeichnung"`
	}
	rows := [][]string{
		{"Kontenplan 2026"},
		{"BEZEICHNUNG", "Kontoart", " konto "},
		{"Software", "1", "120"},
		{"Bank", "2", "2800"},
		{"2 Konten"},
	}
	naming := DefaultStructFieldNamingIgnoreUntagged.WithTitleMatcher(TrimEqualFold)

	bounds := naming.DetectTableBounds(rows, naming.Columns(account{})...)
	require.Equal(t, TableBounds{TitleRow: 1, EndRow: 4, NumColumns: 3}, bounds)

	accounts, err := ViewToStructSlice[account](NewStringsView("", bounds.Rows(rows)), naming, nil, nil, nil, nil, "Konto", "Bezeichnung")
	require.NoError(t, err)
	require.Equal(t, []account{{Number: "120", Name: "Software"}, {Number: "2800", Name: "Bank"}}, accounts)
}
