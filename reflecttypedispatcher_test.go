package retable

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type dispatcherValue struct{ n int }

func (dispatcherValue) String() string { return "dispatcherValue" }
func (dispatcherValue) Error() string  { return "dispatcherValue" }

var (
	typeOfDispatcherValue    = reflect.TypeFor[dispatcherValue]()
	typeOfDispatcherValuePtr = reflect.TypeFor[*dispatcherValue]()
	typeOfFmtStringer        = reflect.TypeFor[fmt.Stringer]()
)

type candidate struct {
	Value string
	Deref bool
}

func collectCandidates(d *ReflectTypeDispatcher[string], typ reflect.Type) []candidate {
	var result []candidate
	for value, deref := range d.Candidates(typ) {
		result = append(result, candidate{value, deref})
	}
	return result
}

// TestReflectTypeDispatcher_Candidates pins the resolution order that
// ReflectTypeCellFormatter and the exceltable writer rely on.
func TestReflectTypeDispatcher_Candidates(t *testing.T) {
	d := (*ReflectTypeDispatcher[string])(nil).
		WithKind(reflect.Struct, "kind struct").
		WithKind(reflect.Pointer, "kind pointer").
		WithInterfaceType(typeOfError, "iface error").
		WithInterfaceType(typeOfFmtStringer, "iface stringer").
		WithType(typeOfDispatcherValue, "type value").
		WithType(typeOfDispatcherValuePtr, "type pointer")

	cases := []struct {
		name string
		typ  reflect.Type
		want []candidate
	}{
		{
			name: "value type: type, interfaces in registration order, kind",
			typ:  typeOfDispatcherValue,
			want: []candidate{
				{"type value", false},
				{"iface error", false},
				{"iface stringer", false},
				{"kind struct", false},
			},
		},
		{
			name: "pointer type: own candidates, then the pointed-to type with deref",
			typ:  typeOfDispatcherValuePtr,
			want: []candidate{
				{"type pointer", false},
				{"iface error", false},
				{"iface stringer", false},
				{"kind pointer", false},
				{"type value", true},
				{"iface error", true},
				{"iface stringer", true},
				{"kind struct", true},
			},
		},
		{
			name: "unregistered type",
			typ:  reflect.TypeFor[int](),
			want: nil,
		},
		{
			name: "nil type",
			typ:  nil,
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, collectCandidates(d, tc.typ))
		})
	}
}

func TestReflectTypeDispatcher_CandidatesStopEarly(t *testing.T) {
	d := (*ReflectTypeDispatcher[string])(nil).
		WithType(typeOfDispatcherValue, "type").
		WithInterfaceType(typeOfError, "iface").
		WithKind(reflect.Struct, "kind")
	var got []string
	for value := range d.Candidates(typeOfDispatcherValue) {
		got = append(got, value)
		break
	}
	require.Equal(t, []string{"type"}, got, "breaking out of the loop must stop the iteration")

	// Breaking at the first deref candidate stops the second pass as well
	got = nil
	for value, deref := range d.Candidates(typeOfDispatcherValuePtr) {
		got = append(got, value)
		if deref {
			break
		}
	}
	require.Equal(t, []string{"iface", "type"}, got)
}

func TestReflectTypeDispatcher_NilReceiver(t *testing.T) {
	var d *ReflectTypeDispatcher[string]
	require.Nil(t, collectCandidates(d, typeOfDispatcherValue))
	require.Equal(t,
		[]candidate{{"type", false}},
		collectCandidates(d.WithType(typeOfDispatcherValue, "type"), typeOfDispatcherValue),
	)
}

func TestReflectTypeDispatcher_ReRegisterInterfaceKeepsPosition(t *testing.T) {
	d := (*ReflectTypeDispatcher[string])(nil).
		WithInterfaceType(typeOfError, "error 1").
		WithInterfaceType(typeOfFmtStringer, "stringer").
		WithInterfaceType(typeOfError, "error 2")
	require.Equal(t,
		[]candidate{{"error 2", false}, {"stringer", false}},
		collectCandidates(d, typeOfDispatcherValue),
	)
}

func TestReflectTypeDispatcher_NilValueRemoves(t *testing.T) {
	type formatter func() string
	f := formatter(func() string { return "f" })
	d := (*ReflectTypeDispatcher[formatter])(nil).
		WithType(typeOfDispatcherValue, f).
		WithInterfaceType(typeOfError, f).
		WithInterfaceType(typeOfFmtStringer, f).
		WithKind(reflect.Struct, f)
	count := func(d *ReflectTypeDispatcher[formatter]) int {
		n := 0
		for range d.Candidates(typeOfDispatcherValue) {
			n++
		}
		return n
	}
	require.Equal(t, 4, count(d))
	require.Equal(t, 3, count(d.WithType(typeOfDispatcherValue, nil)))
	require.Equal(t, 3, count(d.WithInterfaceType(typeOfError, nil)))
	require.Equal(t, 3, count(d.WithKind(reflect.Struct, nil)))
	require.Equal(t, 4, count(d), "removing must not modify the receiver")

	// Removing what was never registered is a no-op
	var empty *ReflectTypeDispatcher[formatter]
	require.Equal(t, 0, count(empty.WithType(typeOfDispatcherValue, nil)))
	require.Equal(t, 0, count(empty.WithInterfaceType(typeOfError, nil)))
	require.Equal(t, 0, count(empty.WithKind(reflect.Struct, nil)))

	// The zero value of a non-nillable F is a value, not a removal
	ds := (*ReflectTypeDispatcher[string])(nil).
		WithInterfaceType(typeOfError, "error").
		WithInterfaceType(typeOfFmtStringer, "stringer")
	require.Equal(t,
		[]candidate{{"", false}, {"stringer", false}},
		collectCandidates(ds.WithInterfaceType(typeOfError, ""), typeOfDispatcherValue),
	)

	// Order of the remaining interfaces is kept after a removal
	df := (*ReflectTypeDispatcher[formatter])(nil).
		WithInterfaceType(typeOfError, f).
		WithInterfaceType(typeOfFmtStringer, formatter(func() string { return "stringer" })).
		WithInterfaceType(typeOfError, nil)
	var got []string
	for value := range df.Candidates(typeOfDispatcherValue) {
		got = append(got, value())
	}
	require.Equal(t, []string{"stringer"}, got)
}

func TestReflectTypeDispatcher_WithInterfaceTypePanics(t *testing.T) {
	var d *ReflectTypeDispatcher[string]
	require.PanicsWithValue(t,
		"retable: retable.dispatcherValue is not an interface type",
		func() { d.WithInterfaceType(typeOfDispatcherValue, "x") },
	)
	require.PanicsWithValue(t,
		"retable: <nil> is not an interface type",
		func() { d.WithInterfaceType(nil, "x") },
	)
	require.NotPanics(t, func() { d.WithInterfaceType(typeOfError, "x") })
}

// TestReflectTypeDispatcher_WithDoesNotMutate proves copy-on-write for all
// three registration kinds, including replacing an existing interface entry,
// which edits the copied slice in place.
func TestReflectTypeDispatcher_WithDoesNotMutate(t *testing.T) {
	base := (*ReflectTypeDispatcher[string])(nil).
		WithType(typeOfDispatcherValue, "type").
		WithInterfaceType(typeOfError, "iface").
		WithKind(reflect.Struct, "kind")
	want := collectCandidates(base, typeOfDispatcherValue)

	_ = base.WithType(typeOfDispatcherValue, "changed")
	_ = base.WithInterfaceType(typeOfError, "changed")
	_ = base.WithInterfaceType(typeOfFmtStringer, "added")
	_ = base.WithKind(reflect.Struct, "changed")

	require.Equal(t, want, collectCandidates(base, typeOfDispatcherValue))
}

// TestReflectTypeDispatcher_NilFuncInInterfaceRemoves: a nil func converted
// to an interface F, like a nil CellFormatterFunc passed as CellFormatter by
// the With*FormatterFunc methods of csvtable and htmltable, removes the
// registration instead of being stored and panicking when called.
func TestReflectTypeDispatcher_NilFuncInInterfaceRemoves(t *testing.T) {
	var nilFunc CellFormatterFunc
	d := (*ReflectTypeDispatcher[CellFormatter])(nil).
		WithType(typeOfDispatcherValue, PrintfCellFormatter("x")).
		WithType(typeOfDispatcherValue, nilFunc)
	for range d.Candidates(typeOfDispatcherValue) {
		t.Fatal("nil func must remove the registration")
	}
	// A typed nil pointer in the interface is a value: its methods may handle nil
	var nilFormatter *ReflectTypeCellFormatter
	d = d.WithType(typeOfDispatcherValue, nilFormatter)
	n := 0
	for range d.Candidates(typeOfDispatcherValue) {
		n++
	}
	require.Equal(t, 1, n)
}
