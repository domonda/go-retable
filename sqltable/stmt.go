package sqltable

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"slices"

	"github.com/domonda/go-retable"
)

type stmt struct {
	view retable.View
}

func newStmt(views map[string]retable.View, query string) (*stmt, error) {
	queryColumns, table, offset, limit, err := parseQuery(query)
	if err != nil {
		return nil, err
	}
	view := views[table]
	if view == nil {
		return nil, fmt.Errorf("view %q not found", table)
	}
	sourceColumns := view.Columns()
	columnsIdentical := slices.Equal(queryColumns, sourceColumns)
	if columnsIdentical && offset == 0 && limit == 0 {
		return &stmt{view: view}, nil
	}
	filtered := &retable.FilteredView{
		Source: view,
		Offset: offset,
		Limit:  limit,
	}
	if !columnsIdentical {
		filtered.ColumnMapping = make([]int, len(queryColumns))
		for i, queryColumn := range queryColumns {
			filtered.ColumnMapping[i] = slices.Index(sourceColumns, queryColumn)
			if filtered.ColumnMapping[i] == -1 {
				return nil, fmt.Errorf("column %q not found", queryColumn)
			}
		}
	}
	return &stmt{view: filtered}, nil
}

func (s *stmt) Close() error {
	return nil
}

func (s *stmt) NumInput() int {
	return 0
}

func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, errors.New("Exec not implemented")
}

func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	return &driverRows{view: s.view}, nil
}

type driverRows struct {
	view     retable.View
	rowIndex int
}

func (r *driverRows) Columns() []string {
	return r.view.Columns()
}

func (r *driverRows) Close() error {
	r.rowIndex = -1
	return nil
}

func (r *driverRows) Next(dest []driver.Value) error {
	if r.rowIndex < 0 || r.rowIndex >= r.view.NumRows() {
		return io.EOF
	}
	row, err := r.view.ReflectRow(r.rowIndex)
	if err != nil {
		return err
	}
	if len(row) != len(dest) {
		panic("Next: len(row) != len(dest)")
	}
	for i, v := range row {
		dest[i], err = driverValue(v)
		if err != nil {
			return err
		}
	}
	r.rowIndex++
	return nil
}

func driverValue(v reflect.Value) (driver.Value, error) {
	if valuer, ok := v.Interface().(driver.Valuer); ok {
		return valuer.Value()
	}
	// Known conversations to a driver.Value type
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return v.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u := v.Uint()
		if u > math.MaxInt64 {
			return nil, fmt.Errorf("uint %d does not fit int64", u)
		}
		return int64(u), nil
	case reflect.Float32:
		return v.Float(), nil
	}
	// v either has a valid driver.Value type
	// or no known conversion which might fail later
	return v.Interface(), nil
}

func parseQuery(query string) (columns []string, table string, offset, limit int, err error) {
	panic("TODO")
}
