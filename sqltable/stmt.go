package sqltable

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/domonda/go-retable"
)

var _ driver.Stmt = new(stmt)

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
		Source:    view,
		RowOffset: offset,
		RowLimit:  limit,
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

var _ driver.Rows = new(driverRows)

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

// Next is called to populate the next row of data into
// the provided slice. The provided slice will be the same
// size as the Columns() are wide.
//
// Next should return io.EOF when there are no more rows.
//
// The dest should not be written to outside of Next. Care
// should be taken when closing Rows not to modify
// a buffer held in dest.
func (r *driverRows) Next(dest []driver.Value) (err error) {
	if r.rowIndex < 0 || r.rowIndex >= r.view.NumRows() {
		return io.EOF
	}
	for col := range dest {
		dest[col], err = driverValue(r.view.AnyValue(r.rowIndex, col))
		if err != nil {
			return err
		}
	}
	r.rowIndex++
	return nil
}

func driverValue(val any) (driver.Value, error) {
	if valuer, ok := val.(driver.Valuer); ok {
		return valuer.Value()
	}
	if !driver.IsValue(val) {
		return nil, fmt.Errorf("value %#v is not a driver.Value", val)
	}
	return val, nil
}

var queryRegexp = regexp.MustCompile(`^(?:SELECT|select)\s+(\*|(?:[a-zA-Z]\w*|"[a-zA-Z]\w*")(?:\s*,\s*[a-zA-Z]\w*|\s*,\s*"[a-zA-Z]\w*")*)\s+(?:FROM|from)\s+([a-zA-Z][\w.]*|"[a-zA-Z][\w.]*")(?:\s*;)*$`)

func parseQuery(query string) (columns []string, table string, offset, limit int, err error) {
	query = strings.TrimSpace(query)
	m := queryRegexp.FindStringSubmatch(query)
	if len(m) != 3 {
		return nil, "", 0, 0, fmt.Errorf("invalid query %q", query)
	}
	columns = strings.Split(m[1], ",")
	for i := range columns {
		columns[i] = unquote(strings.TrimSpace(columns[i]))
	}
	table = unquote(m[2])

	return columns, table, offset, limit, nil
}

func unquote(str string) string {
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		return str[1 : len(str)-1]
	}
	return str
}
