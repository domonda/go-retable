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

// stmt implements driver.Stmt to provide prepared statement execution over
// in-memory retable.View instances. It represents a parsed SQL query that has
// been validated and optimized for execution.
//
// The stmt parses SELECT queries at preparation time, resolves table and column
// references, and creates an optimized execution plan using retable.FilteredView
// when column projection is needed.
type stmt struct {
	view retable.View
}

// newStmt creates a new prepared statement by parsing a SQL query and resolving
// it against the provided views map.
//
// The function performs query parsing, table lookup, column resolution, and
// optimization. If the query selects all columns in their original order with
// no offset/limit, the source view is used directly. Otherwise, a FilteredView
// is created to handle column projection and row slicing.
//
// Query grammar:
//   - SELECT * FROM tablename
//   - SELECT col1, col2 FROM tablename
//   - Column and table names can be quoted with double quotes
//   - Trailing semicolons are allowed
//
// Parameters:
//   - views: Map of table names to retable.View instances
//   - query: The SQL SELECT query to parse and prepare
//
// Returns:
//   - A prepared stmt ready for execution
//   - An error if the query is invalid or references unknown tables/columns
//
// Example:
//
//	views := map[string]retable.View{
//		"users": userView,
//	}
//	stmt, err := newStmt(views, "SELECT name, email FROM users")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer stmt.Close()
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

// Close implements driver.Stmt.
//
// It releases resources associated with the statement. For this in-memory
// driver, it's a no-op since views don't require cleanup.
//
// Returns:
//   - Always returns nil
func (s *stmt) Close() error {
	return nil
}

// NumInput implements driver.Stmt.
//
// It returns the number of placeholder parameters in the query. This driver
// does not support parameterized queries, so it always returns 0.
//
// Returns:
//   - Always returns 0
func (s *stmt) NumInput() int {
	return 0
}

// Exec implements driver.Stmt.
//
// It attempts to execute a non-query statement (INSERT, UPDATE, DELETE). This
// driver is read-only and only supports SELECT queries, so Exec always fails.
//
// Parameters:
//   - args: Query arguments (unused)
//
// Returns:
//   - Always returns nil result
//   - Always returns an error indicating Exec is not implemented
func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, errors.New("Exec not implemented")
}

// Query implements driver.Stmt.
//
// It executes the prepared SELECT query and returns a driver.Rows for iterating
// over the result set. The query is executed immediately against the in-memory
// view and returns a rows iterator.
//
// Parameters:
//   - args: Query arguments (unused, parameterized queries not supported)
//
// Returns:
//   - A driver.Rows for iterating over query results
//   - Always returns nil error (query execution cannot fail for in-memory views)
func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	return &driverRows{view: s.view}, nil
}

var _ driver.Rows = new(driverRows)

// driverRows implements driver.Rows to provide iteration over query results
// from an in-memory retable.View.
//
// It maintains a row index and provides the standard driver.Rows interface
// for the database/sql package to consume query results.
type driverRows struct {
	view     retable.View
	rowIndex int
}

// Columns implements driver.Rows.
//
// It returns the column names from the underlying view in the order they appear
// in the query result.
//
// Returns:
//   - A slice of column names
func (r *driverRows) Columns() []string {
	return r.view.Columns()
}

// Close implements driver.Rows.
//
// It closes the rows iterator and releases resources. For this in-memory driver,
// it simply marks the iterator as closed by setting rowIndex to -1.
//
// Returns:
//   - Always returns nil
func (r *driverRows) Close() error {
	r.rowIndex = -1
	return nil
}

// Next implements driver.Rows.
//
// It populates the next row of data into the provided destination slice.
// The dest slice will be exactly as wide as the number of columns returned
// by Columns().
//
// Next returns io.EOF when there are no more rows. The dest slice should not
// be written to outside of Next, and care should be taken when closing Rows
// not to modify any buffer held in dest.
//
// Parameters:
//   - dest: Slice of driver.Value to populate with the next row's column values
//
// Returns:
//   - nil if a row was successfully read
//   - io.EOF if there are no more rows
//   - An error if value conversion fails
func (r *driverRows) Next(dest []driver.Value) (err error) {
	if r.rowIndex < 0 || r.rowIndex >= r.view.NumRows() {
		return io.EOF
	}
	for col := range dest {
		dest[col], err = driverValue(r.view.Cell(r.rowIndex, col))
		if err != nil {
			return err
		}
	}
	r.rowIndex++
	return nil
}

// driverValue converts a cell value from a retable.View into a driver.Value.
//
// It handles driver.Valuer types by calling their Value() method, and validates
// that other types are compatible with the driver.Value type constraints.
//
// Parameters:
//   - val: The cell value to convert
//
// Returns:
//   - A driver.Value suitable for returning to database/sql
//   - An error if the value cannot be converted to a driver.Value
func driverValue(val any) (driver.Value, error) {
	if valuer, ok := val.(driver.Valuer); ok {
		return valuer.Value()
	}
	if !driver.IsValue(val) {
		return nil, fmt.Errorf("value %#v is not a driver.Value", val)
	}
	return val, nil
}

// queryRegexp is the regular expression used to parse SELECT queries.
//
// It matches queries with the following format:
//   - SELECT (columns or *) FROM tablename [;]
//
// Supported patterns:
//   - Column names: identifier, "quoted identifier"
//   - Table names: identifier, identifier.schema, "quoted identifier"
//   - Whitespace is flexible between keywords
//   - Optional trailing semicolon
var queryRegexp = regexp.MustCompile(`^(?:SELECT|select)\s+(\*|(?:[a-zA-Z]\w*|"[a-zA-Z]\w*")(?:\s*,\s*[a-zA-Z]\w*|\s*,\s*"[a-zA-Z]\w*")*)\s+(?:FROM|from)\s+([a-zA-Z][\w.]*|"[a-zA-Z][\w.]*")(?:\s*;)*$`)

// parseQuery parses a SQL SELECT query and extracts its components.
//
// The parser uses a regular expression to validate query syntax and extract
// the column list and table name. It supports basic SELECT queries with
// column projection and wildcard selection.
//
// Supported query formats:
//   - SELECT * FROM tablename
//   - SELECT col1, col2 FROM tablename
//   - SELECT "quoted col" FROM "quoted.table"
//
// Currently, offset and limit are not parsed from the query (they are reserved
// for future enhancement) and always return 0.
//
// Parameters:
//   - query: The SQL query string to parse
//
// Returns:
//   - columns: Slice of column names (unquoted)
//   - table: Table name (unquoted)
//   - offset: Row offset (always 0, reserved for future use)
//   - limit: Row limit (always 0, reserved for future use)
//   - err: Error if query syntax is invalid
//
// Example:
//
//	cols, table, _, _, err := parseQuery(`SELECT name, age FROM users`)
//	// cols = []string{"name", "age"}
//	// table = "users"
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
