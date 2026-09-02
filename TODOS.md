# TODOS

## sqltable

### Test the virtual database/sql driver

**What:** Cover `sqltable`, which is at 0% for everything except the two query-parsing helpers. 22 exported and unexported functions across `db.go`, `stmt.go` and `view.go` are never executed by a test.

**Why:** `sqltable` exposes a `*sql.DB` to callers, so a defect there surfaces as wrong query results or a panic inside `database/sql`, in code the caller cannot step into. It is the only subpackage of this module with essentially no coverage: `csvtable`, `htmltable` and `exceltable` all have real tests, so a green suite currently says nothing about this one.

**Context:** The obvious objection is that testing a SQL driver needs a database. It does not. `sqltable` *is* the driver: `NewViewsDB(map[string]retable.View) *sql.DB` and `NewViewDB(name, view) *sql.DB` return a real `*sql.DB` backed entirely by in-memory `retable.View` values, via `sql.OpenDB(database{...})`. A test is `db := sqltable.NewViewDB("people", view)` followed by an ordinary `db.Query("SELECT ...")` — no driver registration, no fixture files, no external service.

Current state, measured with `go test -coverprofile -coverpkg=./... ./sqltable/`:

- `stmt_test.go` is the only test file, one test function. It covers `parseQuery` and `unquote` at 100%.
- `db.go`: all 11 functions at 0% — `NewViewsDB`, `NewViewDB`, and the `driver.Driver` / `driver.Connector` / `driver.Conn` / `driver.Tx` implementations (`Connect`, `Driver`, `Open`, `OpenConnector`, `Prepare`, `Close`, `Begin`, `Commit`, `Rollback`).
- `stmt.go`: 9 of 11 at 0% — `newStmt`, `Close`, `NumInput`, `Exec`, `Query`, and the `driver.Rows` side (`Columns`, `Close`, `Next`, `driverValue`).
- `view.go`: both functions at 0% — `ScanRowsAsView` and `Scan`.

Worth covering, in rough order of value:

1. The round trip: build a `retable.View`, expose it with `NewViewDB`, `SELECT *` it back, and assert the rows and column names match the source. That one test alone exercises most of `db.go` and `stmt.go`.
2. Column projection: `SELECT col1, col2`, reordered columns, and double-quoted column names, which `unquote` already handles but nothing drives end to end.
3. The documented limits: `INSERT`, `UPDATE`, `DELETE`, `JOIN`, `WHERE` and `ORDER BY` are not supported, so each should produce an error rather than silently ignoring the clause and returning every row. This is the highest-risk gap, because silently dropping a `WHERE` returns *more* data than asked for.
4. Error paths: an unknown table name, a column name that is not in the view, and a query that is not a `SELECT`.
5. `Exec` on a read-only driver, and the `Begin`/`Commit`/`Rollback` transaction stubs.
6. `ScanRowsAsView`, the other direction: scanning real `*sql.Rows` into a `retable.View`. This one does need a source of `*sql.Rows`, which `NewViewDB` itself can provide, so the two halves can test each other.
7. Cell type preservation through `driverValue`: `database/sql` restricts driver values to a fixed set, so a cell type that is not one of them needs a defined conversion or a defined error.

**Effort:** M
**Priority:** P2
**Depends on:** None

## Completed
