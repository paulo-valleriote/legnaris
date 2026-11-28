package generation_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/llm"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const attemptModel = "deepseek/deepseek-v4.1-flash"

func promptVersionMatcher(version string) bool {
	return strings.HasPrefix(version, "v1+") && len(version) == len("v1+")+12
}

func TestWorkerObserver_Attempts(t *testing.T) {
	t.Run("Happy path: a successful call reports the model, prompt version, usage and result", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		observer := f.attachObserver(t)
		f.expectTemplate(t)
		f.expectClaim(dailyReservation)
		usage := llm.Usage{
			PromptTokens:     11,
			CompletionTokens: 7,
			TotalTokens:      18,
			CostUSD:          ptr(0.001234),
		}
		f.generateResponse(llm.Response{Output: json.RawMessage(validOutput), Model: attemptModel, Usage: usage}, nil)
		f.completeWith(true)

		observer.EXPECT().GenerationAttempt(attemptModel, mock.MatchedBy(promptVersionMatcher),
			mock.AnythingOfType("time.Duration"), usage, generation.AttemptResultSuccess).Once()
		observer.EXPECT().GenerationBreakerState(false).Once()

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})

	t.Run("Error: a provider failure reports an error attempt without usage", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		observer := f.attachObserver(t)
		f.expectTemplate(t)
		f.expectClaim(dailyReservation)
		f.generateReturns("", llm.ErrPermanent)
		f.expectTerminalTx(generation.ErrorCodeProviderFailed, true, dailyReservation)
		f.expectDailyRefund()

		observer.EXPECT().GenerationAttempt("", mock.MatchedBy(promptVersionMatcher),
			mock.AnythingOfType("time.Duration"), llm.Usage{}, generation.AttemptResultError).Once()
		observer.EXPECT().GenerationBreakerState(false).Once()

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})

	t.Run("Edge: each transport retry reports its own attempt", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		observer := f.attachObserver(t)
		f.expectTemplate(t)
		f.expectClaim(dailyReservation)
		f.generateReturns("", llm.ErrTransient)
		f.generateResponse(llm.Response{Output: json.RawMessage(validOutput), Model: attemptModel}, nil)
		f.completeWith(true)

		observer.EXPECT().GenerationAttempt("", mock.MatchedBy(promptVersionMatcher),
			mock.AnythingOfType("time.Duration"), llm.Usage{}, generation.AttemptResultError).Once()
		observer.EXPECT().GenerationAttempt(attemptModel, mock.MatchedBy(promptVersionMatcher),
			mock.AnythingOfType("time.Duration"), llm.Usage{}, generation.AttemptResultSuccess).Once()
		observer.EXPECT().GenerationBreakerState(false).Twice()

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})
}

func TestWorkerObserver_InvalidOutput(t *testing.T) {
	t.Run("Happy path: an invalid output is counted and its retry is scheduled", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		observer := f.attachObserver(t)
		f.expectTemplate(t)
		f.expectClaim(dailyReservation)
		f.generateResponse(llm.Response{Output: json.RawMessage(invalidOutput), Model: attemptModel}, nil)
		f.generateResponse(llm.Response{Output: json.RawMessage(validOutput), Model: attemptModel}, nil)
		f.completeWith(true)

		observer.EXPECT().GenerationAttempt(attemptModel, mock.MatchedBy(promptVersionMatcher),
			mock.AnythingOfType("time.Duration"), llm.Usage{}, generation.AttemptResultSuccess).Twice()
		observer.EXPECT().GenerationInvalidOutput(attemptModel, mock.MatchedBy(promptVersionMatcher)).Once()
		observer.EXPECT().GenerationSchemaRetry(attemptModel, mock.MatchedBy(promptVersionMatcher)).Once()
		observer.EXPECT().GenerationBreakerState(false).Twice()

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})

	t.Run("Edge: the final invalid output is counted but schedules no retry", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		observer := f.attachObserver(t)
		f.expectTemplate(t)
		f.expectClaim(dailyReservation)
		for range generation.MaxSchemaRetries + 1 {
			f.generateResponse(llm.Response{Output: json.RawMessage(invalidOutput), Model: attemptModel}, nil)
		}
		f.expectTerminalTx(generation.ErrorCodeInvalidOutput, true, dailyReservation)
		f.expectDailyRefund()

		observer.EXPECT().GenerationAttempt(attemptModel, mock.MatchedBy(promptVersionMatcher),
			mock.AnythingOfType("time.Duration"), llm.Usage{}, generation.AttemptResultSuccess).
			Times(generation.MaxSchemaRetries + 1)
		observer.EXPECT().GenerationInvalidOutput(attemptModel, mock.MatchedBy(promptVersionMatcher)).
			Times(generation.MaxSchemaRetries + 1)
		observer.EXPECT().GenerationSchemaRetry(attemptModel, mock.MatchedBy(promptVersionMatcher)).
			Times(generation.MaxSchemaRetries)
		observer.EXPECT().GenerationBreakerState(false).Times(generation.MaxSchemaRetries + 1)

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})
}

func TestWorkerObserver_Breaker(t *testing.T) {
	t.Run("Happy path: opening the breaker reports its state and the opening event", func(t *testing.T) {
		f := newWorkerFixture(t, 1)
		observer := f.attachObserver(t)
		f.expectTemplate(t)
		f.expectClaim(dailyReservation)
		f.generateReturns("", llm.ErrTransient)
		f.expectTerminalTx(generation.ErrorCodeProviderUnavailable, true, dailyReservation)
		f.expectDailyRefund()

		observer.EXPECT().GenerationAttempt("", mock.MatchedBy(promptVersionMatcher),
			mock.AnythingOfType("time.Duration"), llm.Usage{}, generation.AttemptResultError).Once()
		observer.EXPECT().GenerationBreakerState(true).Once()
		observer.EXPECT().GenerationBreakerOpened().Once()

		require.NoError(t, f.worker.Process(context.Background(), jobID))
		require.True(t, f.breaker.IsOpen())
	})

	t.Run("Edge: a permanent failure keeps the breaker closed and reports no opening", func(t *testing.T) {
		f := newWorkerFixture(t, 1)
		observer := f.attachObserver(t)
		f.expectTemplate(t)
		f.expectClaim(dailyReservation)
		f.generateReturns("", llm.ErrPermanent)
		f.expectTerminalTx(generation.ErrorCodeProviderFailed, true, dailyReservation)
		f.expectDailyRefund()

		observer.EXPECT().GenerationAttempt("", mock.MatchedBy(promptVersionMatcher),
			mock.AnythingOfType("time.Duration"), llm.Usage{}, generation.AttemptResultError).Once()
		observer.EXPECT().GenerationBreakerState(false).Once()

		require.NoError(t, f.worker.Process(context.Background(), jobID))
		require.False(t, f.breaker.IsOpen())
	})
}
