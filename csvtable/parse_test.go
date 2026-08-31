package csvtable

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

var testRows = map[string][]string{
	"A;\"Line1\nLine2\";B": {
		";", // separator
		"A",
		"Line1\nLine2",
		"B",
	},
	"A;\"Line1\r\nLine2\";B\r\n": {
		";", // separator
		"A",
		"Line1\nLine2",
		"B",
	},
	"A;\"Line1\r\nLine2\";B\r\r\n": {
		";", // separator
		"A",
		"Line1\nLine2",
		"B",
	},
	` Hello ,World ,	!`: {
		",",
		` Hello `,
		`World `,
		`	!`,
	},
	"\n\n\n Hello ,World ,	!\n\n\n": {
		",",
		` Hello `,
		`World `,
		`	!`,
	},
	`" Hello ","World ","	!"`: {
		",",
		` Hello `,
		`World `,
		`	!`,
	},
	`1997,Ford,E350,"Super, luxurious truck"`: {
		",",
		`1997`,
		`Ford`,
		`E350`,
		`Super, luxurious truck`,
	},
	`"SEP=|"` + "\n" + `"A"|"B"|"C"`: {
		"|",
		`A`,
		`B`,
		`C`,
	},
	`SEP=|` + "\r\n" + `A|B|C`: {
		"|",
		`A`,
		`B`,
		`C`,
	},
	`"sep=,"` + "\n" + `"A","B","C"`: {
		",",
		`A`,
		`B`,
		`C`,
	},
	`sep=;` + "\r\n" + `A;B;C`: {
		";",
		`A`,
		`B`,
		`C`,
	},
	`1997,Ford,E350,"Super, ""luxurious"" truck"`: {
		",",
		`1997`,
		`Ford`,
		`E350`,
		`Super, "luxurious" truck`,
	},
	`1997,""Ford"",E350,"Super, luxurious truck"`: {
		",",
		`1997`,
		`"Ford"`,
		`E350`,
		`Super, luxurious truck`,
	},
	`1997,"""Ford""",E350,"Super, luxurious truck"`: {
		",",
		`1997`,
		`"Ford"`,
		`E350`,
		`Super, luxurious truck`,
	},
	`"1997","""Ford""","E350","Super, luxurious truck"`: {
		",",
		`1997`,
		`"Ford"`,
		`E350`,
		`Super, luxurious truck`,
	},
	`"1997","Ford","E350","""Super, luxurious truck"""`: {
		",",
		`1997`,
		`Ford`,
		`E350`,
		`"Super, luxurious truck"`,
	},

	// "INTERPHONE ""LE 4"""
	// """Heimbau"" Gemeinnützige Bau-, Wohnungs- u. Siedlungsgenossenscha"

	`05.10.2018;""Heimbau"" Gemeinnützige Bau-, Wohnungs- u. Siedlungsgenossenscha;AT4112xxxxx;BKAUATWWXXX;;;-85,91;EUR;ENTGELT 10/2018 ""Heimbau"" Gemeinnützige Bau-, Wohnu;12000;;0;05.10.2018`: {
		";", // separator
		`05.10.2018`,
		`"Heimbau" Gemeinnützige Bau-, Wohnungs- u. Siedlungsgenossenscha`,
		`AT4112xxxxx`,
		`BKAUATWWXXX`,
		``,
		``,
		`-85,91`,
		`EUR`,
		`ENTGELT 10/2018 "Heimbau" Gemeinnützige Bau-, Wohnu`,
		`12000`,
		``,
		`0`,
		`05.10.2018`,
	},
	`26.06.2018,25.06.2018,Kreditkarte,"-42,87",EUR,"COURSERA inkl. Fremdwährungsentgelt 0,63 Kurs 1,1600378",`: {
		",", // separator
		`26.06.2018`,
		`25.06.2018`,
		`Kreditkarte`,
		`-42,87`,
		`EUR`,
		`COURSERA inkl. Fremdwährungsentgelt 0,63 Kurs 1,1600378`,
		``,
	},
	`"30.12.2018","21:56:09","CET","charlieBAUM DIVERS ET IMPREVU","PayPal Express-Zahlung","Abgeschlossen","EUR","76,80","-2,42","74,38","charliebaum@wanadoo.fr","joerg@saturo.eu","0PE15874WY2156812","isabelle darrigrand, 15 AVENUE EDOUARD VII, INTERPHONE ""LE 4"", BIARRITZ, 64200, Frankreich","Bestätigt","Ready To Drink - 330 ml - Original, Ready To Drink - 330 ml - Strawberry","","0,00","","0,00","","","","","","201812300043437","{""order_id"":198790,""order_number"":""201812300043437"",""order_key"":""wc_order_5c2930bb3e682""}","5","","6.780,42","15 AVENUE EDOUARD VII","INTERPHONE ""LE 4""","BIARRITZ","","64200","Frankreich","0607069536","Ready To Drink - 330 ml - Original","","Sofort","","T0006","","FR","FR","Haben"`: {
		",", // separator
		"30.12.2018",
		"21:56:09",
		"CET",
		"charlieBAUM DIVERS ET IMPREVU",
		"PayPal Express-Zahlung",
		"Abgeschlossen",
		"EUR",
		"76,80",
		"-2,42",
		"74,38",
		"charliebaum@wanadoo.fr",
		"joerg@saturo.eu",
		"0PE15874WY2156812",
		`isabelle darrigrand, 15 AVENUE EDOUARD VII, INTERPHONE "LE 4", BIARRITZ, 64200, Frankreich`,
		"Bestätigt",
		"Ready To Drink - 330 ml - Original, Ready To Drink - 330 ml - Strawberry",
		"",
		"0,00",
		"",
		"0,00",
		"",
		"",
		"",
		"",
		"",
		"201812300043437",
		`{"order_id":198790,"order_number":"201812300043437","order_key":"wc_order_5c2930bb3e682"}`,
		"5",
		"",
		"6.780,42",
		"15 AVENUE EDOUARD VII",
		`INTERPHONE "LE 4"`,
		"BIARRITZ",
		"",
		"64200",
		"Frankreich",
		"0607069536",
		"Ready To Drink - 330 ml - Original",
		"",
		"Sofort",
		"",
		"T0006",
		"",
		"FR",
		"FR",
		"Haben",
	},
	`"15.12.2019","""Heimbau"" Gemeinnützige Bau-, Wohnungs- u. Siedlungsgenossenscha","AT","BKAUATWWXXX","","12000","-8,70","EUR","ENTGELT","xxxxx","","0","15.12.2019","","","","","0-9x9-05","ATx"`: {
		",", // separator
		"15.12.2019",
		"\"Heimbau\" Gemeinnützige Bau-, Wohnungs- u. Siedlungsgenossenscha",
		"AT",
		"BKAUATWWXXX",
		"",
		"12000",
		"-8,70",
		"EUR",
		"ENTGELT",
		"xxxxx",
		"",
		"0",
		"15.12.2019",
		"",
		"",
		"",
		"",
		"0-9x9-05",
		"ATx",
	},
	// A quoted field can end with an escaped quote before the separator,
	// like a JSON object with a string as last value.
	// See Sentry DOMONDA-SERVER-FH4.
	`"14/12/2025","Look Beautiful Products GmbH","{""orderTransactionId"":""019b1d3ebc9b72c59a12e32e8d8ff142"",""pluginVersion"":""10.1.1""}","Debit"`: {
		",", // separator
		`14/12/2025`,
		`Look Beautiful Products GmbH`,
		`{"orderTransactionId":"019b1d3ebc9b72c59a12e32e8d8ff142","pluginVersion":"10.1.1"}`,
		`Debit`,
	},

	// Every line of the file is a quoted field with doubled quotes inside,
	// as produced by exporting an already exported CSV file.
	`"""a"",""b"""`: {
		",", // separator
		`"a","b"`,
	},

	// A quoted field whose content ends with the separator,
	// so the closing quote is the only content of the split field.
	`a,"b,",c`: {
		",", // separator
		`a`,
		`b,`,
		`c`,
	},

	// A closing quote followed by unquoted characters does not end the field,
	// so the field is kept verbatim instead of losing its last character.
	`"a" ,"b"x,"c"`: {
		",", // separator
		`"a" `,
		`"b"x`,
		`c`,
	},

	// A carriage return within a quoted field is field data and must survive
	// splitting the lines, it is not residue of a wider line ending.
	"A;\"first\n\rsecond\";B\n": {
		";", // separator
		`A`,
		"first\n\rsecond",
		`B`,
	},

	`300150;GH "Zum Ganster";;`: {
		";", // separator
		`300150`,
		`GH "Zum Ganster"`,
		``,
		``,
	},
}

func TestParseStrings(t *testing.T) {
	for csvRow, ref := range testRows {
		t.Run(csvRow, func(t *testing.T) {
			refSeparator, refFields := ref[0], ref[1:]
			rows, format, err := ParseDetectFormat([]byte(csvRow), nil)
			assert.NoError(t, err, "csv.Read")
			assert.NotNil(t, format, "returned Format")
			assert.Equal(t, "UTF-8", format.Encoding, "UTF-8 encoding expected")
			assert.Equalf(t, refSeparator, format.Separator, "%q separator expected", refSeparator)
			rows = SetRowsWithNonUniformColumnsNil(rows)
			rows = RemoveEmptyRows(rows)
			assert.Len(t, rows, 1, "one CSV row expected")
			if len(rows) == 1 {
				rowFields := rows[0]
				assert.Equal(t, len(refFields), len(rowFields), "parsed CSV row field count")
				for i := range rowFields {
					assert.Equalf(t, refFields[i], rowFields[i], "parsed CSV row field %d", i)
				}
			}
		})
	}

}

func TestCountQuotes(t *testing.T) {
	testData := map[string][2]int{
		``:     {0, 0},
		`"`:    {1, 1},
		`""`:   {2, 2},
		`"""`:  {3, 3},
		`""""`: {4, 4},

		`1`:      {0, 0},
		`12`:     {0, 0},
		`123`:    {0, 0},
		` " `:    {0, 0},
		` "" `:   {0, 0},
		`  ""  `: {0, 0},

		`" `:    {1, 0},
		`"" `:   {2, 0},
		`""" `:  {3, 0},
		`"""" `: {4, 0},

		` "`:    {0, 1},
		` ""`:   {0, 2},
		` """`:  {0, 3},
		` """"`: {0, 4},

		`" "`:   {1, 1},
		`"" "`:  {2, 1},
		`""" "`: {3, 1},
		`" ""`:  {1, 2},
		`" """`: {1, 3},

		`"  "`:     {1, 1},
		`""  ""`:   {2, 2},
		`"""  """`: {3, 3},
	}

	for str, counts := range testData {
		t.Run(str, func(t *testing.T) {
			assert.Equal(t, counts[0], countQuotesLeft([]byte(str)), "left quote count")
			assert.Equal(t, counts[1], countQuotesRight([]byte(str)), "right quote count")
		})
	}
}

// Test_closesQuotedField documents which field is accepted as the closing part
// of a quoted field split by a separator or newline. An ordinary quoted field
// must not be accepted, else an unterminated quote swallows the rows up to it.
func Test_closesQuotedField(t *testing.T) {
	testData := map[string]bool{
		``:          false,
		`value`:     false,
		`"`:         true,
		`""`:        false,
		`"""`:       true,
		`""""`:      false,
		`value"`:    true,
		`value""`:   false,
		`value"""`:  true,
		`""value"`:  true,
		`"value"`:   false, // complete quoted field, not a closing part
		`"value`:    false, // opening part, not a closing part
		`""value""`: false,
	}

	for field, want := range testData {
		t.Run(field, func(t *testing.T) {
			assert.Equal(t, want, closesQuotedField([]byte(field)), "closesQuotedField(%q)", field)
		})
	}
}

// Test_splitLines_LosesCarriageReturnBeforeNewline records a known limitation
// of splitting the lines before the fields are parsed, it is not the wanted
// behaviour. A \r directly before the newline the lines are split by can't be
// told apart from the residue of a file with mixed line endings, so it is
// trimmed away together with it.
//
// Change this test to expect "x\r\ny" once the parser tracks the quoted state
// while splitting instead of joining the split fields back together.
func Test_splitLines_LosesCarriageReturnBeforeNewline(t *testing.T) {
	rows, format, err := ParseDetectFormat([]byte("A;\"x\r\ny\";B\nC;D;E\n"), nil)
	assert.NoError(t, err)
	assert.Equal(t, "\n", format.Newline)
	rows = RemoveEmptyRows(rows)
	if assert.Len(t, rows, 2) {
		assert.Equal(t, []string{"A", "x\ny", "B"}, rows[0], "the \\r before the splitting \\n is lost")
		assert.Equal(t, []string{"C", "D", "E"}, rows[1])
	}
}

func Test_sanitizeUTF8(t *testing.T) {
	tests := []struct {
		name string
		str  []byte
		want string
	}{
		{name: "empty", str: nil, want: ""},
		{name: "ASCII is unchanged", str: []byte("abc"), want: "abc"},
		{name: "valid UTF-8 is unchanged", str: []byte("Jänner 20€"), want: "Jänner 20€"},
		{name: "no-break space becomes a space", str: []byte("a\u00a0b"), want: "a b"},
		{name: "replacement character becomes a space", str: []byte("a\ufffdb"), want: "a b"},
		{name: "invalid byte becomes a space", str: []byte{'a', 0xff, 'b'}, want: "a b"},
		{
			// Every invalid byte is replaced on its own,
			// they are not decoded as one broken sequence.
			name: "every invalid byte becomes a space",
			str:  []byte{'a', 0xff, 0xfe, 'b'},
			want: "a  b",
		},
		{
			// A failed encoding detection is not reported,
			// the undecodable bytes just become spaces.
			name: "Windows 1252 read as UTF-8 loses its umlaut",
			str:  []byte{'M', 0xfc, 'l', 'l', 'e', 'r'},
			want: "M ller",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeUTF8(tt.str)
			assert.Equal(t, tt.want, string(got))
			assert.True(t, utf8.Valid(got), "result must be valid UTF-8")
		})
	}
}

func Test_parseSepHeaderLine(t *testing.T) {
	tests := []struct {
		line    string
		wantSep string
	}{
		{line: `SEP=,`, wantSep: ","},
		{line: `"SEP=,"`, wantSep: ","},
		{line: `SEP=;`, wantSep: ";"},
		{line: `"SEP=;"`, wantSep: ";"},
		{line: `sep=,`, wantSep: ","},
		{line: `"sep=,"`, wantSep: ","},
		{line: "sep=\t", wantSep: "\t"},
		{line: `sep=|`, wantSep: "|"},

		// A quote or a control character can never be a separator.
		// Accepting one made every quote branch of readLines nonsensical
		// and the invalid Format was passed on to the caller.
		{line: `sep="`, wantSep: ""},
		{line: `"sep=""`, wantSep: ""},
		{line: "sep=\r", wantSep: ""},
		{line: "sep=\n", wantSep: ""},
		{line: "sep=\x00", wantSep: ""},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if gotSep := parseSepHeaderLine([]byte(tt.line)); gotSep != tt.wantSep {
				t.Errorf("parseSepHeaderLine() = %v, want %v", gotSep, tt.wantSep)
			}
		})
	}
}

// TestParseDetectFormat_MultiLineFields verifies that a quoted field containing
// newlines is joined back into one field regardless of how many quotes its first
// line fragment holds. A fragment consisting only of quotes must not be mistaken
// for a complete field, that emitted one logical row as two malformed rows.
func TestParseDetectFormat_MultiLineFields(t *testing.T) {
	tests := []struct {
		name string
		csv  string
		want [][]string
	}{
		{
			name: "value starting with a newline",
			csv:  "A;\"\nB\";C",
			want: [][]string{{"A", "\nB", "C"}},
		},
		{
			name: "value starting with a quote and a newline",
			csv:  "A;\"\"\"\nfoo\";B",
			want: [][]string{{"A", "\"\nfoo", "B"}},
		},
		{
			name: "value starting with two quotes and a newline",
			csv:  "A;\"\"\"\"\"\nfoo\";B",
			want: [][]string{{"A", "\"\"\nfoo", "B"}},
		},
		{
			name: "value spanning three lines",
			csv:  "A;\"one\ntwo\nthree\";B",
			want: [][]string{{"A", "one\ntwo\nthree", "B"}},
		},
		{
			// The field is split by the separator before it is split by the
			// newline, so the opening part is not the last field of its line
			// and the closing part is not the first field of its line.
			name: "value containing separator and newline",
			csv:  "A;\"one;two\nthree;four\";B",
			want: [][]string{{"A", "one;two\nthree;four", "B"}},
		},
		{
			name: "value that is only separators and a newline",
			csv:  "A;\";\n;\";B",
			want: [][]string{{"A", ";\n;", "B"}},
		},
		{
			name: "value with separator on the closing line only",
			csv:  "A;\"one\ntwo;three\";B",
			want: [][]string{{"A", "one\ntwo;three", "B"}},
		},
		{
			name: "value that is only a quote and a newline",
			csv:  "A;\"\"\"\n\"\"\";B",
			want: [][]string{{"A", "\"\n\"", "B"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, _, err := ParseDetectFormat([]byte(tt.csv), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, RemoveEmptyRows(rows))
		})
	}
}

// TestParseDetectFormat_UnterminatedQuote verifies that an unterminated quote
// does not swallow the rows following it. The closing part of a multi-line field
// may only begin with escaped quotes, so an ordinary quoted field on a later line
// must not be taken for it, which silently destroyed every row in between.
func TestParseDetectFormat_UnterminatedQuote(t *testing.T) {
	rows, _, err := ParseDetectFormat([]byte("a;\"oops\nr2c1;r2c2\n\"r3c1\";r3c2\nr4c1;r4c2\n"), nil)
	assert.NoError(t, err)
	assert.Equal(t, [][]string{
		{"a", `"oops`}, // the unterminated field keeps its opening quote
		{"r2c1", "r2c2"},
		{"r3c1", "r3c2"},
		{"r4c1", "r4c2"},
	}, RemoveEmptyRows(rows))
}

// TestParseDetectFormat_EmptyInput verifies that the returned Format is usable
// even without any data, because callers detect the format once and re-use it
// for parsing and writing further data.
func TestParseDetectFormat_EmptyInput(t *testing.T) {
	for _, data := range []string{"", "\n", "\n\n", "\r\n"} {
		t.Run(strconv.Quote(data), func(t *testing.T) {
			rows, format, err := ParseDetectFormat([]byte(data), nil)
			assert.NoError(t, err)
			assert.Empty(t, RemoveEmptyRows(rows))
			assert.NoError(t, format.Validate(), "returned Format must be valid")
		})
	}
}

// TestParseDetectFormat_SepHeaderNewlineTrimming verifies that a sep= header line
// does not change the parsed field values. Detection used to return early for a
// sep= header, before the line trimming, leaking a \r into the last field.
func TestParseDetectFormat_SepHeaderNewlineTrimming(t *testing.T) {
	withHeader, _, err := ParseDetectFormat([]byte("sep=;\r\nA;B\r\r\n"), nil)
	assert.NoError(t, err)
	withoutHeader, _, err := ParseDetectFormat([]byte("A;B\r\r\n"), nil)
	assert.NoError(t, err)

	assert.Equal(t, [][]string{{"A", "B"}}, RemoveEmptyRows(withHeader))
	assert.Equal(t, RemoveEmptyRows(withoutHeader), RemoveEmptyRows(withHeader), "sep= header must not change field values")
}

// TestParseDetectFormat_ReadsEncodingCSVOutput parses files written by the
// standard library, where a newline inside a quoted field is the same byte
// sequence as the row terminator. That is what Excel and encoding/csv produce
// and the only shape that exercises joining a field across lines.
func TestParseDetectFormat_Detection(t *testing.T) {
	tests := []struct {
		name         string
		csv          string
		wantSep      string
		wantNewline  string
		wantFirstRow []string
	}{
		{
			// The commas within the quoted names are part of a value,
			// not structure, so they must not outvote the semicolons.
			name:         "quoted commas don't outvote semicolons",
			csv:          "Datum;Name;Betrag\n01.01.2025;\"Meier, Hans, Wien, AT\";-1.234,56\n02.01.2025;\"Huber, Franz, Graz, AT\";2.000,00\n",
			wantSep:      ";",
			wantNewline:  "\n",
			wantFirstRow: []string{"Datum", "Name", "Betrag"},
		},
		{
			name:         "quoted semicolons don't outvote commas",
			csv:          "a,b,c\n\"x;y;z\",2,3\n\"p;q;r\",5,6\n",
			wantSep:      ",",
			wantNewline:  "\n",
			wantFirstRow: []string{"a", "b", "c"},
		},
		{
			// Counting occurrences is not enough here: the unquoted city
			// lists hold more commas than the file has semicolons, only the
			// uniform column count identifies the semicolon.
			name:         "unquoted commas don't outvote semicolons",
			csv:          "Name;Beschreibung\nMeier;Wien, Graz, Linz, Salzburg\nHuber;Wels, Steyr, Amstetten, Melk\n",
			wantSep:      ";",
			wantNewline:  "\n",
			wantFirstRow: []string{"Name", "Beschreibung"},
		},
		{
			name:         "unquoted commas don't outvote tabs",
			csv:          "Name\tOrt\nMeier\tWien, Graz, Linz\nHuber\tWels, Steyr, Melk\n",
			wantSep:      "\t",
			wantNewline:  "\n",
			wantFirstRow: []string{"Name", "Ort"},
		},
		{
			name:         "decimal commas don't outvote semicolons",
			csv:          "Artikel;Preis\nSchraube;1,50\nMutter;2,75\nNagel;0,99\n",
			wantSep:      ";",
			wantNewline:  "\n",
			wantFirstRow: []string{"Artikel", "Preis"},
		},
		{
			// A newline within a quoted field must not split the record,
			// otherwise the separator's column count looks non-uniform.
			name:         "multi line field keeps the column count uniform",
			csv:          "a;b;c\nx;\"L1\nL2\";z\np;q;r\n",
			wantSep:      ";",
			wantNewline:  "\n",
			wantFirstRow: []string{"a", "b", "c"},
		},
		{
			name:         "pipe separated",
			csv:          "a|b|c\n1|2|3\n",
			wantSep:      "|",
			wantNewline:  "\n",
			wantFirstRow: []string{"a", "b", "c"},
		},
		{
			name:         "quoted pipes don't outvote commas",
			csv:          "a,b\n\"x|y|z|w\",2\n\"p|q|r|s\",5\n",
			wantSep:      ",",
			wantNewline:  "\n",
			wantFirstRow: []string{"a", "b"},
		},
		{
			name:         "tab separated",
			csv:          "a\tb\tc\n1\t2\t3\n",
			wantSep:      "\t",
			wantNewline:  "\n",
			wantFirstRow: []string{"a", "b", "c"},
		},
		{
			// A \r\n within a quoted field is part of a value,
			// so it must not switch the file to \r\n line endings.
			name:         "quoted CRLF doesn't switch line endings",
			csv:          "a;b\nc;\"x\r\ny\"\nd;e\n",
			wantSep:      ";",
			wantNewline:  "\n",
			wantFirstRow: []string{"a", "b"},
		},
		{
			name:         "CRLF file",
			csv:          "a;b\r\nc;d\r\n",
			wantSep:      ";",
			wantNewline:  "\r\n",
			wantFirstRow: []string{"a", "b"},
		},
		{
			// Splitting a \n\r file by \n would leave a \r at the start of
			// every line, which can't be trimmed away without destroying a
			// carriage return within a quoted field.
			name:         "LFCR line endings",
			csv:          "a;b\n\rc;d\n\re;f\n\r",
			wantSep:      ";",
			wantNewline:  "\n\r",
			wantFirstRow: []string{"a", "b"},
		},
		{
			name:         "mixed line endings, mostly LF",
			csv:          "a;b\r\nc;d\ne;f\ng;h\n",
			wantSep:      ";",
			wantNewline:  "\n",
			wantFirstRow: []string{"a", "b"},
		},
		{
			// Unbalanced quotes make the quoted state useless for everything
			// after them, so every byte is counted instead.
			name:         "unbalanced quote still detects",
			csv:          "a;\"oops\nb;c\nd;e\n",
			wantSep:      ";",
			wantNewline:  "\n",
			wantFirstRow: []string{"a", `"oops`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, format, err := ParseDetectFormat([]byte(tt.csv), nil)
			assert.NoError(t, err)
			assert.NoError(t, format.Validate(), "detected format must be valid")
			assert.Equal(t, tt.wantSep, format.Separator, "detected separator")
			assert.Equal(t, tt.wantNewline, format.Newline, "detected newline")
			rows = RemoveEmptyRows(rows)
			if assert.NotEmpty(t, rows, "parsed rows") {
				assert.Equal(t, tt.wantFirstRow, rows[0], "first parsed row")
			}
		})
	}
}

func TestParseDetectFormat_ReadsEncodingCSVOutput(t *testing.T) {
	values := []string{
		"a\nb",
		"line1\nline2\nline3",
		`He said "hi"`,
		"\"\nfoo",
		"\"\"\nfoo",
		"\"\n\"",
		"trailing newline\n",
		"with;separator\nand newline",
		"a\nb;c",
		"x;y\nz;w\nq",
		";\n;",
		"\";\nfoo",
	}
	for _, value := range values {
		t.Run(strconv.Quote(value), func(t *testing.T) {
			var dest bytes.Buffer
			stdlibWriter := csv.NewWriter(&dest)
			stdlibWriter.Comma = ';'
			err := stdlibWriter.WriteAll([][]string{{value, "z"}})
			assert.NoError(t, err)

			rows, _, err := ParseDetectFormat(dest.Bytes(), nil)
			assert.NoError(t, err)
			assert.Equal(t, [][]string{{value, "z"}}, RemoveEmptyRows(rows), "parsed from %q", dest.String())
		})
	}
}

// TestParseWithFormat_NewlineTrimming verifies that ParseWithFormat trims stray
// newline characters like ParseDetectFormat does. A file can use a line ending
// wider than the format's Newline, which used to leak a \r into the last field
// of every line of the explicitly formatted parse only.
func TestParseWithFormat_NewlineTrimming(t *testing.T) {
	tests := []struct {
		csv    string
		format *Format
		want   [][]string
	}{
		{csv: "A;B\r\r\n", format: NewFormat(";"), want: [][]string{{"A", "B"}}},
		{csv: "A;B\r\nC;D\r\n", format: NewFormat(";"), want: [][]string{{"A", "B"}, {"C", "D"}}},
		{
			// Newline "\n" against \r\n line endings is the case that leaked
			csv:    "A;B\r\nC;D\r\n",
			format: &Format{Encoding: "UTF-8", Separator: ";", Newline: "\n"},
			want:   [][]string{{"A", "B"}, {"C", "D"}},
		},
	}
	for _, tt := range tests {
		t.Run(strconv.Quote(tt.csv)+"/"+strconv.Quote(tt.format.Newline), func(t *testing.T) {
			rows, err := ParseWithFormat([]byte(tt.csv), tt.format)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, RemoveEmptyRows(rows))
		})
	}
}

// TestParseWithFormat_InvalidSeparator verifies that a separator that can never
// work is rejected by Format.Validate instead of being passed to readLines,
// where a quote separator makes every quote branch nonsensical.
func TestParseWithFormat_InvalidSeparator(t *testing.T) {
	for _, sep := range []string{`"`, "\r", "\n", "\x00", "\x7f"} {
		t.Run(strconv.Quote(sep), func(t *testing.T) {
			format := &Format{Encoding: "UTF-8", Separator: sep, Newline: "\r\n"}
			assert.Error(t, format.Validate(), "Validate must reject separator %q", sep)

			_, err := ParseWithFormat([]byte("A;B\r\n"), format)
			assert.Error(t, err, "ParseWithFormat must reject separator %q", sep)
		})
	}
	// Separators that must stay valid
	for _, sep := range []string{",", ";", "\t", "|"} {
		t.Run("valid "+strconv.Quote(sep), func(t *testing.T) {
			assert.NoError(t, NewFormat(sep).Validate())
		})
	}
}
