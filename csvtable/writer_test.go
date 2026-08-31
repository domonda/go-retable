package csvtable

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

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
