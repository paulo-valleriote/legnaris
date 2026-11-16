package sheet

import (
	"slices"

	"legnaris/backend/internal/events"
)

const (
	eventSheetCreated = "sheet.created"
	eventSheetUpdated = "sheet.updated"
	eventSheetDeleted = "sheet.deleted"
)

const layoutChangedKey = "layout"

func (s *ServiceImpl) publishSheetEvent(eventType, ownerID, sheetID string, version int64, data any) {
	actor := ownerID
	s.publisher.Publish(events.Event{
		Type:             eventType,
		ActorID:          &actor,
		AggregateType:    "sheet",
		AggregateID:      sheetID,
		AggregateVersion: version,
		Data:             data,
	})
}

func (s *ServiceImpl) publishSheetUpdated(ownerID, sheetID string, changedKeys []string, version int64) {
	s.publishSheetEvent(eventSheetUpdated, ownerID, sheetID, version, map[string]any{
		"changed_keys":  changedKeys,
		"sheet_version": version,
	})
}

func changedKeysOf(cmd PartialUpdateSheetCommand) []string {
	keys := make([]string, 0, len(cmd.Data)+1)
	for key := range cmd.Data {
		keys = append(keys, key)
	}
	if cmd.Dice != nil {
		keys = append(keys, diceField)
	}
	slices.Sort(keys)
	return keys
}
