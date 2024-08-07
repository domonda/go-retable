package retable

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Parser interface {
	ParseInt(string) (int64, error)
	ParseUnt(string) (uint64, error)
	ParseFloat(string) (float64, error)
	ParseBool(string) (bool, error)
	ParseTime(string) (time.Time, error)
	ParseDuration(string) (time.Duration, error)
}

var _ Parser = new(StringParser)

type StringParser struct {
	TrueStrings  []string `json:"trueStrings"`
	FalseStrings []string `json:"falseStrings"`
	NilStrings   []string `json:"nilStrings"`
	TimeFormats  []string `json:"timeFormats"`
}

func NewStringParser() *StringParser {
	c := &StringParser{
		TrueStrings:  []string{"true", "True", "TRUE", "yes", "Yes", "YES", "1"},
		FalseStrings: []string{"false", "False", "FALSE", "no", "No", "NO", "0"},
		NilStrings:   []string{"", "nil", "<nil>", "null", "NULL"},
		TimeFormats:  timeFormats,
	}
	return c
}

func (p *StringParser) ParseInt(str string) (int64, error) {
	return strconv.ParseInt(str, 10, 64)
}

func (p *StringParser) ParseUnt(str string) (uint64, error) {
	return strconv.ParseUint(str, 10, 64)
}

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

func (p *StringParser) ParseTime(str string) (time.Time, error) {
	for _, format := range p.TimeFormats {
		t, err := time.Parse(format, str)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse %q as time", str)
}

func (p *StringParser) ParseDuration(str string) (time.Duration, error) {
	return time.ParseDuration(str)
}

func ParseTime(str string) (t time.Time, format string, err error) {
	for _, format := range timeFormats {
		t, err = time.Parse(format, str)
		if err == nil {
			return t, format, nil
		}
	}
	return time.Time{}, "", fmt.Errorf("cannot parse %q as time", str)
}

var timeFormats = []string{
	time.RFC3339Nano,       // "2006-01-02T15:04:05.999999999Z07:00"
	time.RFC3339,           // "2006-01-02T15:04:05Z07:00"
	formatBrowserLocalTime, // "2006-01-02T15:04"
	time.RFC1123Z,          // "Mon, 02 Jan 2006 15:04:05 -0700"
	time.RFC850,            // "Monday, 02-Jan-06 15:04:05 MST"
	time.RFC1123,           // "Mon, 02 Jan 2006 15:04:05 MST"
	time.RubyDate,          // "Mon Jan 02 15:04:05 -0700 2006"
	time.UnixDate,          // "Mon Jan _2 15:04:05 MST 2006"
	time.ANSIC,             // "Mon Jan _2 15:04:05 2006"
	time.RFC822Z,           // "02 Jan 06 15:04 -0700"
	time.RFC822,            // "02 Jan 06 15:04 MST"
	time.StampNano,         // "Jan _2 15:04:05.000000000"
	time.StampMicro,        // "Jan _2 15:04:05.000000"
	time.StampMilli,        // "Jan _2 15:04:05.000"
	time.Stamp,             // "Jan _2 15:04:05"
	formatTimeString,       // "2006-01-02 15:04:05.999999999 -0700 MST"
	time.DateTime,          // "2006-01-02 15:04:05"
	formatDateTimeMinute,   // "2006-01-02 15:04"
	time.DateOnly,          // "2006-01-02"
	formatDateTimeGerman,   // "02.01.2006 15:04:05"
	formatDateGerman,       // "02.01.2006"
}

const (
	formatDateTimeMinute   = "2006-01-02 15:04"
	formatDateTimeGerman   = "02.01.2006 15:04:05"
	formatDateGerman       = "02.01.2006"
	formatTimeString       = "2006-01-02 15:04:05.999999999 -0700 MST"
	formatBrowserLocalTime = "2006-01-02T15:04"
)
