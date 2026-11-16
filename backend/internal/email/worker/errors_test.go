package worker_test

import (
	"errors"
	"fmt"
	"testing"

	"legnaris/backend/internal/email/worker"

	"github.com/stretchr/testify/assert"
)

func TestSentinels(t *testing.T) {
	all := []error{worker.ErrUnusableMessage, worker.ErrPublishUnconfirmed}

	for i, sentinel := range all {
		wrapped := fmt.Errorf("context: %w", sentinel)
		assert.True(t, errors.Is(wrapped, sentinel))
		for j, other := range all {
			if i != j {
				assert.False(t, errors.Is(wrapped, other))
			}
		}
	}
}
