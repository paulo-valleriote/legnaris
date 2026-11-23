package template

import (
	"maps"
	"regexp"
	"slices"
	"strings"

	"legnaris/backend/shared/maputils"
)

var recordIDRe = regexp.MustCompile(`^r[0-9a-f]{32}$`)

type RecordUpdate struct {
	ListPath string
	RecordID string
	Fields   map[string]any
	Remove   bool
}

func ValidateRecordID(id string) error {
	if !recordIDRe.MatchString(id) {
		return ErrInvalidRecordID(id)
	}
	return nil
}

func (d *DAG) SplitRecordUpdates(flat map[string]any) (map[string]any, []RecordUpdate, error) {
	if d == nil {
		return nil, nil, ErrNilDAG
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.splitRecordUpdatesLocked(flat)
}

func (d *DAG) splitRecordUpdatesLocked(flat map[string]any) (map[string]any, []RecordUpdate, error) {
	static := make(map[string]any, len(flat))
	grouped := make(map[string]*RecordUpdate)

	for _, key := range slices.Sorted(maps.Keys(flat)) {
		if err := d.splitRecordKey(key, flat[key], static, grouped); err != nil {
			return nil, nil, err
		}
	}

	updates := make([]RecordUpdate, 0, len(grouped))
	for _, key := range slices.Sorted(maps.Keys(grouped)) {
		updates = append(updates, *grouped[key])
	}
	return static, updates, nil
}

func (d *DAG) splitRecordKey(
	key string,
	value any,
	static map[string]any,
	grouped map[string]*RecordUpdate,
) error {
	listPath, list, ok := listByName(d.lists, key)
	if !ok {
		if emptyMap(value) && isListAncestor(d.lists, key) {
			return nil
		}
		static[key] = value
		return nil
	}
	if key == listPath {
		if emptyMap(value) {
			return nil
		}
		return ErrInvalidRecordValue(key, "a list must be changed through its records")
	}

	remainder := strings.TrimPrefix(key, listPath+".")
	segments := strings.Split(remainder, ".")
	if len(segments) > 2 {
		return ErrUnknownRecordField(listPath, strings.Join(segments[1:], "."))
	}

	recordID := segments[0]
	if err := ValidateRecordID(recordID); err != nil {
		return err
	}
	update := groupedRecordUpdate(grouped, listPath, recordID)

	if len(segments) == 1 {
		return applyRecordValue(update, key, value)
	}
	return applyRecordField(update, list, listPath, recordID, segments[1], value)
}

func groupedRecordUpdate(grouped map[string]*RecordUpdate, listPath, recordID string) *RecordUpdate {
	groupKey := listPath + "\x00" + recordID
	update, exists := grouped[groupKey]
	if exists {
		return update
	}
	update = &RecordUpdate{ListPath: listPath, RecordID: recordID, Fields: map[string]any{}}
	grouped[groupKey] = update
	return update
}

func applyRecordValue(update *RecordUpdate, key string, value any) error {
	switch {
	case value == nil:
		update.Remove = true
	case emptyMap(value):
	default:
		return ErrInvalidRecordValue(key, "a record must be an object or null")
	}
	return nil
}

func applyRecordField(
	update *RecordUpdate,
	list ListSchema,
	listPath, recordID, field string,
	value any,
) error {
	key := listPath + "." + recordID + "." + field
	if value == nil {
		return ErrInvalidRecordValue(key, "a record field cannot be null")
	}
	if err := validateRecordField(listPath, list, recordID, field, value); err != nil {
		return err
	}
	update.Fields[field] = value
	return nil
}

func validateRecordField(listPath string, list ListSchema, recordID, field string, value any) error {
	fullPath := listPath + "." + recordID + "." + field
	if field == ListPositionField {
		if _, ok := maputils.ToFloat64(value); !ok {
			return ErrRecordPosition(fullPath)
		}
		return nil
	}

	node, ok := list.Shape[field]
	if !ok || !node.IsLeaf() || node.Schema == nil {
		return ErrUnknownRecordField(listPath, field)
	}
	schema := node.Schema
	if !schema.IsEditable || schema.Formula != "" {
		return ErrNotEditable(fullPath)
	}

	validationNode := Node{Path: Path(strings.Split(fullPath, "."))}
	validationNode.isFlag = schema.Flag
	if schema.MaxLength != nil {
		validationNode.maxLength = *schema.MaxLength
	}
	if err := validationNode.checkFlagValue(value); err != nil {
		return err
	}
	return validationNode.checkTextLength(value)
}

func (d *DAG) ApplyRecordUpdates(data map[string]any, updates []RecordUpdate) error {
	if d == nil {
		return ErrNilDAG
	}
	if len(updates) == 0 {
		return nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.applyRecordUpdatesLocked(data, updates)
}

func (d *DAG) applyRecordUpdatesLocked(data map[string]any, updates []RecordUpdate) error {
	byList := make(map[string][]RecordUpdate)
	for _, update := range updates {
		byList[update.ListPath] = append(byList[update.ListPath], update)
	}

	for _, listPath := range slices.Sorted(maps.Keys(byList)) {
		list, ok := d.lists[listPath]
		if !ok {
			return ErrUnknownRecordField(listPath, "")
		}
		if err := applyListUpdates(data, listPath, list, byList[listPath]); err != nil {
			return err
		}
	}
	return nil
}

func applyListUpdates(data map[string]any, listPath string, list ListSchema, updates []RecordUpdate) error {
	path := strings.Split(listPath, ".")
	listMap := map[string]any{}
	if current, found := maputils.Get(data, path); found {
		existing, ok := current.(map[string]any)
		if !ok {
			return ErrInvalidRecordValue(listPath, "stored list is not an object")
		}
		listMap = maputils.DeepCopy(existing)
	}

	removed := make(map[string]bool)
	for _, update := range updates {
		if update.Remove {
			removed[update.RecordID] = true
			delete(listMap, update.RecordID)
		}
	}

	nextPosition := nextRecordPosition(listMap)
	slices.SortFunc(updates, func(a, b RecordUpdate) int {
		return strings.Compare(a.RecordID, b.RecordID)
	})

	for _, update := range updates {
		if update.Remove || removed[update.RecordID] {
			continue
		}
		record, exists := listMap[update.RecordID].(map[string]any)
		if !exists {
			record = list.SeedRecord(0)
		}
		explicitPosition := false
		for _, field := range slices.Sorted(maps.Keys(update.Fields)) {
			record[field] = update.Fields[field]
			if field == ListPositionField {
				explicitPosition = true
			}
		}
		if record[ListPositionField] == 0.0 && !explicitPosition && !exists {
			record[ListPositionField] = nextPosition
			nextPosition++
		}
		listMap[update.RecordID] = record
	}

	if len(listMap) > list.Capacity {
		return ErrListCapacityExceeded(listPath, len(listMap), list.Capacity)
	}
	return maputils.Set(data, path, listMap)
}

func nextRecordPosition(listMap map[string]any) float64 {
	next := 0.0
	found := false
	for _, value := range listMap {
		record, ok := value.(map[string]any)
		if !ok {
			continue
		}
		position, ok := maputils.ToFloat64(record[ListPositionField])
		if !ok {
			continue
		}
		if !found || position >= next {
			next = position
			found = true
		}
	}
	if !found {
		return 0
	}
	return next + 1
}

func emptyMap(value any) bool {
	m, ok := value.(map[string]any)
	return ok && len(m) == 0
}

func isListAncestor(lists map[string]ListSchema, key string) bool {
	prefix := key + "."
	for listPath := range lists {
		if strings.HasPrefix(listPath, prefix) {
			return true
		}
	}
	return false
}
