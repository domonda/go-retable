package csvtable

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_EmptyRowsWithNonUniformColumns(t *testing.T) {
	testCases := []struct {
		source   [][]string
		expected [][]string
	}{
		{
			source:   nil,
			expected: nil,
		},
		{
			source:   [][]string{nil, {"", "", ""}, nil},
			expected: [][]string{nil, {"", "", ""}, nil}, // nil rows can't dominate
		},
		{
			source:   [][]string{{"1", "2", "3"}, {"0"}, {"4", "5", "6"}},
			expected: [][]string{{"1", "2", "3"}, nil, {"4", "5", "6"}},
		},
		{
			source:   [][]string{{"0"}, {"1", "2", "3"}, {"4", "5", "6"}},
			expected: [][]string{nil, {"1", "2", "3"}, {"4", "5", "6"}},
		},
		{
			source:   [][]string{{"1", "2", "3"}, {"0"}, {"0", "0"}, {"4", "5", "6"}},
			expected: [][]string{{"1", "2", "3"}, nil, nil, {"4", "5", "6"}}, // take longer row if count of columns is identical
		},
		{
			source:   [][]string{{"0", "0"}, {"1", "2", "3"}},
			expected: [][]string{nil, {"1", "2", "3"}}, // take longer row if count of columns is identical
		},
		{
			source:   [][]string{{"1"}, {"2", "2"}, {"3", "3", "3"}},
			expected: [][]string{nil, nil, {"3", "3", "3"}}, // take longer row if count of columns is identical
		},
		{
			// Header and trailer lines of a table are single column rows
			// that must not outvote the actual table rows even if there are more of them.
			source: [][]string{
				{"Kontoauszug Nr. 4"},
				{"Erstellt von: Musterbank AG"},
				{"Konto: AT12 3456"},
				{"Datum", "Text", "Betrag", "Waehrung"},
				{"01.01.2025", "Miete", "-500,00", "EUR"},
				{"Ende des Auszugs"},
				{"Seite 1 von 1"},
			},
			expected: [][]string{
				nil,
				nil,
				nil,
				{"Datum", "Text", "Betrag", "Waehrung"},
				{"01.01.2025", "Miete", "-500,00", "EUR"},
				nil,
				nil,
			},
		},
		{
			// But a table that only has single column rows must be kept.
			source:   [][]string{{"Title"}, {"1"}, {"2"}},
			expected: [][]string{{"Title"}, {"1"}, {"2"}},
		},
	}

	for i, test := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			result := SetRowsWithNonUniformColumnsNil(test.source)
			assert.Equal(t, test.expected, result, "EmptyRowsWithNonUniformColumns")
		})
	}
}

func Test_RemoveEmptyRows(t *testing.T) {
	testCases := []struct {
		source   [][]string
		expected [][]string
	}{
		{
			source:   nil,
			expected: nil,
		},
		{
			source:   [][]string{},
			expected: nil,
		},
		{
			source:   [][]string{nil, {}, nil},
			expected: nil,
		},
		{
			source:   [][]string{nil, {"", "", ""}, nil},
			expected: nil,
		},
		{
			source:   [][]string{nil, {"1", "2", "3"}, nil},
			expected: [][]string{{"1", "2", "3"}},
		},
		{
			source:   [][]string{{"1", "2", "3"}, nil, nil},
			expected: [][]string{{"1", "2", "3"}},
		},
		{
			source:   [][]string{nil, nil, {"1", "2", "3"}},
			expected: [][]string{{"1", "2", "3"}},
		},
		{
			source:   [][]string{{"1", "2", "3"}, nil, {"4", "5", "6"}},
			expected: [][]string{{"1", "2", "3"}, {"4", "5", "6"}},
		},
	}

	for i, test := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			result := RemoveEmptyRows(test.source)
			assert.Equal(t, test.expected, result, "RemoveEmptyRows")
		})
	}
}

func Test_CleanSpacedString(t *testing.T) {
	// Also see http://localhost:5006/payment-import/20e66223-f7ab-4e1b-a59a-d15c104c9562-doc.csv.html
	testCases := map[string]string{
		"":   "",
		" ":  " ",
		"  ": "  ",

		// Whether a string is too short to be spaced out is counted in
		// runes, so a multi-byte string must not be compacted where the
		// same string with single-byte characters is not.
		"A ":                                    "A ",
		"Ä ":                                    "Ä ",
		"A B":                                   "AB",
		"Ä Ö":                                   "ÄÖ",
		"1 2":                                   "12",
		"1 2 3":                                 "123",
		"1 2 3 ":                                "123", // do we want this?
		"Hello World!":                          "Hello World!",
		"S h i n e r g y   S c h ö n b r u n n": "Shinergy Schönbrunn",
		"S a l z b u r g e r   T e n n i s c o u r t s   S ü d": "Salzburger Tenniscourts Süd",
	}
	for source, expected := range testCases {
		t.Run(source, func(t *testing.T) {
			cleaned, modified := compactSpacedString(source)
			assert.Equal(t, expected, cleaned, "cleaned string")
			assert.True(t, modified == (cleaned != source), "modified")
		})
	}
}

func TestReplaceNewlineWithSpace(t *testing.T) {
	rows := [][]string{
		{"unix\nnewline", "windows\r\nnewline"},
		{"carriage\rreturn", "no newline"},
		{"multiple\n\nnewlines", ""},
	}
	ReplaceNewlineWithSpace(rows)
	assert.Equal(t, [][]string{
		{"unix newline", "windows newline"}, // \r\n becomes one space, not two
		{"carriage return", "no newline"},
		{"multiple  newlines", ""},
	}, rows)
}
