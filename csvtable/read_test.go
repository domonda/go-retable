package csvtable

import (
	"bytes"
	"errors"
	"testing"
	"testing/iotest"

	"github.com/domonda/go-retable"
	"github.com/stretchr/testify/require"
)

type booking struct {
	Datum  string  `csv:"Datum"`
	Text   string  `csv:"Text"`
	Betrag float64 `csv:"Betrag"`
	Storno bool    `csv:"Storno"`
}

var bookingNaming = &retable.StructFieldNaming{Tag: "csv"}

// germanCSVParser configures the spellings of a German bank export, which the
// default parser does not know, so that a result that only the passed Parser
// can produce proves the Parser reached the conversions.
func germanCSVParser() retable.Parser {
	p := retable.NewStringParser()
	p.TrueStrings = []string{"ja"}
	p.FalseStrings = []string{"nein"}
	return p
}

const bookingsCSV = "Datum;Text;Betrag;Storno\r\n" +
	"01.01.2025;Miete;-500,00;nein\r\n" +
	"02.01.2025;Gehalt;2.000,50;ja\r\n"

var wantBookings = []booking{
	{Datum: "01.01.2025", Text: "Miete", Betrag: -500, Storno: false},
	{Datum: "02.01.2025", Text: "Gehalt", Betrag: 2000.50, Storno: true},
}

// TestReadBytesWithFormatToStructSlice covers the entry point for a caller that
// already knows the format of its file, including that the passed Parser is
// carried all the way down into the cell conversions.
func TestReadBytesWithFormatToStructSlice(t *testing.T) {
	format := &Format{Encoding: "UTF-8", Separator: ";", Newline: "\r\n"}

	bookings, err := ReadBytesWithFormatToStructSlice[booking](
		[]byte(bookingsCSV), format, bookingNaming, nil, germanCSVParser(), nil, nil,
	)
	require.NoError(t, err)
	require.Equal(t, wantBookings, bookings)

	t.Run("without the parser the boolean column cannot be read", func(t *testing.T) {
		// Proves the case above passes because of the Parser and not
		// because the default one would have parsed "ja" anyway.
		_, err := ReadBytesWithFormatToStructSlice[booking](
			[]byte(bookingsCSV), format, bookingNaming, nil, nil, nil, nil,
		)
		// Naming the cell, so this cannot start passing because some
		// other column failed instead.
		require.ErrorContains(t, err, "bool",
			"the boolean column is what the default parser cannot read")
	})

	t.Run("an invalid format is reported instead of parsed", func(t *testing.T) {
		_, err := ReadBytesWithFormatToStructSlice[booking](
			[]byte(bookingsCSV), &Format{Encoding: "UTF-8", Separator: ";"}, bookingNaming, nil, nil, nil, nil,
		)
		require.ErrorContains(t, err, "missing csv.Format.Newline")
	})
}

// TestReadFileWithFormatToStructSlice covers the same entry point reading from
// an io.Reader, which is the form the callers of this package use.
func TestReadFileWithFormatToStructSlice(t *testing.T) {
	file := bytes.NewReader([]byte(bookingsCSV))
	format := &Format{Encoding: "UTF-8", Separator: ";", Newline: "\r\n"}

	bookings, err := ReadFileWithFormatToStructSlice[booking](
		file, format, bookingNaming, nil, germanCSVParser(), nil, nil,
	)
	require.NoError(t, err)
	require.Equal(t, wantBookings, bookings)
}

// TestReadBytesDetectFormatToStructSlice covers the entry point that has to
// work out separator, encoding and line ending itself, and that has to find
// the table between the header and trailer lines that bank exports wrap it in.
func TestReadBytesDetectFormatToStructSlice(t *testing.T) {
	csv := "Kontoauszug Nr. 4\n" +
		"Zeitraum;01.01.2025;31.01.2025\n" +
		"\n" +
		bookingsCSV +
		"\n" +
		"Erstellt am 03.01.2025\n"

	bookings, format, err := ReadBytesDetectFormatToStructSlice[booking](
		[]byte(csv), nil, bookingNaming, nil, germanCSVParser(), nil, nil,
		"Datum", "Text", "Betrag", "Storno",
	)
	require.NoError(t, err)
	require.Equal(t, ";", format.Separator)
	require.Equal(t, "UTF-8", format.Encoding)
	require.Equal(t, wantBookings, bookings)

	// Format detection runs before anything is parsed, so its failure
	// has to reach the caller. A configuration naming an encoding this
	// package does not know decodes nothing, and returning an empty
	// slice for it would look like an empty file.
	t.Run("a failing format detection is reported", func(t *testing.T) {
		_, _, err := ReadBytesDetectFormatToStructSlice[booking](
			[]byte(bookingsCSV),
			&FormatDetectionConfig{Encodings: []string{"NO-SUCH-ENCODING"}},
			bookingNaming, nil, germanCSVParser(), nil, nil,
		)
		// Naming the encoding, so this cannot start passing because
		// naming or parsing failed for some unrelated reason instead.
		require.ErrorContains(t, err, "NO-SUCH-ENCODING")
	})

	t.Run("a missing required column is reported", func(t *testing.T) {
		// Without the column the table cannot be located either, so the
		// caller has to learn that instead of getting zero valued rows.
		_, _, err := ReadBytesDetectFormatToStructSlice[booking](
			[]byte(csv), nil, bookingNaming, nil, germanCSVParser(), nil, nil, "Waehrung",
		)
		require.ErrorContains(t, err, `required column "Waehrung"`)
	})
}

// TestReadFileDetectFormatToStructSlice covers format detection from an
// io.Reader, including that a UTF-8 BOM written by a spreadsheet export
// does not end up in the first column title.
func TestReadFileDetectFormatToStructSlice(t *testing.T) {
	file := bytes.NewReader(append([]byte("\xEF\xBB\xBF"), bookingsCSV...))

	bookings, format, err := ReadFileDetectFormatToStructSlice[booking](
		file, nil, bookingNaming, nil, germanCSVParser(), nil, nil,
	)
	require.NoError(t, err)
	require.Equal(t, "UTF-8", format.Encoding)
	require.Equal(t, "\r\n", format.Newline)
	require.Equal(t, wantBookings, bookings)
}

// TestReadFileToStructSliceReadError covers that a reader that fails is
// reported as that error. Both reader entry points read everything before
// they parse anything, and a reader that cannot be drained, such as one
// over a missing file the caller opened, has to surface instead of being
// parsed as empty CSV data into an empty slice.
func TestReadFileToStructSliceReadError(t *testing.T) {
	readErr := errors.New("read failed")

	t.Run("with format", func(t *testing.T) {
		format := &Format{Encoding: "UTF-8", Separator: ";", Newline: "\r\n"}
		bookings, err := ReadFileWithFormatToStructSlice[booking](
			iotest.ErrReader(readErr), format, bookingNaming, nil, nil, nil, nil,
		)
		require.ErrorIs(t, err, readErr)
		require.Error(t, err)
		require.Nil(t, bookings)
	})

	t.Run("detecting the format", func(t *testing.T) {
		bookings, format, err := ReadFileDetectFormatToStructSlice[booking](
			iotest.ErrReader(readErr), nil, bookingNaming, nil, nil, nil, nil,
		)
		require.ErrorIs(t, err, readErr)
		require.Nil(t, bookings)
		require.Nil(t, format)
	})
}
