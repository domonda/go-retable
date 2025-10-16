package csvtable

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/domonda/go-types/charset"
)

// ParseDetectFormat parses CSV data with automatic format detection.
// It analyzes the raw bytes to determine encoding, separator, and line endings,
// then parses the data into rows of string fields.
//
// Format Detection Algorithm:
//  1. Encoding Detection: Tests configured encodings against test strings to find
//     the encoding that correctly decodes special characters
//  2. Line Ending Detection: Prefers \r\n if present, otherwise uses \n
//  3. Separator Detection: Counts occurrences of common separators (comma, semicolon, tab)
//     and selects the most frequent one
//  4. Header Line Detection: Checks for "sep=X" header line that explicitly declares separator
//
// The function handles complex CSV formats including:
//   - Multi-line fields (fields containing newlines within quotes)
//   - Quoted fields with embedded separators
//   - Escaped quotes (doubled quotes per RFC 4180)
//   - Mixed quote patterns
//
// Parameters:
//   - csv: Raw CSV data as bytes
//   - config: Format detection configuration. If nil, NewDefaultFormatDetectionConfig() is used
//
// Returns:
//   - rows: Parsed CSV data as slice of string slices (rows and columns)
//   - format: Detected format containing encoding, separator, and newline
//   - err: Any error encountered during detection or parsing
//
// Example:
//
//	csvData := []byte("Name;Age\r\nJohn;30\r\nJane;25")
//	rows, format, err := ParseDetectFormat(csvData, nil)
//	// format.Separator == ";"
//	// format.Encoding == "UTF-8"
//	// format.Newline == "\r\n"
//	// rows == [][]string{{"Name", "Age"}, {"John", "30"}, {"Jane", "25"}}
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

// ParseWithFormat parses CSV data using an explicitly specified format.
// Unlike ParseDetectFormat, this function requires the format to be known in advance.
//
// The function performs the following steps:
//  1. Validates the format configuration
//  2. Decodes the data from the specified encoding to UTF-8
//  3. Removes BOM (Byte Order Mark) if present
//  4. Sanitizes UTF-8 by replacing invalid characters
//  5. Checks for and validates "sep=X" header line
//  6. Splits data into lines and parses each line into fields
//
// Encoding handling:
//   - UTF-8: BOM is trimmed if present, data is used as-is
//   - Other encodings: Data is decoded to UTF-8 using the charset package
//
// Header line detection:
//   - If first line matches pattern "sep=X" or "SEP=X" (possibly quoted),
//     it is treated as a separator declaration and removed from output
//   - The declared separator must match format.Separator or an error is returned
//
// Parameters:
//   - csv: Raw CSV data as bytes in the encoding specified by format.Encoding
//   - format: Format specification (encoding, separator, newline). Must be non-nil and valid
//
// Returns:
//   - rows: Parsed CSV data as slice of string slices
//   - err: Validation errors, encoding errors, or parsing errors
//
// Example:
//
//	format := &Format{
//	    Encoding:  "UTF-8",
//	    Separator: ",",
//	    Newline:   "\r\n",
//	}
//	csvData := []byte("Name,Age\r\nJohn,30\r\nJane,25")
//	rows, err := ParseWithFormat(csvData, format)
//	// rows == [][]string{{"Name", "Age"}, {"John", "30"}, {"Jane", "25"}}
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

// detectFormatAndSplitLines implements the automatic format detection algorithm.
// It analyzes CSV data to determine encoding, line endings, and field separator,
// then splits the data into lines ready for parsing.
//
// Detection Process:
//
// 1. Encoding Detection:
//   - Tests each encoding from config.Encodings in order
//   - Uses charset.AutoDecode with config.EncodingTests to validate
//   - Falls back to UTF-8 if no encoding matches
//   - Sanitizes UTF-8 by replacing invalid characters
//
// 2. Line Ending Detection:
//   - Checks if data contains \r\n sequences
//   - If found, uses \r\n (CRLF - Windows/RFC 4180 standard)
//   - Otherwise uses \n (LF - Unix standard)
//
// 3. Separator Detection:
//   - Checks first line for "sep=X" or "SEP=X" header declaration
//   - If header found: uses declared separator and removes header line
//   - Otherwise: counts occurrences of comma, semicolon, and tab across all non-empty lines
//   - Selects the separator with highest total count
//   - Defaults to comma if counts are equal
//
// The function handles edge cases:
//   - Empty files: returns empty format and nil lines
//   - Files with only empty lines: returns empty rows
//   - Quoted separators within fields: counted but handled during parsing
//
// Parameters:
//   - csv: Raw CSV data as bytes
//   - config: Configuration specifying encodings and test strings. Must not be nil
//
// Returns:
//   - format: Detected format with encoding, separator, and newline
//   - lines: CSV data split into lines as byte slices, with header line removed if present
//   - err: Encoding errors or configuration errors
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

// parseSepHeaderLine parses separator declaration header lines.
// It recognizes lines in the format "sep=X" or "SEP=X" where X is the separator character.
//
// The header line may optionally be enclosed in double quotes: "sep=X"
//
// This format is used by Microsoft Excel and other tools to explicitly
// declare the field separator, avoiding ambiguity in format detection.
//
// Parameters:
//   - line: First line of CSV file as bytes
//
// Returns:
//   - sep: The declared separator character as string, or empty string if not a header line
//
// Examples:
//
//	parseSepHeaderLine([]byte("sep=,"))      // Returns: ","
//	parseSepHeaderLine([]byte("SEP=;"))      // Returns: ";"
//	parseSepHeaderLine([]byte(`"sep=\t"`))   // Returns: "\t"
//	parseSepHeaderLine([]byte("Name,Age"))   // Returns: "" (not a header)
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

// readLines parses CSV lines into rows of string fields.
// This is the core CSV parsing logic that handles complex quoting scenarios
// and multi-line fields according to RFC 4180.
//
// Quoting and Escaping Rules (RFC 4180):
//   - Fields containing separator, newline, or quotes must be quoted
//   - Quotes within quoted fields are escaped by doubling: "" represents "
//   - Quoted fields begin and end with exactly one quote (after removing outer quotes)
//   - Unquoted fields may not contain quotes (except when doubled)
//
// Multi-line Field Handling:
//   - If a field begins with a quote but doesn't end with one, and it's the last
//     field in the line, the parser searches subsequent lines for the closing quote
//   - All intermediate lines are joined with newlineReplacement
//   - The joined lines are marked as empty (nil) to maintain correct row indices
//
// Embedded Separator Handling:
//   - When a field begins with a quote but doesn't end with one (and it's not the
//     last field or no matching line is found), the parser assumes the separator
//     appeared within a quoted field
//   - It joins subsequent fields until finding one that ends with a quote
//
// Quote Pattern Recognition:
// The parser recognizes various quote patterns:
//   - Unquoted fields: no quotes at start or end
//   - Quoted fields: single quote at start and end
//   - Escaped quotes: field with doubled quotes (e.g., ""value"" or internal "")
//
// Parameters:
//   - lines: CSV data split into lines as byte slices
//   - separator: Field separator as bytes (typically comma, semicolon, or tab)
//   - newlineReplacement: String to replace newlines within multi-line fields (typically "\n")
//
// Returns:
//   - rows: Parsed data as slice of string slices. Empty lines become nil entries
//   - err: Parsing errors when encountering invalid quote patterns
//
// Example edge cases handled:
//
//	// Multi-line field
//	"Name","Address"
//	"John","123 Main St
//	Apt 4B"
//	// Becomes: [["John", "123 Main St\nApt 4B"]]
//
//	// Embedded separator
//	"Name","Description"
//	"Product","Size: small, medium, large"
//	// Becomes: [["Product", "Size: small, medium, large"]]
//
//	// Escaped quotes
//	"Name","Quote"
//	"John","He said ""Hello"""
//	// Becomes: [["John", `He said "Hello"`]]
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

// countQuotesLeft counts consecutive quote characters from the start of a byte slice.
// It returns the number of leading quotes, or the length if the entire slice is quotes.
//
// Used to determine quoting patterns in CSV fields.
//
// Example:
//
//	countQuotesLeft([]byte(`"value"`))   // Returns: 1
//	countQuotesLeft([]byte(`""value"`))  // Returns: 2
//	countQuotesLeft([]byte(`value`))     // Returns: 0
func countQuotesLeft(str []byte) int {
	for i, c := range str {
		if c != '"' {
			return i
		}
	}
	return len(str)
}

// countQuotesRight counts consecutive quote characters from the end of a byte slice.
// It returns the number of trailing quotes, or the length if the entire slice is quotes.
//
// Used to determine quoting patterns in CSV fields.
//
// Example:
//
//	countQuotesRight([]byte(`value"`))   // Returns: 1
//	countQuotesRight([]byte(`value""`))  // Returns: 2
//	countQuotesRight([]byte(`value`))    // Returns: 0
func countQuotesRight(str []byte) int {
	for i := len(str) - 1; i >= 0; i-- {
		if str[i] != '"' {
			return len(str) - 1 - i
		}
	}
	return len(str)
}

// countQuotesLeftRight counts consecutive quotes from both ends of a byte slice.
// It returns separate counts for leading and trailing quotes.
//
// Special case: If the entire slice consists of quotes, they are split between
// left and right counts (with left getting one more if odd number).
//
// This is used in the CSV parser to identify quoting patterns and determine
// whether a field is properly quoted, has escaped quotes, or needs special handling.
//
// Parameters:
//   - str: The byte slice to analyze
//
// Returns:
//   - left: Number of consecutive leading quotes
//   - right: Number of consecutive trailing quotes
//
// Example:
//
//	countQuotesLeftRight([]byte(`"value"`))    // Returns: 1, 1 (quoted field)
//	countQuotesLeftRight([]byte(`""value""`))  // Returns: 2, 2 (escaped quotes)
//	countQuotesLeftRight([]byte(`value`))      // Returns: 0, 0 (unquoted)
//	countQuotesLeftRight([]byte(`""""`))       // Returns: 2, 2 (all quotes, split evenly)
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
