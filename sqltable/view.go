// Package sqltable provides a virtual SQL database driver that allows querying
// in-memory retable.View data structures using database/sql and SQL-like queries.
//
// This package implements the database/sql/driver interfaces to create a lightweight
// SQL query emulation layer over retable views without requiring an actual database.
// It enables SQL-based data access patterns while keeping data entirely in memory.
//
// Basic usage example:
//
//	// Create a view with data
//	view := &retable.AnyValuesView{
//		Cols: []string{"id", "name", "age"},
//		Rows: [][]any{
//			{1, "Alice", 30},
//			{2, "Bob", 25},
//		},
//	}
//
//	// Create a virtual database
//	db := sqltable.NewViewDB("users", view)
//	defer db.Close()
//
//	// Query using standard database/sql
//	rows, err := db.Query("SELECT name, age FROM users")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer rows.Close()
//
//	for rows.Next() {
//		var name string
//		var age int
//		rows.Scan(&name, &age)
//		fmt.Printf("%s: %d\n", name, age)
//	}
//
// The package supports basic SELECT queries with column selection and can handle
// multiple named views in a single database instance. It integrates seamlessly
// with standard database/sql APIs while providing fast, in-memory query execution.
package sqltable

import (
	"context"
	"database/sql"
	"slices"

	"github.com/domonda/go-retable"
)

// ScanRowsAsView scans all rows from a Rows result set and converts them into
// a retable.AnyValuesView containing the column names and row data.
//
// This function is useful for converting database query results into in-memory
// views that can be manipulated using retable operations or queried again using
// the sqltable driver.
//
// The function respects context cancellation and will return early if ctx is
// cancelled during row iteration. All byte slice values are cloned to ensure
// data remains valid after the underlying rows are closed.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - rows: The Rows interface (typically *sql.Rows) to scan from
//
// Returns:
//   - A populated AnyValuesView with all scanned data
//   - An error if column retrieval, scanning, or iteration fails
//
// Example:
//
//	ctx := context.Background()
//	rows, err := db.QueryContext(ctx, "SELECT id, name FROM users")
//	if err != nil {
//		return err
//	}
//
//	view, err := sqltable.ScanRowsAsView(ctx, rows)
//	if err != nil {
//		return err
//	}
//
//	// Now use the view with retable operations
//	fmt.Printf("Retrieved %d rows with columns: %v\n", view.NumRows(), view.Columns())
func ScanRowsAsView(ctx context.Context, rows Rows) (*retable.AnyValuesView, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	view := &retable.AnyValuesView{Cols: columns}

	defer rows.Close()
	for rows.Next() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		scannedValues := make([]any, len(columns))
		valueScanners := make([]any, len(columns))
		for i := range valueScanners {
			valueScanners[i] = valueScanner{&scannedValues[i]}
		}
		err = rows.Scan(valueScanners...)
		if err != nil {
			return view, err
		}
		view.Rows = append(view.Rows, scannedValues)
	}
	return view, rows.Err()
}

var (
	_ sql.Scanner = new(valueScanner)
	// _ sql.Scanner = new(reflectValueScanner)
)

// valueScanner is an internal sql.Scanner implementation that scans database
// values into an any pointer destination. It handles byte slice cloning to
// ensure scanned data remains valid after the scan operation completes.
//
// This type is used internally by ScanRowsAsView to capture arbitrary typed
// values from database rows without knowing their specific types in advance.
type valueScanner struct {
	dest *any
}

// Scan implements the database/sql.Scanner interface.
//
// It scans the source value into the destination any pointer. Byte slices are
// cloned to prevent data corruption when the underlying row buffer is reused.
// All other value types are assigned directly.
//
// Parameters:
//   - src: The source value from the database driver
//
// Returns:
//   - Always returns nil (no scanning errors are possible)
func (s valueScanner) Scan(src any) error {
	if b, ok := src.([]byte); ok {
		// Copy bytes because they won't be valid after this method call
		src = slices.Clone(b)
	}
	*s.dest = src
	return nil
}

// type reflectValueScanner struct {
// 	dest *reflect.Value
// }

// // Scan implements the database/sql.Scanner interface.
// func (s reflectValueScanner) Scan(src any) error {
// 	if b, ok := src.([]byte); ok {
// 		// Copy bytes because they won't be valid after this method call
// 		src = slices.Clone(b)
// 	}
// 	*s.dest = reflect.ValueOf(src)
// 	return nil
// }
