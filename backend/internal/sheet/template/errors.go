package template

import (
	"errors"
	"fmt"
	"slices"
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

// ExclusiveGroupConflictError reports an update that turns on two or more
// members of the same exclusive group in a single payload. The intent is
// ambiguous (the group allows at most one non-zero member), so the caller must
// resolve it instead of letting the DAG apply both and double-count.
type ExclusiveGroupConflictError struct {
	Group string
	Keys  []string
}

func (e *ExclusiveGroupConflictError) Error() string {
	return fmt.Sprintf("conflicting exclusive options in group '%s': %v", e.Group, e.Keys)
}

// ExclusiveValueTypeError reports a non-numeric value written to a member of an
// exclusive group. Members are 0/1 numeric proxies, so a string, bool or null is
// not a value the group can reason about: treating it as "off" would let a
// payload bypass the at-most-one rule, and letting it through would corrupt the
// numeric formula environment.
type ExclusiveValueTypeError struct {
	Group string
	Key   string
}

func (e *ExclusiveValueTypeError) Error() string {
	return fmt.Sprintf("exclusive option in group '%s' must be numeric: %s", e.Group, e.Key)
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
// instead of backend/shared/apperr: this package must not import apperr — services
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
	// same translation backend/shared/json errors get (surface as HTTP 400).
	ErrTemplateIDNotProvided = errors.New("template id not provided")

	// ErrTemplateIDInvalid reports a template ID that does not have the slug
	// shape code-defined templates use ([a-z][a-z0-9_]*). Its message is
	// user-facing and gets the same 400 translation as ErrTemplateIDNotProvided.
	ErrTemplateIDInvalid = errors.New("template id is not a valid slug")
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

// ErrExclusiveGroupConflict returns an error when one payload turns on two or
// more members of the same exclusive group.
func ErrExclusiveGroupConflict(group string, keys []string) error {
	return &ExclusiveGroupConflictError{Group: group, Keys: slices.Clone(keys)}
}

// ErrExclusiveValueType returns an error when a payload writes a non-numeric
// value to a member of an exclusive group.
func ErrExclusiveValueType(group string, key string) error {
	return &ExclusiveValueTypeError{Group: group, Key: key}
}

// UnknownLayoutSectionError reports a Layout that names a Section the Template
// does not have in section_order.
type UnknownLayoutSectionError struct {
	Key string
}

func (e *UnknownLayoutSectionError) Error() string {
	return fmt.Sprintf("unknown layout section '%s'", e.Key)
}

// UnknownLayoutRowError reports a pinned key that is not a Row of any Section.
type UnknownLayoutRowError struct {
	Key string
}

func (e *UnknownLayoutRowError) Error() string {
	return fmt.Sprintf("unknown pinned row '%s'", e.Key)
}

// DuplicateLayoutKeyError reports a key listed twice in one Layout list. List
// names it (section_order or pinned).
type DuplicateLayoutKeyError struct {
	List string
	Key  string
}

func (e *DuplicateLayoutKeyError) Error() string {
	return fmt.Sprintf("duplicate key '%s' in %s", e.Key, e.List)
}

// UnknownSuggestedPinError reports a suggested pin that is not a Row of any
// Section: an unknown path, a table column, an exclusive-group member or a bare
// Section key.
type UnknownSuggestedPinError struct {
	Key string
}

func (e *UnknownSuggestedPinError) Error() string {
	return fmt.Sprintf("suggested pin '%s' is not a row of any section", e.Key)
}

// DuplicateSuggestedPinError reports a Row listed twice in suggested_pins.
type DuplicateSuggestedPinError struct {
	Key string
}

func (e *DuplicateSuggestedPinError) Error() string {
	return fmt.Sprintf("duplicate suggested pin '%s'", e.Key)
}

// TooManySuggestedPinsError reports a suggested_pins list longer than the
// strip capacity (MaxSuggestedPins).
type TooManySuggestedPinsError struct {
	Count int
	Max   int
}

func (e *TooManySuggestedPinsError) Error() string {
	return fmt.Sprintf("%d suggested pins exceed the maximum of %d", e.Count, e.Max)
}

// UnknownDiceSectionError reports a SectionDice key that is not a Section of
// the Template (a Section with no Row does not count).
type UnknownDiceSectionError struct {
	Key string
}

// Error returns the human-readable reason.
func (e *UnknownDiceSectionError) Error() string {
	return fmt.Sprintf("dice section '%s' is not a section of the template", e.Key)
}

// UnknownDiceRowError reports a RowDice key that is not a Row of any Section:
// an unknown path, a table column, an exclusive member or a bare Section key.
type UnknownDiceRowError struct {
	Key string
}

// Error returns the human-readable reason.
func (e *UnknownDiceRowError) Error() string {
	return fmt.Sprintf("dice row '%s' is not a row of any section", e.Key)
}

// InvalidDiceError reports a Template Dice default the dice grammar refuses, or
// that cannot be rolled where it is attached. Scope is "section" or "row"; Err
// is the *dice.Error, reachable through errors.Is/As and dice.ClassOf.
type InvalidDiceError struct {
	Scope string
	Key   string
	Err   error
}

// Error returns the human-readable reason, naming the Section or Row.
func (e *InvalidDiceError) Error() string {
	return fmt.Sprintf("invalid dice expression on %s '%s': %v", e.Scope, e.Key, e.Err)
}

// Unwrap returns the *dice.Error behind the refusal.
func (e *InvalidDiceError) Unwrap() error {
	return e.Err
}
