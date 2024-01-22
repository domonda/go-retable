package sqltable

import (
	"reflect"
	"testing"
)

func Test_parseQuery(t *testing.T) {
	tests := []struct {
		query       string
		wantColumns []string
		wantTable   string
		wantOffset  int
		wantLimit   int
		wantErr     bool
	}{
		{
			query:       `select * from table`,
			wantColumns: []string{"*"},
			wantTable:   `table`,
		},
		{
			query:       `select * from my.table`,
			wantColumns: []string{"*"},
			wantTable:   `my.table`,
		},
		{
			query:       `select * from "table"`,
			wantColumns: []string{"*"},
			wantTable:   `table`,
		},
		{
			query:       `select * from "my.table"`,
			wantColumns: []string{"*"},
			wantTable:   `my.table`,
		},
		{
			query:       `SELECT * FROM table`,
			wantColumns: []string{"*"},
			wantTable:   `table`,
		},
		{
			query:       `SELECT * FROM my.table`,
			wantColumns: []string{"*"},
			wantTable:   `my.table`,
		},
		{
			query:       `SELECT * FROM "table"`,
			wantColumns: []string{"*"},
			wantTable:   `table`,
		},
		{
			query:       `SELECT * FROM "my.table"`,
			wantColumns: []string{"*"},
			wantTable:   `my.table`,
		},

		{
			query:       `select a,B , "Col3",column4 from table`,
			wantColumns: []string{"a", "B", "Col3", "column4"},
			wantTable:   `table`,
		},
		{
			query:       `select a,B , "Col3",column4 from my.table`,
			wantColumns: []string{"a", "B", "Col3", "column4"},
			wantTable:   `my.table`,
		},
		{
			query:       `select a,B , "Col3",column4 from "table"`,
			wantColumns: []string{"a", "B", "Col3", "column4"},
			wantTable:   `table`,
		},
		{
			query:       `select a,B , "Col3",column4 from "my.table"`,
			wantColumns: []string{"a", "B", "Col3", "column4"},
			wantTable:   `my.table`,
		},
		{
			query:       `SELECT a,B , "Col3",column4 FROM table`,
			wantColumns: []string{"a", "B", "Col3", "column4"},
			wantTable:   `table`,
		},
		{
			query:       `SELECT a,B , "Col3",column4 FROM my.table`,
			wantColumns: []string{"a", "B", "Col3", "column4"},
			wantTable:   `my.table`,
		},
		{
			query:       `SELECT a,B , "Col3",column4 FROM "table"`,
			wantColumns: []string{"a", "B", "Col3", "column4"},
			wantTable:   `table`,
		},
		{
			query:       `SELECT a,B , "Col3",column4 FROM "my.table"`,
			wantColumns: []string{"a", "B", "Col3", "column4"},
			wantTable:   `my.table`,
		},

		// Errors
		{query: "", wantErr: true},
		{query: `SELECT *,b FROM "my.table"`, wantErr: true},
		{query: `SELECT a,* FROM "my.table"`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			gotColumns, gotTable, gotOffset, gotLimit, err := parseQuery(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotColumns, tt.wantColumns) {
				t.Errorf("parseQuery() gotColumns = %v, want %v", gotColumns, tt.wantColumns)
			}
			if gotTable != tt.wantTable {
				t.Errorf("parseQuery() gotTable = %v, want %v", gotTable, tt.wantTable)
			}
			if gotOffset != tt.wantOffset {
				t.Errorf("parseQuery() gotOffset = %v, want %v", gotOffset, tt.wantOffset)
			}
			if gotLimit != tt.wantLimit {
				t.Errorf("parseQuery() gotLimit = %v, want %v", gotLimit, tt.wantLimit)
			}
		})
	}
}
