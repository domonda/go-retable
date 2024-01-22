package exceltable

import (
	"errors"

	"github.com/xuri/excelize/v2"
)

var (
	// ErrEmptySheet indicates that an Excel sheet is empty
	ErrEmptySheet = errors.New("empty sheet")
)

type ErrSheetNotExist = excelize.ErrSheetNotExist
