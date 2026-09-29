package template

import (
	"errors"
	"fmt"
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

// DependencyError reports a DependsOn referencing a non-existent field.
type DependencyError struct {
	DepPath    []string
	TargetNode string
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("field '%s' depends on non-existent path '%v'", e.TargetNode, e.DepPath)
}

// FormulaCompileError reports a formula that failed to compile, with context.
type FormulaCompileError struct {
	Formula string
	NodeID  string
	Err     error
}

func (e *FormulaCompileError) Error() string {
	return fmt.Sprintf("failed to compile formula '%s' on node '%s': %v", e.Formula, e.NodeID, e.Err)
}

func (e *FormulaCompileError) Unwrap() error {
	return e.Err
}

// UnknownFieldError reports an update key that is not part of the DAG.
type UnknownFieldError struct {
	Key string
}

func (e *UnknownFieldError) Error() string {
	return fmt.Sprintf("unknown field '%s' in update", e.Key)
}

// NotEditableError reports an update key that exists in the DAG but is not
// writable: derived fields are computed from formulas, so accepting them from
// a client would let the payload disagree with the next recalculation.
type NotEditableError struct {
	Key string
}

func (e *NotEditableError) Error() string {
	return "field is not editable: " + e.Key
}

// FieldNameError reports a field name rejected at DAG build time.
// Field names must match ^[A-Za-z][A-Za-z0-9]*$ so the flat DAG map key
// (dot-joined) and the expr-lang engine key (underscore-joined) stay
// collision-free (see LGN-52).
type FieldNameError struct {
	Field  string
	Reason string
}

func (e *FieldNameError) Error() string {
	return fmt.Sprintf("invalid field name '%s': %s", e.Field, e.Reason)
}

// ErrInfiniteCycle and the sentinel errors below are declared in this package
// instead of shared/apperr: this package must not import apperr — services
// translate these typed errors to apperr codes at the layer boundary.
var (
	// ErrInfiniteCycle is deprecated; use ErrCycleDetected/CycleError instead.
	ErrInfiniteCycle = errors.New("an infinite cycle was detected on template formulas")

	// ErrNilFormulaEngine, ErrNilDAG and ErrNilNode report internal invariant
	// violations (always surface as HTTP 500).
	ErrNilFormulaEngine = errors.New("formula engine is required")
	ErrNilDAG           = errors.New("DAG is nil")
	ErrNilNode          = errors.New("node is nil")

	// ErrNilOriginalData and ErrNilUpdatedData report invalid recalculation
	// inputs (surface as HTTP 400).
	ErrNilOriginalData = errors.New("original data is nil")
	ErrNilUpdatedData  = errors.New("updated data is nil")

	// ErrFormulaEval marks runtime formula evaluation failures (bad input types).
	ErrFormulaEval = errors.New("formula evaluation failed")

	// ErrTemplateIDNotProvided reports a template fetch with no ID. Its message
	// is user-facing: the controller boundary turns it into a 400 apperr, the
	// same translation shared/json errors get (surface as HTTP 400).
	ErrTemplateIDNotProvided = errors.New("template id not provided")
)

// NewFormulaCompileError wraps formula compilation errors with context.
func NewFormulaCompileError(formula string, nodeID string, err error) error {
	return &FormulaCompileError{Formula: formula, NodeID: nodeID, Err: err}
}

// ErrCycleDetected returns a CycleError with cycle details for debugging.
// Wraps ErrInfiniteCycle for backward compatibility with error.Is checks.
func ErrCycleDetected(nodes []string) error {
	return &CycleError{Nodes: nodes}
}

// ErrInvalidDependency returns an error when DependsOn references a non-existent field.
func ErrInvalidDependency(depPath []string, targetNode string) error {
	return &DependencyError{DepPath: depPath, TargetNode: targetNode}
}

// ErrInvalidFieldName returns an error when a template field name violates
// the naming rules validated at DAG build time (no '.', no '_', etc.).
func ErrInvalidFieldName(field string, reason string) error {
	return &FieldNameError{Field: field, Reason: reason}
}

// ErrUnknownField returns an error when an update references a field not in the DAG.
func ErrUnknownField(key string) error {
	return &UnknownFieldError{Key: key}
}

// ErrNotEditable returns an error when an update references a field the DAG
// computes itself instead of accepting from the caller.
func ErrNotEditable(key string) error {
	return &NotEditableError{Key: key}
}
