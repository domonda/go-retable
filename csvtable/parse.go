package csvtable

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/domonda/go-types/charset"
)

// ParseDetectFormat returns a slice of strings per row
// with the format detected via the optional FormatDetectionConfig.
// NewDefaultFormatDetectionConfig() will be used if config is nil.
func ParseDetectFormat(csv []byte, config *FormatDetectionConfig) (rows [][]string, format *Format, err error) {
	if config == nil {
		config = NewDefaultFormatDetectionConfig()
	}

	format, lines, err := detectFormatAndSplitLines(csv, config)
	if err != nil {
		return nil, format, err
	}

	rows, err = readLines(lines, []byte(format.Separator), "\n")
	return rows, format, err
}

func ParseWithFormat(csv []byte, format *Format) (rows [][]string, err error) {
	err = format.Validate()
	if err != nil {
		return nil, err
	}

	if format.Encoding == "UTF-8" {
		csv = charset.TrimBOM(csv, charset.BOMUTF8)
	} else {
		enc, err := charset.GetEncoding(format.Encoding)
		if err != nil {
			return nil, err
		}
		csv, err = enc.Decode(csv)
		if err != nil {
			return nil, err
		}
	}

	csv = sanitizeUTF8(csv)

	lines := bytes.Split(csv, []byte(format.Newline))
	if len(lines) > 0 {
		if headerSep := parseSepHeaderLine(lines[0]); headerSep != "" {
			if headerSep != format.Separator {
				return nil, fmt.Errorf("separator '%s' in header line is different from format.Separator '%s'", headerSep, format.Separator)
			}
			lines = lines[1:]
		}
	}

	return readLines(lines, []byte(format.Separator), "\n")
}

func detectFormatAndSplitLines(csv []byte, config *FormatDetectionConfig) (format *Format, lines [][]byte, err error) {
	if config == nil {
		return nil, nil, errors.New("FormatDetectionConfig must not be nil")
	}

	format = new(Format)

	///////////////////////////////////////////////////////////////////////////
	// Detect charset encoding

	var encodings []charset.Encoding
	for _, name := range config.Encodings {
		enc, err := charset.GetEncoding(name)
		if err != nil {
			return nil, nil, err
		}
		encodings = append(encodings, enc)
	}

	csv, format.Encoding, err = charset.AutoDecode(csv, encodings, config.EncodingTests)
	if err != nil {
		return nil, nil, err
	}
	if format.Encoding == "" {
		format.Encoding = "UTF-8"
	}

	csv = sanitizeUTF8(csv)

	///////////////////////////////////////////////////////////////////////////
	// Detect line endings

	// var (
	// 	numLinesR  = bytes.Count(data, []byte{'\r'})
	// 	numLinesN  = bytes.Count(data, []byte{'\n'})
	// 	numLinesRN = bytes.Count(data, []byte{'\r', '\n'})
	// )
	// // fmt.Println("n:", numLinesN, "rn:", numLinesRN, "r:", numLinesR)
	// switch {
	// case numLinesR > numLinesN:
	// 	format.Newline = "\r"
	// case numLinesN > numLinesRN:
	// 	format.Newline = "\n"
	// default:
	// 	format.Newline = "\r\n"
	// }

	// Simple rule: if there are \r\n line endings
	// then take those because that's the standard
	if bytes.Contains(csv, []byte{'\r', '\n'}) {
		format.Newline = "\r\n"
	} else {
		format.Newline = "\n"
	}

	///////////////////////////////////////////////////////////////////////////
	// Detect separator

	lines = bytes.Split(csv, []byte(format.Newline))

	if len(lines) > 0 {
		format.Separator = parseSepHeaderLine(lines[0])
		if format.Separator != "" {
			return format, lines[1:], nil
		}
	}

	type sepCounts struct {
		commas     int
		semicolons int
		tabs       int
	}

	var (
		sep sepCounts
		// lineSepCounts  []sepCounts
		// numSeperators    int
		numNonEmptyLines int
		// unusedSeparators string
	)

	for i := range lines {
		// Remove double newlines
		lines[i] = bytes.Trim(lines[i], "\r\n")
		line := lines[i]

		if len(line) == 0 {
			continue
		}

		numNonEmptyLines++

		commas := bytes.Count(line, []byte{','})
		semicolons := bytes.Count(line, []byte{';'})
		tabs := bytes.Count(line, []byte{'\t'})

		sep.commas += commas
		sep.semicolons += semicolons
		sep.tabs += tabs
		// lineSepCounts = append(lineSepCounts, sepCounts{
		// 	commas:     commas,
		// 	semicolons: semicolons,
		// 	tabs:       tabs,
		// })
	}

	if numNonEmptyLines == 0 {
		return format, nil, nil
	}

	switch {
	case sep.commas > sep.semicolons && sep.commas > sep.tabs:
		// numSeperators = sep.commas
		// unusedSeparators = ";\t"
		format.Separator = ","

	case sep.semicolons > sep.commas && sep.semicolons > sep.tabs:
		// numSeperators = sep.semicolons
		// unusedSeparators = ",\t"
		format.Separator = ";"

	case sep.tabs > sep.commas && sep.tabs > sep.semicolons:
		// numSeperators = sep.tabs
		// unusedSeparators = ",;"
		format.Separator = "\t"

	default:
		// numSeperators = sep.commas
		// unusedSeparators = ";\t"
		format.Separator = ","
	}

	///////////////////////////////////////////////////////////////////////////
	// Detect line embedded as single field

	// var (
	// 	escapedQuotedSeparators    = []byte{'"', '"', format.Separator[0], '"', '"'}
	// 	numEscapedQuotedSeparators = 0
	// 	lineAsField                = true
	// )
	// for i, line := range lines {
	// 	if len(line) == 0 {
	// 		continue
	// 	}
	// 	line = bytes.Trim(line, unusedSeparators)
	// 	left, right := countQuotesLeftRight(line)
	// 	if left == 1 && right == 1 {
	// 		line = line[1 : len(line)-1]
	// 		num := bytes.Count(line, escapedQuotedSeparators)
	// 		if num == 0 {
	// 			lineAsField = false
	// 			break
	// 		}
	// 		if i == 0 {
	// 			numEscapedQuotedSeparators = num
	// 		} else {
	// 			if num != numEscapedQuotedSeparators {
	// 				lineAsField = false
	// 				break
	// 			}
	// 		}
	// 	} else {
	// 		lineAsField = false
	// 		break
	// 	}
	// }
	// lineAsField = false // TODO remove and test
	// if lineAsField {
	// 	for i, line := range lines {
	// 		if len(line) == 0 {
	// 			continue
	// 		}
	// 		line = bytes.Trim(line, unusedSeparators)
	// 		line = line[1 : len(line)-1]
	// 		line = bytes.ReplaceAll(line, []byte{'"', '"'}, []byte{'"'})
	// 		lines[i] = line
	// 	}
	// }

	return format, lines, nil
}

// parseSepHeaderLine parses "sep=," or "SEP=," like header lines
// and returns the separator
func parseSepHeaderLine(line []byte) (sep string) {
	if len(line) < 5 {
		return ""
	}
	if line[0] == '"' && line[len(line)-1] == '"' {
		line = line[1 : len(line)-1]
	}
	if len(line) != 5 {
		return ""
	}
	if !bytes.HasPrefix(line, []byte("sep=")) && !bytes.HasPrefix(line, []byte("SEP=")) {
		return ""
	}
	return string(line[4:5])
}

func readLines(lines [][]byte, separator []byte, newlineReplacement string) (rows [][]string, err error) {
	rows = make([][]string, len(lines))
	for lineIndex, line := range lines {
		if len(line) == 0 {
			continue
		}

		fields := bytes.Split(line, separator)
		for i := 0; i < len(fields); i++ {
			field := fields[i]
			if len(field) < 2 {
				continue
			}

			leftQuotes, rightQuotes := countQuotesLeftRight(field)
			switch {
			case leftQuotes == 0 && rightQuotes == 0:
				// Unquoted field

			case leftQuotes == 1 && rightQuotes == 1, // Quoted field
				leftQuotes == 3 && rightQuotes == 1, // Quoted field beginning with escapted quote
				leftQuotes == 1 && rightQuotes == 3, // Quoted field ending with escapted quote
				leftQuotes == 3 && rightQuotes == 3, // Quoted field with escaped quotes inside
				leftQuotes == 2 && rightQuotes == 2: // Field not quoted, but escaped quotes inside

				// Remove outermost quotes
				field = field[1 : len(field)-1]

			case leftQuotes == 0 && rightQuotes >= 1:
				// Field begins without a quote but ends with at least one.
				// This is field internal quoting, no special handling needed

			case leftQuotes >= 1 && rightQuotes == 0:
				// Field begins with quote but does not end with one

				if leftQuotes == 2 {
					// Begins with two quotes wich is an escaped quote,
					// but not with a tripple quote.
					// No special handling needed, will be unescaped futher down

				} else {

					joinLineIndex := -1
					if i == len(fields)-1 {
						// When last field of the line begins with a quote but does not end with one
						// then search following lines for a first field that ends with a quote
						// which will be the right side of this field wrongly splitted into more
						// lines because it contained newline characters.
						// Newlines are allowed in quoted CSV fields.
						for joinLineIndex = lineIndex + 1; joinLineIndex < len(lines); joinLineIndex++ {
							joinLine := lines[joinLineIndex]
							joinLineFields := bytes.Split(joinLine, separator)
							if len(joinLineFields) > 0 && bytes.HasSuffix(joinLineFields[0], []byte{'"'}) {
								// Found the line where the first field holds the closing quote for the multi-line field
								break
							}
						}
					}

					if joinLineIndex > lineIndex && joinLineIndex < len(lines) {
						// Join lines until including joinLineIndex as multi line field
						// then empty those lines so line indices are still correct

						joinLine := lines[joinLineIndex]
						joinLineFields := bytes.Split(joinLine, separator)

						// Join lines between lineIndex and joinLineIndex
						for index := lineIndex + 1; index < joinLineIndex; index++ {
							field = append(field, []byte(newlineReplacement)...)
							field = append(field, lines[index]...)
						}

						// Join first field of line joinLineIndex
						field = append(field, []byte(newlineReplacement)...)
						field = append(field, joinLineFields[0]...)

						// Remove quotes of joined field
						if field[0] != '"' || field[len(field)-1] != '"' {
							return nil, errors.New("should never happen: csv.Read is broken")
						}
						field = field[1 : len(field)-1]

						// Append following fields after first joined field of line joinLineIndex
						fields = append(fields, joinLineFields[1:]...)

						// Empty lines that have been joined
						for i := lineIndex + 1; i <= joinLineIndex; i++ {
							lines[i] = nil
						}

					} else {

						// Begins with quote but does not end with one
						// means that a separator was in a quoted field
						// that has been wrongly splitted into multiple fields.
						// Needs merging of fields:
						for r := i + 1; r < len(fields); r++ {
							// Find following field that does not begin
							// with a quote, but ends with exactly one
							rField := fields[r]
							if len(rField) < 2 {
								continue
							}
							rLeftQuotes, rRightQuotes := countQuotesLeftRight(rField)
							var (
								rLeftOK  = rLeftQuotes == 0 || rLeftQuotes == 2 // right field may only begin with an escaped quote
								rRightOK = (leftQuotes == 1 && rRightQuotes == 1) || (leftQuotes == 1 && rRightQuotes == 3) || (leftQuotes == 3 && rRightQuotes == 1) || (leftQuotes == 3 && rRightQuotes == 3)
							)
							if rLeftOK && rRightOK {
								// Join fields [i..j]
								field = bytes.Join(fields[i:r+1], separator)
								// Remove quotes
								field = field[1 : len(field)-1]
								// Shift remaining slice fields over the ones joined into fields[i]
								copy(fields[i+1:], fields[r+1:])
								fields = fields[:len(fields)-(r-i)]
								break
							}
						}
					}
				}

			default:
				return nil, fmt.Errorf("can't handle CSV field `%s` in line `%s`", field, line)
				// Examples for this error:
				// /var/domonda-data/documents/39/d20/301/65394733/b7e967e7f98ec1e8/2019-01-03_09-46-50.435/doc.csv
				// Double embedded fields:
				// /var/domonda-data/documents/c9/727/af8/9cdf4afd/981ad4331d0fb6ca/2019-11-04_08-18-13.602/doc.csv
			}

			fields[i] = bytes.ReplaceAll(field, []byte(`""`), []byte{'"'})
		}

		row := make([]string, len(fields))
		for i := range fields {
			row[i] = string(fields[i])
		}
		rows[lineIndex] = row
	}

	return rows, nil
}

func countQuotesLeft(str []byte) int {
	for i, c := range str {
		if c != '"' {
			return i
		}
	}
	return len(str)
}

func countQuotesRight(str []byte) int {
	for i := len(str) - 1; i >= 0; i-- {
		if str[i] != '"' {
			return len(str) - 1 - i
		}
	}
	return len(str)
}

func countQuotesLeftRight(str []byte) (left, right int) {
	left = countQuotesLeft(str)
	right = countQuotesRight(str)

	if left == len(str) {
		left = (len(str) + 1) / 2
		right = len(str) - left
	}

	return left, right
}

func sanitizeUTF8(str []byte) []byte {
	return bytes.Map(
		func(r rune) rune {
			switch r {
			// \u00a0 is No-Break Space (NBSP)
			case '�', '\u00a0':
				return ' '
			default:
				return r
			}
		},
		str,
	)
}
