package retable

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// nilCellFormatters returns one ReflectTypeCellFormatter per dispatch
// strategy, each configured to format a string cell with the passed
// formatter, so that every strategy can be checked against the same cell.
func nilCellFormatters(formatter CellFormatter) map[string]*ReflectTypeCellFormatter {
	return map[string]*ReflectTypeCellFormatter{
		"no formatter":        NewReflectTypeCellFormatter(),
		"type formatter":      NewReflectTypeCellFormatter().WithTypeFormatter(reflect.TypeFor[string](), formatter),
		"kind formatter":      NewReflectTypeCellFormatter().WithKindFormatter(reflect.String, formatter),
		"interface formatter": NewReflectTypeCellFormatter().WithInterfaceTypeFormatter(reflect.TypeFor[error](), formatter),
		"default formatter":   NewReflectTypeCellFormatter().WithDefaultFormatter(formatter),
	}
}

// TestReflectTypeCellFormatter_FormatCell_NilInterfaceCell covers a cell that
// holds a nil interface. Every dispatch strategy of ReflectTypeCellFormatter
// keys on the reflect.Type of the cell value, which a nil interface does not
// have, so reading its type used to panic on the invalid reflect.Value and
// take down whoever was writing the table.
//
// Reporting errors.ErrUnsupported instead hands the cell back to the caller
// unformatted, which is what lets a writer treat it as a null value. That also
// holds for a configured default formatter: it is a fallback for a value whose
// type matched nothing, not a handler for a cell that has no value to format.
func TestReflectTypeCellFormatter_FormatCell_NilInterfaceCell(t *testing.T) {
	formatter := CellFormatterFunc(func(context.Context, View, int, int) (string, bool, error) {
		return "FORMATTED", false, nil
	})
	// Column A holds a string so the formatters below are known to match
	// something, column B holds the nil interface under test.
	view := &AnyValuesView{
		Cols: []string{"A", "B"},
		Rows: [][]any{{"a", nil}},
	}

	for name, f := range nilCellFormatters(formatter) {
		t.Run(name, func(t *testing.T) {
			var (
				str string
				raw bool
				err error
			)
			require.NotPanics(t, func() {
				str, raw, err = f.FormatCell(context.Background(), view, 0, 1)
			}, "a nil interface cell must not panic")
			require.ErrorIs(t, err, errors.ErrUnsupported)
			require.Empty(t, str)
			require.False(t, raw)
		})
	}
}

// TestReflectTypeCellFormatter_FormatCell_TypedCell is the counterpart of the
// test above: it proves the formatters used there do fire for a cell that has
// a type, so the unsupported result for the nil cell cannot pass for the
// trivial reason that nothing was configured to format it.
func TestReflectTypeCellFormatter_FormatCell_TypedCell(t *testing.T) {
	formatter := CellFormatterFunc(func(context.Context, View, int, int) (string, bool, error) {
		return "FORMATTED", false, nil
	})
	view := &AnyValuesView{
		Cols: []string{"A", "B"},
		Rows: [][]any{{"a", nil}},
	}

	for name, f := range nilCellFormatters(formatter) {
		t.Run(name, func(t *testing.T) {
			str, _, err := f.FormatCell(context.Background(), view, 0, 0)
			if name == "no formatter" || name == "interface formatter" {
				// A string neither has a formatter registered
				// nor implements the error interface.
				require.ErrorIs(t, err, errors.ErrUnsupported)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "FORMATTED", str)
		})
	}
}

// probeFormatter returns a CellFormatter that appends name to *calls and
// returns result and err. It records the type of the cell value it was
// handed in *seen, so a test can tell whether it got the original view
// or a dereferenced one.
func probeFormatter(name string, calls *[]string, seen *[]string, result string, err error) CellFormatter {
	return CellFormatterFunc(func(_ context.Context, view View, row, col int) (string, bool, error) {
		*calls = append(*calls, name)
		if seen != nil {
			*seen = append(*seen, reflect.TypeOf(view.Cell(row, col)).String())
		}
		return result, false, err
	})
}

type characterizationStringer struct{ s string }

func (c characterizationStringer) String() string { return c.s }

var (
	typeOfStringer           = reflect.TypeFor[interface{ String() string }]()
	typeOfCharStringer       = reflect.TypeFor[characterizationStringer]()
	typeOfCharStringerPtr    = reflect.TypeFor[*characterizationStringer]()
	errCharacterizationFatal = errors.New("fatal")
)

// TestReflectTypeCellFormatter_FormatCell_Contract pins the resolution
// contract of FormatCell: exact type, then interfaces, then kind, each
// falling through on errors.ErrUnsupported and stopping at any other
// error; for a non-nil pointer the same three steps again on the
// pointed-to type with a dereferenced view; finally Default with the
// original view. These cases were written against the map based
// implementation before it moved onto ReflectTypeDispatcher and must
// keep passing unchanged.
func TestReflectTypeCellFormatter_FormatCell_Contract(t *testing.T) {
	value := characterizationStringer{s: "v"}
	type testCase struct {
		name      string
		cell      any
		build     func(calls, seen *[]string) *ReflectTypeCellFormatter
		wantStr   string
		wantErr   error
		wantCalls []string
		wantSeen  []string
	}
	unsupported := errors.ErrUnsupported
	cases := []testCase{
		{
			name: "type formatter wins",
			cell: value,
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithTypeFormatter(typeOfCharStringer, probeFormatter("type", calls, seen, "T", nil)).
					WithInterfaceTypeFormatter(typeOfStringer, probeFormatter("iface", calls, seen, "I", nil)).
					WithKindFormatter(reflect.Struct, probeFormatter("kind", calls, seen, "K", nil)).
					WithDefaultFormatter(probeFormatter("default", calls, seen, "D", nil))
			},
			wantStr:   "T",
			wantCalls: []string{"type"},
		},
		{
			name: "type unsupported falls through to interface",
			cell: value,
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithTypeFormatter(typeOfCharStringer, probeFormatter("type", calls, seen, "", unsupported)).
					WithInterfaceTypeFormatter(typeOfStringer, probeFormatter("iface", calls, seen, "I", nil)).
					WithKindFormatter(reflect.Struct, probeFormatter("kind", calls, seen, "K", nil))
			},
			wantStr:   "I",
			wantCalls: []string{"type", "iface"},
		},
		{
			name: "interface unsupported falls through to kind",
			cell: value,
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithInterfaceTypeFormatter(typeOfStringer, probeFormatter("iface", calls, seen, "", unsupported)).
					WithKindFormatter(reflect.Struct, probeFormatter("kind", calls, seen, "K", nil)).
					WithDefaultFormatter(probeFormatter("default", calls, seen, "D", nil))
			},
			wantStr:   "K",
			wantCalls: []string{"iface", "kind"},
		},
		{
			name: "kind unsupported falls through to default",
			cell: value,
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithKindFormatter(reflect.Struct, probeFormatter("kind", calls, seen, "", unsupported)).
					WithDefaultFormatter(probeFormatter("default", calls, seen, "D", nil))
			},
			wantStr:   "D",
			wantCalls: []string{"kind", "default"},
		},
		{
			name: "all unsupported without default",
			cell: value,
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithTypeFormatter(typeOfCharStringer, probeFormatter("type", calls, seen, "", unsupported)).
					WithInterfaceTypeFormatter(typeOfStringer, probeFormatter("iface", calls, seen, "", unsupported)).
					WithKindFormatter(reflect.Struct, probeFormatter("kind", calls, seen, "", unsupported))
			},
			wantErr:   unsupported,
			wantCalls: []string{"type", "iface", "kind"},
		},
		{
			name: "type error stops",
			cell: value,
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithTypeFormatter(typeOfCharStringer, probeFormatter("type", calls, seen, "", errCharacterizationFatal)).
					WithInterfaceTypeFormatter(typeOfStringer, probeFormatter("iface", calls, seen, "I", nil)).
					WithDefaultFormatter(probeFormatter("default", calls, seen, "D", nil))
			},
			wantErr:   errCharacterizationFatal,
			wantCalls: []string{"type"},
		},
		{
			name: "interface error stops",
			cell: value,
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithInterfaceTypeFormatter(typeOfStringer, probeFormatter("iface", calls, seen, "", errCharacterizationFatal)).
					WithKindFormatter(reflect.Struct, probeFormatter("kind", calls, seen, "K", nil))
			},
			wantErr:   errCharacterizationFatal,
			wantCalls: []string{"iface"},
		},
		{
			name: "kind error stops",
			cell: value,
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithKindFormatter(reflect.Struct, probeFormatter("kind", calls, seen, "", errCharacterizationFatal)).
					WithDefaultFormatter(probeFormatter("default", calls, seen, "D", nil))
			},
			wantErr:   errCharacterizationFatal,
			wantCalls: []string{"kind"},
		},
		{
			name: "pointer type formatter wins before deref",
			cell: &value,
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithTypeFormatter(typeOfCharStringerPtr, probeFormatter("ptrtype", calls, seen, "P", nil)).
					WithTypeFormatter(typeOfCharStringer, probeFormatter("type", calls, seen, "T", nil))
			},
			wantStr:   "P",
			wantCalls: []string{"ptrtype"},
			wantSeen:  []string{"*retable.characterizationStringer"},
		},
		{
			name: "pointer kind formatter wins before deref",
			cell: &value,
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithKindFormatter(reflect.Pointer, probeFormatter("ptrkind", calls, seen, "PK", nil)).
					WithTypeFormatter(typeOfCharStringer, probeFormatter("type", calls, seen, "T", nil))
			},
			wantStr:   "PK",
			wantCalls: []string{"ptrkind"},
			wantSeen:  []string{"*retable.characterizationStringer"},
		},
		{
			name: "non-nil pointer derefs to type with deref view",
			cell: &value,
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithTypeFormatter(typeOfCharStringer, probeFormatter("type", calls, seen, "T", nil))
			},
			wantStr:   "T",
			wantCalls: []string{"type"},
			wantSeen:  []string{"retable.characterizationStringer"},
		},
		{
			name: "deref type unsupported falls through to deref kind",
			cell: &value,
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithTypeFormatter(typeOfCharStringer, probeFormatter("type", calls, seen, "", unsupported)).
					WithKindFormatter(reflect.Struct, probeFormatter("kind", calls, seen, "K", nil))
			},
			wantStr:   "K",
			wantCalls: []string{"type", "kind"},
			wantSeen:  []string{"retable.characterizationStringer", "retable.characterizationStringer"},
		},
		{
			name: "deref error stops",
			cell: &value,
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithKindFormatter(reflect.Struct, probeFormatter("kind", calls, seen, "", errCharacterizationFatal)).
					WithDefaultFormatter(probeFormatter("default", calls, seen, "D", nil))
			},
			wantErr:   errCharacterizationFatal,
			wantCalls: []string{"kind"},
		},
		{
			name: "default gets the original view for a pointer",
			cell: &value,
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithTypeFormatter(typeOfCharStringer, probeFormatter("type", calls, nil, "", unsupported)).
					WithDefaultFormatter(probeFormatter("default", calls, seen, "D", nil))
			},
			wantStr:   "D",
			wantCalls: []string{"type", "default"},
			wantSeen:  []string{"*retable.characterizationStringer"},
		},
		{
			name: "nil pointer skips deref",
			cell: (*characterizationStringer)(nil),
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithTypeFormatter(typeOfCharStringer, probeFormatter("type", calls, seen, "T", nil)).
					WithKindFormatter(reflect.Struct, probeFormatter("kind", calls, seen, "K", nil)).
					WithDefaultFormatter(probeFormatter("default", calls, seen, "D", nil))
			},
			wantStr:   "D",
			wantCalls: []string{"default"},
			wantSeen:  []string{"*retable.characterizationStringer"},
		},
		{
			name: "nil pointer without default is unsupported",
			cell: (*characterizationStringer)(nil),
			build: func(calls, seen *[]string) *ReflectTypeCellFormatter {
				return NewReflectTypeCellFormatter().
					WithTypeFormatter(typeOfCharStringer, probeFormatter("type", calls, seen, "T", nil))
			},
			wantErr: unsupported,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls, seen []string
			f := tc.build(&calls, &seen)
			view := &AnyValuesView{Cols: []string{"A"}, Rows: [][]any{{tc.cell}}}
			str, _, err := f.FormatCell(context.Background(), view, 0, 0)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantStr, str)
			}
			require.Equal(t, tc.wantCalls, calls)
			if tc.wantSeen != nil {
				require.Equal(t, tc.wantSeen, seen)
			}
		})
	}
}

// TestReflectTypeCellFormatter_FormatCell_ContextAndNil covers the two
// early exits: a nil formatter reports errors.ErrUnsupported and a
// cancelled context is returned before any formatter runs.
func TestReflectTypeCellFormatter_FormatCell_ContextAndNil(t *testing.T) {
	view := &AnyValuesView{Cols: []string{"A"}, Rows: [][]any{{"a"}}}

	var nilFormatter *ReflectTypeCellFormatter
	_, _, err := nilFormatter.FormatCell(context.Background(), view, 0, 0)
	require.ErrorIs(t, err, errors.ErrUnsupported)

	var calls []string
	f := NewReflectTypeCellFormatter().
		WithTypeFormatter(reflect.TypeFor[string](), probeFormatter("type", &calls, nil, "T", nil)).
		WithDefaultFormatter(probeFormatter("default", &calls, nil, "D", nil))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = f.FormatCell(ctx, view, 0, 0)
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, calls)
}

// TestReflectTypeCellFormatter_WithDoesNotMutate proves the builder is
// copy-on-write: every With* method leaves the receiver formatting cells
// exactly as before, also when the receiver already has registrations.
func TestReflectTypeCellFormatter_WithDoesNotMutate(t *testing.T) {
	view := &AnyValuesView{Cols: []string{"A", "B"}, Rows: [][]any{{"a", 1}}}
	format := func(f *ReflectTypeCellFormatter, col int) string {
		t.Helper()
		str, _, err := f.FormatCell(context.Background(), view, 0, col)
		if errors.Is(err, errors.ErrUnsupported) {
			return "unsupported"
		}
		require.NoError(t, err)
		return str
	}

	base := NewReflectTypeCellFormatter().
		WithTypeFormatter(reflect.TypeFor[string](), PrintfCellFormatter("type:%v")).
		WithInterfaceTypeFormatter(typeOfStringer, PrintfCellFormatter("iface:%v")).
		WithKindFormatter(reflect.Int, PrintfCellFormatter("kind:%v"))
	require.Equal(t, "type:a", format(base, 0))
	require.Equal(t, "kind:1", format(base, 1))

	_ = base.WithTypeFormatter(reflect.TypeFor[string](), PrintfCellFormatter("changed:%v"))
	_ = base.WithTypeFormatter(reflect.TypeFor[int](), PrintfCellFormatter("changed:%v"))
	_ = base.WithInterfaceTypeFormatter(reflect.TypeFor[error](), PrintfCellFormatter("changed:%v"))
	_ = base.WithKindFormatter(reflect.Int, PrintfCellFormatter("changed:%v"))
	_ = base.WithDefaultFormatter(PrintfCellFormatter("changed:%v"))

	require.Equal(t, "type:a", format(base, 0))
	require.Equal(t, "kind:1", format(base, 1))
	require.Nil(t, base.Default)

	var nilFormatter *ReflectTypeCellFormatter
	built := nilFormatter.WithKindFormatter(reflect.Int, PrintfCellFormatter("kind:%v"))
	require.Equal(t, "kind:1", format(built, 1))
}

// BenchmarkReflectTypeCellFormatter_FormatCell measures the per-cell cost
// of the lookup that csvtable and htmltable run for every typed cell.
func BenchmarkReflectTypeCellFormatter_FormatCell(b *testing.B) {
	value := characterizationStringer{s: "v"}
	noop := CellFormatterFunc(func(context.Context, View, int, int) (string, bool, error) {
		return "x", false, nil
	})
	f := NewReflectTypeCellFormatter().
		WithTypeFormatter(reflect.TypeFor[string](), noop).
		WithTypeFormatter(typeOfCharStringer, noop).
		WithInterfaceTypeFormatter(reflect.TypeFor[error](), noop).
		WithInterfaceTypeFormatter(typeOfStringer, noop)
	cases := []struct {
		name string
		cell any
	}{
		{"type hit", "a"},
		{"interface hit", time.Second},
		{"pointer deref", &value},
		{"miss", 42},
	}
	ctx := context.Background()
	for _, c := range cases {
		view := &AnyValuesView{Cols: []string{"A"}, Rows: [][]any{{c.cell}}}
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _, _ = f.FormatCell(ctx, view, 0, 0)
			}
		})
	}
}

// TestReflectTypeCellFormatter_InterfaceRegistrationOrder: a type that
// implements two registered interfaces is formatted by the interface
// registered first. With the former map of interfaces this was random.
func TestReflectTypeCellFormatter_InterfaceRegistrationOrder(t *testing.T) {
	view := &AnyValuesView{Cols: []string{"A"}, Rows: [][]any{{dispatcherValue{}}}}
	for range 20 {
		f := NewReflectTypeCellFormatter().
			WithInterfaceTypeFormatter(typeOfError, PrintfCellFormatter("error:%T")).
			WithInterfaceTypeFormatter(typeOfFmtStringer, PrintfCellFormatter("stringer:%T"))
		str, _, err := f.FormatCell(context.Background(), view, 0, 0)
		require.NoError(t, err)
		require.Equal(t, "error:retable.dispatcherValue", str)

		f = NewReflectTypeCellFormatter().
			WithInterfaceTypeFormatter(typeOfFmtStringer, PrintfCellFormatter("stringer:%T")).
			WithInterfaceTypeFormatter(typeOfError, PrintfCellFormatter("error:%T"))
		str, _, err = f.FormatCell(context.Background(), view, 0, 0)
		require.NoError(t, err)
		require.Equal(t, "stringer:retable.dispatcherValue", str)
	}
}

// TestReflectTypeCellFormatter_InterfaceField covers struct fields of
// interface type: their cells have the static interface type, so the
// formatter for the dynamic type of the value is tried after nothing
// matched the interface type itself.
func TestReflectTypeCellFormatter_InterfaceField(t *testing.T) {
	type row struct{ A any }
	value := characterizationStringer{s: "v"}
	cases := []struct {
		name    string
		cell    any
		f       *ReflectTypeCellFormatter
		want    string
		wantErr error
	}{
		{
			name: "int held in any reaches the int kind formatter",
			cell: 5,
			f:    NewReflectTypeCellFormatter().WithKindFormatter(reflect.Int, PrintfCellFormatter("int:%v")),
			want: "int:5",
		},
		{
			name: "pointer held in any derefs to the type formatter",
			cell: &value,
			f:    NewReflectTypeCellFormatter().WithTypeFormatter(typeOfCharStringer, PrintfCellFormatter("type:%v")),
			want: "type:v",
		},
		{
			name:    "nil in any stays unsupported",
			cell:    nil,
			f:       NewReflectTypeCellFormatter().WithKindFormatter(reflect.Int, PrintfCellFormatter("int:%v")),
			wantErr: errors.ErrUnsupported,
		},
		{
			name: "formatter for the interface type itself wins",
			cell: 5,
			f: NewReflectTypeCellFormatter().
				WithKindFormatter(reflect.Int, PrintfCellFormatter("int:%v")).
				WithKindFormatter(reflect.Interface, PrintfCellFormatter("interface:%v")),
			want: "interface:5",
		},
		{
			name: "default runs after the dynamic type",
			cell: "s",
			f:    NewReflectTypeCellFormatter().WithDefaultFormatter(PrintfCellFormatter("default:%v")),
			want: "default:s",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			view, err := DefaultStructFieldNaming.NewView("", []row{{A: tc.cell}})
			require.NoError(t, err)
			str, _, err := tc.f.FormatCell(context.Background(), view, 0, 0)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, str)
		})
	}
}

func TestReflectTypeCellFormatter_NilFormatterRemoves(t *testing.T) {
	view := &AnyValuesView{Cols: []string{"A"}, Rows: [][]any{{dispatcherValue{}}}}
	format := func(f *ReflectTypeCellFormatter) error {
		_, _, err := f.FormatCell(context.Background(), view, 0, 0)
		return err
	}
	formatter := PrintfCellFormatter("x")
	require.NoError(t, format(NewReflectTypeCellFormatter().WithTypeFormatter(typeOfDispatcherValue, formatter)))
	require.ErrorIs(t, format(NewReflectTypeCellFormatter().
		WithTypeFormatter(typeOfDispatcherValue, formatter).
		WithTypeFormatter(typeOfDispatcherValue, nil)), errors.ErrUnsupported)
	require.ErrorIs(t, format(NewReflectTypeCellFormatter().
		WithInterfaceTypeFormatter(typeOfError, formatter).
		WithInterfaceTypeFormatter(typeOfError, nil)), errors.ErrUnsupported)
	require.ErrorIs(t, format(NewReflectTypeCellFormatter().
		WithKindFormatter(reflect.Struct, formatter).
		WithKindFormatter(reflect.Struct, nil)), errors.ErrUnsupported)
}

func TestReflectTypeCellFormatter_WithInterfaceTypeFormatterPanics(t *testing.T) {
	require.Panics(t, func() {
		NewReflectTypeCellFormatter().WithInterfaceTypeFormatter(typeOfDispatcherValue, PrintfCellFormatter("x"))
	})
}
