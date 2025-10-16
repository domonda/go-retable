package sqltable

import "database/sql"

var _ Rows = &sql.Rows{}

// Rows is an interface that abstracts the essential methods of database/sql.Rows,
// allowing code to work with row sets from either real databases or the sqltable
// virtual driver without depending on concrete types.
//
// This interface matches the key methods of *sql.Rows and enables testing and
// mocking of database query results. It provides a minimal surface area for
// iterating over tabular data and scanning values into Go variables.
//
// The interface is compatible with *sql.Rows from the standard library, meaning
// any function accepting Rows can work with real database query results.
//
// Usage example:
//
//	func ProcessRows(rows sqltable.Rows) error {
//		defer rows.Close()
//
//		columns, err := rows.Columns()
//		if err != nil {
//			return err
//		}
//
//		for rows.Next() {
//			// Create slice for scanning
//			values := make([]any, len(columns))
//			if err := rows.Scan(values...); err != nil {
//				return err
//			}
//			// Process values...
//		}
//
//		return rows.Err()
//	}
//
// The Rows interface follows the same usage patterns as sql.Rows:
//  1. Call Next() to advance to each row
//  2. Call Scan() to read column values into variables
//  3. Call Close() when done to release resources
//  4. Call Err() to check for iteration errors
type Rows interface {
	// Columns returns the names of the columns in the result set.
	//
	// Column names are returned in the order they appear in the query or
	// view definition. The returned slice should not be modified.
	//
	// Returns:
	//   - A slice of column names
	//   - An error if column metadata cannot be retrieved
	Columns() ([]string, error)

	// Scan copies the column values from the current row into the variables
	// pointed to by dest.
	//
	// The number of values in dest must match the number of columns returned
	// by Columns(). Scan converts column values to the destination types as
	// appropriate, following sql.Rows behavior.
	//
	// Scan must be called after Next() returns true and before the next call
	// to Next(). Calling Scan without a preceding successful Next() call or
	// after Next() returns false results in undefined behavior.
	//
	// Parameters:
	//   - dest: Pointers to variables to receive column values
	//
	// Returns:
	//   - An error if scanning fails or column count doesn't match
	Scan(dest ...any) error

	// Close closes the Rows, preventing further enumeration and releasing
	// any associated resources.
	//
	// If Next() is called and returns false with no further result sets,
	// Rows is automatically closed and calling Close() explicitly is optional.
	// However, it's good practice to defer Close() immediately after obtaining
	// Rows to ensure cleanup even if an error occurs.
	//
	// Close is idempotent and safe to call multiple times. It does not affect
	// the result of Err().
	//
	// Returns:
	//   - An error if cleanup fails (rare, often returns nil)
	Close() error

	// Next prepares the next result row for reading with Scan.
	//
	// It returns true if a row is available, or false if there are no more
	// rows or an error occurred while preparing the next row. Use Err() to
	// distinguish between normal end-of-rows and error conditions.
	//
	// Every call to Scan, even the first one, must be preceded by a call to
	// Next() that returns true. Typical usage iterates with a for loop:
	//
	//	for rows.Next() {
	//		// Scan and process row
	//	}
	//	if err := rows.Err(); err != nil {
	//		// Handle iteration error
	//	}
	//
	// Returns:
	//   - true if a row is available for scanning
	//   - false if no more rows exist or an error occurred
	Next() bool

	// Err returns the error, if any, that was encountered during iteration.
	//
	// Err should be checked after Next() returns false to determine whether
	// iteration stopped due to an error or natural end-of-rows. It can be
	// called after an explicit or implicit Close().
	//
	// Returns:
	//   - nil if iteration completed normally
	//   - An error if one occurred during iteration
	Err() error
}
