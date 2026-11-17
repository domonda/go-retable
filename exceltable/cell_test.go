package exceltable

import (
	"context"
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/domonda/go-retable"
)

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	require.NoError(t, err)
	return loc
}

// TestTimeSerial compares timeSerial with serials that go-structtable
// produced through github.com/tealeg/xlsx/v3 v3.3.13
// (Cell.SetDateWithOptions with the time's own location).
// They were generated once outside this module and are hardcoded,
// because the files customers already have must keep their dates.
// The arithmetic is identical; the writer then stores the serial with
// Excel's 15 significant digits, see TestRoundToExcelPrecision.
func TestTimeSerial(t *testing.T) {
	vienna := mustLoadLocation(t, "Europe/Vienna")
	kiritimati := mustLoadLocation(t, "Pacific/Kiritimati")
	newYork := mustLoadLocation(t, "America/New_York")
	cases := []struct {
		time time.Time
		want float64
	}{
		{time.Date(2026, 11, 16, 19, 1, 35, 123456789, vienna), 46342.7927676326},
		{time.Date(2026, 7, 1, 12, 0, 0, 0, vienna), 46204.5}, // summer time
		{time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), 45351},
		{time.Date(1999, 12, 31, 23, 59, 59, 0, time.FixedZone("X", -5*3600)), 36525.99998842592},
		{time.Date(1900, 3, 1, 0, 0, 0, 0, time.UTC), 61},
		// Before the non-existent 1900-02-29 that Excel counts as serial 60,
		// day zero is 1899-12-31, like in Excel. go-structtable wrote these
		// one day late; the corrected serials are not from tealeg/xlsx.
		{time.Date(1900, 2, 28, 12, 0, 0, 0, time.UTC), 59.5},
		{time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC), 1},
		{time.Date(2038, 1, 19, 3, 14, 7, 999999999, time.UTC), 50424.13481481482},
		{time.Date(2026, 1, 1, 0, 30, 0, 0, kiritimati), 46023.02083333333}, // UTC+14
		{time.Date(2026, 3, 8, 1, 59, 59, 0, newYork), 46089.08332175926},
	}
	for _, tc := range cases {
		t.Run(tc.time.Format(time.RFC3339Nano), func(t *testing.T) {
			require.Equal(t, tc.want, timeSerial(tc.time))
		})
	}
}

func TestDurationCell(t *testing.T) {
	require.Equal(t, NumberCell(1.25, "[h]:mm:ss"), durationCell(30*time.Hour, "[h]:mm:ss"))
	require.Equal(t, NumberCell(5401.5/86400, ""), durationCell(90*time.Minute+1500*time.Millisecond, ""))
	require.Equal(t, NumberCell(0, "[h]:mm:ss"), durationCell(0, "[h]:mm:ss"))
	// Excel shows #### for a negative time value, so it is written as text
	require.Equal(t, StringCell("-1:00:00"), durationCell(-time.Hour, "[h]:mm:ss"))
	require.Equal(t, StringCell("-0:01:30.5"), durationCell(-90500*time.Millisecond, "[h]:mm:ss"))
}

func TestClockDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0:00:00"},
		{time.Nanosecond, "0:00:00.000000001"},
		{1500 * time.Millisecond, "0:00:01.5"},
		{30*time.Hour + 2*time.Minute + 3*time.Second, "30:02:03"},
		{-90 * time.Minute, "-1:30:00"},
		{math.MinInt64, "-2562047:47:16.854775808"},
		{math.MaxInt64, "2562047:47:16.854775807"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			got := clockDuration(tc.d)
			require.Equal(t, tc.want, got)
			parsed, err := retable.NewStringParser().ParseDuration(got)
			require.NoError(t, err, "ParseDuration reads the clock form back")
			require.Equal(t, tc.d, parsed)
		})
	}
}

func TestCellConstructors(t *testing.T) {
	vienna := mustLoadLocation(t, "Europe/Vienna")
	someTime := time.Date(2026, 11, 16, 19, 1, 35, 0, vienna)

	require.Equal(t, Cell{}, DateTimeCell(time.Time{}, "dd.mm.yyyy"), "zero time is an empty cell")
	require.Equal(t, Cell{}, DateTimeCell(time.Time{}.In(vienna), ""), "zero time in a location is an empty cell")
	require.Equal(t,
		Cell{kind: cellDateTime, value: timeSerial(someTime), format: "dd.mm.yyyy"},
		DateTimeCell(someTime, "dd.mm.yyyy"),
	)
	// Excel dates cover the years 1900 to 9999, other times are text
	require.Equal(t,
		StringCell("1899-12-31T23:59:59.5"),
		DateTimeCell(time.Date(1899, 12, 31, 23, 59, 59, 5e8, vienna), "dd.mm.yyyy"),
	)
	require.Equal(t,
		StringCell("10000-01-01T00:00:00"),
		DateTimeCell(time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), ""),
	)

	// Cells are comparable, which is what formatter tests rely on
	require.True(t, NumberCell(1.5, "0.0") == NumberCell(1.5, "0.0"))
	require.False(t, NumberCell(1.5, "0.0") == NumberCell(1.5, "0.00"))
	require.True(t, StringCell("a") == StringCell("a"))
	require.False(t, StringCell("1") == NumberCell(1, ""))
	require.True(t, BoolCell(true) == BoolCell(true))
	require.True(t, DateTimeCell(someTime, "") == DateTimeCell(someTime.UTC().In(vienna), ""))
	require.False(t, intCell(1) == NumberCell(1, ""), "integer cells keep the exact integer")

	nan := NumberCell(math.NaN(), "")
	require.Equal(t, cellNumber, nan.kind)
}

func TestStringCellFormatter(t *testing.T) {
	view := &retable.AnyValuesView{Cols: []string{"A"}, Rows: [][]any{{42}}}
	ctx := context.Background()

	cell, err := StringCellFormatter(retable.PrintfCellFormatter("#%v")).FormatCell(ctx, view, 0, 0)
	require.NoError(t, err)
	require.Equal(t, StringCell("#42"), cell)

	cell, err = StringCellFormatter(retable.PrintfRawCellFormatter("<b>%v</b>")).FormatCell(ctx, view, 0, 0)
	require.NoError(t, err)
	require.Equal(t, StringCell("<b>42</b>"), cell, "raw is ignored")

	_, err = StringCellFormatter(retable.UnsupportedCellFormatter{}).FormatCell(ctx, view, 0, 0)
	require.ErrorIs(t, err, errors.ErrUnsupported)

	failing := retable.CellFormatterFunc(func(context.Context, retable.View, int, int) (string, bool, error) {
		return "", false, errors.New("boom")
	})
	_, err = StringCellFormatter(failing).FormatCell(ctx, view, 0, 0)
	require.EqualError(t, err, "boom")
}

func TestRoundToExcelPrecision(t *testing.T) {
	cases := []struct {
		value float64
		want  float64
	}{
		{0, 0},
		{1.0 / 24, 0.0416666666666667},
		{46342.792766203704, 46342.7927662037},
		{-2.0 / 3, -0.666666666666667},
		{1234.5, 1234.5},
		{123456789012345678, 123456789012346000},
		{1e-300 / 3, 3.33333333333333e-301},
		{math.MaxFloat64, math.MaxFloat64}, // rounding up would overflow
	}
	for _, tc := range cases {
		t.Run(strconv.FormatFloat(tc.value, 'g', -1, 64), func(t *testing.T) {
			require.Equal(t, tc.want, roundToExcelPrecision(tc.value))
		})
	}
	require.True(t, math.IsNaN(roundToExcelPrecision(math.NaN())))
	require.Equal(t, math.Inf(-1), roundToExcelPrecision(math.Inf(-1)))
}

// TestTimeSerialStaysOnDay: the float64 serial of a time a few nanoseconds
// before midnight of 9999-12-31 rounds to the next day's integer; timeSerial
// keeps it on its day, and keeps sub-millisecond precision for current dates.
func TestTimeSerialStaysOnDay(t *testing.T) {
	last := timeSerial(time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC))
	require.Less(t, last, 2958466.0)
	require.Equal(t, 2958465.0, math.Floor(last))

	a := timeSerial(time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC))
	b := timeSerial(time.Date(2026, 11, 16, 10, 0, 0, 500000, time.UTC))
	require.Greater(t, b, a, "half a millisecond must change the serial")
}
