package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"legnaris/backend/internal/generation"
	generationmocks "legnaris/backend/internal/generation/mocks"
	llmfactory "legnaris/backend/internal/llm/factory"
	llmmocks "legnaris/backend/internal/llm/mocks"
	"legnaris/backend/internal/llm/openrouter"
	"legnaris/backend/shared/env"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeGeneration struct {
	started chan struct{}
	stopped chan struct{}
}

func newFakeGeneration() *fakeGeneration {
	return &fakeGeneration{started: make(chan struct{}), stopped: make(chan struct{})}
}

func (g *fakeGeneration) Run(ctx context.Context) error {
	close(g.started)
	<-ctx.Done()
	close(g.stopped)
	return nil
}

func generationEnv() env.Env {
	cfg := testEnv()
	cfg.Variables.GenerationEnabled = true
	cfg.Variables.LLMAPIKey = "test-key"
	return cfg
}

func TestRun_GenerationOffNeverBuildsTheConsumer(t *testing.T) {
	p := &probe{}
	broker := newFakeBroker(t)
	broker.MockRepublisher.EXPECT().PublishTo(mock.Anything, mock.Anything, mock.Anything).Maybe()
	built := false
	deps := p.deps(broker, nil)
	deps.newGeneration = func(env.Env) (generationRunner, func(), error) {
		built = true
		return newFakeGeneration(), func() {}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, run(ctx, testEnv(), deps))
	assert.False(t, built)
}

func TestRun_GenerationBuildFailureStopsBeforeConsuming(t *testing.T) {
	p := &probe{}
	deps := p.deps(newFakeBroker(t), nil)
	deps.newGeneration = func(env.Env) (generationRunner, func(), error) {
		return nil, nil, openrouter.ErrMissingAPIKey
	}

	err := run(context.Background(), generationEnv(), deps)
	require.ErrorIs(t, err, openrouter.ErrMissingAPIKey)
	assert.False(t, p.dialed, "the broker must not be dialed when generation cannot start")
}

func TestRun_GenerationRunsBesideEmailAndCleansUp(t *testing.T) {
	p := &probe{}
	broker := newFakeBroker(t)
	broker.MockRepublisher.EXPECT().PublishTo(mock.Anything, mock.Anything, mock.Anything).Maybe()
	generation := newFakeGeneration()
	cleaned := make(chan struct{})
	deps := p.deps(broker, nil)
	deps.newGeneration = func(env.Env) (generationRunner, func(), error) {
		return generation, func() { close(cleaned) }, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, generationEnv(), deps) }()

	require.Eventually(t, func() bool { return broker.declared.Load() }, 5*time.Second, 5*time.Millisecond)
	select {
	case <-generation.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the generation consumer never started")
	}
	cancel()

	require.NoError(t, <-done)
	<-generation.stopped
	<-cleaned
	assert.True(t, broker.closed)
}

func TestBuildGeneration(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*env.Env)
		wantErr error
	}{
		{name: "Error: a missing LLM_API_KEY fails at boot", mutate: func(c *env.Env) { c.Variables.LLMAPIKey = "" }, wantErr: openrouter.ErrMissingAPIKey},
		{name: "Error: a blank LLM_API_KEY fails at boot", mutate: func(c *env.Env) { c.Variables.LLMAPIKey = "   " }, wantErr: openrouter.ErrMissingAPIKey},
		{name: "Error: a :free model is refused at boot", mutate: func(c *env.Env) { c.Variables.LLMModel = "z-ai/glm-5.3-flash:free" }, wantErr: openrouter.ErrFreeModel},
		{name: "Error: an invalid reasoning effort is refused at boot", mutate: func(c *env.Env) { c.Variables.LLMReasoningEffort = "turbo" }, wantErr: openrouter.ErrInvalidReasoningEffort},
		{name: "Error: an unknown provider is refused at boot", mutate: func(c *env.Env) { c.Variables.LLMProvider = "carrier-pigeon" }, wantErr: llmfactory.ErrUnknownProvider},
		{name: "Error: a missing database is refused after the LLM check", mutate: func(c *env.Env) { c.Variables.DatabaseURI = "" }, wantErr: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := generationEnv()
			tc.mutate(&cfg)
			runner, cleanup, err := buildGeneration(cfg)
			require.Error(t, err)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr, "got %v", err)
			}
			assert.Nil(t, runner)
			assert.Nil(t, cleanup)
		})
	}
}

func TestBuildGenerationWorker_WiresTheConfiguredLLMTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{name: "Happy path: the default LLM timeout derives the claim lease", timeout: env.DefaultLLMTimeout},
		{name: "Happy path: a configured LLM timeout derives the claim lease", timeout: 300 * time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := generationEnv()
			cfg.Variables.LLMTimeout = tc.timeout

			jobs := generationmocks.NewMockJobStore(t)
			jobs.EXPECT().
				ClaimQueued(mock.Anything, "job", mock.Anything, generation.LeaseDurationFor(tc.timeout)).
				Return(generation.Document{}, false, nil).
				Once()

			worker, err := buildGenerationWorker(cfg, jobs,
				generationmocks.NewMockCatalog(t), llmmocks.NewMockClient(t),
				generationmocks.NewMockQuota(t), generationmocks.NewMockTransactor(t))
			require.NoError(t, err)

			require.NoError(t, worker.Process(context.Background(), "job"))
		})
	}
}

func TestBuildGenerationWorker_RefusesANonPositiveLLMTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{name: "Edge: a zero LLM timeout", timeout: 0},
		{name: "Edge: a negative LLM timeout", timeout: -time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := generationEnv()
			cfg.Variables.LLMTimeout = tc.timeout

			worker, err := buildGenerationWorker(cfg,
				generationmocks.NewMockJobStore(t), generationmocks.NewMockCatalog(t),
				llmmocks.NewMockClient(t), generationmocks.NewMockQuota(t),
				generationmocks.NewMockTransactor(t))
			require.Error(t, err)
			assert.Nil(t, worker)
		})
	}
}

func TestRunConcurrently(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name    string
		runners []func(context.Context) error
		wantErr error
	}{
		{name: "Happy path: every runner returns nil", runners: []func(context.Context) error{
			func(context.Context) error { return nil }, func(context.Context) error { return nil },
		}},
		{name: "Error: a failing runner's error is joined", runners: []func(context.Context) error{
			func(context.Context) error { return nil }, func(context.Context) error { return boom },
		}, wantErr: boom},
		{name: "Edge: no runner at all returns nil", runners: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := runConcurrently(context.Background(), tc.runners...)
			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}
