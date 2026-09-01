package csvtable

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"

	"golang.org/x/text/encoding/charmap"
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
// tested. The two byte UTF-16LE mark is tested before the four byte
// UTF-32LE mark that begins with it, so UTF-32LE data is detected as
// UTF-16LE. That is what go-types did and it is kept unchanged here,
// because the alternative ordering makes the UTF-16 decoding of data
// whose content happens to start with those bytes fail. UTF-32 is not
// among the encodings that format detection tries by default.
var allBOMs = []charsetBOM{bomUTF8, bomUTF16LE, bomUTF16BE, bomUTF32LE, bomUTF32BE}

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

// decode decodes data that follows the byte order mark.
// Data that begins with a further byte order mark has to repeat
// the same one, which is then removed as well.
func (bom charsetBOM) decode(data []byte) ([]byte, error) {
	dataBOM, data := splitBOM(data)
	if dataBOM != noBOM && dataBOM != bom {
		return nil, fmt.Errorf("wrong BOM in data: %v, expected: %v", []byte(dataBOM), []byte(bom))
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

func decodeUTF8(b []byte) ([]byte, error) {
	bom, rest := splitBOM(b)
	if bom != noBOM && bom != bomUTF8 {
		return nil, fmt.Errorf("wrong BOM in data: %v, expected: %v", []byte(bom), []byte(bomUTF8))
	}
	return rest, nil
}

func decodeUTF16(b []byte, byteOrder binary.ByteOrder) ([]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}
	if len(b)&1 != 0 {
		return nil, fmt.Errorf("odd length of UTF-16 string: %d", len(b))
	}
	// A byte order mark must match the expected byte order
	if bom, rest := splitBOM(b); bom != noBOM {
		expected := bomUTF16LE
		if byteOrder == binary.BigEndian {
			expected = bomUTF16BE
		}
		if bom != expected {
			return nil, fmt.Errorf("expected %s BOM but got %s", expected.name(), bom.name())
		}
		b = rest
	}
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = byteOrder.Uint16(b[i*2:])
	}
	var buf bytes.Buffer
	buf.Grow(len(b))
	for _, r := range utf16.Decode(u16) {
		buf.WriteRune(r)
	}
	return buf.Bytes(), nil
}

func decodeUTF32(b []byte, byteOrder binary.ByteOrder) ([]byte, error) {
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

var utfEncodings = map[string]*charsetEncoding{
	"UTF-8":    {name: "UTF-8", decode: decodeUTF8},
	"UTF-16LE": {name: "UTF-16LE", decode: func(b []byte) ([]byte, error) { return decodeUTF16(b, binary.LittleEndian) }},
	"UTF-16BE": {name: "UTF-16BE", decode: func(b []byte) ([]byte, error) { return decodeUTF16(b, binary.BigEndian) }},
	"UTF-32LE": {name: "UTF-32LE", decode: func(b []byte) ([]byte, error) { return decodeUTF32(b, binary.LittleEndian) }},
	"UTF-32BE": {name: "UTF-32BE", decode: func(b []byte) ([]byte, error) { return decodeUTF32(b, binary.BigEndian) }},
}

// sharedEncodings maps names that golang.org/x/text does not
// know to the charmap that implements them.
var sharedEncodings = map[string]*charmap.Charmap{
	"ISO-8859-6E": charmap.ISO8859_6,
	"ISO-8859-6I": charmap.ISO8859_6,
	"ISO-8859-8E": charmap.ISO8859_8,
	"ISO-8859-8I": charmap.ISO8859_8,
}

func charmapEncoding(name string, cm *charmap.Charmap) *charsetEncoding {
	return &charsetEncoding{
		name:   name,
		decode: func(b []byte) ([]byte, error) { return cm.NewDecoder().Bytes(b) },
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
	for _, e := range charmap.All {
		if cm, ok := e.(*charmap.Charmap); ok {
			if strings.ToUpper(cm.String()) == nameUpper {
				return charmapEncoding(cm.String(), cm), nil
			}
		}
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

	bom, rest := splitBOM(data)
	if bom != noBOM {
		text, err = bom.decode(rest)
		if err != nil {
			return nil, "", err
		}
		return text, bom.name(), nil
	}

	keyWordsBytes := make([][]byte, len(keyWords))
	for i, keyWord := range keyWords {
		keyWordsBytes[i] = []byte(keyWord)
	}

	type candidate struct {
		encoding string
		decoded  []byte
		score    int
	}
	var candidates []candidate

	for _, enc := range encodings {
		c := candidate{encoding: enc.name}
		c.decoded, err = enc.decode(data)
		if err != nil {
			continue
		}
		for _, keyWord := range keyWordsBytes {
			c.score += bytes.Count(c.decoded, keyWord)
		}
		if c.score > 0 {
			candidates = append(candidates, c)
		}
	}

	if len(candidates) == 0 {
		return data, "", nil
	}

	slices.SortStableFunc(candidates, func(a, b candidate) int { return cmp.Compare(b.score, a.score) })

	return candidates[0].decoded, candidates[0].encoding, nil
}
