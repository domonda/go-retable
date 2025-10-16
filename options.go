package retable

import "strings"

// Option represents formatting options that can be applied when rendering tables.
// Options are implemented as bit flags and can be combined using bitwise OR.
type Option int

const (
	// OptionAddHeaderRow adds a header row containing column titles
	// as the first row in the formatted output.
	//
	// When this option is set, the View's Columns() method is called to get column titles,
	// and these are rendered as the first row before any data rows.
	//
	// Example:
	//
	//	rows, err := FormatViewAsStrings(ctx, view, nil, OptionAddHeaderRow)
	//	// First row will contain: ["Column1", "Column2", "Column3"]
	//	// Subsequent rows contain data
	OptionAddHeaderRow Option = 1 << iota
)

// Has checks whether this Option has the specified option flag set.
// Since Option is a bit flag type, multiple options can be combined
// and this method tests if a specific option is present.
//
// Example:
//
//	opts := OptionAddHeaderRow
//	if opts.Has(OptionAddHeaderRow) {
//	    // Header row option is enabled
//	}
func (o Option) Has(option Option) bool {
	return o&option != 0
}

// String returns a human-readable string representation of the Option.
// Multiple options are joined with "|". If no options are set, returns "no Option".
//
// Example:
//
//	opt := OptionAddHeaderRow
//	fmt.Println(opt.String()) // Output: "AddHeaderRow"
//
//	opt = 0
//	fmt.Println(opt.String()) // Output: "no Option"
func (o Option) String() string {
	var b strings.Builder
	if o.Has(OptionAddHeaderRow) {
		if b.Len() > 0 {
			b.WriteString("|")
		}
		b.WriteString("AddHeaderRow")
	}
	if b.Len() == 0 {
		return "no Option"
	}
	return b.String()
}

// HasOption checks whether any Option in the slice has the specified option flag set.
// Returns true if at least one Option in the slice contains the given option.
//
// This is a convenience function for checking options in a slice of Options,
// which is commonly used when passing variadic option parameters.
//
// Example:
//
//	options := []Option{OptionAddHeaderRow}
//	if HasOption(options, OptionAddHeaderRow) {
//	    // Header row option is present in the slice
//	}
//
//	// With variadic parameters:
//	func formatTable(data any, options ...Option) {
//	    if HasOption(options, OptionAddHeaderRow) {
//	        // Add header row
//	    }
//	}
func HasOption(options []Option, option Option) bool {
	for _, o := range options {
		if o.Has(option) {
			return true
		}
	}
	return false
}
