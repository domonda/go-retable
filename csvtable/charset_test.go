package csvtable

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"
)

// encodeUTF16LE encodes s as UTF-16LE without a byte order mark.
// It is hand written on purpose, so that it stays an oracle
// independent of the golang.org/x/text decoder under test.
// Only the basic multilingual plane is encoded, runes above it
// would need surrogate pairs.
func encodeUTF16LE(s string) []byte {
	var buf bytes.Buffer
	for _, r := range s {
		buf.WriteByte(byte(r))
		buf.WriteByte(byte(r >> 8))
	}
	return buf.Bytes()
}

// encodeUTF32LE encodes s as UTF-32LE without a byte order mark.
// Hand written for the same reason as encodeUTF16LE.
func encodeUTF32LE(s string) []byte {
	var buf bytes.Buffer
	for _, r := range s {
		buf.WriteByte(byte(r))
		buf.WriteByte(byte(r >> 8))
		buf.WriteByte(byte(r >> 16))
		buf.WriteByte(byte(r >> 24))
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
		append([]byte(bomUTF32LE), encodeUTF32LE(csv)...),
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

// TestSplitBOM covers the detection order. The UTF-16LE mark FF FE is a
// prefix of the UTF-32LE mark FF FE 00 00, so the longer one has to be
// tested first or UTF-32LE data is silently decoded as UTF-16LE.
func TestSplitBOM(t *testing.T) {
	tests := []struct {
		name string
		b    []byte
		want charsetBOM
		rest []byte
	}{
		{name: "no BOM", b: []byte("hi"), want: noBOM, rest: []byte("hi")},
		{name: "UTF-8", b: append([]byte(bomUTF8), 'h'), want: bomUTF8, rest: []byte{'h'}},
		{name: "UTF-16BE", b: []byte{0xFE, 0xFF, 0x00, 'h'}, want: bomUTF16BE, rest: []byte{0x00, 'h'}},
		{name: "UTF-16LE", b: []byte{0xFF, 0xFE, 'h', 0x00}, want: bomUTF16LE, rest: []byte{'h', 0x00}},
		{name: "UTF-32BE", b: []byte{0x00, 0x00, 0xFE, 0xFF, 0, 0, 0, 'h'}, want: bomUTF32BE, rest: []byte{0, 0, 0, 'h'}},

		// All 4 bytes of the UTF-32LE mark have to be split off, splitting
		// only the first 2 would leave a bogus 00 00 before the payload.
		{name: "UTF-32LE", b: []byte{0xFF, 0xFE, 0x00, 0x00, 'h', 0, 0, 0}, want: bomUTF32LE, rest: []byte{'h', 0, 0, 0}},

		// Only FF FE 00 00 wins over UTF-16LE, FF FE followed by
		// anything else is still UTF-16LE.
		{name: "UTF-16LE with 00 in the second byte pair", b: []byte{0xFF, 0xFE, 'h', 0x00, 0x00, 0x01}, want: bomUTF16LE, rest: []byte{'h', 0x00, 0x00, 0x01}},
		// Too short to be the UTF-32LE mark, so it can only be UTF-16LE
		{name: "UTF-16LE BOM only", b: []byte{0xFF, 0xFE}, want: bomUTF16LE, rest: []byte{}},
		{name: "UTF-16LE BOM plus one byte", b: []byte{0xFF, 0xFE, 0x00}, want: bomUTF16LE, rest: []byte{0x00}},

		// Accepted ambiguity: UTF-16LE text starting with U+0000
		// serializes to the same bytes and is reported as UTF-32LE,
		// because a leading NUL is not plain text in practice.
		{name: "UTF-16LE starting with U+0000 reads as UTF-32LE", b: []byte{0xFF, 0xFE, 0x00, 0x00, 'h', 0x00}, want: bomUTF32LE, rest: []byte{'h', 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bom, rest := splitBOM(tt.b)
			require.Equal(t, tt.want, bom)
			require.Equal(t, tt.rest, rest)
		})
	}
}

// TestDecodeKnownEncodingBeatsBOMAmbiguity covers that the ambiguity
// resolved by splitBOM only applies while guessing. A caller that names
// the encoding has resolved it already and must not be second-guessed,
// otherwise asking for the encoding you know turns valid input into an
// error.
func TestDecodeKnownEncodingBeatsBOMAmbiguity(t *testing.T) {
	// FF FE 00 00 is the UTF-32LE mark, but equally a UTF-16LE mark
	// followed by U+0000
	ambiguous := []byte{0xFF, 0xFE, 0x00, 0x00, 'h', 0x00}

	utf16, err := decodeUTF16Encoding(ambiguous, bomUTF16LE)
	require.NoError(t, err)
	require.Equal(t, []byte("\x00h"), utf16)

	viaBOM, err := bomUTF16LE.decode(ambiguous)
	require.NoError(t, err)
	require.Equal(t, []byte("\x00h"), viaBOM)

	// A UTF-16LE mark not followed by 00 00 still decodes normally
	normal, err := decodeUTF16Encoding([]byte{0xFF, 0xFE, 'h', 0, 'i', 0}, bomUTF16LE)
	require.NoError(t, err)
	require.Equal(t, []byte("hi"), normal)

	// A mark of a different encoding is still an error
	_, err = decodeUTF16Encoding([]byte{0xFE, 0xFF, 0x00, 'h'}, bomUTF16LE)
	require.ErrorContains(t, err, "wrong BOM in data")
}

// TestParseDetectFormatUTF32LEBOM is the user visible effect of the
// detection order: a UTF-32LE file used to be decoded as UTF-16LE into
// NUL padded garbage without any error.
func TestParseDetectFormatUTF32LEBOM(t *testing.T) {
	csv := "Name;Ort\nMüller;Köln\n"
	data := append([]byte(bomUTF32LE), encodeUTF32LE(csv)...)

	rows, format, err := ParseDetectFormat(data, nil)
	require.NoError(t, err)
	require.Equal(t, "UTF-32LE", format.Encoding)
	require.Equal(t, [][]string{{"Name", "Ort"}, {"Müller", "Köln"}}, rows[:2])

	// The byte order mark must not become the first character of the
	// first cell when the detected encoding is used explicitly
	same, err := ParseWithFormat(data, format)
	require.NoError(t, err)
	require.Equal(t, rows, same)
}

// TestDecodeUTF32TruncatedIsAnError covers that a UTF-32 file whose
// last code unit is cut short is reported instead of silently altered.
// golang.org/x/text decodes a partial trailing unit to U+FFFD with a nil
// error, and sanitizeUTF8 then turns that into a plain space, so the
// last cell quietly gained a character and the row count could change.
// decodeUTF16 has rejected an odd length all along.
func TestDecodeUTF32TruncatedIsAnError(t *testing.T) {
	csv := "Text;Betrag\nA;100\n"
	full := append([]byte(bomUTF32LE), encodeUTF32LE(csv)...)

	rows, format, err := ParseDetectFormat(full, nil)
	require.NoError(t, err)
	require.Equal(t, "UTF-32LE", format.Encoding)
	require.Equal(t, [][]string{{"Text", "Betrag"}, {"A", "100"}}, rows[:2])

	for cut := 1; cut <= 3; cut++ {
		t.Run(fmt.Sprintf("truncated by %d", cut), func(t *testing.T) {
			_, _, err := ParseDetectFormat(full[:len(full)-cut], nil)
			require.ErrorContains(t, err, "not a multiple of 4")
		})
	}
}

// TestRepeatedBOMRoundTrip covers that detection and a named encoding
// consume the same number of byte order marks. Detection used to split
// one mark off and then strip a second one while a named encoding
// stripped only one, so a file that begins with two marks parsed
// differently through ParseDetectFormat than through ParseWithFormat
// with the very Format that detection had just reported, leaving an
// invisible U+FEFF at the start of the first cell.
func TestRepeatedBOMRoundTrip(t *testing.T) {
	csv := "Name;Ort\nMüller;Köln\n"

	tests := []struct {
		name string
		data []byte
	}{
		{name: "UTF-8", data: append(append([]byte(bomUTF8), []byte(bomUTF8)...), csv...)},
		{name: "UTF-16LE", data: append(append([]byte(bomUTF16LE), []byte(bomUTF16LE)...), encodeUTF16LE(csv)...)},
		{name: "UTF-32LE", data: append(append([]byte(bomUTF32LE), []byte(bomUTF32LE)...), encodeUTF32LE(csv)...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detected, format, err := ParseDetectFormat(tt.data, nil)
			require.NoError(t, err)

			same, err := ParseWithFormat(tt.data, format)
			require.NoErrorf(t, err, "ParseWithFormat with detected encoding %q", format.Encoding)
			require.Equalf(t, detected, same, "detection and the encoding it reported must agree for %q", format.Encoding)
		})
	}
}
