package retable

import "strings"

type Option int

const (
	OptionAddHeaderRow Option = 1 << iota
)

func (o Option) Has(option Option) bool {
	return o&option != 0
}

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

func HasOption(options []Option, option Option) bool {
	for _, o := range options {
		if o.Has(option) {
			return true
		}
	}
	return false
}
