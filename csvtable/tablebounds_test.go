package csvtable

import (
	"testing"

	"github.com/domonda/go-retable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectTableBounds(t *testing.T) {
	tests := []struct {
		name           string
		rows           [][]string
		expectedTitles []string
		wantBounds     TableBounds
		wantRows       [][]string
	}{
		{
			// The header lines must not be mistaken for column titles,
			// and the trailer lines must not be scanned as data.
			name: "bank statement with header and trailer lines",
			rows: [][]string{
				{"Kontoauszug Nr. 4"},
				{"Zeitraum", "01.01.2025", "31.01.2025"},
				nil,
				{"Datum", "Text", "Betrag", "Waehrung"},
				{"01.01.2025", "Miete", "-500,00", "EUR"},
				{"02.01.2025", "Gehalt", "2000,00", "EUR"},
				nil,
				{"Erstellt am 03.01.2025"},
			},
			expectedTitles: []string{"Datum", "Betrag"},
			wantBounds:     TableBounds{TitleRow: 3, EndRow: 6, NumColumns: 4},
			wantRows: [][]string{
				{"Datum", "Text", "Betrag", "Waehrung"},
				{"01.01.2025", "Miete", "-500,00", "EUR"},
				{"02.01.2025", "Gehalt", "2000,00", "EUR"},
			},
		},
		{
			// A header line with as many columns as the table can only be
			// told apart from the column titles by the expected titles.
			name: "header line with the table's column count",
			rows: [][]string{
				{"Export", "Konto", "4711", "EUR"},
				{"Datum", "Text", "Betrag", "Waehrung"},
				{"01.01.2025", "Miete", "-500,00", "EUR"},
			},
			expectedTitles: []string{"Datum", "Text", "Betrag", "Waehrung"},
			wantBounds:     TableBounds{TitleRow: 1, EndRow: 3, NumColumns: 4},
			wantRows: [][]string{
				{"Datum", "Text", "Betrag", "Waehrung"},
				{"01.01.2025", "Miete", "-500,00", "EUR"},
			},
		},
		{
			// A field containing a newline leaves an empty row behind
			// which must not end the table.
			name: "empty row from a joined multi line field",
			rows: [][]string{
				{"Datum", "Text"},
				{"01.01.2025", "Zeile1\nZeile2"},
				nil,
				{"02.01.2025", "Miete"},
			},
			expectedTitles: []string{"Datum", "Text"},
			wantBounds:     TableBounds{TitleRow: 0, EndRow: 4, NumColumns: 2},
			wantRows: [][]string{
				{"Datum", "Text"},
				{"01.01.2025", "Zeile1\nZeile2"},
				nil,
				{"02.01.2025", "Miete"},
			},
		},
		{
			name:           "table without header lines",
			rows:           [][]string{{"A", "B"}, {"1", "2"}},
			expectedTitles: []string{"A", "B"},
			wantBounds:     TableBounds{TitleRow: 0, EndRow: 2, NumColumns: 2},
			wantRows:       [][]string{{"A", "B"}, {"1", "2"}},
		},
		{
			name:           "titles are matched without case and whitespace",
			rows:           [][]string{{" datum ", "BETRAG"}, {"01.01.2025", "-500,00"}},
			expectedTitles: []string{"Datum", "Betrag"},
			wantBounds:     TableBounds{TitleRow: 0, EndRow: 2, NumColumns: 2},
			wantRows:       [][]string{{" datum ", "BETRAG"}, {"01.01.2025", "-500,00"}},
		},
		{
			name:           "no row matches the expected titles",
			rows:           [][]string{{"A", "B"}, {"1", "2"}},
			expectedTitles: []string{"Datum", "Betrag"},
			wantBounds:     TableBounds{TitleRow: -1},
		},
		{
			// Without expected titles only the column count and non-empty
			// fields can be used, which finds the first candidate row.
			name:       "without expected titles",
			rows:       [][]string{{"Kontoauszug Nr. 4"}, {"Datum", "Betrag"}, {"01.01.2025", "-500,00"}},
			wantBounds: TableBounds{TitleRow: 1, EndRow: 3, NumColumns: 2},
			wantRows:   [][]string{{"Datum", "Betrag"}, {"01.01.2025", "-500,00"}},
		},
		{
			name:       "no rows",
			rows:       nil,
			wantBounds: TableBounds{TitleRow: -1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bounds := DetectTableBounds(tt.rows, tt.expectedTitles...)
			assert.Equal(t, tt.wantBounds, bounds, "detected bounds")
			assert.Equal(t, tt.wantRows, bounds.Rows(tt.rows), "table rows")
		})
	}
}

// TestReadStringsToStructSlice_HeaderAndTrailerLines is why DetectTableBounds
// exists: without it the first row of the file is used as column titles, so
// the header line of a bank statement becomes the column titles and every
// required column is reported as missing.
func TestReadStringsToStructSlice_HeaderAndTrailerLines(t *testing.T) {
	type booking struct {
		Datum  string `csv:"Datum"`
		Text   string `csv:"Text"`
		Betrag string `csv:"Betrag"`
	}
	csv := "Kontoauszug Nr. 4\n" +
		"Zeitraum;01.01.2025;31.01.2025\n" +
		"\n" +
		"Datum;Text;Betrag\n" +
		"01.01.2025;Miete;-500,00\n" +
		"02.01.2025;Gehalt;2000,00\n" +
		"\n" +
		"Erstellt am 03.01.2025\n"

	rows, format, err := ParseDetectFormat([]byte(csv), nil)
	require.NoError(t, err)
	require.Equal(t, ";", format.Separator)

	naming := &retable.StructFieldNaming{Tag: "csv"}
	bookings, err := ReadStringsToStructSlice[booking](rows, naming, nil, nil, nil, "Datum", "Text", "Betrag")
	require.NoError(t, err)
	assert.Equal(t, []booking{
		{Datum: "01.01.2025", Text: "Miete", Betrag: "-500,00"},
		{Datum: "02.01.2025", Text: "Gehalt", Betrag: "2000,00"},
	}, bookings)
}
