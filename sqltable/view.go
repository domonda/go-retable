package sqltable

import (
	"context"
	"database/sql"
	"slices"

	"github.com/domonda/go-retable"
)

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

type valueScanner struct {
	dest *any
}

// Scan implements the database/sql.Scanner interface.
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
