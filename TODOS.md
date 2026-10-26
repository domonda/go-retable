# TODOS

## v1 release

### Tag v1.0.0, or decide not to

**What:** This branch is the v1 API freeze. It renames `View.Columns` to `ColumnNames` and adds `NumColumns`, renames the `Tit` field to `TableTitle` on the five view types that have it, renames `ExtraRowView` to `ExtraRowsView`, drops the unused `io.Writer` parameter from `SprintlnView` and `SprintlnTable`, and removes `github.com/ungerik/go-fs` from the public API by taking an `io.Reader` in the two `csvtable` reader entry points. Nothing after it can change those names without breaking callers.

**Why:** `git ls-remote --tags` returns nothing, and `CHANGELOG.md` states the current model outright: "Not tagged: this repository carries no version tags, so consumers resolve it as a pseudo-version of the `main` commit." A v1 is therefore a change of release model, not the next entry in a sequence. It is also what makes the freeze mean anything: the renames above have been free precisely because no tag has ever promised otherwise.

**Context:** `VERSION` now holds `v1.0.0-beta.1`, so the freeze has a name without committing to the compatibility promise yet. Go treats a prerelease specially: `go get` will not select one unless it is asked for by name, so tagging `v1.0.0-beta.1` publishes this API for opt-in use without upgrading any existing consumer. Whichever version is tagged, tag the root module and the submodule as `exceltable/<version>` in lockstep. Leave the `replace github.com/domonda/go-retable => ..` and the placeholder pseudo-version in `exceltable/go.mod` untouched; that pattern is deliberate and is what lets an external consumer resolve the submodule against its own requirement on the root module. If the answer is no, the rest of this section still applies except the tagging step, and the remaining items below become ordinary quality work rather than release blockers.

**Effort:** S
**Priority:** P1
**Depends on:** The two naming decisions below, and every decision in the "v1 API design" section. The API review of 2026-09-06 found that the three consumer repositories in the local GOPATH pin three different pseudo-versions (2024-10, 2026-03, 2026-09) and still call `htmltable.Render`, `htmltable.NewRenderer`, `retable.NewTypeFormatters` and `retable.NewComputedView`, none of which exist any more. Every consumer therefore already faces a breaking upgrade, so a redesign before the tag costs them about the same as the freeze would, and a tag now protects nobody who is currently on an old commit.

### Two names the API freeze leaves unsettled

**What:** `ReadFileWithFormatToStructSlice` and `ReadFileDetectFormatToStructSlice` take an `io.Reader` and no longer open anything, so `File` in their names describes what they used to do. Separately, `ReplaceNewlineWithSpacefunc` survives as a deprecated alias of `ReplaceNewlineWithSpace`.

**Why:** Both are free to change for exactly as long as no tag exists, and permanent afterwards. The alias is the sharper case: its own doc comment says it "only exists because the misspelled name is part of the published API", but nothing has been published, so the reason it gives for keeping it does not hold yet.

**Context:** `exceltable` already models the alternative naming in this module with `Read(io.Reader)` alongside `ReadLocalFile(filename string)`, so `csvtable` could either rename to match what the functions now take or regain a filename-based entry point. Deciding to keep either name is a valid outcome; what is not is deciding by default at tag time.

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

## v1 API design

Findings of the API surface review of 2026-09-06. Each item below is an open
architecture question whose deliverable is a decision, recorded here, before
`v1.0.0` is tagged. Unless an item says "Decisions, made <date>", the
recommendation in it is the reviewer's, not a decision that has been made; the
SmartAssign item is the only fully decided one so far, and it also settles the `Formatter` half of the raw output item. The verified defects the same review found are
listed under "retable" further down; they need fixing whichever way the
decisions go.

Surface counts at the time of the review:

| Package    | Exported symbols | Notes                                       |
| ---------- | ---------------- | ------------------------------------------- |
| retable    | 63               | plus 32 methods on `csvtable.Writer`        |
| csvtable   | 20               | 5 `Read*ToStructSlice` variants, 8 params   |
| htmltable  | 9                | plus 22 methods on `Writer`                 |
| exceltable | 6                | 2x2 matrix of reader functions              |
| sqltable   | 4                | 15% coverage, 3 consumer call sites         |

### Decide whether ReflectCell is part of View or not

**What:** `View` has `Cell(row, col) any` and `ReflectCellView` adds `ReflectCell(row, col) reflect.Value`. Every view type and every wrapper implements both, and `AsReflectCellView` appears 56 times in consumer code, so the "optional optimization" is knowledge every caller needs. The halfway state is the worst option: two access paths to maintain, and consumers still have to know which one they hold.

**Why:** This is the one decision that shapes every other type in the root package. If `ReflectCell` is in `View`, `AsReflectCellView` and `wrapAsReflectCellView` go away and every wrapper loses a method. If it is out, `ReflectValuesView`, `SingleReflectValueView`, `NewSingleReflectValueView` and `ExtraColsReflectValueFuncView` go away, and `reflect.Value` stops being part of the public data model. Either way the surface shrinks; only the current state keeps both.

**Context:** Of the 14 view types, all but `AnyValuesView`, `ExtraColsView`, `ExtraRowsView`, `FilteredView` and `HeaderView` already implement `ReflectCell`. The cost `ReflectCellView` avoids is the interface boxing of `reflect.Value.Interface()` on `StructRowsView`, which is real for large struct slices but has never been measured. Recommendation: fold `ReflectCell` into `View`, because the package is named for reflection and every non-trivial view already pays for the method; measure the boxing cost first so the decision is backed by a number rather than a guess.

**Effort:** S to decide, M to implement
**Priority:** P1
**Depends on:** None

### Decide the Viewer model: generic or untyped, and one struct viewer type

**What:** `Viewer.NewView(title string, table any) (View, error)` is untyped, so the type parameter of `csvtable.Writer[T]` and `htmltable.Writer[T]` buys nothing: `Write(ctx, dest, table T)` forwards `table` to `any`. Separately, `StructFieldNaming` and `StructRowsViewer` both implement `Viewer`, and `StructRowsViewer` embeds `StructFieldNaming`, so there are two types for the concept "make a View from a struct slice". `SelectViewer` is a mutable package-level function var that dispatches between `StringsViewer` and `&DefaultStructFieldNaming`.

**Why:** A generic writer over an untyped viewer is a half-generic API that will confuse every reader of the signature. Two viewer types for structs doubles the `With*` methods and the docs. A mutable dispatch var means library code can change what every `PrintlnTable` in the process does; verified by setting `DefaultStructFieldNaming.Untagged` and watching `SelectViewer` produce a view with no columns.

**Context:** Options: (a) drop the type parameter from both writers and keep `Viewer` untyped; (b) make the viewer generic, `func ViewOf[T any](title string, rows []T) (View, error)`, and keep `Writer[T]`; (c) keep everything. Recommendation: (a), because the writers never use `T` for anything, and a generic viewer cannot express `[][]string` and `[]Struct` through one interface without falling back to `any` anyway. Fold `StructFieldNaming`'s `NewView` into `StructRowsViewer` so only one type is a `Viewer`, and turn `SelectViewer` into either a parameter of the four `*Table` print functions or a registry with an explicit `Register` call.

**Effort:** S to decide, M to implement
**Priority:** P1
**Depends on:** None

### Decide how raw output is expressed, and whether Formatter survives

**What:** The `Formatter` half of this item is decided (2026-09-06, see the SmartAssign item): the interface and its bridges are removed. What remains open is the raw flag. `CellFormatter.FormatCell` returns `(str string, raw bool, err error)`. The `raw` flag is an HTML escaping concern living in the format-neutral core interface; its doc comment needs forty lines, and it means "do not quote" in `csvtable` and "do not escape" in `htmltable`. It doubles the surface everywhere it touches: `PrintfCellFormatter` and `PrintfRawCellFormatter`, `StringIfTrue` and `RawStringIfTrue`, `SprintCellFormatter(rawResult bool)`, `CellFormatterFromFormatter(f, rawResult)`, `ReflectCellFormatterFunc(fn, rawResult)`, and `WithTypeFormatterReflectFunc` beside `WithTypeFormatterReflectRawFunc` on both writers. Separately, `Formatter` (`Format(reflect.Value) (string, error)`) exists beside `CellFormatter` with two bridge functions between them, and is only consumed by `SmartAssign`'s `srcFormatter` parameter.

**Why:** Every new formatter has to be written twice or take a bool, and every writer has to interpret the bool for its own format. One `Raw(CellFormatter) CellFormatter` wrapper, or a typed result such as `type RawString string` that the writer type-switches on, collapses all of the twins into single definitions. Removing `Formatter` removes two bridge functions and one of the five `SmartAssign` parameters.

**Context:** The chain convention is also inconsistent and should be fixed in the same pass: `errors.ErrUnsupported` means "try the next formatter", but `LayoutFormatter` returns a hard error on a type mismatch and `StringIfTrue` panics on a non-bool cell (verified). Recommendation: keep the bool in the interface only if the alternative measurably complicates `TryFormattersOrSprint`; otherwise a `Raw` wrapper, one formatter per behavior, and a rule that a formatter which cannot handle the cell's type returns `ErrUnsupported`, never panics, never returns a hard error.

**Effort:** S to decide, M to implement
**Priority:** P1
**Depends on:** None

### Replace SmartAssign with an injectable Converter and a documented null contract

**What:** `SmartAssign(dst, src reflect.Value, dstScanner Scanner, parser Parser, srcFormatter Formatter)` runs fifteen ordered strategies, turns panics into errors with a `recover()`, and has behavior that depends entirely on strategy order. Every CSV import in every consumer depends on those accidental details; the doc comment needs a paragraph to explain why `"NULL"` assigned to a `*string` yields a pointer to `"NULL"` rather than `nil` (verified).

**Why:** Tagging v1 with the current function freezes the heuristics, not a contract. Any reordering to fix a bug becomes a breaking change, and there is no table a caller can consult to know what will happen to a given source and destination type pair. The defaults are also the unsafe ones: a `NULL` in a numeric column imports as `0`, and every consumer import overrides that.

**Context:** Usage, checked 2026-09-06. Inside the module the only non-recursive caller is `ViewToStructSlice`. In the consumers there are two direct calls; everything else goes through `ViewToStructSlice` or the `csvtable` readers. The three collaborators `dstScanner`, `parser` and `srcFormatter` always travel together as a triple, through `ViewToStructSlice`, into `SmartAssign` and through its four recursive calls; `Scanner.ScanString` additionally receives the `Parser`. In every consumer call site read, `srcFormatter` is `nil`. `domonda-service/pkg/accounting/importparser.go` is a hundred-line comment plus two `StringParser` vars that exists only to explain how `Parser.IsNil`, the `StrictNilStrings` scanner and the zero-value strategy interact, and every import there overrides the default nil policy because `NULL` in a numeric column importing as `0` stored wrong account numbers. The "Passing any Scanner silently discards a configured DefaultParser" item below is the same triple assembled inconsistently.

**Decisions, made 2026-09-06.** These are decided, not proposals; the rest of the section still carries recommendations only.

1. **Name.** The function converts a source value of any type into a destination of any type, parsing strings on the way; "Smart" names nothing. It becomes the `Assign` method of a `Converter` type, and the five-parameter reflect-level function is unexported. (`database/sql` has the same shape and heuristic list as its unexported `convertAssign`, so `ConvertAssign` would be the name if it had to stay a function.)

2. **Injection.** The scanner and parser plus the null policy become one value, and everything that took the old triple takes one `Assigner`. The `Formatter` interface is deleted (decided 2026-09-06): on the assign side it only answered how to render a source value into a string field, which `encoding.TextMarshaler`, `fmt.Stringer` and the `fmt.Sprint` fallback already answer, and no consumer call site ever passed one; on the writer side `CellFormatter` is the only interface needed. With it go the `srcFormatter` parameter, the strategy that used it, and the two bridges `CellFormatterFromFormatter` and `FormatterFromCellFormatter`. Adding a field later is backward compatible, so leaving it out at v1 is the safe direction.

   ```go
   // A nil *Converter is valid and uses the defaults.
   type Converter struct {
   	Parser  Parser  // nil selects the default StringParser
   	Scanner Scanner // nil means no custom scanning
   	// NullToZero assigns the zero value of the destination when the source
   	// carries no meaningful value and the destination cannot hold absence.
   	// The default reports an error instead; declare an optional column as
   	// a pointer to keep the absence visible.
   	NullToZero bool
   }

   func (c *Converter) Assign(dst, src reflect.Value) error

   type Assigner interface{ Assign(dst, src reflect.Value) error }
   ```

   `ViewToStructSlice` and the readers take one `Assigner`, inside the options struct of the reader entry point item, instead of three positional parameters. A consumer names its cell policy once, as `accounting.CSVImportConverter`, and can inject a wholly different implementation. Configuration is of the inputs only; the strategy order is never configurable.

3. **Terminology: null, not nil.** The concept is a cell that carries no meaningful value. For a string source that is whatever the parser classifies, and `""` is one spelling beside `NULL`, `N/A` and `None`; none of them is nil in Go and `NULL` is not the zero value of `string`, so neither "nil" nor "zero to zero" describes it. The package currently uses three names for it, `Parser.IsNil`, `StringParser.NilStrings` and `StrictNilStrings`, plus `IsNullLike` for the same concept on a `reflect.Value`. Null matches the cell domain, matches `IsNullLike`, and does not collide with Go's `nil`. Renames: `Parser.IsNil` to `IsNull`, `StringParser.NilStrings` to `NullStrings`, `StrictNilStrings` is deleted and replaced by the `NullToZero` field, which is a policy and was never a scanner.

4. **What a null source is.** One predicate for absence in the whole package (decided 2026-09-06): a source is null when `IsNullLike` reports it so, or when it is a string that `Parser.IsNull` classifies. `IsNullLike` is what the CSV and HTML writers already use to render a cell as the nil value, so the read side now agrees with the write side. That covers the four cases SmartAssign gated separately, an `IsNull()` that is true, a nil pointer, a `struct{}` value and the null string, and additionally nil slices, maps, channels and funcs, a `driver.Valuer` that returns `nil`, and any value whose `IsZero()` is true, which includes a zero `time.Time`. A zero time is in practical terms never a meaningful value, so it is treated as null like every other zero that says so about itself. The flag's meaning is: there is no meaningful source, so the destination gets its zero value. The zero value of `Converter` is the strict behavior, because that is what every consumer import configures today and what a reader expects; turning absence into `0` across types is the "too smart" part and has to be asked for.

5. **The null source rule.** One strategy in the conversion table, four rows, checked in order:

   | Destination                          | `NullToZero: false` (default) | `NullToZero: true`   |
   | ------------------------------------ | ----------------------------- | -------------------- |
   | directly convertible from the source | the source, converted as is   | the same             |
   | implements `SetNull()`               | `SetNull()` is called         | `SetNull()` is called|
   | zero value is itself null-like       | its zero value                | its zero value       |
   | anything else                        | error                         | zero value           |

   Before any row, a configured `Scanner` sees a string source first, as today, so a type can define for itself what `""` means.

   The first row is Go's own conversion and runs before the null check, as the direct conversion step does today. It is what keeps a same-type copy working: a zero `time.Time` into a `time.Time` field stays a zero time, and `""` or `"NULL"` into a `string`, a type defined as string, `[]byte` or `[]rune` is assigned verbatim, because those destinations can hold any string and the parser's classification never applies to them. A null source that is not convertible follows the other rows: a nil `*int` into a `string` is an error by default, because `""` is a value and not absence.

   The second and third rows are "the destination can hold absence". The second row (decided 2026-09-06) is a destination that implements `SetNull()`, looked up on the addressable value so that pointer-receiver methods count, which is how the nullable types in `go-types` declare it. It is checked before the zero-value test and takes precedence over it, because a type that offers `SetNull()` has its own idea of what null looks like and that need not be the zero value; the method is called and nothing is assigned. The third row is tested as `IsNullLike(reflect.Zero(dstType))`: the type's own zero value reports itself as null. That is every pointer, every nil-able kind, every `IsNull()` type such as a nullable ID, and every `IsZero()` type such as `time.Time`. For those a null source is their zero value and no flag is involved, because the destination is explicit about being zeroable: by implementing `IsZero()` or `IsNull()`, or by being a pointer or nil-able kind, the type declares that its zero value means "no value", so assigning it loses nothing and the field afterwards says "null" the same way the source did. An `int` or `float64` makes no such declaration; `0` has no special meaning and no interface says otherwise, so in strict mode an empty string cannot become one. A pointer is never allocated for a null source, so `""` into `*string` is `nil` and a zero `time.Time` into `*time.Time` is `nil`, not a pointer to the zero time. A `""` date cell into a plain `time.Time` field is the zero time, which is that type's null.

   The last row is the destinations whose zero value is a real value and that offer no `SetNull()`, `0`, `false`, `""` for a non-string source, a plain struct. Only here does `NullToZero` decide between an error and the zero value, because only here would the zero value pretend to be data.

6. **The conversion table is the contract.** Strategy order stays fixed and is written out as a table, source kind by destination kind, and that table is what the tests pin. The implementation is free to change as long as the table holds.

7. **Round trip stability** (decided 2026-09-06). Reading a table into structs, writing those structs with a writer, and reading the output again yields equal structs. This is the property the null rule above is derived from: the writers render every `IsNullLike` cell as their nil value, `""` by default, and `""` reads back as null, so every field that can hold absence has to accept a null source without a flag, which is what the second row guarantees. Under the earlier "pointer only" row a struct with a plain `time.Time` field holding a zero time wrote `""` and then failed to read, so that row was wrong by this principle.

   What the principle requires of the rest of the package:
   - The writer's nil value and the parser's null strings are one contract: the reader must classify whatever the writer emits for null. The defaults agree (`""` is in `NullStrings`), and `Writer.WithNilValue` has to document that a value outside the parser's `NullStrings` breaks the round trip.
   - The default `fmt.Sprint` rendering of each supported type must be one the parser accepts: `time.Time` prints in the layout the parser lists as `formatTimeString`, `time.Duration` prints what `ParseDuration` reads, floats print the shortest representation that parses back exactly, bools print `true` and `false`. A type-specific formatter that changes the text has to keep it parseable by the matching scanner, which is the caller's responsibility and is documented as such.
   - The equality is of values, not of text. A null cell read with `NullToZero: true` into an `int` becomes `0`, writes as `0`, and reads as `0`; the struct is stable, the text is not, and that is fine.
   - One inherent exception, to be documented rather than fixed: a `*string` holding a pointer to `""` or to `"NULL"` writes as that text and reads back as `nil`, because a text format cannot distinguish absence from the spelling of absence.
   - The contract is pinned by a round trip test over one struct with every supported field kind in value and pointer form, through `csvtable` and `htmltable` where a reader exists, and through `ViewToStructSlice` from a `StringsView` for the general case.

**Behavior changes to record in the changelog when this lands:**

- A null source into a number, bool or `time.Duration` destination, or any other destination whose zero value is a real value, is an error by default; today it is the zero value. Callers that want the old behavior set `NullToZero: true`. A `time.Time` destination is not affected, because its zero value is null-like and a null source becomes the zero time with or without the flag, as today. In the local consumers the only such caller is the Excel path in `domonda-service`, whose `ExcelImportParser` documents that it wants `""` to mean `0` for the conthaus special rows.
- A nil pointer, `IsNull()` or `struct{}` source into a non-pointer destination is an error by default; today it is the zero value.
- A null string into `*string` (and any pointer to a string kind) is `nil`; today it is a pointer to the string, and the `SmartAssign` doc comment documents that outcome.
- A source whose `IsZero()` is true, a zero `time.Time` above all, is null: into a pointer destination it becomes `nil` where today it becomes a pointer to the zero value; into a non-convertible destination such as a `string` field it is an error by default where today `fmt.Stringer` produces `0001-01-01 00:00:00 +0000 UTC`. A same-type or convertible destination is unaffected. The same applies to nil slices, maps, channels and funcs and to a `driver.Valuer` returning `nil`.
- A null source into a non-pointer destination that can hold absence, such as a nullable ID type with `IsNull()`, becomes its zero value where today it is an error, because the null string strategy only covered numbers, bools, times and durations.
- A null source into a destination with a `SetNull()` method calls that method; today nothing looks for it, and the null string strategy only covered numbers, bools, times and durations.
- `StrictNilStrings` is removed; `Parser.IsNil` and `StringParser.NilStrings` are renamed. The two `StringParser` vars and the `StrictNilStrings` argument in `domonda-service/pkg/accounting` become two `Converter` values.
- The `Formatter` interface, `FormatterFunc`, `SprintFormatter`, `UnsupportedFormatter`, `CellFormatterFromFormatter` and `FormatterFromCellFormatter` are removed, and `SmartAssign`'s `srcFormatter` parameter with them.

Nothing in this item is still open.

**Effort:** M
**Priority:** P1
**Depends on:** None

### Decide the reader entry point shape: options struct, one function per package

**What:** `ViewToStructSlice[T]` takes six positional parameters plus a variadic; the README example passes five `nil` values in a row. `csvtable` has five `Read*ToStructSlice` functions (`Strings`, `Bytes`, `File`, times `WithFormat` and `DetectFormat`) with eight parameters each. `exceltable` has four functions, reader or filename times all sheets or first sheet, each with a bare `bool`.

**Why:** Positional `nil` parameters cannot be extended without breaking every caller, and the combinatorial naming means each new input source or format mode multiplies the function count. This is the change that most reduces the exported symbol count and the one that is impossible after the tag.

**Context:** Proposed shape: `type ReadOptions struct { Naming *StructFieldNaming; Scanner Scanner; Parser Parser; Validate func(reflect.Value) error; RequiredColumns []string }` in the root package, `ViewToStructSlice[T](view View, opts *ReadOptions)`, and in `csvtable` one `Read[T](r io.Reader, format *Format, opts *ReadOptions)` where a nil `format` means detect. That also settles the "File in the name" item above by removing the names. `exceltable` becomes `Read(r io.Reader, opts *ReadOptions) ([]View, error)` with `RawCellStrings` and `FirstSheetOnly` as fields, and the filename variants go, because `os.Open` is one line for the caller. Recommendation: do it; the sixteen consumer call sites of `ReadFileDetectFormatToStructSlice` are a mechanical rewrite.

**Effort:** M
**Priority:** P1
**Depends on:** The SmartAssign decision, which fixes which fields the options struct has

### Extract one formatting pipeline for both writers, and drop the Func and Raw twins

**What:** `csvtable.Writer` (32 methods) and `htmltable.Writer` (22 methods) duplicate the viewer, column formatter, type formatter and nil fallback logic verbatim, including `cellString`, and have drifted: CSV `Write` takes no title while HTML `Write` takes `caption ...string` joined with a space; CSV has getters for seven fields but not `Padding` or `HeaderRow`; HTML `WithTemplate` names its first parameter `tableTemplate` but sets `headerTemplate`; CSV's `headerWriter` drops column formatters but keeps type formatters, so a `string` kind formatter is applied to header names. Every `WithXFormatter` has a `WithXFormatterFunc` twin that only saves the caller a type conversion, and the `WithTypeFormatterReflectFunc` variants panic on a bad function signature in the middle of a builder chain.

**Why:** Two copies of the same pipeline means every formatter fix lands twice or once, and the drift above shows it has already landed once. The twins are roughly a third of the writer surface and carry no behavior.

**Context:** Proposed: a `retable.CellPipeline` (name open) holding column formatters, a `ReflectTypeCellFormatter` and the nil value, with `FormatCell` returning the final string and raw flag, embedded by both writers. Both writers keep their format-specific `With*` methods only. Delete the `Func` twins and let callers write `retable.CellFormatterFunc(f)`. Replace the two `ReflectFunc` methods with one that returns an error, or keep the panic and document that it is a programming error like `template.Must`.

**Effort:** M
**Priority:** P1
**Depends on:** The raw output decision

### Finish the column naming rename and give the wrappers one construction style

**What:** The interface says `ColumnNames()` and `NumColumns()`, but the fields are still `Cols` on `StringsView`, `AnyValuesView`, `ReflectValuesView`, `HeaderView` and `StringsViewer`, `Col` on `SingleReflectValueView`, the parameter is `columns` on `NewStructRowsView`, the method is `StructFieldNaming.Columns`, the parameter is `requiredCols` on `ViewToStructSlice`, and the types are `ExtraColsView`, `SingleColView` and `ExtraColsAnyValueFuncView`. The wrappers have no consistent shape: `FilteredView`, `ExtraColsView` and `ExtraRowsView` are exported structs or slices built as literals; `DerefView`, `ViewWithTitle`, `ExtraColsAnyValueFuncView` and `SingleColView` are constructor functions returning `View` or `ReflectCellView`, inconsistently. `SingleColView`, `SingleCellView` and `SingleReflectValueView` are three types for one column, and the generic one has a runtime special case for `T == reflect.Value`. `FilteredView` has no predicate; it does offset, limit and column projection. `HeaderView` models the header as a one-row view so formatters can run on it, which is an internal trick that is exported.

**Why:** A rename that stops at the interface leaves the package speaking two vocabularies, and it is only free before the tag. The wrapper inconsistency means a reader cannot tell from a name whether they get a value they can mutate or an opaque interface.

**Context:** Recommendation: `Columns` everywhere (field, parameter, type name) to match the method; constructor functions for every wrapper, returning the concrete pointer type, with the struct fields unexported; rename `FilteredView` to `SliceView` or `ProjectionView`; keep one generic single-column type and drop the `reflect.Value` special case once the `ReflectCell` decision is made; unexport `HeaderView` or rename it to say what it is for.

**Effort:** M
**Priority:** P1
**Depends on:** The ReflectCell decision

### Decide the fate of the mutable package globals

**What:** `SelectViewer`, `DefaultParser`, `DefaultStructFieldNaming`, `DefaultStructFieldNamingIgnoreUntagged`, `htmltable.HeaderTemplate`, `RowTemplate` and `FooterTemplate`, and the `htmltable` formatter `var`s of func type (`HTMLPreCellFormatter`, `HTMLPreCodeCellFormatter`, `ValueAsHTMLAnchorCellFormatter`) are all mutable package state.

**Why:** Every one of them is a data race the moment a library and an application both touch it, and the "Passing any Scanner silently discards a configured DefaultParser" item below is the first bug of that class already on the list. `DefaultStructFieldNaming` is the address `SelectViewer` returns, so mutating the var changes every default view in the process (verified).

**Context:** Options per global: make it a function returning a fresh value (`DefaultParser()`), make it a constant-like unexported value with an exported accessor, or make it a parameter. The templates are the hardest case because `WithTemplate` already exists as the per-writer override, so the globals can become unexported defaults. The formatter vars can be funcs. Recommendation: no exported mutable var in the root package at v1.

**Effort:** S
**Priority:** P1
**Depends on:** The Viewer decision, which settles `SelectViewer`

### Decide the fate of sqltable and the subpackage asymmetry

**What:** `sqltable` is a regexp `SELECT` parser whose `parseQuery` returns `offset` and `limit` that are always zero, has 15% coverage, and three consumer call sites. Across the subpackages: CSV reads and writes, HTML only writes, Excel only reads and returns every cell as a string even with `rawCellStrings=false`, and compacts empty rows and columns so indices no longer match the sheet. `exceltable` exports `type ErrSheetNotExist = excelize.ErrSheetNotExist`, leaking the dependency's type into the API. `csvtable.Encoder` is a bespoke `Bytes([]byte) ([]byte, error)` interface where `golang.org/x/text/transform.Transformer` already exists. `csvtable.Writer` defaults to `;` and `\r\n`, a German Excel convention rather than RFC 4180. `csvtable.Format.Encoding` and `FormatDetectionConfig.Encodings` are stringly typed.

**Why:** A v1 promises each of these as they are. `sqltable` in particular is the least tested code in the module and the only subpackage whose defects surface inside `database/sql` where the caller cannot step in.

**Context:** Recommendation: move `sqltable` into its own module like `exceltable`, which lets it version separately and keeps the root module free of it, or drop it if the three call sites can use `ScanRowsAsView` alone. Drop the `ErrSheetNotExist` alias and wrap the excelize error. Keep the `;` default only if the decision is recorded as deliberate; otherwise default to `,` and `\n`. Replace `Encoder` with `transform.Transformer`. Turn the encoding names into a typed enum with the five supported values.

**Effort:** M
**Priority:** P2
**Depends on:** None

### Trim the utility grab-bag in the root package

**What:** `Option` is a bit-flag enum with exactly one value, `OptionAddHeaderRow`, plus a `HasOption` helper, used by one function. `SpacePascalCase` turns `HTTPServer` into `H T T P Server` by documented design and is the default `Untagged` function, while `SpaceGoCase` handles acronyms correctly. There are three empty-row removers with three semantics: `retable.RemoveEmptyStringRows` mutates in place, `csvtable.RemoveEmptyRows` allocates, `csvtable.SetEmptyRowsNil` nils the rows. `csvtable` also exports `CompactSpacedStrings`, `ReplaceNewlineWithSpace`, its typo alias `ReplaceNewlineWithSpacefunc`, `TrimSpace` and `SetRowsWithNonUniformColumnsNil` as ad hoc `[][]string` cleanups. The struct reflection helpers `StructFieldTypes`, `StructFieldReflectValues`, `IndexedStructFieldReflectValues`, `StructFieldAnyValues`, `IndexedStructFieldAnyValues` and `StructFieldIndex` are internals of `StructRowsView`; only `MustStructFieldIndex` is load-bearing, with twenty consumer uses for `WithIgnoreField`.

**Why:** Each of these is a name the tag makes permanent. The `Option` type in particular will look like the start of a system that never came.

**Context:** Recommendation: `OptionAddHeaderRow` becomes a `bool` parameter or the header handling moves into the pipeline; `SpaceGoCase` becomes the default and `SpacePascalCase` is either fixed or removed; one row cleanup family in one package, non-mutating, or as `View` wrappers; unexport the struct reflection helpers except `StructFieldIndex` and `MustStructFieldIndex`.

**Effort:** S
**Priority:** P2
**Depends on:** The reader entry point decision, which owns the row cleanup functions

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

### StructRowsViewer rejects arrays that its error message says it accepts

**What:** `StructRowsViewer.NewView` checks `rows.Kind() != reflect.Slice || rows.Kind() == reflect.Array`, which is true for every array, so `[2]struct{...}` is rejected with "table must be slice or array kind".

**Why:** The doc comment, the error message and `NewStructRowsView`, which accepts arrays, all say arrays work. Reproduced during the API review of 2026-09-06.

**Context:** One-character fix at `structrowsviewer.go:141`: `&&` instead of `||`. Add a test with an array so the message and the check agree.

**Effort:** S
**Priority:** P2
**Depends on:** None

### Wrappers panic where View promises nil

**What:** `View.Cell` documents that out-of-bounds access returns `nil`. At the time of the review, `DerefView.Cell` panicked for an out-of-bounds coordinate ("call of reflect.Value.Elem on zero Value") and for a nil pointer cell ("call of reflect.Value.Interface on zero Value"), `FilteredView.ColumnNames` and `Cell` panicked with an index out of range when `ColumnMapping` held an index the source does not have, and `StringIfTrue` and `RawStringIfTrue` panicked on any non-bool cell. All four reproduced.

**Why:** The `nil`-for-out-of-bounds contract is the reason `View` has no error returns; a wrapper that panics instead breaks that contract for every consumer that wraps a view it did not build. `DerefView`'s own doc lists the panics, which documents the defect rather than fixing it.

**Context:** The out-of-bounds cases of `DerefView` and the `ColumnMapping` case of `FilteredView` are being fixed in the working tree as of 2026-09-06 (see the "Unreleased" section of `CHANGELOG.md`); that change keeps the nil pointer panic in `DerefView.Cell` on purpose, as a defect of the cell rather than of the position. Still open after it: whether a nil pointer read through `DerefView.Cell` should be `nil` too, since a `View` consumer cannot know the cell is a pointer, and the two `StringIfTrue` formatters, which should return `errors.ErrUnsupported` as the raw output decision above sets as the rule. `FilteredView` should additionally validate `ColumnMapping` once, in a constructor, which the "one construction style" item above proposes anyway.

**Effort:** S
**Priority:** P2
**Depends on:** None for the fixes; the constructor and formatter rules above for the shape

### NewHeaderView trims the caller's slice in place

**What:** `NewHeaderView(cols ...string)` writes `strings.TrimSpace(col)` back into `cols[i]`, so a caller that passes a slice with `...` has its slice modified. Reproduced: `[" a ", " b "]` becomes `["a", "b"]` in the caller.

**Why:** A constructor with a hidden side effect on its argument. `NewStringsView` copies before trimming; this one does not.

**Context:** Copy first, as `NewStringsView` does.

**Effort:** S
**Priority:** P3
**Depends on:** None

### RemoveEmptyStringRows corrupts the input's backing array

**What:** `RemoveEmptyStringRows` removes rows with `append(rows[:i], rows[i+1:]...)` and returns the shortened slice, leaving the caller's original slice with a duplicated tail. Reproduced: input `[[a] [] [b]]` returns `[[a] [b]]` and leaves the original as `[[a] [b] [b]]`.

**Why:** In-place slice deletion that the caller cannot see. `exceltable.readSheet` relies on it, so an Excel read leaves `excelize`'s row slice in that state, which is harmless there only because nothing reads it again.

**Context:** Either document the in-place semantics and the aliasing, or allocate. The "utility grab-bag" item above proposes one non-mutating row cleanup family, which would resolve this by deletion.

**Effort:** S
**Priority:** P3
**Depends on:** None

### StringsViewer documents default column names it never produces

**What:** The `StringsViewer` doc says that when `Cols` has fewer entries than the data has columns, "remaining columns will use default names". `NewStringsView` sets the column count to `len(cols)`, so the extra data columns are unreachable: `Cell(0, 1)` returns `nil` for a view with one name over three-column rows. Reproduced.

**Why:** A caller reading the doc will build a partial header and lose data silently.

**Context:** Either generate names for the extra columns, or reject the mismatch with an error from `NewView`, or fix the doc to say the columns are dropped. The third is the smallest and the least useful.

**Effort:** S
**Priority:** P3
**Depends on:** None

## Future adapters

### Add JSON and JSON Lines table readers

**What:** Add a `jsontable` adapter for a normal JSON document containing an array of row objects, and for JSON Lines (`.jsonl`/`.ndjson`) containing one row object per line. Both forms become an in-memory `retable.View`; JSON `null` remains a nil cell, while scalar values retain their JSON types so the `Converter` controls the destination assignment.

**Why:** JSON is a common tabular interchange format, but decoding it directly into a struct slice bypasses the package's header validation, column selection, null handling and error reporting. JSON Lines is especially useful for exports and logs because it can be decoded incrementally even though the final `View` is materialized in memory.

**Context:** Give the reader one options value with a format selector or auto-detection, a maximum input/row limit, and an explicit column policy. Object keys do not provide a schema: with no configured columns, gather keys in first-seen order across the input; with configured columns, preserve that order and either ignore or reject unknown keys. Reject non-object rows and nested arrays/objects by default rather than stringify them implicitly; a deliberate `json.RawMessage` or JSON-text option can add that behavior later. Include fixtures for missing versus null fields, ragged objects, numbers, escapes, duplicate keys, invalid JSON, invalid JSONL line numbers and a final line without a newline.

**Effort:** M
**Priority:** P2
**Depends on:** The `ReadOptions`/single-reader entry-point decision above

### Add Google Sheets and Microsoft Graph Excel readers

**What:** Add separate, optional cloud adapters that fetch a bounded online spreadsheet range once and return an in-memory `retable.View`: `googlesheettable` for the Google Sheets API and `graphtable` for the Microsoft Graph Excel workbook API. Neither adapter owns OAuth, stores refresh tokens or makes a request from `View.Cell`; callers supply an authenticated HTTP client, spreadsheet/workbook identifier and explicit A1 range(s).

**Why:** The core `View` contract is synchronous, error-free and in-memory after construction. A remote-cell implementation would violate that contract with hidden latency, retry and authentication failures. Materializing selected ranges gives Google Sheets and Excel Online the same import path as CSV and XLSX while keeping cloud SDK and OAuth dependencies out of the root module.

**Context:** Google reads should use `spreadsheets.values.batchGet`, with a documented choice between formatted, unformatted and formula values; writes, if added, use `batchUpdate`. Authentication is either user OAuth or a service account shared on the target spreadsheet. Microsoft Graph reads and writes worksheet ranges directly in a OneDrive for Business or SharePoint `.xlsx` workbook using delegated OAuth; its workbook endpoints do not support application-only permissions and do not support `.xls`. For unattended Microsoft imports, use the Graph Drive API to download the file under application permissions and pass it to the file-format adapter instead. Test both adapters against a local HTTP server for pagination, range names with escaping, short rows, rate limits, cancellation and provider error mapping; use separate live smoke tests with non-secret fixtures.

**Effort:** L
**Priority:** P2
**Depends on:** The `ReadOptions`/single-reader entry-point decision above

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
