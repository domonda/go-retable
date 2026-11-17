package csvtable

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/domonda/go-retable"
)

func TestWriter_WriteView(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		writer   *Writer[any]
		view     retable.View
		wantDest string
		wantErr  bool
	}{
		{
			name:     "empty view",
			writer:   NewWriter[any](),
			view:     &retable.AnyValuesView{},
			wantDest: ``,
		},
		{
			name: "simple",
			writer: NewWriter[any]().
				WithHeaderRow(true),
			view: &retable.AnyValuesView{
				Cols: []string{"A", "B", "C"},
				Rows: [][]any{
					{1, "Hello", nil},
					{2, "world!", new(float64)},
				},
			},
			wantDest: "" +
				`A;B;C` + "\r\n" +
				`1;Hello;` + "\r\n" +
				`2;world!;0` + "\r\n",
		},
		{
			name: "simple no header",
			writer: NewWriter[any]().
				WithHeaderRow(true).
				WithHeaderRow(false),
			view: &retable.AnyValuesView{
				Cols: []string{"A", "B", "C"},
				Rows: [][]any{
					{1, "Hello", nil},
					{2, "world!", new(float64)},
				},
			},
			wantDest: "" +
				`1;Hello;` + "\r\n" +
				`2;world!;0` + "\r\n",
		},
		{
			name: "simple padded align left",
			writer: NewWriter[any]().
				WithHeaderRow(true).
				WithDelimiter('|').
				WithPadding(AlignLeft),
			view: &retable.AnyValuesView{
				Cols: []string{"A", "B", "Blah"},
				Rows: [][]any{
					{1, "Hello", nil},
					{123, "world!", new(float64)},
				},
			},
			wantDest: "" +
				`A  |B     |Blah` + "\r\n" +
				`1  |Hello |    ` + "\r\n" +
				`123|world!|0   ` + "\r\n",
		},
		{
			name: "simple padded align center",
			writer: NewWriter[any]().
				WithHeaderRow(true).
				WithDelimiter('|').
				WithPadding(AlignCenter),
			view: &retable.AnyValuesView{
				Cols: []string{"A", "B", "Blah"},
				Rows: [][]any{
					{1, "Hello", nil},
					{123, "world!", new(float64)},
				},
			},
			wantDest: "" +
				` A |  B   |Blah` + "\r\n" +
				` 1 |Hello |    ` + "\r\n" +
				`123|world!| 0  ` + "\r\n",
		},
		{
			name: "simple padded align right",
			writer: NewWriter[any]().
				WithHeaderRow(true).
				WithDelimiter('|').
				WithPadding(AlignRight),
			view: &retable.AnyValuesView{
				Cols: []string{"A", "B", "Blah"},
				Rows: [][]any{
					{1, "Hello", nil},
					{123, "world!", new(float64)},
				},
			},
			wantDest: "" +
				`  A|     B|Blah` + "\r\n" +
				`  1| Hello|    ` + "\r\n" +
				`123|world!|   0` + "\r\n",
		},
		{
			name: "command and quoted fields",
			writer: NewWriter[any]().
				WithHeaderRow(true).
				WithDelimiter(',').
				WithQuoteAllFields(true),
			view: &retable.AnyValuesView{
				Cols: []string{" A ", "B", "C"},
				Rows: [][]any{
					{1, "Hello", nil},
					{2, "world!", new(float64)},
				},
			},
			wantDest: "" +
				`" A ","B","C"` + "\r\n" +
				`"1","Hello",""` + "\r\n" +
				`"2","world!","0"` + "\r\n",
		},
		{
			// A field containing a quote has to be quoted with its quotes doubled,
			// see TestWriter_FieldsWithQuotesAreParsable for why.
			name: "fields containing quotes",
			writer: NewWriter[any]().
				WithHeaderRow(true),
			view: &retable.AnyValuesView{
				Cols: []string{"A", "B"},
				Rows: [][]any{
					{`He said "hi"`, "plain"},
					{`"quoted"`, `a;b"`},
				},
			},
			wantDest: "" +
				`A;B` + "\r\n" +
				`"He said ""hi""";plain` + "\r\n" +
				`"""quoted""";"a;b"""` + "\r\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dest bytes.Buffer
			if err := tt.writer.WriteView(ctx, &dest, tt.view); (err != nil) != tt.wantErr {
				t.Errorf("Writer.WriteView() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotDest := dest.String(); gotDest != tt.wantDest {
				t.Errorf("Writer.WriteView() wrote:\n%s\nbut want:\n%s", gotDest, tt.wantDest)
			}
		})
	}
}

// TestWriter_FieldsWithQuotesAreParsable is the reason why a field containing a
// quote is written as a quoted field with doubled quotes instead of an unquoted
// field with doubled quotes: only the quoted form can be read back by RFC 4180
// parsers like encoding/csv. The unquoted form is only understood by ParseWithFormat.
//
// The values are also tested with a delimiter that occurs within them,
// because then the written field gets split by the delimiter again while parsing
// and has to be joined together using only the quoting of the split parts.
func TestWriter_FieldsWithQuotesAreParsable(t *testing.T) {
	values := []string{
		`He said "hi"`,
		`"quoted"`,
		`"`,
		`""`,
		`{"k":"v"}`,
		`{"k":"v","k2":"v2"}`,
		`trailing"`,
		`"leading`,
		`Super, luxurious truck`,
		`a,b"`,
		`"a,b`,
		`b,`,
		`,`,
		";",
		"a\nb",
		"\"\nfoo",
		"\"\"\nfoo",
		"multi\nline\nvalue",
	}
	for _, delimiter := range []rune{';', ','} {
		for _, value := range values {
			t.Run(fmt.Sprintf("%c/%s", delimiter, value), func(t *testing.T) {
				var dest bytes.Buffer
				view := &retable.AnyValuesView{
					Cols: []string{"A", "B"},
					Rows: [][]any{{value, "z"}},
				}
				err := NewWriter[any]().WithDelimiter(delimiter).WriteView(context.Background(), &dest, view)
				if err != nil {
					t.Fatalf("WriteView() error = %v", err)
				}

				stdlibReader := csv.NewReader(bytes.NewReader(dest.Bytes()))
				stdlibReader.Comma = delimiter
				stdlibRecords, err := stdlibReader.ReadAll()
				if err != nil {
					t.Fatalf("encoding/csv can't read written %q: %v", dest.String(), err)
				}
				if len(stdlibRecords) != 1 || stdlibRecords[0][0] != value {
					t.Errorf("encoding/csv read %q from written %q, but want field %q", stdlibRecords, dest.String(), value)
				}

				rows, err := ParseWithFormat(dest.Bytes(), NewFormat(string(delimiter)))
				if err != nil {
					t.Fatalf("ParseWithFormat() error for written %q: %v", dest.String(), err)
				}
				rows = RemoveEmptyRows(rows)
				if len(rows) != 1 || rows[0][0] != value {
					t.Errorf("ParseWithFormat read %q from written %q, but want field %q", rows, dest.String(), value)
				}
			})
		}
	}
}

// TestWriter_NilCellWithFormatter verifies that a cell of a nil interface is
// written as the nil value instead of panicking. Type formatters dispatch on
// the reflect.Type of the cell, which a nil interface does not have, so
// retable.ReflectTypeCellFormatter reports such a cell as unsupported
// instead of panicking, which lets the null value fallback here handle it.
func TestWriter_NilCellWithFormatter(t *testing.T) {
	formatter := retable.CellFormatterFunc(func(ctx context.Context, view retable.View, row, col int) (string, bool, error) {
		return "FORMATTED", false, nil
	})
	writers := map[string]*Writer[any]{
		"type formatter":           NewWriter[any]().WithTypeFormatter(reflect.TypeOf(""), formatter),
		"kind formatter":           NewWriter[any]().WithKindFormatter(reflect.String, formatter),
		"interface type formatter": NewWriter[any]().WithInterfaceTypeFormatter(reflect.TypeFor[error](), formatter),
		"column formatter":         NewWriter[any]().WithColumnFormatter(0, formatter),
		"no formatter":             NewWriter[any](),
	}
	view := &retable.AnyValuesView{
		Cols: []string{"A", "B"},
		Rows: [][]any{{"s", nil}},
	}
	for name, writer := range writers {
		t.Run(name, func(t *testing.T) {
			var dest bytes.Buffer
			err := writer.WithNewLine("\n").WriteView(context.Background(), &dest, view)
			if err != nil {
				t.Fatalf("WriteView() error = %v", err)
			}
			// The nil cell of column B must be written as the empty nil value
			if got := dest.String(); !strings.HasSuffix(got, ";\n") {
				t.Errorf("nil cell written as %q, want an empty last field", got)
			}
		})
	}
}

// TestWriter_ColumnFormatterNotAppliedToHeader verifies that a column formatter
// only formats cell values. Applying it to the header row overwrote the column
// title with the formatted value.
func TestWriter_ColumnFormatterNotAppliedToHeader(t *testing.T) {
	writer := NewWriter[any]().
		WithNewLine("\n").
		WithHeaderRow(true).
		WithColumnFormatterFunc(0, func(ctx context.Context, view retable.View, row, col int) (string, bool, error) {
			return "FORMATTED", false, nil
		})
	view := &retable.AnyValuesView{
		Cols: []string{"A", "B"},
		Rows: [][]any{{"1", "2"}},
	}

	var dest bytes.Buffer
	err := writer.WriteView(context.Background(), &dest, view)
	if err != nil {
		t.Fatalf("WriteView() error = %v", err)
	}
	if got, want := dest.String(), "A;B\nFORMATTED;2\n"; got != want {
		t.Errorf("WriteView() wrote %q, want %q", got, want)
	}

	rows, err := writer.ViewStrings(context.Background(), view)
	if err != nil {
		t.Fatalf("ViewStrings() error = %v", err)
	}
	if want := [][]string{{"A", "B"}, {"FORMATTED", "2"}}; !reflect.DeepEqual(rows, want) {
		t.Errorf("ViewStrings() = %q, want %q", rows, want)
	}
}

// TestWriter_FormatterFuncVariants verifies that the FormatterFunc methods
// register the same formatter as the CellFormatter methods they delegate to.
func TestWriter_FormatterFuncVariants(t *testing.T) {
	formatterFunc := func(ctx context.Context, view retable.View, row, col int) (string, bool, error) {
		return "FORMATTED", false, nil
	}
	tests := map[string]struct {
		writer *Writer[any]
		// value has to be formatted by the registered formatter,
		// so it must match the registered type, kind or interface
		value any
	}{
		"WithTypeFormatterFunc": {
			writer: NewWriter[any]().WithTypeFormatterFunc(reflect.TypeOf(0), formatterFunc),
			value:  123,
		},
		"WithKindFormatterFunc": {
			writer: NewWriter[any]().WithKindFormatterFunc(reflect.Int, formatterFunc),
			value:  123,
		},
		"WithInterfaceTypeFormatterFunc": {
			writer: NewWriter[any]().WithInterfaceTypeFormatterFunc(reflect.TypeFor[fmt.Stringer](), formatterFunc),
			value:  time.Second, // time.Duration implements fmt.Stringer
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var dest bytes.Buffer
			view := &retable.AnyValuesView{
				Cols: []string{"A"},
				Rows: [][]any{{tt.value}},
			}
			err := tt.writer.WithNewLine("\n").WriteView(context.Background(), &dest, view)
			if err != nil {
				t.Fatalf("WriteView() error = %v", err)
			}
			if got, want := dest.String(), "FORMATTED\n"; got != want {
				t.Errorf("WriteView() wrote %q, want %q", got, want)
			}
		})
	}
}

// TestWriter_WriteAndWriteWithViewer covers the two entry points that
// take a table value rather than a View. Every existing test drove
// WriteView directly, so Write, WriteWithViewer and the viewer
// selection they perform were never executed.
func TestWriter_WriteAndWriteWithViewer(t *testing.T) {
	type Row struct {
		Name string
		Age  int
	}
	rows := []Row{{Name: "Erik", Age: 42}, {Name: "Ann", Age: 7}}

	t.Run("Write selects a viewer when none is configured", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewWriter[[]Row]().WithHeaderRow(true)
		require.NoError(t, w.Write(context.Background(), &buf, rows))
		require.Equal(t, "Name;Age\r\nErik;42\r\nAnn;7\r\n", buf.String())
	})

	// A configured viewer only proves it ran if it produces something
	// selection would not. Tagged columns do that: selection reads the
	// col tags, NoTagsStructRowsViewer ignores them, so the header row
	// says which viewer was used.
	type Tagged struct {
		Name string `col:"Vorname"`
		Age  int    `col:"Alter"`
	}
	tagged := []Tagged{{Name: "Erik", Age: 42}}

	t.Run("selection reads the col tags", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewWriter[[]Tagged]().WithHeaderRow(true)
		require.NoError(t, w.Write(context.Background(), &buf, tagged))
		require.Equal(t, "Vorname;Alter\r\nErik;42\r\n", buf.String())
	})

	t.Run("WithTableViewer is used instead of selecting one", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewWriter[[]Tagged]().
			WithHeaderRow(true).
			WithTableViewer(retable.NoTagsStructRowsViewer())
		require.NoError(t, w.Write(context.Background(), &buf, tagged))
		require.Equal(t, "Name;Age\r\nErik;42\r\n", buf.String(),
			"the field names, not the col tags, so the configured viewer ran")
	})

	t.Run("WriteWithViewer takes the viewer per call", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewWriter[[]Row]().WithHeaderRow(true)
		require.NoError(t, w.WriteWithViewer(context.Background(), &buf, retable.DefaultStructRowsViewer(), rows))
		require.Equal(t, "Name;Age\r\nErik;42\r\nAnn;7\r\n", buf.String())
	})

	t.Run("a viewer error is reported", func(t *testing.T) {
		var buf bytes.Buffer
		// Selection itself cannot fail here: retable.SelectViewer
		// returns &DefaultStructFieldNaming for anything that is not
		// [][]string. The error comes from NewView, which has no view
		// to build from an int.
		w := NewWriter[int]()
		require.Error(t, w.Write(context.Background(), &buf, 42))
	})
}

// TestWriter_OptionsAndGetters covers the builder options and their
// getters, which had no test at all. Each option returns a clone, so a
// silent aliasing bug would let one writer reconfigure another.
func TestWriter_OptionsAndGetters(t *testing.T) {
	base := NewWriter[[][]string]()

	// The documented defaults
	require.False(t, base.QuoteAllFields())
	require.False(t, base.QuoteEmptyFields())
	require.Equal(t, ';', base.Delimiter())
	require.Equal(t, `""`, base.EscapeQuotes())
	require.Equal(t, "", base.NilValue())
	require.Equal(t, "\r\n", base.NewLine())
	require.Nil(t, base.Encoder())

	configured := base.
		WithQuoteAllFields(true).
		WithQuoteEmptyFields(true).
		WithDelimiter(',').
		WithEscapeQuotes(`\"`).
		WithNilValue("NULL").
		WithNewLine("\n").
		WithEncoder(PassthroughEncoder())

	require.True(t, configured.QuoteAllFields())
	require.True(t, configured.QuoteEmptyFields())
	require.Equal(t, ',', configured.Delimiter())
	require.Equal(t, `\"`, configured.EscapeQuotes())
	require.Equal(t, "NULL", configured.NilValue())
	require.Equal(t, "\n", configured.NewLine())
	require.NotNil(t, configured.Encoder())

	// The original must be untouched: every option clones
	require.False(t, base.QuoteAllFields())
	require.Equal(t, ';', base.Delimiter())
	require.Equal(t, "\r\n", base.NewLine())
	require.Nil(t, base.Encoder())
}

// TestWriter_EncoderIsApplied covers that a configured Encoder actually
// sees the output bytes, and that its error is propagated rather than
// producing a silently truncated file.
func TestWriter_EncoderIsApplied(t *testing.T) {
	// The first row of a StringsView is its header, so a data row is
	// needed for anything to be written at all.
	view := retable.NewStringsView("", [][]string{{"a", "b"}, {"x", "y"}})

	t.Run("PassthroughEncoder leaves the bytes unchanged", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewWriter[[][]string]().WithHeaderRow(true).WithEncoder(PassthroughEncoder())
		require.NoError(t, w.WriteView(context.Background(), &buf, view))
		require.Equal(t, "a;b\r\nx;y\r\n", buf.String())
	})

	t.Run("an encoder error is reported", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewWriter[[][]string]().WithHeaderRow(true).WithEncoder(EncoderFunc(func([]byte) ([]byte, error) {
			return nil, errors.New("encoder failed")
		}))
		err := w.WriteView(context.Background(), &buf, view)
		require.ErrorContains(t, err, "encoder failed")
	})

	t.Run("the encoder transforms the output", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewWriter[[][]string]().WithHeaderRow(true).WithEncoder(EncoderFunc(func(b []byte) ([]byte, error) {
			return bytes.ToUpper(b), nil
		}))
		require.NoError(t, w.WriteView(context.Background(), &buf, view))
		require.Equal(t, "A;B\r\nX;Y\r\n", buf.String())
	})
}

// TestWriter_WithTypeFormattersAndReflectFuncs covers the type
// formatter entry points that had no test: the whole-set replacement
// and the two reflection based function registrations.
func TestWriter_WithTypeFormattersAndReflectFuncs(t *testing.T) {
	// The first row of a StringsView is its header, so a data row is
	// needed for a cell formatter to be reached at all.
	view := retable.NewStringsView("", [][]string{{"col"}, {"x"}})

	t.Run("WithTypeFormatters replaces the whole set", func(t *testing.T) {
		var buf bytes.Buffer
		formatters := new(retable.ReflectTypeCellFormatter).
			WithTypeFormatter(reflect.TypeFor[string](), retable.CellFormatterFunc(
				func(context.Context, retable.View, int, int) (string, bool, error) {
					return "replaced", false, nil
				},
			))
		w := NewWriter[[][]string]().WithTypeFormatters(formatters)
		require.NoError(t, w.WriteView(context.Background(), &buf, view))
		require.Contains(t, buf.String(), "replaced")
		require.NotContains(t, buf.String(), "x", "the registered formatter must replace the cell value")
	})

	t.Run("WithTypeFormatterReflectFunc", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewWriter[[][]string]().WithTypeFormatterReflectFunc(
			func(s string) (string, error) { return "fn:" + s, nil },
		)
		require.NoError(t, w.WriteView(context.Background(), &buf, view))
		require.Contains(t, buf.String(), "fn:x")
	})

	t.Run("WithTypeFormatterReflectRawFunc", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewWriter[[][]string]().WithTypeFormatterReflectRawFunc(
			func(s string) (string, error) { return "raw:" + s, nil },
		)
		require.NoError(t, w.WriteView(context.Background(), &buf, view))
		require.Contains(t, buf.String(), "raw:x")
	})
}

// TestWriter_PointerToNullValue pins the csv side of the IsNullLike change:
// a non-nil pointer to a null-like value is written as the nil value.
// Before, IsNullLike reported every non-nil pointer as not null and the
// writer printed the pointed-to value, "0001-01-01 00:00:00 +0000 UTC"
// for a pointer to a zero time.
func TestWriter_PointerToNullValue(t *testing.T) {
	type row struct {
		Time *time.Time
		Text *string
	}
	var zeroTime time.Time
	empty := ""
	var dest bytes.Buffer
	err := NewWriter[[]row]().
		WithNewLine("\n").
		WithNilValue("NULL").
		Write(context.Background(), &dest, []row{{Time: &zeroTime, Text: &empty}})
	require.NoError(t, err)
	// The empty string is a value, the zero time is null-like
	require.Equal(t, "NULL;\n", dest.String())
}

// TestWriter_NilRow: a nil element of a slice of struct pointers is a row
// of null cells written as the nil value. It used to panic in reflect.
func TestWriter_NilRow(t *testing.T) {
	type row struct {
		Name  string
		Count int
	}
	var dest bytes.Buffer
	err := NewWriter[[]*row]().
		WithNewLine("\n").
		WithNilValue("NULL").
		Write(context.Background(), &dest, []*row{{"a", 1}, nil, {"c", 3}})
	require.NoError(t, err)
	require.Equal(t, "a;1\nNULL;NULL\nc;3\n", dest.String())
}

// TestWriter_NilFormatterFuncRemoves: passing nil to a With*FormatterFunc
// method removes the registration like passing nil to With*Formatter,
// instead of storing a nil func that panics when the table is written.
func TestWriter_NilFormatterFuncRemoves(t *testing.T) {
	view := &retable.AnyValuesView{Cols: []string{"A"}, Rows: [][]any{{42}}}
	formatter := func(context.Context, retable.View, int, int) (string, bool, error) {
		return "FORMATTED", false, nil
	}
	writers := map[string]*Writer[any]{
		"type": NewWriter[any]().
			WithTypeFormatterFunc(reflect.TypeFor[int](), formatter).
			WithTypeFormatterFunc(reflect.TypeFor[int](), nil),
		"interface": NewWriter[any]().
			WithInterfaceTypeFormatterFunc(reflect.TypeFor[fmt.Stringer](), formatter).
			WithInterfaceTypeFormatterFunc(reflect.TypeFor[fmt.Stringer](), nil),
		"kind": NewWriter[any]().
			WithKindFormatterFunc(reflect.Int, formatter).
			WithKindFormatterFunc(reflect.Int, nil),
	}
	for name, writer := range writers {
		t.Run(name, func(t *testing.T) {
			var dest bytes.Buffer
			require.NotPanics(t, func() {
				require.NoError(t, writer.WithNewLine("\n").WriteView(context.Background(), &dest, view))
			})
			require.Equal(t, "42\n", dest.String())
		})
	}
}

// TestWriter_NilColumnFormatterFuncRemoves: WithColumnFormatterFunc(i, nil)
// removes the column formatter as documented, instead of storing a nil func
// that panicked on the first cell.
func TestWriter_NilColumnFormatterFuncRemoves(t *testing.T) {
	view := &retable.AnyValuesView{Cols: []string{"A"}, Rows: [][]any{{42}}}
	writer := NewWriter[any]().
		WithColumnFormatterFunc(0, func(context.Context, retable.View, int, int) (string, bool, error) {
			return "FORMATTED", false, nil
		}).
		WithColumnFormatterFunc(0, nil)
	var dest bytes.Buffer
	require.NotPanics(t, func() {
		require.NoError(t, writer.WithNewLine("\n").WriteView(context.Background(), &dest, view))
	})
	require.Equal(t, "42\n", dest.String())
}

func TestWriter_WithUTF8BOM(t *testing.T) {
	type row struct {
		Name  string `col:"Name"`
		Count int    `col:"Count"`
	}
	rows := []row{{Name: "Müller", Count: 1}}
	view := retable.NewStringsView("", [][]string{{"Name", "Count"}, {"Müller", "1"}})

	tests := []struct {
		name  string
		write func(ctx context.Context, w *Writer[[]row], buf *bytes.Buffer) error
		w     *Writer[[]row]
		want  string
	}{
		{
			name:  "Write with header row",
			w:     NewWriter[[]row]().WithHeaderRow(true).WithUTF8BOM(true),
			write: func(ctx context.Context, w *Writer[[]row], buf *bytes.Buffer) error { return w.Write(ctx, buf, rows) },
			want:  "\xEF\xBB\xBFName;Count\r\nMüller;1\r\n",
		},
		{
			name: "WriteWithViewer without header row",
			w:    NewWriter[[]row]().WithUTF8BOM(true),
			write: func(ctx context.Context, w *Writer[[]row], buf *bytes.Buffer) error {
				return w.WriteWithViewer(ctx, buf, &retable.DefaultStructFieldNaming, rows)
			},
			want: "\xEF\xBB\xBFMüller;1\r\n",
		},
		{
			name: "WriteView",
			w:    NewWriter[[]row]().WithHeaderRow(true).WithUTF8BOM(true),
			write: func(ctx context.Context, w *Writer[[]row], buf *bytes.Buffer) error {
				return w.WriteView(ctx, buf, view)
			},
			want: "\xEF\xBB\xBFName;Count\r\nMüller;1\r\n",
		},
		{
			name: "padded",
			w:    NewWriter[[]row]().WithHeaderRow(true).WithPadding(AlignLeft).WithUTF8BOM(true),
			write: func(ctx context.Context, w *Writer[[]row], buf *bytes.Buffer) error {
				return w.WriteView(ctx, buf, view)
			},
			want: "\xEF\xBB\xBFName  ;Count\r\nMüller;1    \r\n",
		},
		{
			name:  "no rows",
			w:     NewWriter[[]row]().WithUTF8BOM(true),
			write: func(ctx context.Context, w *Writer[[]row], buf *bytes.Buffer) error { return w.Write(ctx, buf, nil) },
			want:  UTF8BOM,
		},
		{
			name: "switched off again",
			w:    NewWriter[[]row]().WithHeaderRow(true).WithUTF8BOM(true).WithUTF8BOM(false),
			write: func(ctx context.Context, w *Writer[[]row], buf *bytes.Buffer) error {
				return w.WriteView(ctx, buf, view)
			},
			want: "Name;Count\r\nMüller;1\r\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, tt.write(t.Context(), tt.w, &buf))
			require.Equal(t, tt.want, buf.String())
		})
	}
}

// TestWriter_WithUTF8BOMAndEncoder covers that a UTF-8 byte order mark
// is never written in front of output an encoder converts to another
// encoding, where it would misdeclare the encoding of the file.
func TestWriter_WithUTF8BOMAndEncoder(t *testing.T) {
	view := retable.NewStringsView("", [][]string{{"a"}, {"x"}})
	for _, encoder := range []Encoder{
		PassthroughEncoder(),
		EncoderFunc(func(b []byte) ([]byte, error) { return bytes.ToUpper(b), nil }),
	} {
		var buf bytes.Buffer
		w := NewWriter[[][]string]().WithHeaderRow(true).WithUTF8BOM(true).WithEncoder(encoder)
		err := w.WriteView(t.Context(), &buf, view)
		require.ErrorIs(t, err, ErrUTF8BOMWithEncoder)
		require.Empty(t, buf.String(), "nothing must be written")

		err = w.WithPadding(AlignLeft).WriteView(t.Context(), &buf, view)
		require.ErrorIs(t, err, ErrUTF8BOMWithEncoder)
		require.Empty(t, buf.String(), "nothing must be written")
	}
}

// TestWriter_WithUTF8BOMFormatterError covers that the byte order mark
// goes out with the first row: an error before it leaves dest untouched,
// so that for example an http.ResponseWriter can still send an error status.
func TestWriter_WithUTF8BOMFormatterError(t *testing.T) {
	view := retable.NewStringsView("", [][]string{{"a"}, {"x"}})
	errFormat := errors.New("format failed")
	w := NewWriter[[][]string]().
		WithUTF8BOM(true).
		WithColumnFormatterFunc(0, func(context.Context, retable.View, int, int) (string, bool, error) {
			return "", false, errFormat
		})
	for name, w := range map[string]*Writer[[][]string]{
		"unpadded": w,
		"padded":   w.WithPadding(AlignLeft),
	} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			err := w.WriteView(t.Context(), &buf, view)
			require.ErrorIs(t, err, errFormat)
			require.Empty(t, buf.String(), "nothing must be written")
		})
	}
}

func TestWriter_WithUTF8BOMOptions(t *testing.T) {
	base := NewWriter[[][]string]()
	require.False(t, base.UTF8BOM(), "default")
	withBOM := base.WithUTF8BOM(true)
	require.True(t, withBOM.UTF8BOM())
	require.False(t, base.UTF8BOM(), "the original must be untouched")

	// ViewStrings returns cells, not output, and has no byte order mark
	view := retable.NewStringsView("", [][]string{{"a"}, {"x"}})
	rows, err := withBOM.WithHeaderRow(true).ViewStrings(t.Context(), view)
	require.NoError(t, err)
	require.Equal(t, [][]string{{"a"}, {"x"}}, rows)

	// The output parses back without the byte order mark in the first title
	var buf bytes.Buffer
	require.NoError(t, withBOM.WithHeaderRow(true).WriteView(t.Context(), &buf, view))
	parsed, _, err := ParseDetectFormat(buf.Bytes(), nil)
	require.NoError(t, err)
	require.Equal(t, []string{"a"}, parsed[0])
	require.Equal(t, []string{"x"}, parsed[1])
}
