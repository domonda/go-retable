package csvtable

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/encoding/unicode/utf32"
)

// This file replaces the parts of github.com/domonda/go-types/charset
// that csvtable used, so that go-retable does not depend on go-types.
// go-types is MIT licensed with the same copyright holder as this
// repository. The encoding name matching and the autoDecode algorithm
// are kept identical to the original because the accepted names are
// part of the FormatDetectionConfig.Encodings and Format.Encoding
// contract, while the decoding itself uses the standard library and
// golang.org/x/text directly instead of the wrapper types of go-types.

// charsetEncoding decodes bytes of a character encoding into UTF-8.
type charsetEncoding struct {
	name   string
	decode func(b []byte) ([]byte, error)
}

// charsetBOM is a Unicode Byte Order Mark.
type charsetBOM string

const (
	noBOM      charsetBOM = ""
	bomUTF8    charsetBOM = "\xEF\xBB\xBF"
	bomUTF16BE charsetBOM = "\xFE\xFF"
	bomUTF16LE charsetBOM = "\xFF\xFE"
	bomUTF32BE charsetBOM = "\x00\x00\xFE\xFF"
	bomUTF32LE charsetBOM = "\xFF\xFE\x00\x00"
)

// allBOMs lists the byte order marks in the order in which they are
// tested. The four byte marks come before the two byte ones, because
// the UTF-16LE mark FF FE is a prefix of the UTF-32LE mark FF FE 00 00
// and would otherwise make it unreachable.
//
// Those two are not distinguishable from the bytes alone: UTF-16LE text
// whose first character is U+0000 serializes to the same FF FE 00 00
// and is reported as UTF-32LE here. Preferring the longer mark is the
// conventional resolution, used by ICU, .NET and file(1) among others,
// because a leading NUL is not plain text while real UTF-32LE data is.
//
// The ambiguity only exists while detecting an unknown encoding.
// Decoding with a known encoding goes through trimExpectedBOM instead,
// where FF FE can only mean UTF-16LE.
var allBOMs = []charsetBOM{bomUTF8, bomUTF32LE, bomUTF32BE, bomUTF16LE, bomUTF16BE}

// name returns the encoding name of the byte order mark,
// which is what format detection reports as Format.Encoding.
func (bom charsetBOM) name() string {
	switch bom {
	case bomUTF8:
		return "UTF-8"
	case bomUTF16BE:
		return "UTF-16BE"
	case bomUTF16LE:
		return "UTF-16LE"
	case bomUTF32BE:
		return "UTF-32BE"
	case bomUTF32LE:
		return "UTF-32LE"
	}
	return "No BOM"
}

// byteOrder returns the byte order of an UTF-16 or UTF-32
// byte order mark, or nil for any other one.
func (bom charsetBOM) byteOrder() binary.ByteOrder {
	switch bom {
	case bomUTF16LE, bomUTF32LE:
		return binary.LittleEndian
	case bomUTF16BE, bomUTF32BE:
		return binary.BigEndian
	}
	return nil
}

// trimExpectedBOM strips a leading bom from data and returns the rest.
//
// bom is what the caller already knows the encoding to be, so it is
// matched before falling back to detection. That order matters for the
// UTF-16LE and UTF-32LE overlap: the valid UTF-16LE sequence FF FE 00 00,
// a byte order mark followed by U+0000, is indistinguishable from the
// UTF-32LE mark, and detection resolves it to UTF-32LE. A caller that
// passes bomUTF16LE has already resolved that and must not be
// second-guessed.
func trimExpectedBOM(data []byte, bom charsetBOM) ([]byte, error) {
	if bom != noBOM && bytes.HasPrefix(data, []byte(bom)) {
		return data[len(bom):], nil
	}
	if dataBOM, _ := splitBOM(data); dataBOM != noBOM && dataBOM != bom {
		return nil, fmt.Errorf("wrong BOM in data: %v, expected: %v", []byte(dataBOM), []byte(bom))
	}
	return data, nil
}

// decode decodes data that follows the byte order mark.
// Data that begins with a further byte order mark has to repeat
// the same one, which is then removed as well.
func (bom charsetBOM) decode(data []byte) ([]byte, error) {
	data, err := trimExpectedBOM(data, bom)
	if err != nil {
		return nil, err
	}

	switch bom {
	case noBOM, bomUTF8:
		return data, nil
	case bomUTF16LE, bomUTF16BE:
		return decodeUTF16(data, bom.byteOrder())
	case bomUTF32LE, bomUTF32BE:
		return decodeUTF32AfterBOM(data, bom.byteOrder())
	}
	return nil, fmt.Errorf("unsupported BOM: %v", []byte(bom))
}

// splitBOM returns the leading byte order mark of b
// and the remaining bytes after it.
func splitBOM(b []byte) (charsetBOM, []byte) {
	for _, bom := range allBOMs {
		if bytes.HasPrefix(b, []byte(bom)) {
			return bom, b[len(bom):]
		}
	}
	return noBOM, b
}

// trimUTF8BOM removes a leading UTF-8 byte order mark.
func trimUTF8BOM(b []byte) []byte {
	return bytes.TrimPrefix(b, []byte(bomUTF8))
}

func decodeUTF16(b []byte, byteOrder binary.ByteOrder) ([]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}
	if len(b)&1 != 0 {
		return nil, fmt.Errorf("odd length of UTF-16 string: %d", len(b))
	}
	endian := unicode.LittleEndian
	if byteOrder == binary.BigEndian {
		endian = unicode.BigEndian
	}
	return unicode.UTF16(endian, unicode.IgnoreBOM).NewDecoder().Bytes(b)
}

func decodeUTF32(b []byte, byteOrder binary.ByteOrder) ([]byte, error) {
	// A truncated code unit is a broken file, not a replacement
	// character. golang.org/x/text decodes a partial trailing unit to
	// U+FFFD with a nil error, which sanitizeUTF8 then turns into a
	// plain space, so the last cell silently changes instead of the
	// file being rejected. decodeUTF16 rejects an odd length the same way.
	if len(b)%4 != 0 {
		return nil, fmt.Errorf("length of UTF-32 string is not a multiple of 4: %d", len(b))
	}
	endian := utf32.LittleEndian
	if byteOrder == binary.BigEndian {
		endian = utf32.BigEndian
	}
	return utf32.UTF32(endian, utf32.IgnoreBOM).NewDecoder().Bytes(b)
}

// decodeUTF32AfterBOM returns nil instead of an empty slice for empty
// data, like decodeUTF16 does. Decoding after a byte order mark and
// decoding through an encoding returned by getCharsetEncoding differ
// in this detail, which is kept from the go-types implementation.
func decodeUTF32AfterBOM(b []byte, byteOrder binary.ByteOrder) ([]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}
	return decodeUTF32(b, byteOrder)
}

// decodeUTF32Encoding decodes UTF-32 for a named encoding, stripping a
// leading byte order mark of that encoding.
//
// golang.org/x/text decodes the mark to U+FEFF instead of removing it,
// and go-types passed it straight through, so an encoding named
// "UTF-32LE" turned the mark into the first character of the first cell.
// That was unreachable before the byte order mark detection was fixed,
// because UTF-32LE data was never detected as UTF-32LE.
//
// The UTF-16 encodings need no counterpart: charsetBOM.decode already
// is one for them, so utfEncodings uses the method directly.
func decodeUTF32Encoding(b []byte, bom charsetBOM) ([]byte, error) {
	b, err := trimExpectedBOM(b, bom)
	if err != nil {
		return nil, err
	}
	return decodeUTF32(b, bom.byteOrder())
}

var utfEncodings = map[string]*charsetEncoding{
	"UTF-8":    {name: "UTF-8", decode: bomUTF8.decode},
	"UTF-16LE": {name: "UTF-16LE", decode: bomUTF16LE.decode},
	"UTF-16BE": {name: "UTF-16BE", decode: bomUTF16BE.decode},
	"UTF-32LE": {name: "UTF-32LE", decode: func(b []byte) ([]byte, error) { return decodeUTF32Encoding(b, bomUTF32LE) }},
	"UTF-32BE": {name: "UTF-32BE", decode: func(b []byte) ([]byte, error) { return decodeUTF32Encoding(b, bomUTF32BE) }},
}

// sharedEncodings maps names that golang.org/x/text does not
// know to the charmap that implements them.
var sharedEncodings = map[string]*charmap.Charmap{
	"ISO-8859-6E": charmap.ISO8859_6,
	"ISO-8859-6I": charmap.ISO8859_6,
	"ISO-8859-8E": charmap.ISO8859_8,
	"ISO-8859-8I": charmap.ISO8859_8,
}

// charmapsByUpperName indexes the display names of
// golang.org/x/text/encoding/charmap by their upper case spelling,
// so that getCharsetEncoding does not have to upper case every one
// of them again on every lookup.
var charmapsByUpperName = sync.OnceValue(func() map[string]*charmap.Charmap {
	m := make(map[string]*charmap.Charmap, len(charmap.All))
	for _, e := range charmap.All {
		if cm, ok := e.(*charmap.Charmap); ok {
			m[strings.ToUpper(cm.String())] = cm
		}
	}
	return m
})

func charmapEncoding(name string, cm *charmap.Charmap) *charsetEncoding {
	return &charsetEncoding{
		name: name,
		decode: func(b []byte) ([]byte, error) {
			// Excel writes a UTF-8 byte order mark in front of files it
			// otherwise encodes in a code page, and a code page has no
			// mark of its own, so it can only be that one. Decoding it
			// as text puts "ï»¿" in front of the first column title,
			// which then matches no struct field and leaves that whole
			// column at its zero value for every row, silently.
			return cm.NewDecoder().Bytes(trimUTF8BOM(b))
		},
	}
}

// getCharsetEncoding returns the encoding with the passed name.
// The name is matched case insensitively against the display names
// of golang.org/x/text/encoding/charmap, like "ISO 8859-1",
// "Windows 1252" and "Macintosh", which are written with a space
// and not with a hyphen. A missing hyphen after "UTF" is added,
// so "UTF8" is accepted for "UTF-8".
func getCharsetEncoding(name string) (*charsetEncoding, error) {
	nameUpper := strings.ToUpper(name)
	if strings.HasPrefix(nameUpper, "UTF") && !strings.HasPrefix(nameUpper, "UTF-") {
		nameUpper = "UTF-" + nameUpper[3:]
	}
	if enc, ok := utfEncodings[nameUpper]; ok {
		return enc, nil
	}
	if cm, ok := charmapsByUpperName()[nameUpper]; ok {
		return charmapEncoding(cm.String(), cm), nil
	}
	if cm, ok := sharedEncodings[nameUpper]; ok {
		return charmapEncoding(nameUpper, cm), nil
	}
	return nil, fmt.Errorf("encoding not found: %q", name)
}

// autoDecode tries to automatically decode the passed data as text.
// If data begins with an UTF BOM, then the BOM information will be used for decoding.
// If there is no BOM, then data will be decoded with all passed encodings
// and the passed keyWords will be counted in the error free decoded texts.
// The decoded text and encoding name will be returned for the encoding with
// the most key-word matches. Encodings with the same number of matches keep
// the order in which they were passed, so the caller decides which one wins.
// If no key-word was found for any of the encodings,
// then data will be returned unchanged with an empty string as encoding name.
func autoDecode(data []byte, encodings []*charsetEncoding, keyWords []string) (text []byte, encName string, err error) {
	if len(data) == 0 {
		return nil, "", nil
	}

	// Pass the whole data, not the remainder: bom.decode removes the
	// mark itself, so splitting it off here as well would consume two
	// consecutive marks while a named encoding consumes only one, and
	// the Format reported here would not reproduce these cells.
	bom, _ := splitBOM(data)
	if bom != noBOM {
		text, err = bom.decode(data)
		if err != nil {
			return nil, "", err
		}
		return text, bom.name(), nil
	}

	keyWordsBytes := make([][]byte, len(keyWords))
	for i, keyWord := range keyWords {
		keyWordsBytes[i] = []byte(keyWord)
	}

	// Only the best scoring decoded text is kept, so that the decoded
	// copies of the whole input made for the other encodings can be
	// collected right away. The comparison is a strict greater than,
	// which keeps the first passed encoding of equally scoring ones.
	var bestScore int
	for _, enc := range encodings {
		decoded, decodeErr := enc.decode(data)
		if decodeErr != nil {
			continue
		}
		score := 0
		for _, keyWord := range keyWordsBytes {
			score += bytes.Count(decoded, keyWord)
		}
		if score > bestScore {
			text, encName, bestScore = decoded, enc.name, score
		}
	}

	if bestScore == 0 {
		return data, "", nil
	}
	return text, encName, nil
}
