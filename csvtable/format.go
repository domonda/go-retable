// Package csvtable provides CSV parsing, writing, and format detection functionality
// with support for various encodings, separators, and RFC 4180 compliance.
//
// The package handles common CSV edge cases including:
//   - Multiple character encodings (UTF-8, UTF-16LE, ISO 8859-1, Windows 1252, Macintosh)
//   - Various field separators (comma, semicolon, tab)
//   - Different line endings (\n, \r\n, \n\r)
//   - Quoted fields with embedded newlines, delimiters, and quotes
//   - Automatic format detection from CSV data
//   - Multi-line field support
//   - Quote escaping per RFC 4180
package csvtable

import (
	"errors"
	"fmt"
	"strings"
)

// Format describes the encoding and structural format of a CSV file.
// It specifies how the CSV data should be interpreted during parsing
// or written during serialization.
//
// Format validation ensures compliance with common CSV standards:
//   - Encoding must be specified and supported by the charset package
//   - Separator must be exactly one character
//   - Newline must be one of: "\n", "\r\n", or "\n\r"
//
// Example:
//
//	format := &Format{
//	    Encoding:  "UTF-8",
//	    Separator: ",",
//	    Newline:   "\r\n",
//	}
type Format struct {
	// Encoding specifies the character encoding of the CSV data.
	// Common values: "UTF-8", "UTF-16LE", "ISO 8859-1", "Windows 1252", "Macintosh"
	Encoding string `json:"encoding"`

	// Separator is the field delimiter character (must be single character).
	// Common values: "," (comma), ";" (semicolon), "\t" (tab)
	Separator string `json:"separator"`

	// Newline specifies the line ending sequence.
	// Valid values: "\n" (LF), "\r\n" (CRLF), "\n\r" (LFCR)
	Newline string `json:"newline"`
}

// NewFormat creates a new Format with the specified separator,
// UTF-8 encoding, and Windows-style line endings (\r\n).
//
// This is a convenience constructor for creating standard CSV formats.
// The returned format uses RFC 4180 compliant CRLF line endings.
//
// Parameters:
//   - separator: The field delimiter (commonly ",", ";", or "\t")
//
// Returns a Format with UTF-8 encoding and \r\n newlines.
//
// Example:
//
//	format := NewFormat(";")
//	// Returns: &Format{Encoding: "UTF-8", Separator: ";", Newline: "\r\n"}
func NewFormat(separator string) *Format {
	return &Format{
		Encoding:  "UTF-8",
		Separator: separator,
		Newline:   "\r\n",
	}
}

// Validate checks if the Format configuration is valid.
// It can be safely called on a nil receiver.
//
// Validation rules:
//   - Format must not be nil
//   - Encoding must be specified (non-empty)
//   - Separator must be specified and exactly one character long
//   - Newline must be one of: "\n", "\r\n", or "\n\r"
//
// Returns an error describing the validation failure, or nil if valid.
//
// Example:
//
//	format := &Format{Encoding: "UTF-8", Separator: ",", Newline: "\r\n"}
//	if err := format.Validate(); err != nil {
//	    log.Fatal(err)
//	}
func (f *Format) Validate() error {
	switch {
	case f == nil:
		return errors.New("<nil> csv.Format")
	case f.Encoding == "":
		return errors.New("missing csv.Format.Encoding")
	case f.Separator == "":
		return errors.New("missing csv.Format.Separator")
	case len(f.Separator) > 1:
		return fmt.Errorf("invalid csv.Format.Separator: %q", f.Separator)
	case f.Newline == "":
		return errors.New("missing csv.Format.Newline")
	case f.Newline != "\n" && f.Newline != "\n\r" && f.Newline != "\r\n":
		return fmt.Errorf("invalid csv.Format.Newline: %q", f.Newline)
	}
	return nil
}

// FormatDetectionConfig configures the automatic CSV format detection algorithm.
// It specifies which character encodings to test and which test strings to use
// for validating encoding detection accuracy.
//
// The detection algorithm uses these test strings to verify that the selected
// encoding correctly decodes the data. Test strings typically contain characters
// with special encodings like umlauts, accented characters, and currency symbols.
type FormatDetectionConfig struct {
	// Encodings is the list of character encodings to test during detection,
	// in priority order. The first encoding that successfully decodes the data
	// and passes the EncodingTests will be selected.
	Encodings []string `json:"encodings"`

	// EncodingTests contains strings with special characters used to validate
	// encoding detection. These should include characters that have different
	// byte representations across encodings (e.g., ä, ö, ü, €, Cyrillic chars).
	EncodingTests []string `json:"encodingTests"`
}

// NewDefaultFormatDetectionConfig returns a FormatDetectionConfig with
// sensible defaults for European and Cyrillic CSV files.
//
// Default encodings (in priority order):
//   - UTF-8 (universal)
//   - UTF-16LE (Windows Unicode)
//   - ISO 8859-1 (Latin-1)
//   - Windows 1252 (Western European, like ANSI)
//   - Macintosh (legacy Mac encoding)
//
// Default test characters include:
//   - German umlauts: ä, ö, ü, ß
//   - Common symbols: §, €
//   - Cyrillic characters: д, ъ, б, л, и, ж
//
// These test strings help distinguish between encodings that might otherwise
// appear identical for ASCII-only content.
//
// Example:
//
//	config := NewDefaultFormatDetectionConfig()
//	rows, format, err := ParseDetectFormat(csvData, config)
func NewDefaultFormatDetectionConfig() *FormatDetectionConfig {
	return &FormatDetectionConfig{
		Encodings: []string{
			"UTF-8",
			"UTF-16LE",
			"ISO 8859-1",
			"Windows 1252", // like ANSI
			"Macintosh",
		},
		EncodingTests: []string{
			"ä",
			"Ä",
			"ö",
			"Ö",
			"ü",
			"Ü",
			"ß",
			"§",
			"€",
			"д",
			"Д",
			"ъ",
			"Ъ",
			"б",
			"Б",
			"л",
			"Л",
			"и",
			"И",
			"ж",
			// "ährung",
			// "mpfänger",
			// "rsprünglich",
			// "ückerstatt",
			// "übertrag",
			// "für",
			// "Jänner",
			// "März",
			// "cc§google.com",
		},
	}
}

// EscapeQuotes escapes double quotes in a CSV field value according to RFC 4180.
// Each double quote character (") is replaced with two double quotes ("").
//
// This is the standard CSV escaping mechanism: when a field contains double quotes,
// the entire field must be quoted and internal quotes must be doubled.
//
// Parameters:
//   - val: The string value to escape
//
// Returns the escaped string with doubled quotes.
//
// Example:
//
//	escaped := EscapeQuotes(`Say "Hello"`)
//	// Returns: `Say ""Hello""`
//	//
//	// When written to CSV with quotes around field:
//	// "Say ""Hello"""
func EscapeQuotes(val string) string {
	return strings.ReplaceAll(val, `"`, `""`)
}
