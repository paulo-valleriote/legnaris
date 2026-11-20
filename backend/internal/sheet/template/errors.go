package template

import (
	"errors"
	"fmt"
	"slices"
)

type CycleError struct {
	Nodes []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("cycle detected involving nodes: %v", e.Nodes)
}

func (e *CycleError) Unwrap() error {
	return ErrInfiniteCycle
}

type DependencyError struct {
	DepPath    []string
	TargetNode string
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("field '%s' depends on non-existent path '%v'", e.TargetNode, e.DepPath)
}

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

type UnknownFieldError struct {
	Key string
}

func (e *UnknownFieldError) Error() string {
	return fmt.Sprintf("unknown field '%s' in update", e.Key)
}

type NotEditableError struct {
	Key string
}

func (e *NotEditableError) Error() string {
	return "field is not editable: " + e.Key
}

type ExclusiveGroupConflictError struct {
	Group string
	Keys  []string
}

func (e *ExclusiveGroupConflictError) Error() string {
	return fmt.Sprintf("conflicting exclusive options in group '%s': %v", e.Group, e.Keys)
}

type ExclusiveValueTypeError struct {
	Group string
	Key   string
}

func (e *ExclusiveValueTypeError) Error() string {
	return fmt.Sprintf("exclusive option in group '%s' must be numeric: %s", e.Group, e.Key)
}

type FieldNameError struct {
	Field  string
	Reason string
}

func (e *FieldNameError) Error() string {
	return fmt.Sprintf("invalid field name '%s': %s", e.Field, e.Reason)
}

type TextTooLongError struct {
	Key    string
	Length int
	Max    int
}

func (e *TextTooLongError) Error() string {
	return fmt.Sprintf("field '%s' has %d characters (limit %d)", e.Key, e.Length, e.Max)
}

var (
	ErrInfiniteCycle = errors.New("an infinite cycle was detected on template formulas")

	ErrNilFormulaEngine = errors.New("formula engine is required")
	ErrNilDAG           = errors.New("DAG is nil")
	ErrNilNode          = errors.New("node is nil")

	ErrNilOriginalData = errors.New("original data is nil")
	ErrNilUpdatedData  = errors.New("updated data is nil")

	ErrFormulaEval = errors.New("formula evaluation failed")

	ErrTemplateIDNotProvided = errors.New("template id not provided")

	ErrTemplateIDInvalid = errors.New("template id is not a valid slug")

	ErrTemplateNotFound = errors.New("template not found")
)

func NewFormulaCompileError(formula string, nodeID string, err error) error {
	return &FormulaCompileError{Formula: formula, NodeID: nodeID, Err: err}
}

func ErrCycleDetected(nodes []string) error {
	return &CycleError{Nodes: nodes}
}

func ErrInvalidDependency(depPath []string, targetNode string) error {
	return &DependencyError{DepPath: depPath, TargetNode: targetNode}
}

func ErrInvalidFieldName(field string, reason string) error {
	return &FieldNameError{Field: field, Reason: reason}
}

func ErrUnknownField(key string) error {
	return &UnknownFieldError{Key: key}
}

func ErrTextTooLong(key string, length, maxLength int) error {
	return &TextTooLongError{Key: key, Length: length, Max: maxLength}
}

func ErrNotEditable(key string) error {
	return &NotEditableError{Key: key}
}

func ErrExclusiveGroupConflict(group string, keys []string) error {
	return &ExclusiveGroupConflictError{Group: group, Keys: slices.Clone(keys)}
}

func ErrExclusiveValueType(group string, key string) error {
	return &ExclusiveValueTypeError{Group: group, Key: key}
}

type UnknownLayoutSectionError struct {
	Key string
}

func (e *UnknownLayoutSectionError) Error() string {
	return fmt.Sprintf("unknown layout section '%s'", e.Key)
}

type UnknownLayoutRowError struct {
	Key string
}

func (e *UnknownLayoutRowError) Error() string {
	return fmt.Sprintf("unknown pinned row '%s'", e.Key)
}

type DuplicateLayoutKeyError struct {
	List string
	Key  string
}

func (e *DuplicateLayoutKeyError) Error() string {
	return fmt.Sprintf("duplicate key '%s' in %s", e.Key, e.List)
}

type UnknownDiceSectionError struct {
	Key string
}

func (e *UnknownDiceSectionError) Error() string {
	return fmt.Sprintf("dice section '%s' is not a section of the template", e.Key)
}

type UnknownDiceRowError struct {
	Key string
}

func (e *UnknownDiceRowError) Error() string {
	return fmt.Sprintf("dice row '%s' is not a row of any section", e.Key)
}

type InvalidDiceError struct {
	Scope string
	Key   string
	Err   error
}

func (e *InvalidDiceError) Error() string {
	return fmt.Sprintf("invalid dice expression on %s '%s': %v", e.Scope, e.Key, e.Err)
}

func (e *InvalidDiceError) Unwrap() error {
	return e.Err
}

type TooManyDiceOverridesError struct {
	Count int
	Max   int
}

func (e *TooManyDiceOverridesError) Error() string {
	return fmt.Sprintf("%d dice overrides exceed the maximum of %d", e.Count, e.Max)
}
