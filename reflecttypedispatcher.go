package retable

import (
	"fmt"
	"iter"
	"maps"
	"reflect"
	"slices"
)

// ReflectTypeDispatcher maps reflect types to values of type F and yields
// the values registered for a type in a fixed resolution order.
// It holds the lookup that ReflectTypeCellFormatter and the exceltable
// writer share, with F being the formatter type of each.
//
// Resolution order for a type T, see Candidates:
//
//	exact type T
//	interfaces implemented by T, in registration order
//	kind of T
//	if T is a pointer: the same three steps for T.Elem(), marked as deref
//
// A ReflectTypeDispatcher is immutable: every With* method returns a
// modified copy and leaves the receiver unchanged, so a dispatcher can
// be shared between goroutines. The With* methods accept a nil receiver
// and start from an empty dispatcher, and Candidates of a nil
// dispatcher yields nothing.
type ReflectTypeDispatcher[F any] struct {
	types      map[reflect.Type]F
	interfaces []reflectInterfaceEntry[F]
	kinds      map[reflect.Kind]F
}

// reflectInterfaceEntry is one registration of an interface type.
// A slice of these keeps the registration order, which a map would lose.
type reflectInterfaceEntry[F any] struct {
	typ   reflect.Type
	value F
}

// WithType returns a copy of the dispatcher with value registered
// for the exact type typ, replacing an earlier registration.
// A nil value removes the registration for typ.
func (d *ReflectTypeDispatcher[F]) WithType(typ reflect.Type, value F) *ReflectTypeDispatcher[F] {
	mod := d.clone()
	if isNilValue(value) {
		delete(mod.types, typ)
		return mod
	}
	if mod.types == nil {
		mod.types = make(map[reflect.Type]F)
	}
	mod.types[typ] = value
	return mod
}

// WithInterfaceType returns a copy of the dispatcher with value registered
// for every type that implements the interface type typ.
//
// Interfaces are tried in the order they were first registered.
// Registering an interface again replaces its value and keeps its position.
// A nil value removes the registration for typ.
//
// WithInterfaceType panics if typ is nil or not an interface type,
// because that is a programming error that would otherwise only
// surface as a panic of reflect.Type.Implements while writing a table.
func (d *ReflectTypeDispatcher[F]) WithInterfaceType(typ reflect.Type, value F) *ReflectTypeDispatcher[F] {
	if typ == nil || typ.Kind() != reflect.Interface {
		panic(fmt.Sprintf("retable: %v is not an interface type", typ))
	}
	mod := d.clone()
	index := slices.IndexFunc(mod.interfaces, func(e reflectInterfaceEntry[F]) bool { return e.typ == typ })
	switch {
	case isNilValue(value) && index >= 0:
		mod.interfaces = slices.Delete(mod.interfaces, index, index+1)
	case isNilValue(value):
		// Nothing registered to remove
	case index >= 0:
		mod.interfaces[index].value = value
	default:
		mod.interfaces = append(mod.interfaces, reflectInterfaceEntry[F]{typ: typ, value: value})
	}
	return mod
}

// WithKind returns a copy of the dispatcher with value registered
// for all types of the passed kind, replacing an earlier registration.
// A nil value removes the registration for kind.
func (d *ReflectTypeDispatcher[F]) WithKind(kind reflect.Kind, value F) *ReflectTypeDispatcher[F] {
	mod := d.clone()
	if isNilValue(value) {
		delete(mod.kinds, kind)
		return mod
	}
	if mod.kinds == nil {
		mod.kinds = make(map[reflect.Kind]F)
	}
	mod.kinds[kind] = value
	return mod
}

// Candidates yields the values registered for typ in resolution order:
// the exact type, the implemented interfaces in registration order,
// and the kind of typ. If typ is a pointer type, the same three steps
// follow for typ.Elem() with deref set to true.
//
// A caller formatting a value stops at the first candidate that handles
// it. For candidates with deref set it has to pass the dereferenced value
// (for example a DerefView) and must skip them for a nil pointer.
func (d *ReflectTypeDispatcher[F]) Candidates(typ reflect.Type) iter.Seq2[F, bool] {
	return func(yield func(F, bool) bool) {
		if d == nil || typ == nil {
			return
		}
		if !d.yieldCandidates(typ, false, yield) {
			return
		}
		if typ.Kind() == reflect.Pointer {
			d.yieldCandidates(typ.Elem(), true, yield)
		}
	}
}

// yieldCandidates yields the type, interface and kind registrations
// for typ and reports whether the caller wants more.
func (d *ReflectTypeDispatcher[F]) yieldCandidates(typ reflect.Type, deref bool, yield func(F, bool) bool) bool {
	if value, ok := d.types[typ]; ok {
		if !yield(value, deref) {
			return false
		}
	}
	for _, e := range d.interfaces {
		if typ.Implements(e.typ) {
			if !yield(e.value, deref) {
				return false
			}
		}
	}
	if value, ok := d.kinds[typ.Kind()]; ok {
		if !yield(value, deref) {
			return false
		}
	}
	return true
}

// clone returns a copy of d that shares no mutable state with it,
// or an empty dispatcher if d is nil.
func (d *ReflectTypeDispatcher[F]) clone() *ReflectTypeDispatcher[F] {
	if d == nil {
		return new(ReflectTypeDispatcher[F])
	}
	return &ReflectTypeDispatcher[F]{
		types:      maps.Clone(d.types),
		interfaces: slices.Clone(d.interfaces),
		kinds:      maps.Clone(d.kinds),
	}
}

// isNilValue reports whether value is the nil value of a nillable type,
// for example an untyped nil passed as an interface, func or pointer F.
// An interface holding a typed nil pointer is not nil.
func isNilValue[F any](value F) bool {
	v := reflect.ValueOf(&value).Elem()
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer, reflect.Func, reflect.Map, reflect.Slice, reflect.Chan, reflect.UnsafePointer:
		return v.IsNil()
	}
	return false
}
