package campaign_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/internal/events"
	eventsmocks "legnaris/backend/internal/events/mocks"
)

var eventTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)

func TestEventCatalog_NamesMatchContract(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
	}{
		{"Happy path: created", campaign.EventCreated},
		{"Happy path: member joined", campaign.EventMemberJoined},
		{"Happy path: member left", campaign.EventMemberLeft},
		{"Happy path: role changed", campaign.EventRoleChanged},
		{"Happy path: ownership transferred (LGN-217 hand-off)", campaign.EventOwnershipTransferred},
		{"Happy path: deleted", campaign.EventDeleted},
		{"Happy path: sheet linked", campaign.EventSheetLinked},
		{"Happy path: sheet unlinked", campaign.EventSheetUnlinked},
		{"Happy path: sheet revealed", campaign.EventSheetRevealed},
		{"Happy path: sheet hidden", campaign.EventSheetHidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Regexp(t, eventTypePattern, tc.eventType)
			assert.Regexp(t, `^campaign\.`, tc.eventType)
		})
	}
}

func TestPublishMemberJoined(t *testing.T) {
	c := campaigntest.Build(t, 0, 0)
	c.Version = 4
	joiner := campaigntest.NewID(t)

	t.Run("Happy path: publishes the join with actor, aggregate version and no PII", func(t *testing.T) {
		pub := eventsmocks.NewMockPublisher(t)
		var got events.Event
		pub.EXPECT().Publish(mock.Anything).Run(func(ev events.Event) { got = ev }).Return().Once()

		campaign.PublishMemberJoined(pub, c, joiner)

		assert.Equal(t, campaign.EventMemberJoined, got.Type)
		assert.Equal(t, "campaign", got.AggregateType)
		assert.Equal(t, c.ID, got.AggregateID)
		assert.Equal(t, int64(4), got.AggregateVersion)
		assert.NotNil(t, got.ActorID)
		assert.Equal(t, joiner, *got.ActorID)
		assert.Equal(t, map[string]any{"user_id": joiner, "role": "player"}, got.Data)
	})

	t.Run("Edge: a nil publisher is replaced by a no-op and does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() { campaign.PublishMemberJoined(nil, c, joiner) })
	})
}
