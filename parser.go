package retable

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Parser is the interface for parsing string representations into primitive Go types.
// This is the counterpart to formatting - it handles the conversion from strings back
// to typed values.
//
// Parser provides a centralized place to configure parsing behavior, including:
//   - Which strings represent boolean true/false values
//   - Which strings represent nil/null values
//   - Which time formats to try when parsing time values
//   - Locale-specific number formatting (e.g., comma vs. period decimal separators)
//
// The Parser interface is used by Scanners to perform the actual string-to-value
// conversions. By abstracting parsing into an interface, different parsing strategies
// can be used (strict vs. lenient, different locale conventions, etc.).
//
// Example usage:
//
//	parser := NewStringParser()
//	// Configure custom boolean strings
//	parser.TrueStrings = []string{"true", "yes", "1", "on"}
//	parser.FalseStrings = []string{"false", "no", "0", "off"}
//
//	// Parse various types
//	i, _ := parser.ParseInt("42")          // 42
//	f, _ := parser.ParseFloat("3,14")      // 3.14 (handles comma decimal)
//	b, _ := parser.ParseBool("yes")        // true
//	t, _ := parser.ParseTime("2024-03-15") // time.Time value
type Parser interface {
	// ParseInt parses a string into a 64-bit signed integer.
	// Returns an error if the string is not a valid integer.
	ParseInt(string) (int64, error)

	// ParseUint parses a string into a 64-bit unsigned integer.
	// Returns an error if the string is not a valid unsigned integer.
	ParseUint(string) (uint64, error)

	// ParseFloat parses a string into a 64-bit floating point number.
	// May handle locale-specific formatting (e.g., comma vs. period decimals).
	// Returns an error if the string is not a valid float.
	ParseFloat(string) (float64, error)

	// ParseBool parses a string into a boolean value.
	// The strings recognized as true/false are implementation-specific.
	// Returns an error if the string is not recognized as a boolean.
	ParseBool(string) (bool, error)

	// ParseTime parses a string into a time.Time value.
	// May try multiple time formats in sequence.
	// Returns an error if the string doesn't match any recognized format.
	ParseTime(string) (time.Time, error)

	// ParseDuration parses a string into a time.Duration value.
	// Uses Go's standard duration format (e.g., "1h30m", "5s").
	// Returns an error if the string is not a valid duration.
	ParseDuration(string) (time.Duration, error)
}

// Ensure StringParser implements Parser
var _ Parser = new(StringParser)

// StringParser is a configurable implementation of the Parser interface that handles
// string-to-value conversions with support for multiple conventions and formats.
//
// Key features:
//   - Configurable boolean string representations (e.g., "yes"/"no", "1"/"0")
//   - Configurable nil/null string representations
//   - Multiple time format attempts for flexible time parsing
//   - Locale-aware float parsing (e.g., handling comma decimal separators)
//
// The StringParser uses standard Go parsing functions (strconv, time.Parse) internally,
// but adds flexibility through configuration and fallback strategies.
//
// Example usage:
//
//	parser := NewStringParser()
//	// Parser comes pre-configured with sensible defaults
//
//	// Customize boolean parsing
//	parser.TrueStrings = append(parser.TrueStrings, "enabled", "on")
//	parser.FalseStrings = append(parser.FalseStrings, "disabled", "off")
//
//	// Add custom time formats
//	parser.TimeFormats = append([]string{"01/02/2006"}, parser.TimeFormats...)
//
//	// Parse with custom configuration
//	b, _ := parser.ParseBool("enabled") // true
//	t, _ := parser.ParseTime("03/15/2024") // uses custom format
type StringParser struct {
	// TrueStrings lists all strings that should be parsed as boolean true.
	// Default includes: "true", "True", "TRUE", "yes", "Yes", "YES", "1"
	TrueStrings []string `json:"trueStrings"`

	// FalseStrings lists all strings that should be parsed as boolean false.
	// Default includes: "false", "False", "FALSE", "no", "No", "NO", "0"
	FalseStrings []string `json:"falseStrings"`

	// NilStrings lists all strings that represent nil/null values.
	// Default includes: "", "nil", "<nil>", "null", "NULL"
	// This is used by higher-level scanning logic to detect null values.
	NilStrings []string `json:"nilStrings"`

	// TimeFormats lists time layout strings to try when parsing time values.
	// Formats are tried in order until one succeeds.
	// Default includes RFC3339, ISO dates, and several common formats.
	TimeFormats []string `json:"timeFormats"`
}

// NewStringParser creates a new StringParser with sensible default configurations.
//
// Default configurations:
//   - TrueStrings: "true", "True", "TRUE", "yes", "Yes", "YES", "1"
//   - FalseStrings: "false", "False", "FALSE", "no", "No", "NO", "0"
//   - NilStrings: "", "nil", "<nil>", "null", "NULL"
//   - TimeFormats: Comprehensive list of common formats (RFC3339, ISO, etc.)
//
// The returned parser can be used as-is or customized by modifying its fields.
//
// Returns:
//   - A new StringParser with default configurations
//
// Example:
//
//	parser := NewStringParser()
//	// Use with defaults
//	value, _ := parser.ParseBool("yes") // true
//
//	// Or customize
//	parser.TrueStrings = append(parser.TrueStrings, "ja", "oui", "si")
func NewStringParser() *StringParser {
	c := &StringParser{
		TrueStrings:  []string{"true", "True", "TRUE", "yes", "Yes", "YES", "1"},
		FalseStrings: []string{"false", "False", "FALSE", "no", "No", "NO", "0"},
		NilStrings:   []string{"", "nil", "<nil>", "null", "NULL"},
		TimeFormats:  timeFormats,
	}
	return c
}

// ParseInt parses a string into a 64-bit signed integer using base 10.
// Uses strconv.ParseInt internally.
//
// Parameters:
//   - str: The string to parse
//
// Returns:
//   - int64: The parsed integer value
//   - error: Parsing error if the string is not a valid integer
//
// Example:
//
//	i, err := parser.ParseInt("42")    // 42, nil
//	i, err := parser.ParseInt("-123")  // -123, nil
//	i, err := parser.ParseInt("abc")   // 0, error
func (p *StringParser) ParseInt(str string) (int64, error) {
	return strconv.ParseInt(str, 10, 64)
}

// ParseUint parses a string into a 64-bit unsigned integer using base 10.
// Uses strconv.ParseUint internally.
//
// Parameters:
//   - str: The string to parse
//
// Returns:
//   - uint64: The parsed unsigned integer value
//   - error: Parsing error if the string is not a valid unsigned integer
//
// Example:
//
//	u, err := parser.ParseUint("42")    // 42, nil
//	u, err := parser.ParseUint("0")     // 0, nil
//	u, err := parser.ParseUint("-1")    // 0, error (negative not allowed)
func (p *StringParser) ParseUint(str string) (uint64, error) {
	return strconv.ParseUint(str, 10, 64)
}

// ParseFloat parses a string into a 64-bit floating point number with locale awareness.
// This method tries multiple strategies to handle different number formats:
//
//  1. Standard parsing using strconv.ParseFloat (handles "123.45")
//  2. If that fails, tries handling comma as decimal separator ("123,45" -> "123.45")
//  3. More strategies could be added for thousand separators, etc.
//
// This flexibility is important for parsing numbers from different locales or user input.
//
// Parameters:
//   - str: The string to parse
//
// Returns:
//   - float64: The parsed floating point value
//   - error: Parsing error if the string is not a valid number in any recognized format
//
// Example:
//
//	f, _ := parser.ParseFloat("3.14")    // 3.14 (standard)
//	f, _ := parser.ParseFloat("3,14")    // 3.14 (comma decimal)
//	f, _ := parser.ParseFloat("-2.5e10") // -2.5e10 (scientific notation)
func (p *StringParser) ParseFloat(str string) (float64, error) {
	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		numDot := strings.Count(str, ".")
		numComma := strings.Count(str, ",")
		switch {
		case numComma == 1 && numDot == 0:
			f, e := strconv.ParseFloat(strings.ReplaceAll(str, ",", "."), 64)
			if e != nil {
				return 0, err // return original error
			}
			return f, nil

			// TODO: add more cases
		}
		return 0, err
	}
	return f, nil
}

// ParseBool parses a string into a boolean value based on the configured
// TrueStrings and FalseStrings lists.
//
// The parser checks if the string exactly matches (case-sensitive) any string in
// TrueStrings or FalseStrings. If no match is found, an error is returned.
//
// Parameters:
//   - str: The string to parse
//
// Returns:
//   - bool: The parsed boolean value
//   - error: Error if string doesn't match any configured true/false string
//
// Example:
//
//	parser := NewStringParser()
//	b, _ := parser.ParseBool("true")  // true, nil
//	b, _ := parser.ParseBool("yes")   // true, nil
//	b, _ := parser.ParseBool("1")     // true, nil
//	b, _ := parser.ParseBool("false") // false, nil
//	b, _ := parser.ParseBool("no")    // false, nil
//	b, err := parser.ParseBool("maybe") // false, error
func (p *StringParser) ParseBool(str string) (bool, error) {
	for _, val := range p.TrueStrings {
		if str == val {
			return true, nil
		}
	}
	for _, val := range p.FalseStrings {
		if str == val {
			return false, nil
		}
	}
	return false, fmt.Errorf("cannot parse %q as bool", str)
}

// ParseTime parses a string into a time.Time value by trying multiple time formats.
//
// The parser tries each format in the TimeFormats slice in order until one succeeds.
// This allows parsing of various time string formats without needing to know the exact
// format in advance.
//
// The default TimeFormats include RFC3339, ISO 8601, common date formats, and several
// others. You can customize the TimeFormats slice to add or prioritize specific formats.
//
// Parameters:
//   - str: The string to parse
//
// Returns:
//   - time.Time: The parsed time value
//   - error: Error if string doesn't match any configured time format
//
// Example:
//
//	parser := NewStringParser()
//	t, _ := parser.ParseTime("2024-03-15T14:30:00Z")    // RFC3339
//	t, _ := parser.ParseTime("2024-03-15")              // ISO date
//	t, _ := parser.ParseTime("15.03.2024")              // German date format
//	t, _ := parser.ParseTime("2024-03-15 14:30:00")     // DateTime format
//	_, err := parser.ParseTime("not a date")            // error
func (p *StringParser) ParseTime(str string) (time.Time, error) {
	for _, format := range p.TimeFormats {
		t, err := time.Parse(format, str)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse %q as time", str)
}

// ParseDuration parses a string into a time.Duration value.
// Uses Go's standard time.ParseDuration which accepts strings like "1h30m", "5s", "100ms".
//
// Valid units: "ns" (nanoseconds), "us" (microseconds), "ms" (milliseconds),
// "s" (seconds), "m" (minutes), "h" (hours).
//
// Parameters:
//   - str: The string to parse (e.g., "1h30m", "5s")
//
// Returns:
//   - time.Duration: The parsed duration value
//   - error: Error if string is not a valid duration format
//
// Example:
//
//	d, _ := parser.ParseDuration("1h30m")    // 1 hour 30 minutes
//	d, _ := parser.ParseDuration("5s")       // 5 seconds
//	d, _ := parser.ParseDuration("100ms")    // 100 milliseconds
//	d, _ := parser.ParseDuration("2h45m30s") // 2 hours 45 minutes 30 seconds
//	_, err := parser.ParseDuration("invalid") // error
func (p *StringParser) ParseDuration(str string) (time.Duration, error) {
	return time.ParseDuration(str)
}

// ParseTime is a standalone function that parses a time string and returns both the
// parsed time and the format that successfully parsed it.
//
// This is useful when you need to know which format was used, for example to maintain
// consistent formatting when round-tripping time values, or for validation purposes.
//
// The function tries all formats in the package-level timeFormats list in order.
//
// Parameters:
//   - str: The string to parse
//
// Returns:
//   - t: The parsed time.Time value
//   - format: The layout string that successfully parsed the time
//   - err: Error if no format could parse the string
//
// Example:
//
//	t, format, err := ParseTime("2024-03-15T14:30:00Z")
//	// t = time value, format = "2006-01-02T15:04:05Z07:00" (RFC3339), err = nil
//
//	t, format, err := ParseTime("2024-03-15")
//	// t = time value, format = "2006-01-02" (DateOnly), err = nil
//
//	// Use the format to maintain consistency
//	formatted := t.Format(format) // Returns string in same format as input
func ParseTime(str string) (t time.Time, format string, err error) {
	for _, format := range timeFormats {
		t, err = time.Parse(format, str)
		if err == nil {
			return t, format, nil
		}
	}
	return time.Time{}, "", fmt.Errorf("cannot parse %q as time", str)
}

// timeFormats is the default list of time layout strings tried when parsing time values.
// The formats are ordered from most specific/common to less common, with ISO/RFC formats
// prioritized. This ordering optimizes parsing performance for typical use cases.
//
// The list includes:
//   - RFC3339 formats (ISO 8601) - most common in APIs and web applications
//   - RFC formats (RFC1123, RFC822, etc.) - common in email and HTTP headers
//   - Standard Go time constants (UnixDate, ANSIC, etc.)
//   - Custom formats for common use cases (browser inputs, database outputs, etc.)
//   - German date formats (DD.MM.YYYY) - locale-specific example
//
// Applications can customize this list globally or use StringParser.TimeFormats
// for per-parser customization.
var timeFormats = []string{
	time.RFC3339Nano,       // "2006-01-02T15:04:05.999999999Z07:00" - ISO 8601 with nanoseconds
	time.RFC3339,           // "2006-01-02T15:04:05Z07:00" - ISO 8601, most common API format
	formatBrowserLocalTime, // "2006-01-02T15:04" - HTML5 datetime-local input format
	time.RFC1123Z,          // "Mon, 02 Jan 2006 15:04:05 -0700" - HTTP date format
	time.RFC850,            // "Monday, 02-Jan-06 15:04:05 MST" - Old HTTP format
	time.RFC1123,           // "Mon, 02 Jan 2006 15:04:05 MST" - Email/HTTP dates
	time.RubyDate,          // "Mon Jan 02 15:04:05 -0700 2006" - Ruby time format
	time.UnixDate,          // "Mon Jan _2 15:04:05 MST 2006" - Unix date command output
	time.ANSIC,             // "Mon Jan _2 15:04:05 2006" - ANSI C asctime() format
	time.RFC822Z,           // "02 Jan 06 15:04 -0700" - RFC 822 with numeric timezone
	time.RFC822,            // "02 Jan 06 15:04 MST" - RFC 822 format
	time.StampNano,         // "Jan _2 15:04:05.000000000" - Timestamp with nanoseconds
	time.StampMicro,        // "Jan _2 15:04:05.000000" - Timestamp with microseconds
	time.StampMilli,        // "Jan _2 15:04:05.000" - Timestamp with milliseconds
	time.Stamp,             // "Jan _2 15:04:05" - Unix timestamp format
	formatTimeString,       // "2006-01-02 15:04:05.999999999 -0700 MST" - Complete time string
	time.DateTime,          // "2006-01-02 15:04:05" - SQL datetime format
	formatDateTimeMinute,   // "2006-01-02 15:04" - DateTime without seconds
	time.DateOnly,          // "2006-01-02" - ISO date only, SQL date format
	formatDateTimeGerman,   // "02.01.2006 15:04:05" - German datetime format
	formatDateGerman,       // "02.01.2006" - German date format (DD.MM.YYYY)
}

// Custom time format constants for common patterns not included in the time package.
const (
	formatDateTimeMinute   = "2006-01-02 15:04"                        // SQL datetime without seconds
	formatDateTimeGerman   = "02.01.2006 15:04:05"                     // German datetime (DD.MM.YYYY HH:MM:SS)
	formatDateGerman       = "02.01.2006"                              // German date (DD.MM.YYYY)
	formatTimeString       = "2006-01-02 15:04:05.999999999 -0700 MST" // Complete Go time string
	formatBrowserLocalTime = "2006-01-02T15:04"                        // HTML5 datetime-local input
)
