package sqltable

import (
	"context"
	"database/sql"
	"database/sql/driver"

	"github.com/domonda/go-retable"
)

func NewViewsDB(views map[string]retable.View) *sql.DB {
	return sql.OpenDB(database{views: views})
}

func NewViewDB(viewName string, view retable.View) *sql.DB {
	return NewViewsDB(map[string]retable.View{
		viewName: view,
	})
}

type database struct {
	views map[string]retable.View
}

func (c database) Connect(context.Context) (driver.Conn, error) {
	return c, nil
}

func (c database) Driver() driver.Driver {
	return c
}

func (c database) Open(string) (driver.Conn, error) {
	return c, nil
}

func (c database) OpenConnector(string) (driver.Connector, error) {
	return c, nil
}

func (c database) Prepare(query string) (driver.Stmt, error) {
	return newStmt(c.views, query)
}

func (database) Close() error {
	return nil
}

func (c database) Begin() (driver.Tx, error) {
	return c, nil
}

func (database) Commit() error {
	return nil
}

func (database) Rollback() error {
	return nil
}

// func (c conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
// 	panic("TODO")
// }
