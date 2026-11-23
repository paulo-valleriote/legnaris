package template

import (
	"errors"
	"fmt"
	"slices"
)

type InvalidTemplateStateError struct {
	TemplateID string
	State      State
}

func (e *InvalidTemplateStateError) Error() string {
	if e.State == "" {
		return fmt.Sprintf("template '%s' has no lifecycle state", e.TemplateID)
	}
	return fmt.Sprintf("template '%s' has unknown lifecycle state '%s'", e.TemplateID, e.State)
}

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

type FlagValueError struct {
	Key string
}

func (e *FlagValueError) Error() string {
	return fmt.Sprintf("field '%s' must be 0 or 1", e.Key)
}

type TextTooLongError struct {
	Key    string
	Length int
	Max    int
}

func (e *TextTooLongError) Error() string {
	return fmt.Sprintf("field '%s' has %d characters (limit %d)", e.Key, e.Length, e.Max)
}

type ReservedListFieldError struct {
	Path  string
	Field string
}

func (e *ReservedListFieldError) Error() string {
	return fmt.Sprintf("list '%s' uses the reserved shape field '%s'", e.Path, e.Field)
}

type NestedListError struct {
	Path string
}

func (e *NestedListError) Error() string {
	return fmt.Sprintf("list '%s' is nested inside another list", e.Path)
}

type InvalidListShapeError struct {
	Path   string
	Reason string
}

func (e *InvalidListShapeError) Error() string {
	return fmt.Sprintf("invalid list shape at '%s': %s", e.Path, e.Reason)
}

type ListPathTooDeepError struct {
	Path string
	Max  int
}

func (e *ListPathTooDeepError) Error() string {
	return fmt.Sprintf("list path '%s' is deeper than %d segments", e.Path, e.Max)
}

type ListCapacityError struct {
	Path     string
	Capacity int
	Min      int
	Max      int
}

func (e *ListCapacityError) Error() string {
	return fmt.Sprintf(
		"list '%s' capacity %d is outside %d-%d",
		e.Path, e.Capacity, e.Min, e.Max,
	)
}

type InvalidAggregateError struct {
	Path   string
	Reason string
}

func (e *InvalidAggregateError) Error() string {
	return fmt.Sprintf("invalid aggregate '%s': %s", e.Path, e.Reason)
}

type InvalidResetActionError struct {
	Action string
	Reason string
}

func (e *InvalidResetActionError) Error() string {
	return fmt.Sprintf("invalid reset action '%s': %s", e.Action, e.Reason)
}

type UnknownResetActionError struct {
	ID string
}

func (e *UnknownResetActionError) Error() string {
	return fmt.Sprintf("unknown reset action '%s'", e.ID)
}

type RecordIDError struct {
	ID string
}

func (e *RecordIDError) Error() string {
	return fmt.Sprintf("record id '%s' is not r followed by 32 lowercase hex characters", e.ID)
}

type UnknownRecordFieldError struct {
	ListPath string
	Field    string
}

func (e *UnknownRecordFieldError) Error() string {
	return fmt.Sprintf("unknown field '%s' in list '%s'", e.Field, e.ListPath)
}

type InvalidRecordValueError struct {
	Path   string
	Reason string
}

func (e *InvalidRecordValueError) Error() string {
	return fmt.Sprintf("invalid record value at '%s': %s", e.Path, e.Reason)
}

type RecordPositionError struct {
	Path string
}

func (e *RecordPositionError) Error() string {
	return fmt.Sprintf("record position at '%s' must be numeric", e.Path)
}

type ListCapacityExceededError struct {
	Path     string
	Count    int
	Capacity int
}

func (e *ListCapacityExceededError) Error() string {
	return fmt.Sprintf("list '%s' holds %d records (capacity %d)", e.Path, e.Count, e.Capacity)
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

func ErrInvalidTemplateState(id string, state State) error {
	return &InvalidTemplateStateError{TemplateID: id, State: state}
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

func ErrFlagValue(key string) error {
	return &FlagValueError{Key: key}
}

func ErrReservedListField(path, field string) error {
	return &ReservedListFieldError{Path: path, Field: field}
}

func ErrNestedList(path string) error {
	return &NestedListError{Path: path}
}

func ErrInvalidListShape(path, reason string) error {
	return &InvalidListShapeError{Path: path, Reason: reason}
}

func ErrListPathTooDeep(path string, max int) error {
	return &ListPathTooDeepError{Path: path, Max: max}
}

func ErrListCapacity(path string, capacity, minCapacity, maxCapacity int) error {
	return &ListCapacityError{Path: path, Capacity: capacity, Min: minCapacity, Max: maxCapacity}
}

func ErrUnknownNotD20Section(key string) error {
	return &UnknownNotD20SectionError{Key: key}
}

func ErrUnknownNotD20Row(key string) error {
	return &UnknownNotD20RowError{Key: key}
}

func ErrInvalidRecordID(id string) error {
	return &RecordIDError{ID: id}
}

func ErrInvalidAggregate(path, reason string) error {
	return &InvalidAggregateError{Path: path, Reason: reason}
}

func ErrUnknownRecordField(listPath, field string) error {
	return &UnknownRecordFieldError{ListPath: listPath, Field: field}
}

func ErrInvalidResetAction(action, reason string) error {
	return &InvalidResetActionError{Action: action, Reason: reason}
}

func ErrUnknownResetAction(id string) error {
	return &UnknownResetActionError{ID: id}
}

func ErrInvalidRecordValue(path, reason string) error {
	return &InvalidRecordValueError{Path: path, Reason: reason}
}

func ErrRecordPosition(path string) error {
	return &RecordPositionError{Path: path}
}

func ErrListCapacityExceeded(path string, count, capacity int) error {
	return &ListCapacityExceededError{Path: path, Count: count, Capacity: capacity}
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

type UnknownNotD20SectionError struct {
	Key string
}

func (e *UnknownNotD20SectionError) Error() string {
	return fmt.Sprintf("not a d20 test section '%s' is not a section of the template", e.Key)
}

type UnknownNotD20RowError struct {
	Key string
}

func (e *UnknownNotD20RowError) Error() string {
	return fmt.Sprintf("not a d20 test row '%s' is not a row of any section", e.Key)
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
