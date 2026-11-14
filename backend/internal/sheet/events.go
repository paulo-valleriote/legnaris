package sheet

import (
	"slices"

	"legnaris/backend/internal/events"
)

// Event types emitted by the sheet service (ADR 0010 catalog v1).
const (
	eventSheetCreated = "sheet.created"
	eventSheetUpdated = "sheet.updated"
	eventSheetDeleted = "sheet.deleted"
)

// layoutChangedKey is the changed_keys entry of a layout save, which writes no
// data key and does not bump the sheet version.
const layoutChangedKey = "layout"

// publishSheetEvent hands one best-effort sheet event to the publisher. The
// caller emits only after its write succeeded, so a failed or conflicting save
// never reaches the bus.
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

// publishSheetUpdated emits one sheet.updated for a save that changed changedKeys
// and reports the sheet version the save left behind.
func (s *ServiceImpl) publishSheetUpdated(ownerID, sheetID string, changedKeys []string, version int64) {
	s.publishSheetEvent(eventSheetUpdated, ownerID, sheetID, version, map[string]any{
		"changed_keys":  changedKeys,
		"sheet_version": version,
	})
}

// changedKeysOf lists the top-level keys a PATCH wrote, sorted so the payload is
// stable across requests: each data key, plus "dice" when the Dice block is replaced.
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
