package sqltable

import (
	"context"
	"database/sql"
	"reflect"

	"github.com/domonda/go-retable"
)

func NewView(ctx context.Context, rows Rows) (retable.View, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	view := &retable.CachedView{Cols: columns}

	defer rows.Close()
	for rows.Next() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var (
			reflectValues   = make([]reflect.Value, len(columns))
			reflectScanners = make([]interface{}, len(columns))
		)
		for i := range reflectValues {
			reflectScanners[i] = reflectScanner{&reflectValues[i]}
		}
		err = rows.Scan(reflectScanners...)
		if err != nil {
			return view, err
		}
		view.Rows = append(view.Rows, reflectValues)
	}
	return view, rows.Err()
}

var _ sql.Scanner = &reflectScanner{}

type reflectScanner struct {
	v *reflect.Value
}

// Scan implements the database/sql.Scanner interface.
func (s *reflectScanner) Scan(src interface{}) error {
	if b, ok := src.([]byte); ok {
		// Copy bytes because they won't be valid after this method call
		src = append([]byte(nil), b...)
	}
	*s.v = reflect.ValueOf(src)
	return nil
}
