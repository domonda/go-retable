package retable

import (
	"reflect"
)

type Scanner interface {
	ScanString(dest reflect.Value, str string, parser Parser) error
}

type ScannerFunc func(dest reflect.Value, str string, parser Parser) error

func (f ScannerFunc) ScanString(dest reflect.Value, str string, parser Parser) error {
	return f(dest, str, parser)
}
