package retable

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStringParser_ParseFloat(t *testing.T) {
	tests := []struct {
		str     string
		want    float64
		wantErr bool
	}{
		// Everything strconv.ParseFloat accepts must be parsed by it
		// unchanged, the locale handling below is only a fallback.
		{str: "0", want: 0},
		{str: "42", want: 42},
		{str: "-42", want: -42},
		{str: "+1.5", want: 1.5},
		{str: "3.14", want: 3.14},
		{str: "-3.14", want: -3.14},
		{str: ".5", want: 0.5},
		{str: "5.", want: 5},
		{str: "1e3", want: 1000},
		{str: "-2.5e10", want: -2.5e10},

		// A single comma without any dot is the decimal separator of
		// locales like German, which spreadsheets and CSV files exported
		// by them write as "3,14" where Go writes "3.14".
		{str: "3,14", want: 3.14},
		{str: "-3,14", want: -3.14},
		{str: ",5", want: 0.5},
		{str: "5,", want: 5},

		// Deliberate ambiguity: a single comma is always the decimal
		// separator, never an English thousands separator, so "1,234"
		// is 1.234 and not 1234. Nothing in the string distinguishes
		// the two, and the decimal reading is the one that loses no
		// digits if it guesses wrong.
		{str: "1,234", want: 1.234},
		// A single dot stays what strconv.ParseFloat makes of it, which
		// is the decimal separator, for the same reason.
		{str: "1.234", want: 1.234},

		// With both separators present the last one is the decimal
		// separator and the other one groups thousands, which resolves
		// the ambiguity for both locales without any configuration.
		{str: "1,234.56", want: 1234.56},
		{str: "1.234,56", want: 1234.56},
		{str: "-1,234.56", want: -1234.56},
		{str: "-1.234,56", want: -1234.56},
		{str: "1,234,567.89", want: 1234567.89},
		{str: "1.234.567,89", want: 1234567.89},

		// Repeated separators of one kind without the other can only be
		// thousands separators, there is nothing left for them to group.
		{str: "1,234,567", want: 1234567},
		{str: "1.234.567", want: 1234567},

		// Accounting and ERP exports write the minus sign after the number.
		{str: "1.234,56-", want: -1234.56},
		{str: "1,234.56-", want: -1234.56},

		// Surrounding whitespace is trimmed, and spaces and apostrophes
		// are recognized as thousands separators, because spreadsheets
		// export French and Swiss number formats that way.
		{str: " 3.14", want: 3.14},
		{str: "3.14 ", want: 3.14},
		{str: "1 234,56", want: 1234.56},
		{str: "1'234.56", want: 1234.56},

		// Strings that are not numbers in any of the handled conventions.
		{str: "", wantErr: true},
		{str: "abc", wantErr: true},
		{str: "--1", wantErr: true},
		{str: "1e", wantErr: true},
		// Currency symbols and other decoration are not stripped.
		{str: "€1,5", wantErr: true},
		{str: "1,5%", wantErr: true},
		// Both separators present but not resolvable to a single
		// decimal separator followed by digits.
		{str: "1,234.5.6", wantErr: true},
		{str: "1.234,56,7", wantErr: true},
		// Digit groups before the decimal separator have to be 3 digits
		// long, so wrongly grouped strings are rejected instead of being
		// parsed into an arbitrary number.
		{str: "12.34,56", wantErr: true},
		{str: "1.23.456,78", wantErr: true},
	}
	p := NewStringParser()
	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			got, err := p.ParseFloat(tt.str)
			if tt.wantErr {
				require.Error(t, err, "ParseFloat(%q)", tt.str)
				require.Zero(t, got, "ParseFloat(%q) value on error", tt.str)
				return
			}
			require.NoError(t, err, "ParseFloat(%q)", tt.str)
			require.Equal(t, tt.want, got, "ParseFloat(%q)", tt.str)
		})
	}
}

// TestStringParser_ParseFloatErrorQuotesOriginalString ensures the error
// reports the string that was passed in and not one of the intermediate
// strings with removed separators, which the caller never saw.
func TestStringParser_ParseFloatErrorQuotesOriginalString(t *testing.T) {
	_, err := NewStringParser().ParseFloat("1,234.5.6")
	require.ErrorContains(t, err, `"1,234.5.6"`)
}

// TestStringParser_ParseFloatSpecialValues documents that the strconv
// syntax for floats is passed through, including forms that are unlikely
// in table data and are accepted only because strconv.ParseFloat accepts them.
func TestStringParser_ParseFloatSpecialValues(t *testing.T) {
	p := NewStringParser()

	f, err := p.ParseFloat("NaN")
	require.NoError(t, err)
	require.True(t, math.IsNaN(f))

	f, err = p.ParseFloat("inf")
	require.NoError(t, err)
	require.Equal(t, math.Inf(1), f)

	f, err = p.ParseFloat("-Infinity")
	require.NoError(t, err)
	require.Equal(t, math.Inf(-1), f)

	// Go literal syntax for underscores and hexadecimal floats
	f, err = p.ParseFloat("1_000.5")
	require.NoError(t, err)
	require.Equal(t, 1000.5, f)

	f, err = p.ParseFloat("0x1p-2")
	require.NoError(t, err)
	require.Equal(t, 0.25, f)
}

// TestStringParserZeroValueFalseAndTimeDefaults completes what
// TestStringParserZeroValueUsesDefaults covers for the true strings and
// the nil strings: the two remaining nil-field fallbacks. A parser built
// from a configuration file that only names its true strings still has
// to recognize the false strings and the time formats, or half of the
// columns of the file it configures stop parsing without anyone having
// asked for that.
func TestStringParserZeroValueFalseAndTimeDefaults(t *testing.T) {
	var zero StringParser

	t.Run("default false strings", func(t *testing.T) {
		// ParseBool returns before it reads the false strings for
		// anything that is a true string, so only a false one reaches
		// the fallback under test here.
		for _, str := range []string{"false", "FALSE", "no", "0", "f"} {
			b, err := zero.ParseBool(str)
			require.NoErrorf(t, err, "ParseBool(%q)", str)
			require.Falsef(t, b, "ParseBool(%q)", str)
		}
	})

	t.Run("default time formats", func(t *testing.T) {
		tm, err := zero.ParseTime("2024-03-15T14:30:00Z")
		require.NoError(t, err)
		require.Equal(t, time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC), tm)
	})
}
