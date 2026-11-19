package sheeterr

import "errors"

var (
	ErrSheetNotFound     = errors.New("sheet not found")
	ErrVersionConflict   = errors.New("sheet version conflict")
	ErrSheetLimitReached = errors.New("sheet limit reached")
)
