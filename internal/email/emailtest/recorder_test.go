package emailtest_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"legnaris/internal/email"
	"legnaris/internal/email/emailtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRecorder proves the recording double records sends and surfaces the
// injected sentinel, so no suite needs the real provider (EML-10).
func TestRecorder(t *testing.T) {
	t.Run("Happy path: records the outbound and returns nil", func(t *testing.T) {
		recorder := emailtest.NewRecorder()
		outbound := emailtest.Outbound()

		require.NoError(t, recorder.Send(context.Background(), outbound))

		assert.Equal(t, 1, recorder.Count())
		assert.Equal(t, []email.Outbound{outbound}, recorder.Sent())
	})

	t.Run("Error: surfaces the injected sentinel while still recording", func(t *testing.T) {
		recorder := emailtest.NewRecorder()
		recorder.SetFailure(email.ErrTransient)

		err := recorder.Send(context.Background(), emailtest.Outbound())

		require.Error(t, err)
		assert.ErrorIs(t, err, email.ErrTransient)
		assert.Equal(t, 1, recorder.Count(), "a failed send is still recorded")
	})

	t.Run("Edge: clearing the failure makes sends succeed again", func(t *testing.T) {
		recorder := emailtest.NewRecorder()
		recorder.SetFailure(email.ErrPermanent)
		require.Error(t, recorder.Send(context.Background(), emailtest.Outbound()))

		recorder.SetFailure(nil)

		require.NoError(t, recorder.Send(context.Background(), emailtest.Outbound()))
		assert.Equal(t, 2, recorder.Count())
	})

	t.Run("Edge: the returned slice does not alias the recorder", func(t *testing.T) {
		recorder := emailtest.NewRecorder()
		require.NoError(t, recorder.Send(context.Background(), emailtest.Outbound()))

		sent := recorder.Sent()
		sent[0].To = "mutated@example.com"

		assert.Equal(t, "player@example.com", recorder.Sent()[0].To,
			"mutating the result must not mutate the recorder's state")
	})

	t.Run("Happy path: the recorder satisfies the Sender port", func(t *testing.T) {
		var sender email.Sender = emailtest.NewRecorder()
		require.NoError(t, sender.Send(context.Background(), emailtest.Outbound()))
	})
}

// TestRecorder_Concurrent proves the recorder is safe under concurrent sends
// (the race detector must stay clean).
func TestRecorder_Concurrent(t *testing.T) {
	recorder := emailtest.NewRecorder()

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = recorder.Send(context.Background(), emailtest.Outbound())
		}()
	}
	wg.Wait()

	assert.Equal(t, goroutines, recorder.Count())
}

// TestRecorder_UnrelatedErrorIsNotASentinel guards against a sentinel sneaking
// in from an unrelated error.
func TestRecorder_UnrelatedErrorIsNotASentinel(t *testing.T) {
	recorder := emailtest.NewRecorder()
	recorder.SetFailure(errors.New("boom"))

	err := recorder.Send(context.Background(), emailtest.Outbound())
	require.Error(t, err)
	assert.NotErrorIs(t, err, email.ErrTransient)
	assert.NotErrorIs(t, err, email.ErrPermanent)
}
