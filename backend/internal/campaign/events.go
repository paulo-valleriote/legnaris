package campaign

import (
	"legnaris/backend/internal/events"
)

const (
	EventCreated              = "campaign.created"
	EventMemberJoined         = "campaign.member_joined"
	EventMemberLeft           = "campaign.member_left"
	EventRoleChanged          = "campaign.role_changed"
	EventOwnershipTransferred = "campaign.ownership_transferred"
	EventDeleted              = "campaign.deleted"
	EventSheetLinked          = "campaign.sheet_linked"
	EventSheetUnlinked        = "campaign.sheet_unlinked"
	EventSheetRevealed        = "campaign.sheet_revealed"
	EventSheetHidden          = "campaign.sheet_hidden"
)

const (
	aggregateType = "campaign"
	dataKeyUserID = "user_id"
)

func (s *ServiceImpl) publishCampaignEvent(eventType, actorID, campaignID string, version int64, data map[string]any) {
	actor := actorID
	s.publisher.Publish(events.Event{
		Type:             eventType,
		ActorID:          &actor,
		AggregateType:    aggregateType,
		AggregateID:      campaignID,
		AggregateVersion: version,
		Data:             data,
	})
}

func PublishMemberJoined(publisher events.Publisher, c Campaign, userID string) {
	if publisher == nil {
		publisher = events.NoOp{}
	}
	actor := userID
	publisher.Publish(events.Event{
		Type:             EventMemberJoined,
		ActorID:          &actor,
		AggregateType:    aggregateType,
		AggregateID:      c.ID,
		AggregateVersion: c.Version,
		Data: map[string]any{
			dataKeyUserID: userID,
			"role":        string(RolePlayer),
		},
	})
}
