package csvtable

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFormat_Validate covers every rejection of Validate, because it is the
// only gate in front of ParseWithFormat: a format that gets through it is used
// to split bytes into fields without any further checking.
func TestFormat_Validate(t *testing.T) {
	tests := []struct {
		name    string
		format  *Format
		wantErr string
	}{
		{
			name:    "nil format",
			format:  nil,
			wantErr: "<nil> csv.Format",
		},
		{
			name:    "missing encoding",
			format:  &Format{Separator: ",", Newline: "\n"},
			wantErr: "missing csv.Format.Encoding",
		},
		{
			name:    "missing separator",
			format:  &Format{Encoding: "UTF-8", Newline: "\n"},
			wantErr: "missing csv.Format.Separator",
		},
		{
			name:    "multi character separator",
			format:  &Format{Encoding: "UTF-8", Separator: ";;", Newline: "\n"},
			wantErr: `invalid csv.Format.Separator: ";;"`,
		},
		{
			// A quote separator would make every quote in readLines
			// mean two things at once, so it can never be one.
			name:    "quote separator",
			format:  &Format{Encoding: "UTF-8", Separator: `"`, Newline: "\n"},
			wantErr: `invalid csv.Format.Separator: "\""`,
		},
		{
			// A newline separator would be consumed by the line
			// splitting before any field could be separated by it.
			name:    "newline separator",
			format:  &Format{Encoding: "UTF-8", Separator: "\n", Newline: "\n"},
			wantErr: `invalid csv.Format.Separator: "\n"`,
		},
		{
			name:    "NUL separator",
			format:  &Format{Encoding: "UTF-8", Separator: "\x00", Newline: "\n"},
			wantErr: `invalid csv.Format.Separator`,
		},
		{
			name:    "DEL separator",
			format:  &Format{Encoding: "UTF-8", Separator: "\x7f", Newline: "\n"},
			wantErr: `invalid csv.Format.Separator`,
		},
		{
			name:    "missing newline",
			format:  &Format{Encoding: "UTF-8", Separator: ","},
			wantErr: "missing csv.Format.Newline",
		},
		{
			name:    "unsupported newline",
			format:  &Format{Encoding: "UTF-8", Separator: ",", Newline: "\r"},
			wantErr: `invalid csv.Format.Newline: "\r"`,
		},
		{
			name:   "comma and LF",
			format: &Format{Encoding: "UTF-8", Separator: ",", Newline: "\n"},
		},
		{
			// Tab is the one control character that is a real world
			// separator, so it must pass the control character check.
			name:   "tab separated",
			format: &Format{Encoding: "UTF-8", Separator: "\t", Newline: "\r\n"},
		},
		{
			name:   "NewFormat is valid",
			format: NewFormat(";"),
		},
		{
			name:   "LFCR newline",
			format: &Format{Encoding: "UTF-8", Separator: ",", Newline: "\n\r"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.format.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// Test_validSeparator walks the byte boundaries of the separator check instead
// of only its named cases, because it is stated as a range comparison whose
// ends are easy to get wrong by one.
func Test_validSeparator(t *testing.T) {
	invalid := map[byte]string{
		0x00:     "NUL",
		0x09 - 1: "before tab",
		0x0a:     "LF",
		0x0d:     "CR",
		0x1f:     "last control character below space",
		'"':      "quote",
		0x7f:     "DEL",
	}
	valid := map[byte]string{
		0x09: "tab",
		' ':  "space",
		',':  "comma",
		';':  "semicolon",
		'|':  "pipe",
		0x7e: "last printable ASCII",
		0x80: "first byte above DEL",
		0xff: "highest byte",
	}
	for c, name := range invalid {
		t.Run(fmt.Sprintf("invalid %s 0x%02x", name, c), func(t *testing.T) {
			require.False(t, validSeparator(c))
		})
	}
	for c, name := range valid {
		t.Run(fmt.Sprintf("valid %s 0x%02x", name, c), func(t *testing.T) {
			require.True(t, validSeparator(c))
		})
	}
}

// TestEscapeQuotes covers the exported RFC 4180 escaper, which had no
// test. Every quote in a value has to be doubled or the field ends
// early when the file is read back.
func TestEscapeQuotes(t *testing.T) {
	tests := []struct{ in, want string }{
		{in: "", want: ""},
		{in: "plain", want: "plain"},
		{in: `say "hi"`, want: `say ""hi""`},
		{in: `"`, want: `""`},
		{in: `""`, want: `""""`},
		{in: `a"b"c`, want: `a""b""c`},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			require.Equal(t, tt.want, EscapeQuotes(tt.in))
		})
	}
}
