package sqltable

import (
	"context"
	"database/sql"
	"database/sql/driver"

	"github.com/domonda/go-retable"
)

// NewViewsDB creates a virtual database/sql.DB that allows querying multiple
// named in-memory retable.View instances using SQL SELECT statements.
//
// The returned *sql.DB implements a lightweight SQL driver that emulates query
// execution without requiring an actual database connection. Each view in the
// map becomes a queryable "table" accessible by its map key name.
//
// This function is useful for:
//   - Testing database code with in-memory fixtures
//   - Providing SQL-like query interfaces over application data
//   - Converting between retable views and database/sql patterns
//   - Building data pipelines that combine SQL and in-memory processing
//
// Supported SQL syntax (basic SELECT only):
//   - SELECT * FROM tablename
//   - SELECT col1, col2 FROM tablename
//   - Column names can be quoted with double quotes
//
// Note: The returned DB does not support INSERT, UPDATE, DELETE, JOIN, WHERE,
// ORDER BY, or other complex SQL features. It provides simple column projection
// and full table scans only.
//
// Parameters:
//   - views: A map of table names to retable.View instances
//
// Returns:
//   - A *sql.DB that can be used with standard database/sql operations
//
// Example:
//
//	usersView := &retable.AnyValuesView{
//		Cols: []string{"id", "name", "email"},
//		Rows: [][]any{
//			{1, "Alice", "alice@example.com"},
//			{2, "Bob", "bob@example.com"},
//		},
//	}
//
//	ordersView := &retable.AnyValuesView{
//		Cols: []string{"id", "user_id", "total"},
//		Rows: [][]any{
//			{101, 1, 49.99},
//			{102, 2, 89.50},
//		},
//	}
//
//	db := sqltable.NewViewsDB(map[string]retable.View{
//		"users":  usersView,
//		"orders": ordersView,
//	})
//	defer db.Close()
//
//	// Query different tables
//	rows, err := db.Query("SELECT name, email FROM users")
//	// ... process rows
//
//	rows, err = db.Query("SELECT * FROM orders")
//	// ... process rows
func NewViewsDB(views map[string]retable.View) *sql.DB {
	return sql.OpenDB(database{views: views})
}

// NewViewDB creates a virtual database/sql.DB for querying a single named
// in-memory retable.View using SQL SELECT statements.
//
// This is a convenience wrapper around NewViewsDB for the common case of
// exposing a single view as a queryable table. It's equivalent to calling
// NewViewsDB with a single-entry map.
//
// The created database supports the same SQL syntax as NewViewsDB, limited
// to basic SELECT statements with column projection.
//
// Parameters:
//   - viewName: The table name to use in SQL queries
//   - view: The retable.View to expose as the table
//
// Returns:
//   - A *sql.DB that can be queried using the specified viewName
//
// Example:
//
//	view := &retable.AnyValuesView{
//		Cols: []string{"id", "name", "status"},
//		Rows: [][]any{
//			{1, "Task A", "done"},
//			{2, "Task B", "pending"},
//		},
//	}
//
//	db := sqltable.NewViewDB("tasks", view)
//	defer db.Close()
//
//	// Query using the specified table name
//	var name, status string
//	err := db.QueryRow("SELECT name, status FROM tasks").Scan(&name, &status)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("%s: %s\n", name, status)
func NewViewDB(viewName string, view retable.View) *sql.DB {
	return NewViewsDB(map[string]retable.View{
		viewName: view,
	})
}

// database implements the database/sql/driver interfaces to provide a virtual
// SQL database backed by in-memory retable.View instances.
//
// It implements driver.Driver, driver.Connector, driver.Conn, and driver.Tx
// to satisfy the database/sql contract while keeping all data in memory.
// The implementation is read-only and does not support transactions in a
// meaningful way (Begin/Commit/Rollback are no-ops).
type database struct {
	views map[string]retable.View
}

// Connect implements driver.Connector.
//
// It returns the database itself as a connection since all state is contained
// in the views map and there's no actual connection to establish.
//
// Parameters:
//   - ctx: Context (unused, no connection is established)
//
// Returns:
//   - The database as a driver.Conn
//   - Always returns nil error
func (c database) Connect(context.Context) (driver.Conn, error) {
	return c, nil
}

// Driver implements driver.Connector.
//
// It returns the database itself as a driver since the same struct implements
// both the driver and connector interfaces.
//
// Returns:
//   - The database as a driver.Driver
func (c database) Driver() driver.Driver {
	return c
}

// Open implements driver.Driver.
//
// It returns the database itself as a connection. The name parameter is ignored
// since this is an in-memory driver with no external resources to open.
//
// Parameters:
//   - name: Data source name (unused)
//
// Returns:
//   - The database as a driver.Conn
//   - Always returns nil error
func (c database) Open(string) (driver.Conn, error) {
	return c, nil
}

// OpenConnector implements driver.DriverContext.
//
// It returns the database itself as a connector. The name parameter is ignored
// since this is an in-memory driver.
//
// Parameters:
//   - name: Data source name (unused)
//
// Returns:
//   - The database as a driver.Connector
//   - Always returns nil error
func (c database) OpenConnector(string) (driver.Connector, error) {
	return c, nil
}

// Prepare implements driver.Conn.
//
// It parses a SQL query and creates a prepared statement that can execute the
// query against the registered views. The query is parsed immediately to validate
// syntax and resolve table/column references.
//
// Only SELECT queries are supported with the following grammar:
//   - SELECT * FROM tablename
//   - SELECT col1, col2, ... FROM tablename
//   - Column and table names can be double-quoted
//
// Parameters:
//   - query: The SQL SELECT query to prepare
//
// Returns:
//   - A driver.Stmt that can execute the query
//   - An error if the query is invalid or references unknown tables/columns
func (c database) Prepare(query string) (driver.Stmt, error) {
	return newStmt(c.views, query)
}

// Close implements driver.Conn.
//
// It's a no-op since there are no resources to release in this in-memory driver.
//
// Returns:
//   - Always returns nil
func (database) Close() error {
	return nil
}

// Begin implements driver.Conn.
//
// It returns a transaction handle, but transactions are not meaningfully supported
// by this in-memory read-only driver. Commit and Rollback are no-ops.
//
// Returns:
//   - The database as a driver.Tx
//   - Always returns nil error
func (c database) Begin() (driver.Tx, error) {
	return c, nil
}

// Commit implements driver.Tx.
//
// It's a no-op since this in-memory driver is read-only and doesn't support
// transactional semantics.
//
// Returns:
//   - Always returns nil
func (database) Commit() error {
	return nil
}

// Rollback implements driver.Tx.
//
// It's a no-op since this in-memory driver is read-only and doesn't support
// transactional semantics.
//
// Returns:
//   - Always returns nil
func (database) Rollback() error {
	return nil
}

// func (c conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
// 	panic("TODO")
// }
