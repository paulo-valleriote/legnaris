package worker_test

import (
	"fmt"
	"testing"

	"legnaris/backend/internal/email/worker"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSentinels(t *testing.T) {
	all := []error{worker.ErrUnusableMessage, worker.ErrPublishUnconfirmed}

	for i, sentinel := range all {
		wrapped := fmt.Errorf("context: %w", sentinel)
		require.ErrorIs(t, wrapped, sentinel)
		for j, other := range all {
			if i != j {
				assert.NotErrorIs(t, wrapped, other)
			}
		}
	}
}
