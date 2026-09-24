package template

import (
	"errors"
	"fmt"
	"strings"

	"legnaris/shared/apperr"
)

// CycleError holds details about a detected cycle.
type CycleError struct {
	Nodes []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("cycle detected involving nodes: %v", e.Nodes)
}

// Unwrap returns ErrInfiniteCycle for backward compatibility with error.Is checks.
func (e *CycleError) Unwrap() error {
	return ErrInfiniteCycle
}

// ErrCompileFormula wraps formula compilation errors with context.
var ErrCompileFormula = func(formula string, nodeID string, err error) error {
	return apperr.NewError(
		apperr.CodeInvalidRequest,
		fmt.Sprintf("failed to compile formula '%s' on node '%s'", formula, nodeID),
		err,
	)
}

// ErrInfiniteCycle is deprecated; use ErrCycleDetected instead.
var ErrInfiniteCycle = errors.New("an infinite cycle was detected on template formulas")

// ErrCycleDetected returns an apperr with cycle details for debugging.
// Wraps ErrInfiniteCycle for backward compatibility with error.Is checks.
func ErrCycleDetected(nodes []string) error {
	return apperr.NewError(
		apperr.CodeCycleDetected,
		fmt.Sprintf("cycle detected involving nodes: %v", nodes),
		&CycleError{Nodes: nodes},
	)
}

// ErrInvalidDependency returns an error when DependsOn references a non-existent field.
func ErrInvalidDependency(depPath []string, targetNode string) error {
	return apperr.NewError(
		apperr.CodeInvalidDependency,
		fmt.Sprintf("field '%s' depends on non-existent path '%v'", targetNode, depPath),
		nil,
	)
}

// ErrInvalidPath returns an error for invalid path operations.
func ErrInvalidPath(path string, reason string) error {
	return apperr.NewError(
		apperr.CodeInvalidPath,
		fmt.Sprintf("invalid path '%s': %s", path, reason),
		nil,
	)
}

// ErrUnknownField returns an error when an update references a field not in the DAG.
func ErrUnknownField(key string) error {
	return apperr.NewError(
		apperr.CodeUnknownField,
		fmt.Sprintf("unknown field '%s' in update", key),
		nil,
	)
}

// pathToString converts a []string path to a flat key string for error messages.
func pathToString(path []string) string {
	return strings.Join(path, ".")
}
