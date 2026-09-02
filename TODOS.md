# TODOS

## v1 release

### Tag v1.0.0, or decide not to

**What:** This branch is the v1 API freeze. It renames `View.Columns` to `ColumnNames` and adds `NumColumns`, renames the `Tit` field to `TableTitle` on the five view types that have it, renames `ExtraRowView` to `ExtraRowsView`, drops the unused `io.Writer` parameter from `SprintlnView` and `SprintlnTable`, and removes `github.com/ungerik/go-fs` from the public API by taking an `io.Reader` in the two `csvtable` reader entry points. Nothing after it can change those names without breaking callers.

**Why:** `git ls-remote --tags` returns nothing, and `CHANGELOG.md` states the current model outright: "Not tagged: this repository carries no version tags, so consumers resolve it as a pseudo-version of the `main` commit." A v1 is therefore a change of release model, not the next entry in a sequence. It is also what makes the freeze mean anything: the renames above have been free precisely because no tag has ever promised otherwise.

**Context:** If the answer is yes, add a `VERSION` file and tag the root module `v1.0.0` and the submodule `exceltable/v1.0.0` in lockstep. Leave the `replace github.com/domonda/go-retable => ..` and the placeholder pseudo-version in `exceltable/go.mod` untouched; that pattern is deliberate and is what lets an external consumer resolve the submodule against its own requirement on the root module. If the answer is no, the rest of this section still applies except the tagging step, and the remaining items below become ordinary quality work rather than release blockers.

**Effort:** S
**Priority:** P1
**Depends on:** The two naming decisions below

### Two names the API freeze leaves unsettled

**What:** `ReadFileWithFormatToStructSlice` and `ReadFileDetectFormatToStructSlice` take an `io.Reader` and no longer open anything, so `File` in their names describes what they used to do. Separately, `ReplaceNewlineWithSpacefunc` survives as a deprecated alias of `ReplaceNewlineWithSpace`.

**Why:** Both are free to change for exactly as long as no tag exists, and permanent afterwards. The alias is the sharper case: its own doc comment says it "only exists because the misspelled name is part of the published API", but nothing has been published, so the reason it gives for keeping it does not hold yet.

**Context:** `exceltable` already models the alternative naming in this module with `Read(io.Reader)` alongside `ReadLocalFile(filename string)`, so `csvtable` could either rename to match what the functions now take or regain a filename-based entry point. Deciding to keep either name is a valid outcome; what is not is deciding by default at tag time.

**Effort:** S
**Priority:** P1
**Depends on:** None

### The breaking API changes are not in the changelog

**What:** `CHANGELOG.md` has one section, `## 2026-09-02`, already marked released, and no mention of `ColumnNames`, `NumColumns`, `TableTitle`, `ExtraRowsView`, the dropped `SprintlnView` parameter or the `go-fs` removal.

**Why:** These are the only changes in the repository's history that break a caller's build. A consumer upgrading across them gets compile errors with nothing in the changelog explaining which name replaced which, which is the one situation the file exists for.

**Context:** Needs a new unreleased section rather than an edit to the released one. Each entry can be short, but has to name both sides of the rename so the fix is mechanical for the reader.

**Effort:** S
**Priority:** P1
**Depends on:** None

### CI never tests exceltable and has never run gosec

**What:** `.github/workflows/go.yml` runs `go test ./...` from the root, which resolves to four packages; `exceltable` is a separate module and is not among them. `.github/workflows/gosec.yml` triggers on `master` while the default branch is `main`, so it has no runs at all. `codeql-analysis.yml` pins `github/codeql-action/*@v1`, which is retired, and `setup-go` pins Go 1.23 against a `go.mod` that requires 1.26, passing only because the toolchain directive downloads 1.26.

**Why:** Tagging a v1 whose security scan has never executed, and whose Excel reader is never exercised by CI, states a level of assurance that has not been measured. `./test-workspace.sh` already iterates the workspace modules correctly and is what the workflow should call.

**Context:** No linter runs either, and there is no `-race`. Coverage is uneven rather than low: root 89.5%, `csvtable` 94.5%, `exceltable` 92.5%, against `htmltable` 45.4% and `sqltable` 15.7%, the latter already tracked above.

**Effort:** S
**Priority:** P1
**Depends on:** None

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

## htmltable

### A malformed JSON cell truncates the document after bytes are already flushed

**What:** `JSONCellFormatter` returns a `json.SyntaxError` for a cell that is not valid JSON, `Writer.WriteView` propagates it and returns, but the header, caption and every preceding row are already written to `dest`. The caller gets an unclosed `<table>` with no `</table>`. The same shape now has a second trigger: a custom template that fails at `Execute` truncates the document the same way, with no formatter involved.

**Why:** The trigger is trivial and comes straight from the data: any non-JSON `string`, `[]byte` or `json.RawMessage` cell, including `"hello world"` or even `"   "`. On an `http.ResponseWriter` those bytes are already gone when the error surfaces, so a caller that logs the error and moves on has emitted a broken fragment that swallows or foster-parents whatever the surrounding page writes next.

**Context:** Pre-existing, found by the adversarial pass during the /ship of the JSON escaping fix. Verified with four rows where row three is malformed: output stops after two complete `<tr>` elements. `TestJSONCellFormatter_MalformedJSONAbortsWrite` now pins the truncation so a change to it is visible, but pinning is not deciding. Two coherent resolutions: buffer the document and write only on success, or have the formatter fall back to escaping the raw text instead of failing. Only the first covers the template-execution trigger as well, which is worth weighing before implementing the narrow formatter-side fix. The first changes memory behavior for large tables, the second changes what a malformed cell means, so this needs a call rather than a patch.

**Effort:** M
**Priority:** P2
**Depends on:** None

### json.Indent amplifies deeply nested JSON about 10000x

**What:** With a non-empty indent, `JSONCellFormatter` passes the cell through `json.Indent`, which indents every nesting level. Go's scanner allows nesting up to 10000, so a small deeply nested value expands enormously.

**Why:** One cell can exhaust memory and stall the request. Measured with `JSONCellFormatter("  ")` and a `json.RawMessage` of `[[[[...]]]]` at depth 9999: 19,998 bytes in, 199,960,013 bytes out, about 11 seconds of CPU. Escaping does not help, because the expansion happens before it.

**Context:** Pre-existing, found by the adversarial pass during the /ship of the JSON escaping fix. Only reachable when the formatter is configured with a non-empty indent, so the compact default is unaffected. Wants a nesting-depth or output-size cap before indenting, with a defined error when the cap is hit.

**Effort:** S
**Priority:** P2
**Depends on:** None

### Invalid UTF-8 in pre-marshaled JSON reaches the document verbatim

**What:** `json.Compact` and `json.Indent` do not validate UTF-8 and `preTextEscaper` works byte-wise, so invalid UTF-8 inside a `json.RawMessage`, `string` or `[]byte` cell is emitted unchanged. The `default` branch does not have this problem: Go's encoder substitutes U+FFFD.

**Why:** Not an injection on any current browser, which decodes overlong sequences to U+FFFD before tokenizing rather than to `<`. It is an output-integrity defect: the document is not well-formed UTF-8, which breaks XHTML and XML serialization and hands something to normalize to any downstream sanitizer or proxy. It is also the one remaining respect in which the escaping does not behave the same on all input branches.

**Context:** Pre-existing, found by the adversarial and red-team passes during the /ship of the JSON escaping fix. Verified with `json.RawMessage("{\"a\":\"\xc0\xbcscript\xc0\xbe\"}")`, whose overlong `<` and `>` bytes pass through untouched. `strings.ToValidUTF8` on the pre-marshaled branches would close it. Same family: the encoder escapes U+2028 and U+2029 even with `SetEscapeHTML(false)` while `Compact` and `Indent` pass them through raw, which only matters if the fragment is later embedded in a JS string.

**Effort:** S
**Priority:** P3
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

### A failed string to number parse is reported as an unsupported operation

**What:** The string to int, uint, float, bool, time and duration cases of
`SmartAssign` called the `Parser` as `if i, e := parser.ParseInt(str); e == nil`,
so a parse error was discarded and the row failed with
`unsupported operation: assigning string "" to int64`, which is also what a
struct field wired to the wrong column type produces.

**Why:** The two need opposite fixes, one is a bad row and the other is a bad
struct, and `errors.Is(err, errors.ErrUnsupported)` was true for both, so a
caller could not separate them either.

**Resolution:** The reason is collected in a `parseErr` local and joined into
the final error with a second `%w`, so `errors.Is(err, errors.ErrUnsupported)`
stays true and every strategy that recurses and continues on it is unchanged,
while `errors.Is(err, strconv.ErrSyntax)` now works and the message names the
cell. Returning the parse error where it happens would have broken the
fall-through the strategies depend on, which the `"90"` to `time.Duration` case
proves: `ParseDuration` rejects a number without a unit and the integer parsing
below is what assigns it. The pointer allocation strategy lifts the reason out
of its recursion's error rather than nesting it, so an optional column declared
as a pointer reports the type the caller declared and the message says
`unsupported operation` once. Four subtests in
`TestSmartAssignReportsWhyAStringWasRejected`, including that a genuine type
mismatch still carries no reason.

**Completed:** 2026-09-03

### Writer.WithTemplate silently discards its arguments

**What:** `htmltable/writer.go` `WithTemplate` built `mod := w.clone()`, assigned all three templates to `mod`, and returned `w`. Every custom template passed to it was dropped, and `w.WithTemplate(...) == w` was true, which broke the immutability contract every other `With` method on the type follows.

**Why:** It failed silently and in the direction that looks like success: the call chained, the writer worked, and the output was simply the default table.

**Resolution:** `return mod`, plus a nil argument now leaves that template unchanged rather than being stored, which would have turned a previously harmless `WithTemplate(nil, rowTmpl, nil)` into a nil dereference after rows were already flushed. Two tests added in `htmltable/writer_test.go`: one asserting custom templates reach the output, one asserting the receiver is unchanged and the returned writer is a distinct value. Both fail against the old code. An audit of every `With` method in `htmltable/writer.go`, `csvtable/writer.go` and `structrowsviewer.go` found no other instance: every other method that doesn't return a clone delegates to one that does. Verified that fixing it opens no injection hole, because `html/template` re-escapes `template.HTML` contextually in attribute and script contexts.

**Completed:** 2026-09-03

