package worker

import "errors"

var (
	ErrUnusableMessage = errors.New("unusable email message")

	ErrPublishUnconfirmed = errors.New("broker did not confirm the publish")
)
