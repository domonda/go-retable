package retable

import "reflect"

func SingleColView[T any](column string, rows []T) View {
	return &singleColsView[T]{
		columns:        []string{column},
		rows:           rows,
		isReflectValue: reflect.TypeOf(rows).Elem() == reflect.TypeOf(reflect.Value{}),
	}
}

type singleColsView[T any] struct {
	columns        []string
	rows           []T
	isReflectValue bool
}

func (s *singleColsView[T]) Title() string {
	return s.columns[0]
}

func (s *singleColsView[T]) Columns() []string {
	return s.columns
}

func (s *singleColsView[T]) NumRows() int {
	return len(s.rows)
}

func (s *singleColsView[T]) AnyValue(row, col int) any {
	if row < 0 || row >= len(s.rows) || col != 0 {
		return nil
	}
	if !s.isReflectValue {
		return s.rows[row]
	}
	// Lack of generic type specialization requires
	// dynamic type assertion
	v := any(s.rows[row]).(reflect.Value)
	if !v.IsValid() {
		return nil
	}
	return v.Interface()
}

func (s *singleColsView[T]) ReflectValue(row, col int) reflect.Value {
	if row < 0 || row >= len(s.rows) || col != 0 {
		return reflect.Value{}
	}
	if !s.isReflectValue {
		return reflect.ValueOf(s.rows[row])
	}
	// Lack of generic type specialization requires
	// dynamic type assertion
	return any(s.rows[row]).(reflect.Value)
}
