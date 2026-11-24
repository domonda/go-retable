package csvtable

import (
	"bytes"
	"errors"
	"reflect"
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
		_, format, err := ReadBytesDetectFormatToStructSlice[booking](
			[]byte(csv), nil, bookingNaming, nil, germanCSVParser(), nil, nil, "Waehrung",
		)
		require.ErrorContains(t, err, `required column "Waehrung"`)
		// The format was detected before the conversion failed and is
		// returned anyway, so the caller can report what it parsed with.
		require.NotNil(t, format)
		require.Equal(t, ";", format.Separator)
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

// panicReader is an io.Reader that panics with value on the first Read,
// standing in for a broken reader a caller passes to the Read*File functions.
type panicReader struct{ value any }

func (r panicReader) Read([]byte) (int, error) { panic(r.value) }

// TestReadToStructSliceRecoversPanics covers that every exported read entry
// point returns a panic of a caller-supplied func or reader as an error
// instead of passing it on, because a panic while importing an uploaded file
// takes down the whole worker process instead of rejecting that file.
func TestReadToStructSliceRecoversPanics(t *testing.T) {
	panicErr := errors.New("caller func is broken")
	panicErrValidate := func(reflect.Value) error { panic(panicErr) }
	panicStringValidate := func(reflect.Value) error { panic("validate is broken") }
	panicScanner := retable.ScannerFunc(func(reflect.Value, string, retable.Parser) error { panic(panicErr) })
	format := &Format{Encoding: "UTF-8", Separator: ";", Newline: "\r\n"}
	csvData := []byte(bookingsCSV)

	tests := []struct {
		name string
		read func(t *testing.T) error
		// wantErr is the panic value when it was an error,
		// which has to stay reachable for errors.Is
		wantErr error
		wantMsg string
		// wantFrame is the function that panicked,
		// which the stack trace in the message has to show
		wantFrame string
	}{
		{
			name: "ReadStringsToStructSlice with error panic",
			read: func(t *testing.T) error {
				rows, err := ParseWithFormat(csvData, format)
				require.NoError(t, err)
				_, err = ReadStringsToStructSlice[booking](rows, bookingNaming, nil, germanCSVParser(), nil, panicErrValidate)
				return err
			},
			wantErr:   panicErr,
			wantMsg:   "csvtable: panic: caller func is broken",
			wantFrame: "retable.ViewToStructSlice",
		},
		{
			name: "ReadStringsToStructSlice with string panic",
			read: func(t *testing.T) error {
				rows, err := ParseWithFormat(csvData, format)
				require.NoError(t, err)
				_, err = ReadStringsToStructSlice[booking](rows, bookingNaming, nil, germanCSVParser(), nil, panicStringValidate)
				return err
			},
			wantMsg:   "csvtable: panic: validate is broken",
			wantFrame: "retable.ViewToStructSlice",
		},
		{
			// retable.SmartAssign recovers a panic of the cell conversion
			// hooks itself, so it carries the retable prefix, not csvtable's
			name: "ReadStringsToStructSlice with panicking Scanner",
			read: func(t *testing.T) error {
				rows, err := ParseWithFormat(csvData, format)
				require.NoError(t, err)
				_, err = ReadStringsToStructSlice[booking](rows, bookingNaming, panicScanner, germanCSVParser(), nil, nil)
				return err
			},
			wantErr:   panicErr,
			wantMsg:   "retable: panic: caller func is broken",
			wantFrame: "retable.SmartAssign",
		},
		{
			name: "ReadBytesWithFormatToStructSlice",
			read: func(t *testing.T) error {
				_, err := ReadBytesWithFormatToStructSlice[booking](csvData, format, bookingNaming, nil, germanCSVParser(), nil, panicErrValidate)
				return err
			},
			wantErr:   panicErr,
			wantMsg:   "csvtable: panic: caller func is broken",
			wantFrame: "retable.ViewToStructSlice",
		},
		{
			name: "ReadFileWithFormatToStructSlice with panicking validate",
			read: func(t *testing.T) error {
				_, err := ReadFileWithFormatToStructSlice[booking](bytes.NewReader(csvData), format, bookingNaming, nil, germanCSVParser(), nil, panicErrValidate)
				return err
			},
			wantErr:   panicErr,
			wantMsg:   "csvtable: panic: caller func is broken",
			wantFrame: "retable.ViewToStructSlice",
		},
		{
			name: "ReadFileWithFormatToStructSlice with panicking reader",
			read: func(t *testing.T) error {
				_, err := ReadFileWithFormatToStructSlice[booking](panicReader{"reader is broken"}, format, bookingNaming, nil, nil, nil, nil)
				return err
			},
			wantMsg:   "csvtable: panic: reader is broken",
			wantFrame: "panicReader.Read",
		},
		{
			name: "ReadBytesDetectFormatToStructSlice",
			read: func(t *testing.T) error {
				_, _, err := ReadBytesDetectFormatToStructSlice[booking](csvData, nil, bookingNaming, nil, germanCSVParser(), nil, panicErrValidate)
				return err
			},
			wantErr:   panicErr,
			wantMsg:   "csvtable: panic: caller func is broken",
			wantFrame: "retable.ViewToStructSlice",
		},
		{
			name: "ReadFileDetectFormatToStructSlice with panicking validate",
			read: func(t *testing.T) error {
				_, _, err := ReadFileDetectFormatToStructSlice[booking](bytes.NewReader(csvData), nil, bookingNaming, nil, germanCSVParser(), nil, panicErrValidate)
				return err
			},
			wantErr:   panicErr,
			wantMsg:   "csvtable: panic: caller func is broken",
			wantFrame: "retable.ViewToStructSlice",
		},
		{
			name: "ReadFileDetectFormatToStructSlice with panicking reader",
			read: func(t *testing.T) error {
				_, _, err := ReadFileDetectFormatToStructSlice[booking](panicReader{panicErr}, nil, bookingNaming, nil, nil, nil, nil)
				return err
			},
			wantErr:   panicErr,
			wantMsg:   "csvtable: panic: caller func is broken",
			wantFrame: "panicReader.Read",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			var err error
			require.NotPanics(t, func() { err = tt.read(t) })

			// then
			requireRecoveredPanic(t, err, tt.wantMsg, tt.wantFrame)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

// fuzzRecord is a small struct with a column of every kind of cell
// conversion that customer uploads go through.
type fuzzRecord struct {
	Name   string  `csv:"Name"`
	Amount float64 `csv:"Amount"`
	Count  int     `csv:"Count"`
	Paid   bool    `csv:"Paid"`
}

func FuzzReadBytesDetectFormatToStructSlice(f *testing.F) {
	for _, seed := range fuzzSeeds() {
		f.Add([]byte(seed), false)
		f.Add([]byte(seed), true)
	}
	f.Add([]byte("Report\nName;Amount;Count;Paid\nMeier;1,50;3;ja\n\nTotal;1,50\n"), true)
	f.Fuzz(func(t *testing.T, data []byte, withRequiredCols bool) {
		// given
		var requiredCols []string
		if withRequiredCols {
			requiredCols = []string{"Name", "Amount"}
		}

		// when
		_, _, err := ReadBytesDetectFormatToStructSlice[fuzzRecord](
			data, nil, bookingNaming, nil, germanCSVParser(), nil, nil, requiredCols...,
		)

		// then
		requireNoRecoveredPanic(t, err)
	})
}

// TestReadStringsToStructSlice_SeveralColumnsMatchOneField covers that the
// naming decides what a file with two columns for one struct field does:
// by default the closest title fills the field, and with
// DisallowAmbiguousTitles the file is refused.
func TestReadStringsToStructSlice_SeveralColumnsMatchOneField(t *testing.T) {
	type booking struct {
		Text   string `col:"Text"`
		Amount string `col:"Betrag"`
	}
	rows := [][]string{
		{"Kontoauszug"},
		{"BETRAG", "Text", "Betrag"},
		{"1,00", "Miete", "2,00"},
		{"3,00", "Strom", ""},
	}
	naming := retable.DefaultStructFieldNamingIgnoreUntagged.WithMatchTitle(retable.TrimEqualFold)

	bookings, err := ReadStringsToStructSlice[booking](rows, naming, nil, nil, nil, nil, "Betrag", "Text")
	require.NoError(t, err)
	// The column titled like the field wins, also where its cell is empty
	require.Equal(t, []booking{{Text: "Miete", Amount: "2,00"}, {Text: "Strom", Amount: ""}}, bookings)

	bookings, err = ReadStringsToStructSlice[booking](rows, naming.WithDisallowAmbiguousTitles(true), nil, nil, nil, nil, "Betrag", "Text")
	require.ErrorIs(t, err, retable.ErrAmbiguousColumnTitle)
	require.ErrorContains(t, err, `columns "BETRAG" and "Betrag" both match "Betrag"`)
	require.Nil(t, bookings)
}
