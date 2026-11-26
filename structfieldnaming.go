package retable

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
)

var _ Viewer = new(StructFieldNaming)

// StructFieldNaming defines how struct fields are mapped to column titles
// as used by View. It provides flexible control over which struct fields
// become columns and what their column titles are through struct tags
// and custom naming functions.
//
// The mapping process follows these rules:
//   - Only exported (public) struct fields are considered for column mapping
//   - Anonymous embedded struct fields are recursively processed, with their fields inlined
//   - If a Tag is specified, the struct tag value is used as the column title
//   - For fields without tags (or if Tag is empty), the Untagged function determines the column title
//   - Fields matching the Ignore value are excluded from the table
//
// Example usage with struct tags:
//
//	type Person struct {
//	    FirstName string `csv:"first_name"`
//	    LastName  string `csv:"last_name"`
//	    Age       int    `csv:"age"`
//	    Internal  string `csv:"-"` // ignored when Ignore is set to "-"
//	}
//
//	naming := &StructFieldNaming{
//	    Tag:    "csv",
//	    Ignore: "-",
//	}
//	columns := naming.Columns(&Person{}) // ["first_name", "last_name", "age"]
//
// Example with custom naming function:
//
//	naming := &StructFieldNaming{
//	    Untagged: SpacePascalCase, // "FirstName" becomes "First Name"
//	}
//
// A nil *StructFieldNaming is a valid value and is equivalent to the zero value,
// which will use all exported struct fields with their field name as column title.
//
// StructFieldNaming implements the Viewer interface, allowing it to create
// Views from slices or arrays of structs.
type StructFieldNaming struct {
	// Tag is the struct field tag to be used as column title.
	// When non-empty, struct fields are checked for this tag name,
	// and the tag value is used as the column title.
	//
	// Tag values support comma-separated options, where only the first
	// part before the comma is used as the column title:
	//   `json:"name,omitempty"` results in column title "name"
	//
	// If Tag is empty, then every struct field will be treated as untagged.
	Tag string

	// Ignore specifies a column title value that marks fields to be excluded.
	// Any field whose column title matches this value will be ignored and
	// not appear in the resulting table view.
	//
	// Common patterns:
	//   - Use "-" to match the convention `json:"-"` for ignored fields
	//   - Use "" (empty string) to ignore all unexported or empty-titled fields
	Ignore string

	// Untagged is called with the struct field name to generate a column title
	// when the field has no tag matching Tag (or Tag is empty).
	//
	// If Untagged is nil, the raw struct field name is used as the column title.
	//
	// Common functions to use:
	//   - SpacePascalCase: "FirstName" -> "First Name"
	//   - SpaceGoCase: "HTTPServer" -> "HTTP Server"
	//   - Custom function for domain-specific naming conventions
	Untagged func(fieldName string) (column string)

	// TitleMatcher reports whether the column title of a view, or of a row
	// of raw cells, matches the column title of a struct field.
	// It is used by every method that looks up a column by its title:
	// ColumnStructFieldValue, ColumnTitlesMatch, CheckUniqueColumns and
	// FindTitleRow, and by ViewToStructSlice and the
	// csvtable Read*ToStructSlice functions.
	//
	// TitleMatcher sets how loose the matching is:
	//   - nil: titles must be equal
	//   - TrimEqual: equal without surrounding whitespace
	//   - TrimEqualFold: equal without surrounding whitespace, ignoring
	//     case, which is what column titles typed by people usually need
	//   - a custom function, for example to accept alternative titles
	//
	// If several columns match the title of one field, the closest match
	// wins, whatever their order: an equal title before one that is equal
	// without surrounding whitespace, before one that also differs in case,
	// before one that only TitleMatcher accepts. A field title that fits a
	// column exactly is therefore never taken over by a looser match.
	// PreferFirstColumn decides between equally close columns,
	// and DisallowAmbiguousTitles makes all of these cases an error.
	//
	// Example:
	//
	//	naming := &StructFieldNaming{Tag: "col", Ignore: "-", TitleMatcher: TrimEqualFold}
	//	naming.ColumnTitlesMatch(" first name ", "First Name") // true
	TitleMatcher func(viewTitle, fieldTitle string) bool

	// DisallowAmbiguousTitles makes it an error wrapping
	// ErrAmbiguousColumnTitle if the columns of a view, or the cells of a
	// title row, can't be assigned to the struct fields in only one way:
	// more than one column matches the title of the same field,
	// or one column matches the titles of more than one field.
	// ViewToStructSlice, the csvtable Read*ToStructSlice functions and
	// FindTitleRow return that error instead of choosing a column
	// by the rules described for TitleMatcher.
	//
	// The default false resolves these cases by the rules.
	DisallowAmbiguousTitles bool

	// PreferFirstColumn decides which of several columns wins that match
	// the title of the same struct field equally closely, see TitleMatcher,
	// like two columns of the same name: the first one if true,
	// the last one with the default false.
	//
	// ViewToStructSlice and the csvtable Read*ToStructSlice functions
	// decide that for every row among the cells that have content,
	// see ViewToStructSlice, FindTitleRow decides it for the cells
	// of the title row.
	PreferFirstColumn bool
}

var (
	// ErrNoColumns is returned by StructFieldNaming.FindTitleRow
	// for a struct without any column.
	ErrNoColumns = errors.New("struct has no columns")

	// ErrDuplicateColumnTitle is returned by StructFieldNaming.CheckUniqueColumns
	// and StructFieldNaming.FindTitleRow, and by ViewToStructSlice for a naming
	// with a TitleMatcher, when two struct fields have column titles
	// that StructFieldNaming.ColumnTitlesMatch reports as matching,
	// so that a column of a view could not be assigned to one field.
	ErrDuplicateColumnTitle = errors.New("duplicate column title")

	// ErrTitleRowNotFound is returned by StructFieldNaming.FindTitleRow
	// when no row contains the titles of all columns.
	ErrTitleRowNotFound = errors.New("no row contains all column titles")

	// ErrAmbiguousColumnTitle is returned by ViewToStructSlice and
	// StructFieldNaming.FindTitleRow for a naming with
	// DisallowAmbiguousTitles, when more than one column matches the title
	// of the same struct field or one column matches the titles of more
	// than one field. In contrast to ErrDuplicateColumnTitle,
	// which is a mistake in the struct, it depends on the table that is read.
	ErrAmbiguousColumnTitle = errors.New("ambiguous column title")
)

// TrimEqual reports whether a and b are equal after removing their
// leading and trailing white space, as defined by Unicode.
// In contrast to TrimEqualFold it is case sensitive.
//
// Use it as StructFieldNaming.TitleMatcher to only ignore
// the whitespace around column titles.
//
// Example:
//
//	TrimEqual(" Name\t", "Name") // true
//	TrimEqual("Name", "NAME")    // false
func TrimEqual(a, b string) bool {
	return strings.TrimSpace(a) == strings.TrimSpace(b)
}

// TrimEqualFold reports whether a and b are equal after removing their
// leading and trailing white space, as defined by Unicode,
// and ignoring case, using strings.EqualFold.
//
// Use it as StructFieldNaming.TitleMatcher to match column titles loosely,
// the way csvtable.DetectTableBounds compares titles.
//
// Example:
//
//	TrimEqualFold(" Name\t", "NAME") // true
//	TrimEqualFold("Name", "Names")   // false
func TrimEqualFold(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// String implements the fmt.Stringer interface for StructFieldNaming.
//
// Valid to call with nil receiver.
func (n *StructFieldNaming) String() string {
	if n == nil {
		return `StructFieldNaming{Tag: "", Ignore: ""}`
	}
	return fmt.Sprintf("StructFieldNaming{Tag: %#v, Ignore: %#v}", n.Tag, n.Ignore)
}

// StructFieldColumn returns the column title for a struct field using
// the naming rules defined in the StructFieldNaming configuration.
//
// The function applies the following logic:
//   - Returns empty string for unexported or anonymous fields
//   - If Tag is set and the field has that tag, uses the tag value (before any comma)
//   - Otherwise calls Untagged function if set, or uses the field name
//
// This method is safe to call with a nil receiver, in which case it returns
// the field name for exported fields and empty string for unexported fields.
func (n *StructFieldNaming) StructFieldColumn(field reflect.StructField) string {
	if !field.IsExported() || field.Anonymous {
		return ""
	}
	if n == nil {
		return field.Name
	}
	if n.Tag != "" {
		if tag, ok := field.Tag.Lookup(n.Tag); ok {
			if i := strings.IndexByte(tag, ','); i != -1 {
				tag = tag[:i]
			}
			if tag != "" {
				return tag
			}
		}
	}
	if n.Untagged == nil {
		return field.Name
	}
	return n.Untagged(field.Name)
}

// IsIgnored returns true if the given column title should be ignored
// and excluded from the table view.
//
// A column is considered ignored if:
//   - The column title is an empty string
//   - The column title matches the Ignore field value (when n is not nil)
//
// This method is safe to call with a nil receiver.
func (n *StructFieldNaming) IsIgnored(column string) bool {
	return column == "" || (n != nil && column == n.Ignore)
}

// ColumnTitlesMatch reports whether viewTitle, the column title of a view or
// of a row of raw cells, matches fieldTitle, the column title of a struct
// field, using TitleMatcher or comparing them for equality if it is nil.
//
// This method is safe to call with a nil receiver,
// which compares for equality.
//
// Example:
//
//	exact := &StructFieldNaming{Tag: "col"}
//	exact.ColumnTitlesMatch("name ", "name") // false
//
//	loose := &StructFieldNaming{Tag: "col", TitleMatcher: TrimEqualFold}
//	loose.ColumnTitlesMatch("Name ", "name") // true
func (n *StructFieldNaming) ColumnTitlesMatch(viewTitle, fieldTitle string) bool {
	if n == nil || n.TitleMatcher == nil {
		return viewTitle == fieldTitle
	}
	return n.TitleMatcher(viewTitle, fieldTitle)
}

// titleMatchRank says how closely a view title matches a field title.
// A lower rank is a closer match.
type titleMatchRank int

const (
	titleMatchEqual     titleMatchRank = iota // the titles are equal
	titleMatchTrimEqual                       // equal without surrounding whitespace
	titleMatchTrimFold                        // equal without surrounding whitespace and case
	titleMatchCustom                          // only TitleMatcher says that they match
	titleMatchNone                            // ColumnTitlesMatch returns false
)

// titleMatchRank returns how closely viewTitle matches fieldTitle,
// or titleMatchNone if ColumnTitlesMatch returns false for them.
// TitleMatcher decides if titles match, the rank only orders
// the titles it accepts.
func (n *StructFieldNaming) titleMatchRank(viewTitle, fieldTitle string) titleMatchRank {
	switch {
	case !n.ColumnTitlesMatch(viewTitle, fieldTitle):
		return titleMatchNone
	case viewTitle == fieldTitle:
		return titleMatchEqual
	case TrimEqual(viewTitle, fieldTitle):
		return titleMatchTrimEqual
	case TrimEqualFold(viewTitle, fieldTitle):
		return titleMatchTrimFold
	default:
		return titleMatchCustom
	}
}

// disallowAmbiguousTitles returns DisallowAmbiguousTitles, false for nil.
func (n *StructFieldNaming) disallowAmbiguousTitles() bool {
	return n != nil && n.DisallowAmbiguousTitles
}

// preferFirstColumn returns PreferFirstColumn, false for nil.
func (n *StructFieldNaming) preferFirstColumn() bool {
	return n != nil && n.PreferFirstColumn
}

// fieldTitleMatch is a struct field whose column title matches a view title.
type fieldTitleMatch struct {
	index []int          // of the field, for reflect.Value.FieldByIndex
	title string         // column title of the field
	rank  titleMatchRank // how closely the view title matches title
}

// appendFieldTitleMatches appends the fields of the struct type t whose
// column titles match column, in the order of the fields, with the fields
// of anonymously embedded structs at their position. parentIndex is the
// index of the embedded struct t within the outermost struct.
//
// Panics if t is not a struct or pointer to struct type.
func (n *StructFieldNaming) appendFieldTitleMatches(matches []fieldTitleMatch, t reflect.Type, parentIndex []int, column string) []fieldTitleMatch {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic("expected struct or pointer to struct instead of " + t.String())
	}
	for i := range t.NumField() {
		field := t.Field(i)
		if field.Anonymous {
			if isEmbeddedStruct(field) {
				matches = n.appendFieldTitleMatches(matches, field.Type, append(slices.Clip(parentIndex), i), column)
			}
			continue
		}
		title := n.StructFieldColumn(field)
		if n.IsIgnored(title) {
			continue
		}
		if rank := n.titleMatchRank(column, title); rank != titleMatchNone {
			matches = append(matches, fieldTitleMatch{
				index: append(slices.Clip(parentIndex), i),
				title: title,
				rank:  rank,
			})
		}
	}
	return matches
}

// closestFieldTitleMatch returns the match with the lowest rank,
// the first one of several with that rank, or false without matches.
func closestFieldTitleMatch(matches []fieldTitleMatch) (closest fieldTitleMatch, ok bool) {
	for i, match := range matches {
		if i == 0 || match.rank < closest.rank {
			closest = match
		}
	}
	return closest, len(matches) > 0
}

// ColumnStructFieldValue returns the reflect.Value of the struct field
// that is mapped to the given column title.
//
// The function searches through all exported struct fields (including
// those from anonymously embedded structs) to find the field whose
// column title matches the given column parameter, see ColumnTitlesMatch.
// If the titles of several fields match, which only a TitleMatcher or
// equal field titles make possible, the closest match wins as described
// for TitleMatcher, and the first field of several equally close ones.
//
// Anonymous embedded structs are recursively searched, allowing fields
// from nested structs to be accessed by their column title.
//
// Returns an invalid reflect.Value if:
//   - The column is ignored (empty or matches Ignore)
//   - No struct field maps to the given column title
//   - The field belongs to an embedded struct pointer that is nil
//   - structVal is not a struct or pointer to struct
//
// Panics if structVal is not a struct or pointer to struct type.
//
// This method is safe to call with a nil receiver.
//
// Example:
//
//	type Person struct {
//	    Name string `csv:"full_name"`
//	    Age  int    `csv:"age"`
//	}
//	naming := &StructFieldNaming{Tag: "csv"}
//	person := Person{Name: "John", Age: 30}
//	val := naming.ColumnStructFieldValue(reflect.ValueOf(person), "full_name")
//	// val.String() == "John"
func (n *StructFieldNaming) ColumnStructFieldValue(structVal reflect.Value, column string) reflect.Value {
	if n.IsIgnored(column) {
		return reflect.Value{}
	}
	if structVal.Kind() == reflect.Pointer {
		structVal = structVal.Elem()
	}
	structType := structVal.Type()
	if structType.Kind() != reflect.Struct {
		panic("expected struct or pointer to struct instead of " + structVal.Type().String())
	}
	match, ok := closestFieldTitleMatch(n.appendFieldTitleMatches(nil, structType, nil, column))
	if !ok {
		return reflect.Value{}
	}
	field, err := structVal.FieldByIndexErr(match.index)
	if err != nil {
		// A nil pointer to an embedded struct has no field values
		return reflect.Value{}
	}
	return field
}

// Columns returns the column titles for all non-ignored exported fields
// of a struct or pointer to a struct.
//
// The returned slice contains column titles in the order they appear in
// the struct definition, with fields from anonymously embedded structs
// inlined in their position.
//
// Fields are processed according to the StructFieldNaming rules:
//   - Tag determines which struct tag to read for column titles
//   - Untagged function transforms field names when no tag is present
//   - Ignore value filters out unwanted columns
//
// Panics if strct is not a struct or pointer to struct type.
//
// This method is safe to call with a nil receiver, in which case it
// returns all exported field names without transformation.
//
// Example:
//
//	type User struct {
//	    ID       int    `db:"user_id"`
//	    Username string `db:"username"`
//	    Password string `db:"-"`
//	}
//	naming := &StructFieldNaming{Tag: "db", Ignore: "-"}
//	cols := naming.Columns(&User{}) // ["user_id", "username"]
func (n *StructFieldNaming) Columns(strct any) []string {
	return n.columns(reflect.TypeOf(strct))
}

func (n *StructFieldNaming) columns(t reflect.Type) []string {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic("expected struct or pointer to struct instead of " + t.String())
	}
	columns := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		field := t.Field(i)
		if field.Anonymous {
			if isEmbeddedStruct(field) {
				columns = append(columns, n.columns(field.Type)...)
			}
			continue
		}
		column := n.StructFieldColumn(field)
		if !n.IsIgnored(column) {
			columns = append(columns, column)
		}
	}
	return columns
}

// CheckUniqueColumns returns an error wrapping ErrDuplicateColumnTitle
// if two columns of strct, a struct or pointer to struct, have titles
// that match each other according to ColumnTitlesMatch, naming both titles.
// A column of a view could then only be assigned to the first of them.
//
// With the default exact matching that means two equal titles, with
// TrimEqualFold as TitleMatcher also titles like "Name" and " name".
//
// FindTitleRow runs this check, and ViewToStructSlice does
// for a naming with a TitleMatcher.
//
// Panics if strct is not a struct or pointer to struct type.
//
// This method is safe to call with a nil receiver.
//
// Example:
//
//	type Account struct {
//	    Number string `col:"Number"`
//	    Code   string `col:"number"`
//	}
//	naming := &StructFieldNaming{Tag: "col", TitleMatcher: TrimEqualFold}
//	err := naming.CheckUniqueColumns(Account{})
//	// errors.Is(err, ErrDuplicateColumnTitle) == true
func (n *StructFieldNaming) CheckUniqueColumns(strct any) error {
	return n.checkUniqueTitles(n.Columns(strct), strct)
}

// checkUniqueTitles implements CheckUniqueColumns for the already
// determined column titles of strct, which is only used in the error.
func (n *StructFieldNaming) checkUniqueTitles(titles []string, strct any) error {
	for i, title := range titles {
		for _, other := range titles[i+1:] {
			// Both argument orders, because TitleMatcher
			// does not have to be symmetric
			if n.ColumnTitlesMatch(other, title) || n.ColumnTitlesMatch(title, other) {
				return fmt.Errorf("%w: %q and %q of %T", ErrDuplicateColumnTitle, title, other, strct)
			}
		}
	}
	return nil
}

// FindTitleRow finds the row of column titles in rows of raw cells,
// as parsed from a CSV or read from a spreadsheet, for the columns of
// strct, a struct or pointer to struct, see Columns.
//
// The title row is the first row that contains a cell matching the
// title of every column of strct according to ColumnTitlesMatch.
// The cells can be in any order and the row can have additional cells,
// the number of cells of a row does not matter.
//
// It returns the index of that row and the index of the cell of each
// column title within the row: columns[i] is the cell index of the
// title Columns(strct)[i]. If several cells of the row match a title,
// it is the closest match as described for TitleMatcher, an equal cell
// before a looser match, and of several equally close cells the last one,
// or the first one with PreferFirstColumn.
// With DisallowAmbiguousTitles a title row with several cells matching
// one title, or one cell matching several titles, is an error instead.
// A FilteredView with columns as ColumnMapping turns the rows after
// titleRow into a view with exactly the columns of strct,
// see the example below.
//
// This differs from DetectTableBounds, which looks for the
// table's column count first, as the one of the majority of rows,
// and then takes the row of that count matching the most expected
// titles, so a title row with some titles missing is still found,
// and which also finds the end of the table.
// FindTitleRow requires every title, which makes it reject a file
// missing a column instead of reading it with an empty field,
// and finds the title row also when the rows have different counts.
//
// Errors:
//   - ErrNoColumns if strct has no columns
//   - ErrDuplicateColumnTitle if two columns match, see CheckUniqueColumns
//   - ErrTitleRowNotFound if no row contains all titles
//   - ErrAmbiguousColumnTitle if DisallowAmbiguousTitles is set
//     and the title row has more than one candidate cell for a title
//
// Panics if strct is not a struct or pointer to struct type.
//
// This method is safe to call with a nil receiver.
//
// Example:
//
//	type Account struct {
//	    Number string `col:"Konto"`
//	    Name   string `col:"Bezeichnung"`
//	}
//	rows := [][]string{
//	    {"Firma: 200670"},
//	    {"Bezeichnung", "Kontoart", " KONTO "},
//	    {"Software", "1", "120"},
//	}
//	naming := &StructFieldNaming{
//	    Tag:        "col",
//	    Ignore:     "-",
//	    Untagged:   func(string) string { return "" }, // only tagged fields are columns
//	    TitleMatcher: TrimEqualFold,
//	}
//	titleRow, columns, err := naming.FindTitleRow(rows, Account{})
//	// titleRow == 1, columns == []int{2, 0}
//
//	view := &FilteredView{
//	    Source:        NewStringsView("", rows[titleRow+1:], rows[titleRow]...),
//	    ColumnMapping: columns,
//	}
//	accounts, err := ViewToStructSlice[Account](view, naming, nil, nil, nil, nil)
//	// accounts == []Account{{Number: "120", Name: "Software"}}
func (n *StructFieldNaming) FindTitleRow(rows [][]string, strct any) (titleRow int, columns []int, err error) {
	titles := n.Columns(strct)
	if len(titles) == 0 {
		return -1, nil, fmt.Errorf("%w: %T", ErrNoColumns, strct)
	}
	err = n.checkUniqueTitles(titles, strct)
	if err != nil {
		return -1, nil, err
	}
	for rowIndex, row := range rows {
		columns, ok := n.titleCellIndices(row, titles)
		if !ok {
			continue
		}
		if n.disallowAmbiguousTitles() {
			err = n.checkUnambiguousTitleCells(row, titles, strct)
			if err != nil {
				return -1, nil, err
			}
		}
		return rowIndex, columns, nil
	}
	return -1, nil, fmt.Errorf("%w %q of %T", ErrTitleRowNotFound, titles, strct)
}

// titleCellIndices returns the index of the cell of row that matches
// each of titles the closest, of several equally close cells the last
// one, or the first one with PreferFirstColumn, or false if a title
// has no matching cell.
func (n *StructFieldNaming) titleCellIndices(row, titles []string) (indices []int, ok bool) {
	indices = make([]int, len(titles))
	for i, title := range titles {
		indices[i] = -1
		closest := titleMatchNone
		for cellIndex, cell := range row {
			rank := n.titleMatchRank(cell, title)
			if rank == titleMatchNone {
				continue
			}
			if rank < closest || (rank == closest && !n.preferFirstColumn()) {
				indices[i], closest = cellIndex, rank
			}
		}
		if indices[i] == -1 {
			return nil, false
		}
	}
	return indices, true
}

// checkUnambiguousTitleCells returns an error wrapping
// ErrAmbiguousColumnTitle if more than one cell of row matches one
// of titles, or if one cell matches more than one of titles.
// strct is only used in the error.
func (n *StructFieldNaming) checkUnambiguousTitleCells(row, titles []string, strct any) error {
	for _, title := range titles {
		first := -1
		for cellIndex, cell := range row {
			if !n.ColumnTitlesMatch(cell, title) {
				continue
			}
			if first != -1 {
				return fmt.Errorf("%w: columns %q and %q both match %q of %T", ErrAmbiguousColumnTitle, row[first], cell, title, strct)
			}
			first = cellIndex
		}
	}
	for _, cell := range row {
		first := ""
		for _, title := range titles {
			if !n.ColumnTitlesMatch(cell, title) {
				continue
			}
			if first != "" {
				return fmt.Errorf("%w: column %q matches both %q and %q of %T", ErrAmbiguousColumnTitle, cell, first, title, strct)
			}
			first = title
		}
	}
	return nil
}

// NewView returns a View for a table made up of a slice or array of structs.
//
// This method implements the Viewer interface for StructFieldNaming, creating
// a StructRowsView by delegating to StructRowsViewer with this naming configuration.
//
// Parameters:
//   - title: The title for the table view
//   - table: A slice or array of structs (or pointers to structs)
//
// Returns an error if table is not a slice or array, or if the element type
// is not a struct or pointer to struct.
//
// Example:
//
//	type Product struct {
//	    Name  string `json:"name"`
//	    Price float64 `json:"price"`
//	}
//	naming := &StructFieldNaming{Tag: "json"}
//	products := []Product{{"Widget", 9.99}, {"Gadget", 19.99}}
//	view, err := naming.NewView("Products", products)
func (n *StructFieldNaming) NewView(title string, table any) (View, error) {
	viewer := StructRowsViewer{StructFieldNaming: *n}
	return viewer.NewView(title, table)
}

// WithTag returns a copy of the StructFieldNaming with the Tag field set
// to the specified value.
//
// This method allows fluent configuration of the naming rules.
//
// Example:
//
//	naming := (&StructFieldNaming{}).WithTag("json").WithIgnore("-")
func (n *StructFieldNaming) WithTag(tag string) *StructFieldNaming {
	mod := *n
	mod.Tag = tag
	return &mod
}

// WithTitleMatcher returns a copy of the StructFieldNaming with the
// TitleMatcher field set to the specified function.
// Pass nil to match titles exactly.
//
// Example:
//
//	naming := DefaultStructFieldNaming.WithTitleMatcher(TrimEqualFold)
func (n *StructFieldNaming) WithTitleMatcher(matcher func(viewTitle, fieldTitle string) bool) *StructFieldNaming {
	mod := *n
	mod.TitleMatcher = matcher
	return &mod
}

// WithDisallowAmbiguousTitles returns a copy of the StructFieldNaming
// with the DisallowAmbiguousTitles field set to the specified value.
//
// Example:
//
//	naming := DefaultStructFieldNaming.
//	    WithTitleMatcher(TrimEqualFold).
//	    WithDisallowAmbiguousTitles(true)
func (n *StructFieldNaming) WithDisallowAmbiguousTitles(disallow bool) *StructFieldNaming {
	mod := *n
	mod.DisallowAmbiguousTitles = disallow
	return &mod
}

// WithPreferFirstColumn returns a copy of the StructFieldNaming
// with the PreferFirstColumn field set to the specified value.
//
// Example:
//
//	naming := DefaultStructFieldNaming.
//	    WithTitleMatcher(TrimEqualFold).
//	    WithPreferFirstColumn(true)
func (n *StructFieldNaming) WithPreferFirstColumn(preferFirst bool) *StructFieldNaming {
	mod := *n
	mod.PreferFirstColumn = preferFirst
	return &mod
}

// WithIgnore returns a copy of the StructFieldNaming with the Ignore field
// set to the specified value.
//
// This method allows fluent configuration of the naming rules.
//
// Example:
//
//	naming := (&StructFieldNaming{Tag: "csv"}).WithIgnore("-")
func (n *StructFieldNaming) WithIgnore(ignore string) *StructFieldNaming {
	mod := *n
	mod.Ignore = ignore
	return &mod
}
