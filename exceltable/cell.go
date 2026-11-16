package exceltable

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/domonda/go-retable"
)

// cellKind tells what a Cell holds.
type cellKind uint8

const (
	cellEmpty cellKind = iota
	cellString
	cellNumber
	cellBool
	cellDateTime
)

// Cell is a typed spreadsheet cell value: empty, a string, a number with
// an optional number format, a bool, or a date/time with a number format.
// It is what a CellFormatter returns, the counterpart of the string that
// a retable.CellFormatter returns for csvtable and htmltable.
//
// The zero Cell is an empty cell. Cell values are comparable with ==,
// so a CellFormatter can be tested by comparing its result with the
// expected Cell.
//
// Excel stores every number as an IEEE 754 double. Integers are written
// to the file exactly, but integers beyond 2^53 lose precision when the
// file is opened by Excel or LibreOffice.
type Cell struct {
	kind cellKind
	// value is a string, float64, int64, uint64 or bool,
	// or the date serial as float64 for cellDateTime.
	value  any
	format string
}

// StringCell returns a Cell holding s as text.
// Text never becomes a formula, even if it starts with "=".
// Excel limits a cell to 32767 UTF-16 code units; longer strings are
// truncated to that length when written.
func StringCell(s string) Cell {
	return Cell{kind: cellString, value: s}
}

// NumberCell returns a Cell holding the number n, displayed with the
// passed Excel number format code, for example "#,##0.00" or
// "#,##0.00 [$EUR];-#,##0.00 [$EUR]". The code is written as given.
// An empty format uses Excel's General format.
//
// The number is stored rounded to 15 significant digits,
// the precision of Excel numbers.
// NaN and ±Inf cannot be stored as Excel numbers and are written as
// the strings "NaN", "+Inf" and "-Inf".
func NumberCell(n float64, format string) Cell {
	return Cell{kind: cellNumber, value: n, format: format}
}

// intCell returns a number Cell holding n exactly, used for integer kinds.
func intCell(n int64) Cell {
	return Cell{kind: cellNumber, value: n}
}

// uintCell returns a number Cell holding n exactly, used for unsigned integer kinds.
func uintCell(n uint64) Cell {
	return Cell{kind: cellNumber, value: n}
}

// BoolCell returns a Cell holding the Excel boolean b.
func BoolCell(b bool) Cell {
	return Cell{kind: cellBool, value: b}
}

// DateTimeCell returns a Cell holding t as an Excel date serial number
// in the 1900 date system, displayed with the passed number format code,
// for example "dd.mm.yyyy" or "dd.mm.yyyy hh:mm:ss".
// The code is written as given and never mapped onto one of Excel's
// built-in date formats, which Excel would display per the reader's locale.
//
// The serial represents the wall clock time of t in t's own location,
// because Excel date/time values have no time zone, truncated to whole
// milliseconds, the resolution of Excel times. The truncation keeps a
// time just before midnight on its day, which a float64 serial of the
// year 9999 could not hold at nanosecond precision. Excel still rounds
// the displayed time to the precision of the format, so 23:59:59.999
// shows as the next day with a format that has no fraction of a second.
// An empty format uses the writer's time format, see Writer.WithTimeFormat.
// A zero t returns an empty Cell.
//
// Excel dates start at 1900-01-01 and end with the year 9999.
// A t outside that range returns a string Cell with the wall clock time
// in the form "2006-01-02T15:04:05.999999999", which StringParser.ParseTime
// reads back.
func DateTimeCell(t time.Time, format string) Cell {
	if t.IsZero() {
		return Cell{}
	}
	if year := t.Year(); year < 1900 || year > 9999 {
		return StringCell(t.Format(outOfRangeTimeFormat))
	}
	return Cell{kind: cellDateTime, value: timeSerial(t.Truncate(time.Millisecond)), format: format}
}

// outOfRangeTimeFormat is the text form of a time that Excel cannot store as a date.
const outOfRangeTimeFormat = "2006-01-02T15:04:05.999999999"

// Constants of the date serial arithmetic, equal to the ones of
// github.com/tealeg/xlsx/v3 that go-structtable used. The serials are
// stored with Excel's 15 significant digits (see roundToExcelPrecision),
// so they equal the ones in the files go-structtable produced to that
// precision: about 9 microseconds for current dates (10 decimal places
// of a 5-digit serial), finer before 1927.
const (
	secondsPerDay = float64(24 * time.Hour / time.Second)
	nanosPerDay   = float64(24 * time.Hour / time.Nanosecond)
	// daysBetween1900And1970 is the serial of 1970-01-01 in the 1900 date
	// system, whose day zero is 1899-12-30 because Excel counts the
	// non-existent 1900-02-29.
	daysBetween1900And1970 = 25569.0
	// serialOf1900March1 is the first serial after the non-existent
	// 1900-02-29, which Excel counts as serial 60. Before it, day zero
	// is 1899-12-31 and 1900-01-01 is serial 1.
	serialOf1900March1 = 61.0
)

// timeSerial returns the Excel 1900 date system serial of the wall clock
// time of t in its own location. t must not be before 1900-01-01.
func timeSerial(t time.Time) float64 {
	_, offset := t.Zone()
	wallClock := time.Unix(t.Unix()+int64(offset), int64(t.Nanosecond())).UTC()
	serial := float64(wallClock.Unix())/secondsPerDay + daysBetween1900And1970 + float64(wallClock.Nanosecond())/nanosPerDay
	if serial < serialOf1900March1 {
		serial--
	}
	return serial
}

// durationCell returns d as a fraction of a day, which Excel displays
// as a duration with format, for example "[h]:mm:ss".
// Excel cannot display a negative date or time value, so a negative d
// returns a string Cell with d in the clock form -H:MM:SS[.fraction],
// which StringParser.ParseDuration reads back.
func durationCell(d time.Duration, format string) Cell {
	if d < 0 {
		return StringCell(clockDuration(d))
	}
	return NumberCell(float64(d)/float64(24*time.Hour), format)
}

// clockDuration formats d as [-]H:MM:SS[.fraction] with the fraction
// of a second only as long as needed and none for whole seconds.
func clockDuration(d time.Duration) string {
	sign := ""
	if d < 0 {
		sign = "-"
	}
	// Each part is negated on its own, because -d overflows
	// for the smallest time.Duration
	hours := int64(absDuration(d / time.Hour))
	minutes := int64(absDuration(d % time.Hour / time.Minute))
	seconds := int64(absDuration(d % time.Minute / time.Second))
	nanos := int64(absDuration(d % time.Second))
	str := fmt.Sprintf("%s%d:%02d:%02d", sign, hours, minutes, seconds)
	if nanos != 0 {
		str += strings.TrimRight(fmt.Sprintf(".%09d", nanos), "0")
	}
	return str
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// CellFormatter converts the view cell at a row/col position into a typed Cell.
// It is the exceltable counterpart of retable.CellFormatter and is
// registered with the same With*Formatter methods of Writer.
//
// Returning errors.ErrUnsupported tells the writer to try the next
// formatter or its built-in conversion; any other error aborts the sheet
// and poisons the workbook, see Workbook.
type CellFormatter interface {
	FormatCell(ctx context.Context, view retable.View, row, col int) (Cell, error)
}

// CellFormatterFunc implements CellFormatter with a function.
//
// Example:
//
//	amountCell := exceltable.CellFormatterFunc(func(ctx context.Context, view retable.View, row, col int) (exceltable.Cell, error) {
//	    amount, ok := view.Cell(row, col).(money.Amount)
//	    if !ok {
//	        return exceltable.Cell{}, errors.ErrUnsupported
//	    }
//	    return exceltable.NumberCell(float64(amount), "#,##0.00"), nil
//	})
type CellFormatterFunc func(ctx context.Context, view retable.View, row, col int) (Cell, error)

// FormatCell implements CellFormatter by calling f.
func (f CellFormatterFunc) FormatCell(ctx context.Context, view retable.View, row, col int) (Cell, error) {
	return f(ctx, view, row, col)
}

// StringCellFormatter adapts a string formatter of csvtable or htmltable
// to a CellFormatter that writes its result as a string Cell.
// The raw result of f is ignored: a spreadsheet string cell needs no escaping.
// errors.ErrUnsupported and other errors of f are passed through.
func StringCellFormatter(f retable.CellFormatter) CellFormatter {
	return CellFormatterFunc(func(ctx context.Context, view retable.View, row, col int) (Cell, error) {
		str, _, err := f.FormatCell(ctx, view, row, col)
		if err != nil {
			return Cell{}, err
		}
		return StringCell(str), nil
	})
}

// excelSignificantDigits is the precision of Excel numbers.
const excelSignificantDigits = 15

// roundToExcelPrecision rounds n to the 15 significant digits Excel holds.
// excelize (since v2.11.1) writes floats with at most 15 significant
// digits by cutting off the rest, which would store one hour, 1/24 of a
// day, as 0.0416666666666666, slightly less than an hour, so that Excel
// functions like HOUR see 0:59:59. Rounding first stores the nearest
// 15-digit value, which excelize then writes unchanged, because every
// decimal of 15 significant digits round-trips through a float64.
func roundToExcelPrecision(n float64) float64 {
	if n == 0 || math.IsNaN(n) || math.IsInf(n, 0) {
		return n
	}
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(n, 'g', excelSignificantDigits, 64), 64)
	if err != nil {
		// Rounding up overflowed beyond math.MaxFloat64
		return n
	}
	return rounded
}
