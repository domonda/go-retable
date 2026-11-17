package csvtable

import (
	"bytes"
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
)

// ParseDetectFormat parses CSV data with automatic format detection.
// It analyzes the raw bytes to determine encoding, separator, and line endings,
// then parses the data into rows of string fields.
//
// Format Detection Algorithm:
//  1. Encoding Detection: A byte order mark decides on its own, otherwise the
//     configured encodings are tested against test strings to find the encoding
//     that correctly decodes special characters
//  2. Line Ending Detection: Counts \r\n, \n\r and bare \n outside of quoted
//     fields and takes the most frequent one
//  3. Separator Detection: Scores comma, semicolon, tab and pipe by how uniform
//     the resulting column count is instead of by how often they occur, counting
//     only outside of quoted fields
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
//
// ParseDetectFormat never panics on malformed input: a panic while parsing
// is returned as an error with the prefix "csvtable: panic"
// followed by the stack trace of the panic.
func ParseDetectFormat(csv []byte, config *FormatDetectionConfig) (rows [][]string, format *Format, err error) {
	defer recoverParsePanic(&err)

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
//   - Other encodings: Data is decoded to UTF-8 by getCharsetEncoding
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
//
// ParseWithFormat never panics on malformed input: a panic while parsing
// is returned as an error with the prefix "csvtable: panic"
// followed by the stack trace of the panic.
func ParseWithFormat(csv []byte, format *Format) (rows [][]string, err error) {
	defer recoverParsePanic(&err)

	err = format.Validate()
	if err != nil {
		return nil, err
	}

	if format.Encoding == "UTF-8" {
		csv = trimUTF8BOM(csv)
	} else {
		enc, err := getCharsetEncoding(format.Encoding)
		if err != nil {
			return nil, err
		}
		csv, err = enc.decode(csv)
		if err != nil {
			return nil, err
		}
	}

	csv = sanitizeUTF8(csv)

	lines := splitLines(csv, format.Newline)
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

// recoverParsePanic is the backstop that keeps the exported parse and read
// functions from panicking on malformed input, which would take down the
// whole process parsing an uploaded file instead of rejecting that file.
// It has to be deferred directly by the function whose named error result
// err points to, because recover only stops a panic there.
//
// A recovered panic is assigned to *err with the prefix "csvtable: panic",
// wrapping the panic value with %w if it is an error so that errors.Is and
// errors.As still find it, and formatting it with %v otherwise.
// The stack trace of the panic follows on the next lines of the message,
// as with github.com/domonda/go-errs.AsErrorWithDebugStack, because a panic
// that reaches this backstop is a bug and the stack is what locates it.
// Any error already assigned to *err is replaced, the panic is the more
// important one to report.
func recoverParsePanic(err *error) {
	p := recover()
	if p == nil {
		return
	}
	// Still the stack of the panicking goroutine,
	// deferred functions run before it is unwound
	stack := debug.Stack()
	if pErr, ok := p.(error); ok {
		*err = fmt.Errorf("csvtable: panic: %w\n%s", pErr, stack)
	} else {
		*err = fmt.Errorf("csvtable: panic: %v\n%s", p, stack)
	}
}

// detectFormatAndSplitLines implements the automatic format detection algorithm.
// It analyzes CSV data to determine encoding, line endings, and field separator,
// then splits the data into lines ready for parsing.
//
// Detection Process:
//
// 1. Encoding Detection:
//   - Tests each encoding from config.Encodings in order
//   - Uses autoDecode with config.EncodingTests to validate
//   - Falls back to UTF-8 if no encoding matches
//   - Sanitizes UTF-8 by replacing invalid characters
//
// 2. Line Ending Detection:
//   - Counts \r\n, \n\r and bare \n outside of quoted fields
//   - Uses the most frequent one, defaulting to \n
//
// 3. Separator Detection:
//   - Checks first line for "sep=X" or "SEP=X" header declaration
//   - If header found: uses declared separator and removes header line
//   - Otherwise: scores comma, semicolon, tab and pipe by how uniform the
//     column count is across rows, not by how often they occur, so a
//     character that is frequent but ragged loses to one that splits
//     every row the same way
//   - Defaults to comma when no candidate scores
//
// The function handles edge cases:
//   - Empty files: returns empty format and nil lines
//   - Files with only empty lines: returns empty rows
//   - Separators and newlines inside quoted fields: excluded from both
//     detections, so a quoted field cannot outvote the real structure
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

	var encodings []*charsetEncoding
	for _, name := range config.Encodings {
		enc, err := getCharsetEncoding(name)
		if err != nil {
			return nil, nil, err
		}
		encodings = append(encodings, enc)
	}

	csv, format.Encoding, err = autoDecode(csv, encodings, config.EncodingTests)
	if err != nil {
		return nil, nil, err
	}
	if format.Encoding == "" {
		format.Encoding = "UTF-8"
	}

	csv = sanitizeUTF8(csv)

	///////////////////////////////////////////////////////////////////////////
	// Scan the structure outside of quoted fields for the detections below

	structure := scanStructure(csv, true)
	if structure.endedQuoted {
		// The data ends within a quoted field, so its quoting is unbalanced
		// and everything after the offending quote was skipped. Scan again
		// without quoting instead of guessing which quote is the wrong one.
		structure = scanStructure(csv, false)
	}

	///////////////////////////////////////////////////////////////////////////
	// Detect line endings

	// Newlines within a quoted field are part of its value and were not
	// counted, so a single quoted \r\n can't switch a whole \n separated
	// file to \r\n line endings. A wider line ending wins a tie because a
	// file using one has no bare \n of its own to count, and \r\n wins over
	// \n\r because it is the standard.
	numBareLF := structure.numLF - structure.numCRLF - structure.numLFCR
	switch {
	case structure.numCRLF > 0 && structure.numCRLF >= structure.numLFCR && structure.numCRLF >= numBareLF:
		format.Newline = "\r\n"
	case structure.numLFCR > 0 && structure.numLFCR >= numBareLF:
		format.Newline = "\n\r"
	default:
		format.Newline = "\n"
	}

	///////////////////////////////////////////////////////////////////////////
	// Detect separator

	lines = splitLines(csv, format.Newline)

	if len(lines) > 0 {
		format.Separator = parseSepHeaderLine(lines[0])
		if format.Separator != "" {
			return format, lines[1:], nil
		}
	}

	// Default separator, also used when there is no line to detect one from,
	// because the returned Format is used by callers for parsing and writing
	// further data and has to be valid in any case.
	format.Separator = ","

	numNonEmptyLines := 0
	for _, line := range lines {
		if len(line) > 0 {
			numNonEmptyLines++
		}
	}
	if numNonEmptyLines == 0 {
		return format, nil, nil
	}

	if separator, ok := structure.bestSeparator(); ok {
		format.Separator = separator
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

// splitLines splits csv into lines separated by newline and removes
// stray newline characters from the end of every line.
//
// Trimming is part of splitting because a file can use a line ending wider
// than the newline it is split by, which would otherwise leak into the last
// field of every line.
//
// Only the end of a line is trimmed. A \r at the start of a line is not
// residue of a \n\r line ending, because those are detected and split by,
// but a carriage return within a quoted field that has to be preserved.
//
// Trimming the end still loses a \r that directly precedes the newline the
// lines are split by, like the one in A;"x\r\ny";B within a file with \n
// line endings. There the \r can't be told apart from the residue of a file
// with mixed line endings, which is what is trimmed here. Telling the two
// apart needs the quoted state while splitting, which this parser only has
// after the lines are split.
func splitLines(csv []byte, newline string) [][]byte {
	lines := bytes.Split(csv, []byte(newline))
	for i := range lines {
		lines[i] = bytes.TrimRight(lines[i], "\r\n")
	}
	return lines
}

// A candidate that bestSeparator only scores by leaving out the records it
// does not separate has to separate at least minSeparatedRecords records
// with its most common column count, and at least one in minSeparatedShare
// of all records.
const (
	minSeparatedRecords = 2
	minSeparatedShare   = 5
)

// separatorCandidates are the separators that can be detected from the data,
// ordered by preference so that the earlier one wins a tie in bestSeparator.
var separatorCandidates = []byte{',', ';', '\t', '|'}

// csvStructure is what scanStructure counted outside of quoted fields.
type csvStructure struct {
	numLF      int // Newlines, of which numCRLF and numLFCR are part of a wider line ending
	numCRLF    int
	numLFCR    int
	numRecords int // Records with any content, a record can span several lines

	// columnsPerRecord maps for every separatorCandidates index
	// the number of columns to the number of records with that many columns
	columnsPerRecord []map[int]int

	// endedQuoted reports that the data ends within a quoted field, meaning
	// that its quoting is unbalanced and everything after the offending quote
	// was skipped
	endedQuoted bool
}

// scanStructure counts the structural bytes of the CSV data that are not part
// of a quoted field value: the line endings, and for every separator candidate
// how many records have how many columns.
//
// A quote toggles the quoted state, except for a doubled quote within a quoted
// field which is an escaped quote that does not end it. Toggling on quotes
// alone keeps the scan independent of the separator, which is not known yet
// while it is detected. Any newline outside of a quoted field ends a record,
// so the record boundaries don't depend on the detected line ending either,
// and a newline within a quoted field does not split a record in two.
//
// With skipQuoted false the quoting is ignored, which is the fallback for data
// whose quoting is unbalanced.
func scanStructure(csv []byte, skipQuoted bool) *csvStructure {
	s := &csvStructure{columnsPerRecord: make([]map[int]int, len(separatorCandidates))}
	for i := range s.columnsPerRecord {
		s.columnsPerRecord[i] = make(map[int]int)
	}

	separators := make([]int, len(separatorCandidates))
	recordHasContent := false
	endRecord := func() {
		if recordHasContent {
			s.numRecords++
			for i, numSeparators := range separators {
				s.columnsPerRecord[i][numSeparators+1]++
			}
		}
		clear(separators)
		recordHasContent = false
	}

	quoted := false
	for i := 0; i < len(csv); i++ {
		c := csv[i]
		if skipQuoted && c == '"' {
			if quoted && i+1 < len(csv) && csv[i+1] == '"' {
				i++ // Escaped quote within a quoted field
				continue
			}
			quoted = !quoted
			recordHasContent = true
			continue
		}
		if quoted {
			continue
		}
		switch c {
		case '\r':
			if i+1 < len(csv) && csv[i+1] == '\n' {
				i++
				s.numLF++
				s.numCRLF++
			}
			endRecord()
		case '\n':
			s.numLF++
			if i+1 < len(csv) && csv[i+1] == '\r' {
				i++
				s.numLFCR++
			}
			endRecord()
		default:
			recordHasContent = true
			for candidate, separator := range separatorCandidates {
				if c == separator {
					separators[candidate]++
				}
			}
		}
	}
	endRecord()

	s.endedQuoted = quoted
	return s
}

// bestSeparator returns the candidate that splits the records into the most
// uniform number of columns, because the right separator is the one that makes
// the data rectangular. Counting occurrences alone is not enough: unquoted text
// containing commas can hold more of them than a semicolon separated file has
// semicolons. More columns win a tie, then the candidate order.
//
// ok is false when no candidate separates the records into more than one
// column, so the caller keeps its default separator.
func (s *csvStructure) bestSeparator() (separator string, ok bool) {
	if s.numRecords == 0 {
		return "", false
	}
	var (
		bestUniformity float64
		bestColumns    int
	)
	for candidate, sep := range separatorCandidates {
		// The most common number of columns of this candidate,
		// more columns win a tie.
		//
		// Records that the candidate does not separate at all are left out of
		// that vote unless it separates no record, wherever they are in the
		// data, because the header and trailer lines of a table are single
		// column records that must not outvote the actual table rows even
		// when there are more of them. A bank statement with more preamble
		// and trailer lines than data rows is the common case, and counting
		// those made the most common column count one, which discarded the
		// real separator below.
		var columns, records int
		for c, r := range s.columnsPerRecord[candidate] {
			if c < 2 && len(s.columnsPerRecord[candidate]) > 1 {
				continue
			}
			if r > records || (r == records && c > columns) {
				columns, records = c, r
			}
		}
		if columns < 2 {
			// The candidate doesn't separate anything in the typical record
			continue
		}
		if s.columnsPerRecord[candidate][1] > 0 &&
			(records < minSeparatedRecords || records*minSeparatedShare < s.numRecords) {
			// Leaving out the records the candidate does not separate lets a
			// few lines decide, so it only counts when those lines can be a
			// table. A single value containing a semicolon, tab or pipe in an
			// otherwise single column file would else become the separator
			// and lose the rest of that value.
			continue
		}
		uniformity := float64(records) / float64(s.numRecords)
		if uniformity > bestUniformity || (uniformity == bestUniformity && columns > bestColumns) {
			bestUniformity, bestColumns = uniformity, columns
			separator, ok = string(sep), true
		}
	}
	return separator, ok
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
	if !validSeparator(line[4]) {
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
	// noClosingFieldInLaterLines caches that no line after the last searched
	// one holds a field closing an unterminated quoted field. The searches
	// begin at ever later lines and lines are only emptied while parsing, so
	// a search that reached the end without a match can never find one in a
	// later line again. Without the cache every field of a file whose lines
	// all begin an unterminated quoted field re-scans the whole rest.
	noClosingFieldInLaterLines := false

	rows = make([][]string, len(lines))
	for lineIndex, line := range lines {
		if len(line) == 0 {
			continue
		}

		fields := bytes.Split(line, separator)
		// Line that the current field begins on. Joining a field across lines
		// continues this line with the fields of the closing line, so every
		// following field of the same row begins on that line and not on
		// lineIndex anymore.
		curLine := lineIndex
		// noClosingFieldInRow caches the same for the fields of the current
		// row that follow the searched one. A later field searches a subset
		// of the fields already searched, and the fields are only rebuilt by
		// a search that found a closing field, so once a search failed every
		// later search of the same row fails too. Without the cache a single
		// line whose fields all open an unterminated quoted field re-scans
		// the rest of the line for every one of them, which is quadratic in
		// the number of fields. A file with bare carriage return line
		// endings is one such line, because those are not detected as a
		// newline and the whole file stays a single line.
		noClosingFieldInRow := false
		// The row is built forward instead of the joined fields being cut out
		// of the split line, because compacting in place moves the whole tail
		// of the row per join, which is quadratic in the number of fields.
		row := make([]string, 0, len(fields))
		for i := 0; i < len(fields); i++ {
			field := fields[i]

			// Only a field beginning with a quote needs quote handling.
			// An empty field and every other field's quotes are literal and
			// are just unescaped below, so counting them would be wasted work.
			if leftQuotes := countQuotesLeft(field); len(field) > 0 && leftQuotes > 0 {
				totalQuotes := bytes.Count(field, []byte{'"'})
				switch {
				case totalQuotes == len(field) && len(field)%2 == 0:
					// Field consists only of an even number of quotes, which is an escaped
					// empty field `""`, an escaped quote `""""`, and so on.
					// An odd number of quotes leaves one quote unescaped that opens a field
					// continued after a separator or newline, which is handled by the case below.
					// Remove outermost quotes
					field = field[1 : len(field)-1]

				case leftQuotes%2 == 1 && totalQuotes%2 == 1:
					// An odd number of leading quotes opens a quoted field
					// and an odd total number of quotes means that the field
					// is not closed again within itself, so it was wrongly split
					// by a separator or a newline inside the quoted field
					// and has to be joined together again.
					//
					// Search for the field that closes the quoted field, first in
					// the remaining fields of this line which were split off by a
					// separator within the quotes, then in the fields of the
					// following lines which were split off by a newline within the
					// quotes. Newlines are allowed in quoted CSV fields.
					// A field can be split by both, so the search must neither stop
					// at the end of this line nor at the first field of a line.
					var (
						closeLine   = -1
						closeField  = -1
						closeFields [][]byte
					)
				findClosingField:
					for l := curLine; l < len(lines); l++ {
						lineFields, r := fields, i+1
						if l > curLine {
							if noClosingFieldInLaterLines {
								break
							}
							lineFields, r = bytes.Split(lines[l], separator), 0
						} else if noClosingFieldInRow {
							continue
						}
						for ; r < len(lineFields); r++ {
							if closesQuotedField(lineFields[r]) {
								closeLine, closeField, closeFields = l, r, lineFields
								break findClosingField
							}
						}
					}
					if closeLine == -1 {
						// The fields following this one and every later line
						// were searched without a match, so neither can hold
						// a closing field for a later field of this row.
						noClosingFieldInRow = true
						noClosingFieldInLaterLines = true
					}

					switch {
					case closeLine == curLine:
						// Only fields of this line were split off by a separator,
						// so join the fields [i..closeField] back together
						field = bytes.Join(fields[i:closeField+1], separator)
						// Remove quotes
						field = field[1 : len(field)-1]
						// Continue after the fields joined into this one. They are
						// skipped rather than cut out of the slice, so joining costs
						// nothing beyond the join itself.
						i = closeField

					case closeLine > curLine:
						// The field was also split off by a newline, so join the
						// remaining fields of this line, the lines in between and
						// the fields of the closing line up to closeField
						joined := bytes.Join(fields[i:], separator)
						for l := curLine + 1; l < closeLine; l++ {
							joined = append(joined, newlineReplacement...)
							joined = append(joined, lines[l]...)
						}
						joined = append(joined, newlineReplacement...)
						joined = append(joined, bytes.Join(closeFields[:closeField+1], separator)...)

						// Remove quotes of joined field
						if joined[0] != '"' || joined[len(joined)-1] != '"' {
							return nil, errors.New("should never happen: csv.Read is broken")
						}
						field = joined[1 : len(joined)-1]

						// Continue this row with the fields of the closing
						// line that follow the closing field
						fields, i = closeFields, closeField

						// Empty lines that have been joined
						// so line indices are still correct
						for l := curLine + 1; l <= closeLine; l++ {
							lines[l] = nil
						}

						// The fields following the closing field begin
						// on the closing line, not on lineIndex
						curLine = closeLine

					case totalQuotes == len(field):
						// Nothing to join the unterminated field with,
						// so only its opening quote is removed and the
						// remaining quotes are unescaped further down.
						field = field[1:]
					}

				case leftQuotes%2 == 1 && field[len(field)-1] == '"':
					// Quoted field that is closed again within itself.
					// Remove outermost quotes
					field = field[1 : len(field)-1]

				default:
					// Field is not quoted, or its closing quote is followed by
					// unquoted characters, so all its quotes are literal
					// and only have to be unescaped further down
				}
			}

			// bytes.ReplaceAll allocates a copy of the field even when
			// there is nothing to replace, so only call it when there is.
			if bytes.Contains(field, []byte(`""`)) {
				field = bytes.ReplaceAll(field, []byte(`""`), []byte{'"'})
			}
			row = append(row, string(field))
		}

		if cap(row) > 2*len(row) {
			// Joining split fields left most of the capacity unused,
			// which the returned rows would keep alive as long as the caller
			// holds them: one quoted value with a million separators would
			// keep a slot for every one of them.
			row = slices.Clone(row)
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

// closesQuotedField reports whether field is the closing part of a quoted
// field that was split by a separator or newline inside the quotes.
//
// The closing part may only begin with escaped quotes and must end with an
// unescaped closing quote. Requiring both is what distinguishes it from an
// ordinary quoted field like `"value"`, which must not be mistaken for the
// closing part of an unterminated field further up.
//
// Example:
//
//	closesQuotedField([]byte(`value"`))    // Returns: true
//	closesQuotedField([]byte(`""value"`))  // Returns: true
//	closesQuotedField([]byte(`"`))         // Returns: true
//	closesQuotedField([]byte(`"value"`))   // Returns: false (a complete field)
//	closesQuotedField([]byte(`value`))     // Returns: false (no closing quote)
func closesQuotedField(field []byte) bool {
	leftQuotes := countQuotesLeft(field)
	if leftQuotes == len(field) {
		// Field consists only of quotes, so it closes the
		// quoted field if one quote is left unescaped
		return leftQuotes%2 == 1
	}
	// A single leading quote can never occur inside a quoted field
	return leftQuotes%2 == 0 && countQuotesRight(field)%2 == 1
}

// sanitizeUTF8 replaces every byte that is not valid UTF-8, every U+FFFD
// replacement character, and every no-break space with a plain space,
// and returns the result as a newly allocated slice.
//
// Invalid bytes are replaced one by one, so a two byte sequence becomes two
// spaces. The result is always valid UTF-8, so the parser and everything
// downstream can treat the data as text without checking it again.
//
// A no-break space is replaced because it reads as a space but is not one to
// code that trims or compares field values, and spreadsheet exports are full
// of them. Note that this also changes field values that legitimately contain
// one.
//
// Sanitizing hides a failed encoding detection instead of reporting it: data
// decoded with the wrong encoding loses its undecodable bytes to spaces rather
// than raising an error, so `Müller` in Windows 1252 read as UTF-8 becomes
// `M ller`. Both callers sanitize directly after decoding, so a caller that
// has to know whether the encoding was right must check the decoded data
// itself.
//
// Example:
//
//	sanitizeUTF8([]byte("Jänner"))        // Returns: "Jänner"
//	sanitizeUTF8([]byte("a\u00a0b"))      // Returns: "a b"
//	sanitizeUTF8([]byte{'M', 0xfc, 'l'})  // Returns: "M l"
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
