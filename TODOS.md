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

## csvtable

### Encoding detection reports UTF-8 for data it could not decode

**What:** When no candidate encoding scores, `autoDecode` returns the raw bytes with an empty encoding name and `ParseDetectFormat` then labels the result `"UTF-8"`. The UTF-8 candidate never validates, so invalid UTF-8 competes as raw bytes and always "succeeds".

**Why:** A Latin-1, CP1252 or Macintosh file whose accented characters are not among the default `EncodingTests` is reported as successfully detected UTF-8, and `sanitizeUTF8` then replaces every high byte with a space. The caller sees a `Format` claiming UTF-8 and cells with characters silently missing, which is why it is invisible rather than merely wrong.

**Context:** Reproduced: `"Nom;Age\nNestl\xe9;30\n"` yields `Format{Encoding: "UTF-8"}` and cells `[["Nom" "Age"] ["Nestl " "30"]]`. The default `EncodingTests` cover German umlauts, `ß`, `§`, `€` and Cyrillic, so French, Spanish, Portuguese and Italian exports (`é à ç í ã`) all fall through. Two candidate directions: report a distinct name (or an error) when `bestScore == 0` and the raw data is not valid UTF-8; or make the UTF-8 candidate validate with `utf8.Valid` so it cannot win by default. The second is smaller but changes which encoding wins for genuinely ambiguous files, so it needs a decision rather than a patch. `sanitizeUTF8`'s own doc acknowledges the replacement; the misreported `Format.Encoding` is the part that hides it.

**Effort:** M
**Priority:** P1
**Depends on:** None

### Invalid UTF-16 and UTF-32 code units become a silent space

**What:** The UTF-32 length guard added on this branch catches a truncated file, but not invalid content. Unpaired surrogates and code points above U+10FFFF decode to U+FFFD with a nil error, and `sanitizeUTF8` turns that into a plain space.

**Why:** It is the same silent cell mutation the length guard was added to prevent, reached through content instead of length. A cell quietly loses a character and the row still parses.

**Context:** Reproduced, all with `err == nil`: UTF-16LE `a, U+D800, b` and UTF-32LE `a, 0x110000, b` and `a, 0xD800, b` all yield `[["a" " " "b"]]`. Options: check for `utf8.RuneError` in the decoded output when the source contained none, or accept the behavior and re-document the guard as length-only so the comment at `csvtable/charset.go` stops claiming more than it delivers. The comment currently states the intent as "a broken code unit must be an error, not a replacement character".

**Effort:** S
**Priority:** P2
**Depends on:** None

### readLines is super-quadratic on unterminated quotes

**What:** `findClosingField` re-splits every following line for every unterminated quoted field, so a file with many unterminated quotes takes time quadratic in its size.

**Why:** A hostile or merely corrupt CSV wedges the parser. This is a library that reads files from outside the process, so it is a denial of service and not only a slow path.

**Context:** Pre-existing at the merge base and not introduced by the current branch.

Use the right fixture: the two entry points blow up on different shapes, and the
original write-up of this item named the wrong pair.

- Through `ParseDetectFormat`, the shape is `"a,b\n` repeated: 5 KB → 10 ms,
  10 KB → 37 ms, 20 KB → 142 ms, 40 KB → 590 ms. Roughly 4x per doubling.
- Through `ParseWithFormat`, that same shape is **linear**. The shape that
  reproduces there is `a,"b\n` repeated: 20 KB → 9 ms, 40 KB → 36 ms,
  80 KB → 147 ms, 160 KB → 535 ms, 320 KB → 2.0 s.

A ~1 MB hostile input wedges the parser for minutes either way.

**Effort:** M
**Priority:** P2
**Depends on:** None

## retable

### Passing any Scanner silently discards a configured DefaultParser

**What:** `ViewToStructSlice` allocates a fresh `NewStringParser()` when a `Scanner` is passed and no `Parser` is, but falls through to `DefaultParser` when there is no `Scanner`. So adding a `Scanner` for one custom column reverts `NilStrings`, `TimeFormats` and the boolean strings to package defaults for every other column.

**Why:** `DefaultParser` is exported and documented as replaceable, so a program that configures it globally loses that configuration by adding an unrelated `Scanner`, with no error and no obvious link between cause and effect.

**Context:** Reproduced with `DefaultParser` configured to treat `"N/A"` as nil and a `Scanner` that handles nothing: without the scanner the row parses, with it the read fails with `unsupported operation: assigning string "N/A" to int`. `ViewToStructSlice`'s own doc says the nil case uses "a default StringParser", which is only true in the Scanner branch. Three coherent resolutions: always allocate, never allocate and always fall through to `DefaultParser`, or keep the split and document that a Scanner overrides the global. The per-view allocation exists to stop a Scanner reconfiguring the shared parser, so removing it needs that hazard addressed another way.

**Effort:** S
**Priority:** P2
**Depends on:** None

## Completed
