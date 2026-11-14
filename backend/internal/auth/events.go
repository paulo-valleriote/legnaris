package auth

import "legnaris/backend/internal/events"

// userRegisteredEvent builds the user.registered fact (ADR 0010 catalog v1). The
// new user is the actor, and the event carries no data: the payload never holds
// the email or the username.
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
