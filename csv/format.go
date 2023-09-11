package csv

import (
	"errors"
	"fmt"
	"strings"
)

type Format struct {
	Encoding  string `json:"encoding"`
	Separator string `json:"separator"`
	Newline   string `json:"newline"`
}

// NewFormat returns a Format with the passed separator,
// UTF-8 encoding, and "\r\n" newlines.
func NewFormat(separator string) *Format {
	return &Format{
		Encoding:  "UTF-8",
		Separator: separator,
		Newline:   "\r\n",
	}
}

// Validate returns an error in case of an invalid Format.
// Can be called on nil receiver.
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

type FormatDetectionConfig struct {
	Encodings     []string `json:"encodings"`
	EncodingTests []string `json:"encodingTests"`
}

func NewFormatDetectionConfig() *FormatDetectionConfig {
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

func EscapeQuotes(val string) string {
	return strings.ReplaceAll(val, `"`, `""`)
}
