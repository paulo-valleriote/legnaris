package engine

import "errors"

var (
	ErrNilProgram            = errors.New("compiled program is nil")
	ErrInvalidFloat          = errors.New("invalid float received on function")
	ErrInvalidLookupArity    = errors.New("lookup received an invalid number of arguments")
	ErrInvalidLookupKeyOrder = errors.New("lookup keys must be strictly ascending")
	ErrCompile               = errors.New("failed to compile formula")
	ErrEvaluate              = errors.New("failed to evaluate formula")
)
