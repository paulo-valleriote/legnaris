package auth

import "legnaris/backend/internal/events"

func userRegisteredEvent(userID string) events.Event {
	actor := userID
	return events.Event{
		Type:          "user.registered",
		ActorID:       &actor,
		AggregateType: "user",
		AggregateID:   userID,
		Data:          map[string]any{},
	}
}
