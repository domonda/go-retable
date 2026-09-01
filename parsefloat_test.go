package retable

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// The tests in this file are ported from
// github.com/domonda/go-types/float/parse_test.go so that the parsing
// code ported into parsefloat.go keeps its original test coverage.

type floatInfo struct {
	f            float64
	thousandsSep rune
	decimalSep   rune
	decimals     int
}

func Test_parseFloatDetails(t *testing.T) {
	// Variations with leading + and - are created automatically, don't put them here
	validDecimalFloats := map[string]floatInfo{
		"100":                  {100, 0, 0, 0},
		"100.9":                {100.9, 0, '.', 1},
		"1e6":                  {1e6, 0, 0, 0},
		"1.2e6":                {1.2e6, 0, '.', 1},
		"1e-6":                 {1e-6, 0, 0, 0},
		"1.2e-6":               {1.2e-6, 0, '.', 1},
		"1e+6":                 {1e6, 0, 0, 0},
		"1.2e+6":               {1.2e+6, 0, '.', 1},
		"2.48689957516035e14":  {2.48689957516035e14, 0, '.', 14},
		"2.48689957516035E14":  {2.48689957516035e14, 0, '.', 14},
		"2.48689957516035e-14": {2.48689957516035e-14, 0, '.', 14},
		"2.48689957516035E-14": {2.48689957516035e-14, 0, '.', 14},
		"2.48689957516035e+14": {2.48689957516035e+14, 0, '.', 14},
		"2.48689957516035E+14": {2.48689957516035e+14, 0, '.', 14},
		",1":                   {0.1, 0, ',', 1},
		".1":                   {0.1, 0, '.', 1},
		"1,":                   {1.0, 0, ',', 0},
		"1.":                   {1.0, 0, '.', 0},
		"123.456":              {123.456, 0, '.', 3},
		"123,456":              {123.456, 0, ',', 3},
		"100 200 300.1234":     {100200300.1234, ' ', '.', 4},
		"100 200 300,1234":     {100200300.1234, ' ', ',', 4},
		"100,200,300.1234":     {100200300.1234, ',', '.', 4},
		"100.200.300,1234":     {100200300.1234, '.', ',', 4},
		"100'200'300.1234":     {100200300.1234, '\'', '.', 4},
		"100'200'300,1234":     {100200300.1234, '\'', ',', 4},
		"1,200,300.1234":       {1200300.1234, ',', '.', 4},
		"1.200.300,1234":       {1200300.1234, '.', ',', 4},
		"1'200'300,1234":       {1200300.1234, '\'', ',', 4},
		"1.234.567":            {1234567, '.', 0, 0},
		"1,234,567":            {1234567, ',', 0, 0},
		"123.456.789":          {123456789, '.', 0, 0},
		"123,456,789":          {123456789, ',', 0, 0},
		"123 456 789":          {123456789, ' ', 0, 0},
		"1000000.8989":         {1000000.8989, 0, '.', 4},
		"1000000,8989":         {1000000.8989, 0, ',', 4},
		"158,00 ":              {158, 0, ',', 2},
		"NaN":                  {math.NaN(), 0, 0, 0},  // No sign prepending in test
		"Inf":                  {math.Inf(1), 0, 0, 0}, // +Inf and -Inf will generated in test by prepending sign
	}

	testFunc := func(str string, refFloat float64, refThousandsSep, refDecimalSep rune, refDecimals int) func(*testing.T) {
		return func(t *testing.T) {
			parsed, thousandsSep, decimalSep, decimals, err := parseFloatDetails(str)
			require.NoError(t, err)
			if math.IsNaN(refFloat) {
				require.True(t, math.IsNaN(parsed), "parseFloatDetails(%#v)", str)
			} else {
				require.Equal(t, refFloat, parsed, "parseFloatDetails(%#v)", str)
			}
			require.Equal(t, string(refThousandsSep), string(thousandsSep), "parseFloatDetails(%#v)", str)
			require.Equal(t, string(refDecimalSep), string(decimalSep), "parseFloatDetails(%#v)", str)
			require.Equal(t, refDecimals, decimals, "parseFloatDetails(%#v)", str)
		}
	}

	for str, ref := range validDecimalFloats {
		t.Run("no sign", testFunc(str, ref.f, ref.thousandsSep, ref.decimalSep, ref.decimals))
		if str == "NaN" {
			continue
		}
		t.Run("plus in front", testFunc("+"+str, ref.f, ref.thousandsSep, ref.decimalSep, ref.decimals))
		if str != "Inf" {
			t.Run("plus in front with space", testFunc("+ "+str, ref.f, ref.thousandsSep, ref.decimalSep, ref.decimals))
			t.Run("plus on end", testFunc(str+"+", ref.f, ref.thousandsSep, ref.decimalSep, ref.decimals))
			t.Run("plus on end with space", testFunc(str+" +", ref.f, ref.thousandsSep, ref.decimalSep, ref.decimals))
		}
		t.Run("minus in front", testFunc("-"+str, -ref.f, ref.thousandsSep, ref.decimalSep, ref.decimals))
		if str != "Inf" {
			t.Run("minus in front with space", testFunc("- "+str, -ref.f, ref.thousandsSep, ref.decimalSep, ref.decimals))
			t.Run("minus on end", testFunc(str+"-", -ref.f, ref.thousandsSep, ref.decimalSep, ref.decimals))
			t.Run("minus on end with space", testFunc(str+" -", -ref.f, ref.thousandsSep, ref.decimalSep, ref.decimals))
		}
	}
}

func Test_parseFloat_invalid(t *testing.T) {
	invalidDecimalFloats := []string{
		"",
		"xxx",
		"e3",
		"--1",
		"1--",
		"++1",
		"1++",
		"-+1",
		"1-+",
		"+-1",
		"1+-",
		"1-2",
		"1+2",
		"1/2",
		"1ee6",
		"123+456",
		"12-3456",
		",1,1",
		"9,1,1",
		"10.2340 560",
		"123.12.1.0",
		"10.000.00,00",
		"10,2340,560",
		"10.2340,560",
		"10.23,560",
		"1.234.56",
		"1,234,56",
		"123-456",
		"123.45.67.890",
		"123.45.67.890,0",
		"1234.567.890,0",
		"-1234.567.890,0",
		"123,456.789,000",
		"123,456.789 000",
		"123,123 123 123",
		"123,123 23 123",
		"123,1234,123.99",
		"123 1234 123.99",
	}

	for _, s := range invalidDecimalFloats {
		_, err := parseFloat(s)
		require.Error(t, err, "parseFloat(%#v)", s)
	}
}

// Test_parseFloat_singleGroupSeparator is a regression test for a
// single space or apostrophe group being read as a decimal separator.
//
// The rule that a lone separator is the decimal separator resolves a
// real ambiguity for "." and ",", where "1,234" can be either reading.
// It must not apply to " " and "'": no locale writes a decimal fraction
// after them, so "1 234" can only be 1234. Parsing it as 1.234 was
// silent and never an error, and it hit exactly the values that carry
// no decimals, so a currency column ended up mixing amounts a factor of
// 1000 apart while the amounts with decimals stayed correct.
//
// csvtable makes this reachable for ordinary files: sanitizeUTF8
// rewrites the non-breaking space U+00A0 that Excel, SAP and French or
// Nordic exports use for grouping into a plain ASCII space before the
// value is parsed.
func Test_parseFloat_singleGroupSeparator(t *testing.T) {
	grouped := []struct {
		str  string
		want float64
	}{
		{str: "1 234", want: 1234},
		{str: "8 500", want: 8500},
		{str: "123 456", want: 123456},
		{str: "1'234", want: 1234},
		{str: "12'000", want: 12000},
		// More than one group already parsed correctly and must stay so
		{str: "1 234 567", want: 1234567},
		{str: "1'234'567", want: 1234567},
		// A decimal separator after the group is unaffected
		{str: "1 234,56", want: 1234.56},
		{str: "1'234.56", want: 1234.56},
	}
	for _, tt := range grouped {
		t.Run(tt.str, func(t *testing.T) {
			got, thousandsSep, decimalSep, _, err := parseFloatDetails(tt.str)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			if decimalSep == 0 {
				require.NotEqual(t, rune(0), thousandsSep, "the separator must be reported as the thousands separator")
			}
		})
	}

	// A space or apostrophe group that is not 3 digits long is not a
	// grouping at all, and must not silently become a fraction.
	for _, str := range []string{"1 23", "1 2345", "1'23", "1'2345", "1 2"} {
		t.Run("invalid group "+str, func(t *testing.T) {
			_, err := parseFloat(str)
			require.Error(t, err, "parseFloat(%q)", str)
		})
	}

	// The ambiguity that does exist is still resolved as before: a lone
	// dot or comma stays the decimal separator, because both readings
	// are real and the decimal one loses no digits when it guesses wrong.
	for _, tt := range []struct {
		str  string
		want float64
	}{
		{str: "1.234", want: 1.234},
		{str: "1,234", want: 1.234},
	} {
		t.Run("unchanged "+tt.str, func(t *testing.T) {
			got, err := parseFloat(tt.str)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
