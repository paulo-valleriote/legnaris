package engine

import "errors"

var (
	ErrNilProgram   = errors.New("compiled program is nil")
	ErrInvalidFloat = errors.New("invalid float received on function")
	ErrCompile      = errors.New("failed to compile formula")
	ErrEvaluate     = errors.New("failed to evaluate formula")
)
