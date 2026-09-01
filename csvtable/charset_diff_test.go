package csvtable

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	upstream "github.com/domonda/go-types/charset"
	"golang.org/x/text/encoding/charmap"
)

// The tests in this file compare charset.go against the
// github.com/domonda/go-types/charset implementation it replaces.
// They exist only for the commit that introduces charset.go and are
// deleted together with the go-types dependency, so the equivalence
// claim is reviewable in the diff instead of being asserted in prose.

func upstreamDecode(t *testing.T, name string, b []byte) (dec []byte, errored bool) {
	t.Helper()
	enc, err := upstream.GetEncoding(name)
	if err != nil {
		return nil, true
	}
	dec, err = enc.Decode(b)
	return dec, err != nil
}

func portedDecode(t *testing.T, name string, b []byte) (dec []byte, errored bool) {
	t.Helper()
	enc, err := getCharsetEncoding(name)
	if err != nil {
		return nil, true
	}
	dec, err = enc.decode(b)
	return dec, err != nil
}

func TestDiffGetCharsetEncoding(t *testing.T) {
	names := upstream.EncodingNames()
	names = append(names,
		"UTF-8", "UTF-16LE", "ISO 8859-1", "Windows 1252", "Macintosh",
		"utf8", "UTF8", "utf-16le", "iso 8859-1", "windows 1252", "macintosh",
		"ISO-8859-6E", "iso-8859-8i",
		"Windows-1252", "ISO-8859-1", "nonsense", "UTF-7",
	)
	sample := []byte("Grüße, Ärger und €uro; a,b\r\nx;y\n")

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			upEnc, upErr := upstream.GetEncoding(name)
			gotEnc, gotErr := getCharsetEncoding(name)

			require.Equal(t, upErr != nil, gotErr != nil, "GetEncoding(%q) error", name)
			if upErr != nil {
				return
			}
			require.Equal(t, upEnc.Name(), gotEnc.name, "GetEncoding(%q).Name()", name)

			upDec, upDecErr := upEnc.Decode(sample)
			gotDec, gotDecErr := gotEnc.decode(sample)
			require.Equal(t, upDecErr != nil, gotDecErr != nil, "Decode error for %q", name)
			if upDecErr == nil {
				require.Equal(t, upDec, gotDec, "Decode output for %q", name)
			}
		})
	}
}

// TestDiffGetCharsetEncodingEmptyName documents the one intentional
// difference: upstream resolves the empty name to CodePage037 through a
// branch comparing against the second result of charmap.Charmap.ID(),
// which is always the empty string, so any name that is not a display
// name silently returned the first charmap. The port reports an error.
func TestDiffGetCharsetEncodingEmptyName(t *testing.T) {
	upEnc, upErr := upstream.GetEncoding("")
	require.NoError(t, upErr)
	require.Equal(t, "", upEnc.Name())

	_, gotErr := getCharsetEncoding("")
	require.Error(t, gotErr)
}

func TestDiffDecodeAllNames(t *testing.T) {
	corpus := [][]byte{
		nil,
		{},
		[]byte("plain ascii"),
		[]byte("Grüße"),
		{0xFF},
		{0xFF, 0xFE},
		{0xFE, 0xFF},
		{0x00, 0x00, 0xFE, 0xFF},
		{0xEF, 0xBB, 0xBF, 'a'},
		{0xFF, 0xFE, 'a', 0x00},
		{0xFE, 0xFF, 0x00, 'a'},
		{0xFF, 0xFE, 'a', 0x00, 'b'}, // odd length after BOM
		{0xD8, 0x00},                 // unpaired surrogate
		{0x00, 0xD8},
	}
	for i := range 256 {
		corpus = append(corpus, []byte{byte(i)}, []byte{byte(i), byte(255 - i)})
	}
	r := rand.New(rand.NewSource(7))
	for range 2000 {
		b := make([]byte, r.Intn(9))
		r.Read(b)
		corpus = append(corpus, b)
	}

	names := []string{"UTF-8", "UTF-16LE", "UTF-16BE", "UTF-32LE", "UTF-32BE", "ISO 8859-1", "Windows 1252", "Macintosh"}
	for _, name := range names {
		for _, b := range corpus {
			upDec, upErr := upstreamDecode(t, name, b)
			gotDec, gotErr := portedDecode(t, name, b)
			require.Equalf(t, upErr, gotErr, "Decode(%q, %#v) error", name, b)
			if !upErr {
				require.Equalf(t, upDec, gotDec, "Decode(%q, %#v)", name, b)
			}
		}
	}
}

func germanFixture() string {
	return "Name;Betrag\nMüller;1.234,56 €\nÖsterreich;99,00 §\nWeiß;7,00\n"
}

func encodeWith(t *testing.T, cm *charmap.Charmap, s string) []byte {
	t.Helper()
	b, err := cm.NewEncoder().Bytes([]byte(s))
	require.NoError(t, err)
	return b
}

func TestDiffAutoDecode(t *testing.T) {
	cfg := NewDefaultFormatDetectionConfig()

	var upEncodings []upstream.Encoding
	var gotEncodings []*charsetEncoding
	for _, name := range cfg.Encodings {
		upEnc, err := upstream.GetEncoding(name)
		require.NoError(t, err)
		upEncodings = append(upEncodings, upEnc)
		gotEnc, err := getCharsetEncoding(name)
		require.NoError(t, err)
		gotEncodings = append(gotEncodings, gotEnc)
	}

	german := germanFixture()
	corpus := [][]byte{
		nil,
		{},
		[]byte("plain ascii only"),
		[]byte(german),
		encodeWith(t, charmap.ISO8859_1, "Müller Öäü ß §"),
		encodeWith(t, charmap.Windows1252, german),
		encodeWith(t, charmap.Macintosh, "Müller Öäü"),
	}
	// UTF-16LE encoded fixture
	var utf16le bytes.Buffer
	for _, r := range german {
		utf16le.WriteByte(byte(r))
		utf16le.WriteByte(byte(r >> 8))
	}
	corpus = append(corpus, utf16le.Bytes())

	// every corpus entry with every BOM prepended
	boms := []charsetBOM{noBOM, bomUTF8, bomUTF16LE, bomUTF16BE, bomUTF32LE, bomUTF32BE}
	var withBOMs [][]byte
	for _, b := range corpus {
		for _, bom := range boms {
			withBOMs = append(withBOMs, append([]byte(bom), b...))
		}
	}
	corpus = append(corpus, withBOMs...)

	r := rand.New(rand.NewSource(11))
	for range 3000 {
		b := make([]byte, r.Intn(24))
		r.Read(b)
		corpus = append(corpus, b)
	}

	for _, b := range corpus {
		upText, upName, upErr := upstream.AutoDecode(b, upEncodings, cfg.EncodingTests)
		gotText, gotName, gotErr := autoDecode(b, gotEncodings, cfg.EncodingTests)
		require.Equalf(t, upErr != nil, gotErr != nil, "AutoDecode(%#v) error: %v vs %v", b, upErr, gotErr)
		if upErr != nil {
			continue
		}
		require.Equalf(t, upName, gotName, "AutoDecode(%#v) encoding name", b)
		require.Equalf(t, upText, gotText, "AutoDecode(%#v) text", b)
	}
}

// TestDiffUTF32LEBOM pins the one known limitation that is carried
// over unchanged: the 2 byte UTF-16LE BOM is tested before the 4 byte
// UTF-32LE BOM that begins with it, so UTF-32LE data is detected and
// decoded as UTF-16LE. Detecting the longer BOM first would break the
// UTF-16 decoding of data whose content starts with those bytes, and
// UTF-32 is not among the encodings that format detection tries.
func TestDiffUTF32LEBOM(t *testing.T) {
	data := append([]byte(bomUTF32LE), 'a', 0, 0, 0, 'b', 0, 0, 0)

	upText, upName, upErr := upstream.AutoDecode(data, nil, nil)
	gotText, gotName, gotErr := autoDecode(data, nil, nil)

	require.NoError(t, upErr)
	require.NoError(t, gotErr)
	require.Equal(t, upName, gotName)
	require.Equal(t, upText, gotText)
	require.Equal(t, "UTF-16LE", gotName)
	require.NotEqual(t, "ab", string(gotText))
}

func FuzzDiffAutoDecode(f *testing.F) {
	cfg := NewDefaultFormatDetectionConfig()
	var upEncodings []upstream.Encoding
	var gotEncodings []*charsetEncoding
	for _, name := range cfg.Encodings {
		upEnc, _ := upstream.GetEncoding(name)
		upEncodings = append(upEncodings, upEnc)
		gotEnc, _ := getCharsetEncoding(name)
		gotEncodings = append(gotEncodings, gotEnc)
	}

	f.Add([]byte("Müller;1.234,56 €"))
	f.Add([]byte{0xEF, 0xBB, 0xBF, 'a'})
	f.Add([]byte{0xFF, 0xFE, 'a', 0x00})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		upText, upName, upErr := upstream.AutoDecode(data, upEncodings, cfg.EncodingTests)
		gotText, gotName, gotErr := autoDecode(data, gotEncodings, cfg.EncodingTests)
		require.Equal(t, upErr != nil, gotErr != nil)
		if upErr != nil {
			return
		}
		require.Equal(t, upName, gotName)
		require.Equal(t, upText, gotText)
	})
}
