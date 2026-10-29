package worker

import "errors"

// The worker's typed failures. They are plain errors.New sentinels, never
// apperr: this is a non-HTTP layer with no status to map, like email.ErrTransient
// (WKR-23). Each documents the effect the worker applies.
var (
	// ErrUnusableMessage reports a message that can never be delivered: an
	// undecodable body, a non-canonical identity, a missing code, a foreign
	// type or a purpose without a template. Effect: dead-letter, acknowledge
	// (WKR-04, WKR-05).
	ErrUnusableMessage = errors.New("unusable email message")

	// ErrPublishUnconfirmed reports a republish or dead-letter publish the
	// broker did not confirm. Effect: leave the original unacknowledged so the
	// broker redelivers it (WKR-13).
	ErrPublishUnconfirmed = errors.New("broker did not confirm the publish")
)
