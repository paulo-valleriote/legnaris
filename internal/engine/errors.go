package engine

import "errors"

var (
	ErrNilProgram   = errors.New("compiled program is nil")
	ErrInvalidFLoat = errors.New("invalid float received on function")
)
