package csvtable

import (
	"testing"

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
			assert.Equalf(t, refSeparator, format.Separator, "'s' separator expected", refSeparator)
			SetRowsWithNonUniformColumnsNil(rows)
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
		`"`:    {1, 0},
		`""`:   {1, 1},
		`"""`:  {2, 1},
		`""""`: {2, 2},

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
			left, right := countQuotesLeftRight([]byte(str))
			assert.Equal(t, counts[0], left, "left quote count")
			assert.Equal(t, counts[1], right, "right quote count")
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
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if gotSep := parseSepHeaderLine([]byte(tt.line)); gotSep != tt.wantSep {
				t.Errorf("parseSepHeaderLine() = %v, want %v", gotSep, tt.wantSep)
			}
		})
	}
}
