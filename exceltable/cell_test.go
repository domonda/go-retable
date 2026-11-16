package exceltable

import (
	"context"
	"errors"
	"math"
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
// because the files customers already have must keep their dates
// bit for bit.
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

func TestDurationSerial(t *testing.T) {
	require.Equal(t, 1.25, durationSerial(30*time.Hour))
	require.Equal(t, 5401.5/86400, durationSerial(90*time.Minute+1500*time.Millisecond))
	require.Equal(t, -1.0/24, durationSerial(-time.Hour))
	require.Equal(t, 0.0, durationSerial(0))
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
