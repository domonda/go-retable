package retable

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strconv"
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

	// Both readings have to survive into the error. strconv's own error
	// already quotes the string, so asserting only on that cannot tell
	// the joined error from the strconv one, and the locale diagnostic
	// could be dropped again without a test noticing. Only the locale
	// half mentions a separator.
	require.ErrorContains(t, err, "separator",
		"the locale reading has to say what it made of the separators")
	require.ErrorIs(t, err, strconv.ErrSyntax,
		"and joining must not break errors.Is against the strconv error")
}

// TestNilStringParserBehavesLikeTheDefaults covers the nil receiver guards
// on the StringParser accessors. A nil *StringParser stored in a Parser
// interface is NOT a nil interface, so the cmp.Or(parser, DefaultParser)
// in SmartAssign does not substitute DefaultParser for it: the nil
// receiver reaches the accessors and used to panic there. Reverting any
// one of the four `p == nil` guards, or the `p != nil` guard that
// ParseFloat needs since it reads StdlibFloatsOnly, has to fail this test.
func TestNilStringParserBehavesLikeTheDefaults(t *testing.T) {
	var parser Parser = (*StringParser)(nil)
	//nolint:staticcheck // comparing the interface, not the pointer in it
	require.True(t, parser != nil,
		"a typed nil is not a nil interface, which is why cmp.Or does not replace it")

	b, err := parser.ParseBool("yes")
	require.NoError(t, err)
	require.True(t, b, "the default true strings")

	b, err = parser.ParseBool("no")
	require.NoError(t, err)
	require.False(t, b, "the default false strings")

	require.True(t, parser.IsNil("NULL"), "the default nil strings")
	require.False(t, parser.IsNil("0"))

	// ParseFloat reads StdlibFloatsOnly off the nil receiver, so it needs
	// the same guard: unset means the locale fallback stays on.
	f, err := parser.ParseFloat("1.234,56")
	require.NoError(t, err)
	require.Equal(t, 1234.56, f, "the default locale-aware float parsing")

	tm, err := parser.ParseTime("2024-03-15T14:30:00Z")
	require.NoError(t, err)
	require.Equal(t, time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC), tm, "the default time formats")

	// Reached the way a caller actually reaches it
	var i int
	err = SmartAssign(reflect.ValueOf(&i).Elem(), reflect.ValueOf("NULL"), nil, parser, nil)
	require.NoError(t, err)
	require.Zero(t, i, "a nil string assigns the zero value, it does not panic")
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

// TestStringParser_StdlibFloatsOnly covers the opt-out from the locale-aware
// fallback of ParseFloat.
//
// The fallback exists for a source that writes German or French numbers,
// but a source that writes Go float literals has no such cell, so a comma
// in one means the cell is corrupt. Without the opt-out the fallback
// guesses a value for that cell instead of failing the row, and the guess
// can be off by a factor of a thousand: "1,234" reads as 1.234 where a
// thousands separator meant 1234. The row is then imported with a wrong
// amount, which is exactly what an import that knows its format needs
// to prevent.
func TestStringParser_StdlibFloatsOnly(t *testing.T) {
	stdlib := &StringParser{StdlibFloatsOnly: true}
	lenient := NewStringParser()

	t.Run("accepts what strconv accepts", func(t *testing.T) {
		for _, tt := range []struct {
			str  string
			want float64
		}{
			{str: "0", want: 0},
			{str: "3.14", want: 3.14},
			{str: "-3.14", want: -3.14},
			{str: "1e3", want: 1000},
			// A lone dot is the decimal separator for both parsers,
			// so the stdlib one does not change this reading.
			{str: "1.234", want: 1.234},
		} {
			got, err := stdlib.ParseFloat(tt.str)
			require.NoErrorf(t, err, "ParseFloat(%q)", tt.str)
			require.Equalf(t, tt.want, got, "ParseFloat(%q)", tt.str)
		}
	})

	t.Run("rejects the locale formats", func(t *testing.T) {
		// Every one of these is parsed by the default parser, which is
		// what makes them worth rejecting: the difference is silent.
		for _, str := range []string{
			"3,14",      // German decimal comma
			"1.234,56",  // German thousands and decimal separators
			"1,234.56",  // English thousands separator
			"1 234,56",  // French thousands separator
			"1'234.56",  // Swiss thousands separator
			"1.234,56-", // trailing minus of accounting exports
		} {
			_, err := lenient.ParseFloat(str)
			require.NoErrorf(t, err, "the default parser has to accept %q for this case to be meaningful", str)

			got, err := stdlib.ParseFloat(str)
			require.Errorf(t, err, "ParseFloat(%q)", str)
			require.Zerof(t, got, "ParseFloat(%q) value on error", str)
			require.ErrorIsf(t, err, strconv.ErrSyntax, "ParseFloat(%q)", str)
		}
	})

	t.Run("keeps the strconv extensions", func(t *testing.T) {
		// strconv.ParseFloat accepts more than the Go float literal
		// syntax, and the field parses with strconv rather than
		// validating it, so these keep parsing. Pinned because a caller
		// who needs NaN and the infinities rejected has to check the
		// parsed value, and tightening this later has to break a test
		// first.
		for _, tt := range []struct {
			str  string
			want float64
		}{
			{str: "1_000.5", want: 1000.5}, // Go literal underscores
			{str: "0x1p-2", want: 0.25},    // hexadecimal float
		} {
			got, err := stdlib.ParseFloat(tt.str)
			require.NoErrorf(t, err, "ParseFloat(%q)", tt.str)
			require.Equalf(t, tt.want, got, "ParseFloat(%q)", tt.str)
		}

		f, err := stdlib.ParseFloat("NaN")
		require.NoError(t, err)
		require.True(t, math.IsNaN(f))

		f, err = stdlib.ParseFloat("-infinity")
		require.NoError(t, err)
		require.Equal(t, math.Inf(-1), f)
	})

	t.Run("survives a JSON round trip", func(t *testing.T) {
		// The field is configurable as data, which is what an embedded
		// override could not be, so the tag name is part of the API.
		var p StringParser
		err := json.Unmarshal([]byte(`{"stdlibFloatsOnly":true}`), &p)
		require.NoError(t, err)
		require.True(t, p.StdlibFloatsOnly, "the stdlibFloatsOnly key has to reach the field")

		_, err = p.ParseFloat("3,14")
		require.Error(t, err, "a parser configured from JSON parses with strconv alone")

		out, err := json.Marshal(&p)
		require.NoError(t, err)
		require.Contains(t, string(out), `"stdlibFloatsOnly":true`)
	})

	t.Run("returns zero for an out of range cell", func(t *testing.T) {
		// The stdlib path discards the value strconv returns alongside
		// its error, and out of range is the only input for which that
		// value is not already zero: strconv.ParseFloat("1e400") returns
		// +Inf. A caller that drops the error must not get an infinity
		// from a stdlib parser where the default one gives zero.
		got, err := stdlib.ParseFloat("1e400")
		require.ErrorIs(t, err, strconv.ErrRange)
		require.Zero(t, got)

		got, err = lenient.ParseFloat("1e400")
		require.ErrorIs(t, err, strconv.ErrRange)
		require.Zero(t, got, "both readings agree on the value of a rejected cell")
	})

	t.Run("rejects a padded cell", func(t *testing.T) {
		// strconv.ParseFloat does not trim and the locale fallback does,
		// so surrounding whitespace is a second thing stdlib parsing
		// rejects. Worth pinning because an export padding its cells is
		// ordinary and the field doc has to warn about it.
		got, err := lenient.ParseFloat(" 3.14")
		require.NoError(t, err)
		require.Equal(t, 3.14, got)

		_, err = stdlib.ParseFloat(" 3.14")
		require.Error(t, err)
	})

	t.Run("reports only the strconv error", func(t *testing.T) {
		// The default parser joins both readings, and only the locale
		// half names a separator, so a stdlib error must not mention one.
		//
		// This has to be asserted on a string BOTH readings reject.
		// Asserting it on one the fallback accepts could never fail:
		// deleting the early return would make ParseFloat succeed there
		// and the missing error would be caught before the message is.
		_, err := lenient.ParseFloat("12.34,56")
		require.ErrorContains(t, err, "separator")

		_, err = stdlib.ParseFloat("12.34,56")
		require.Error(t, err)
		require.NotContains(t, err.Error(), "separator")
	})

	t.Run("reaches SmartAssign", func(t *testing.T) {
		// The field is only worth anything if the parser passed to
		// SmartAssign carries it into the string to float conversion,
		// which is the path an import actually takes.
		var f float64
		err := SmartAssign(reflect.ValueOf(&f).Elem(), reflect.ValueOf("1.234,56"), nil, stdlib, nil)
		require.Error(t, err, "a corrupt cell has to fail the row")
		require.Zero(t, f)

		// The row fails and says why. errors.ErrUnsupported stays in
		// the chain because the strategies of SmartAssign continue on
		// it, and the strconv error rides along beside it, which is
		// what makes a malformed cell distinguishable from a struct
		// field wired to the wrong column type. See
		// TestSmartAssignReportsWhyAStringWasRejected.
		require.ErrorIs(t, err, errors.ErrUnsupported)
		require.ErrorIs(t, err, strconv.ErrSyntax,
			"the reason the cell was rejected has to survive into the error")
		require.ErrorContains(t, err, "1.234,56")

		err = SmartAssign(reflect.ValueOf(&f).Elem(), reflect.ValueOf("1.234,56"), nil, lenient, nil)
		require.NoError(t, err)
		require.Equal(t, 1234.56, f, "the default parser is unchanged")
	})
}

// TestDefaultsAcceptCommonSpellings pins the spellings a table source
// writes that the standard library does not parse.
//
// They are defaults rather than configuration because each has a single
// possible reading in the destination it is parsed into: a boolean column
// holding "on" can only mean true, and a numeric column holding "N/A" can
// only mean absent, so accepting them cannot read a cell wrong. A source
// that means something else by one of them says so on the field.
func TestDefaultsAcceptCommonSpellings(t *testing.T) {
	p := NewStringParser()

	t.Run("booleans beyond strconv", func(t *testing.T) {
		for _, str := range []string{"yes", "Yes", "YES", "on", "On", "ON", "y", "Y"} {
			b, err := p.ParseBool(str)
			require.NoErrorf(t, err, "ParseBool(%q)", str)
			require.Truef(t, b, "ParseBool(%q)", str)
			_, strconvErr := strconv.ParseBool(str)
			require.Errorf(t, strconvErr, "%q is only worth pinning because strconv rejects it", str)
		}
		for _, str := range []string{"no", "No", "NO", "off", "Off", "OFF", "n", "N"} {
			b, err := p.ParseBool(str)
			require.NoErrorf(t, err, "ParseBool(%q)", str)
			require.Falsef(t, b, "ParseBool(%q)", str)
		}
	})

	t.Run("the absent value of every ecosystem that writes into a table", func(t *testing.T) {
		for _, str := range []string{
			"",                 // an empty cell
			"nil",              // Go
			"<nil>",            // what fmt prints for one
			"null",             // JSON
			"NULL",             // SQL
			"None",             // Python
			"N/A", "n/a", "NA", // a spreadsheet
		} {
			require.Truef(t, p.IsNil(str), "IsNil(%q)", str)
		}

		// Not nil, and each for its own reason: in a numeric column a
		// lone "-" is as likely to be a malformed number as a
		// placeholder, and reading it as nil would assign 0 to a cell
		// nobody checked; "NaN" is a float value strconv parses; "0"
		// and "false" are values.
		for _, str := range []string{"-", "--", "NaN", "0", "false", "?"} {
			require.Falsef(t, p.IsNil(str), "IsNil(%q)", str)
		}
	})

	t.Run("time layouts a database and an API write", func(t *testing.T) {
		want := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)
		for _, str := range []string{
			"2024-03-15T14:30:00Z",       // RFC3339
			"2024-03-15T14:30:00",        // ISO without a zone, UTC assumed
			"2024-03-15 14:30:00",        // SQL
			"2024-03-15 14:30:00+00:00",  // PostgreSQL timestamptz
			"2024-03-15 14:30:00+00",     // PostgreSQL, hour only offset
			"2024-03-15 14:30:00.000000", // a fractional second Go parses without a layout for it
		} {
			got, err := p.ParseTime(str)
			require.NoErrorf(t, err, "ParseTime(%q)", str)
			require.Truef(t, got.Equal(want), "ParseTime(%q) = %v", str, got)
		}
	})
}
