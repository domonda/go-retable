package csvtable

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"
)

// encodeUTF16LE encodes s as UTF-16LE without a byte order mark.
func encodeUTF16LE(s string) []byte {
	var buf bytes.Buffer
	for _, r := range s {
		buf.WriteByte(byte(r))
		buf.WriteByte(byte(r >> 8))
	}
	return buf.Bytes()
}

func encodeCharmap(t *testing.T, cm *charmap.Charmap, s string) []byte {
	t.Helper()
	b, err := cm.NewEncoder().Bytes([]byte(s))
	require.NoError(t, err)
	return b
}

// TestGetCharsetEncodingNames pins the encoding names that
// FormatDetectionConfig.Encodings and Format.Encoding accept. The names
// come from the display names of golang.org/x/text/encoding/charmap,
// which write a space where the IANA names write a hyphen. Changing
// this set would break every stored Format and every configuration.
func TestGetCharsetEncodingNames(t *testing.T) {
	accepted := []struct{ name, want string }{
		{"UTF-8", "UTF-8"},
		{"UTF-16LE", "UTF-16LE"},
		{"UTF-16BE", "UTF-16BE"},
		{"UTF-32LE", "UTF-32LE"},
		{"UTF-32BE", "UTF-32BE"},
		{"ISO 8859-1", "ISO 8859-1"},
		{"Windows 1252", "Windows 1252"},
		{"Macintosh", "Macintosh"},
		// The name is matched case insensitively
		{"utf-8", "UTF-8"},
		{"iso 8859-1", "ISO 8859-1"},
		{"WINDOWS 1252", "Windows 1252"},
		// A missing hyphen after UTF is added
		{"UTF8", "UTF-8"},
		{"utf16le", "UTF-16LE"},
		// Names that golang.org/x/text does not know by itself
		{"ISO-8859-6E", "ISO-8859-6E"},
		{"iso-8859-8i", "ISO-8859-8I"},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := getCharsetEncoding(tt.name)
			require.NoError(t, err)
			require.Equal(t, tt.want, enc.name)
		})
	}

	// The hyphenated spellings of the charmap names are not display
	// names and are not accepted, only the spellings with a space are.
	rejected := []string{"", "Windows-1252", "ISO-8859-1", "latin1", "nonsense", "UTF-7"}
	for _, name := range rejected {
		t.Run("rejected "+name, func(t *testing.T) {
			_, err := getCharsetEncoding(name)
			require.Error(t, err)
		})
	}
}

// TestParseDetectFormatEncodings covers the encoding detection of
// ParseDetectFormat, which had no test at all before. The fixtures use
// the characters from FormatDetectionConfig.EncodingTests, because
// those are what the detection counts to score an encoding.
func TestParseDetectFormatEncodings(t *testing.T) {
	// ISO 8859-1 and Windows 1252 are identical above 0xA0, so umlauts
	// alone score equally for both and the config order decides. Only a
	// character below 0xA0 like the Euro sign tells them apart.
	umlauts := "Name;Ort\nMüller;Köln\nWeiß;Österreich\n"
	euro := "Name;Betrag\nMüller;1.234,56 €\n"

	tests := []struct {
		name string
		csv  []byte
		want string
	}{
		{
			name: "UTF-8 without BOM",
			csv:  []byte(umlauts),
			want: "UTF-8",
		},
		{
			name: "ISO 8859-1 wins the tie with Windows 1252",
			csv:  encodeCharmap(t, charmap.ISO8859_1, umlauts),
			want: "ISO 8859-1",
		},
		{
			name: "Euro sign identifies Windows 1252",
			csv:  encodeCharmap(t, charmap.Windows1252, euro),
			want: "Windows 1252",
		},
		{
			name: "Macintosh",
			csv:  encodeCharmap(t, charmap.Macintosh, umlauts),
			want: "Macintosh",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, format, err := ParseDetectFormat(tt.csv, nil)
			require.NoError(t, err)
			require.Equal(t, tt.want, format.Encoding)
			require.Equal(t, "Name", rows[0][0])
			require.Equal(t, "Müller", rows[1][0])
		})
	}
}

// TestParseDetectFormatBOM covers that a byte order mark decides the
// encoding on its own and is not part of the first cell.
func TestParseDetectFormatBOM(t *testing.T) {
	csv := "Name;Ort\nMüller;Köln\n"

	tests := []struct {
		name string
		csv  []byte
		want string
	}{
		{
			name: "UTF-8 BOM",
			csv:  append([]byte(bomUTF8), csv...),
			want: "UTF-8",
		},
		{
			name: "UTF-16LE BOM",
			csv:  append([]byte(bomUTF16LE), encodeUTF16LE(csv)...),
			want: "UTF-16LE",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, format, err := ParseDetectFormat(tt.csv, nil)
			require.NoError(t, err)
			require.Equal(t, tt.want, format.Encoding)
			require.Equal(t, "Name", rows[0][0], "byte order mark must not be part of the first cell")
			require.Equal(t, "Müller", rows[1][0])
		})
	}
}

// TestParseWithFormatEncodingRoundTrip is the invariant that protects
// the encoding names: whatever ParseDetectFormat reports must be usable
// as Format.Encoding for ParseWithFormat and produce the same rows.
func TestParseWithFormatEncodingRoundTrip(t *testing.T) {
	csv := "Name;Ort\nMüller;Köln\nWeiß;Österreich\n"

	inputs := [][]byte{
		[]byte(csv),
		append([]byte(bomUTF8), csv...),
		encodeCharmap(t, charmap.ISO8859_1, csv),
		encodeCharmap(t, charmap.Windows1252, csv+"1.234,56 €\n"),
		encodeCharmap(t, charmap.Macintosh, csv),
		append([]byte(bomUTF16LE), encodeUTF16LE(csv)...),
	}
	for _, in := range inputs {
		detectedRows, format, err := ParseDetectFormat(in, nil)
		require.NoError(t, err)

		rows, err := ParseWithFormat(in, format)
		require.NoErrorf(t, err, "ParseWithFormat with detected encoding %q", format.Encoding)
		require.Equalf(t, detectedRows, rows, "rows for detected encoding %q", format.Encoding)
	}
}

func TestParseWithFormatUTF16OddLength(t *testing.T) {
	format := &Format{Encoding: "UTF-16LE", Separator: ";", Newline: "\n"}

	_, err := ParseWithFormat([]byte{'a', 0x00, 'b'}, format)
	require.ErrorContains(t, err, "odd length of UTF-16 string")
}

func TestParseWithFormatUnknownEncoding(t *testing.T) {
	format := &Format{Encoding: "Windows-1252", Separator: ";", Newline: "\n"}

	_, err := ParseWithFormat([]byte("a;b\n"), format)
	require.ErrorContains(t, err, "encoding not found")
}
