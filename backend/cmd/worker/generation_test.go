package main

import (
	"context"
	"errors"
	"testing"
	"time"

	llmfactory "legnaris/backend/internal/llm/factory"
	"legnaris/backend/internal/llm/openrouter"
	"legnaris/backend/shared/env"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeGeneration is a generation consumer that runs until its context ends and
// records that it started and stopped.
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

// generationEnv returns the worker configuration with generation switched on and an
// LLM key present, so only the wiring under test decides the outcome.
func generationEnv() env.Env {
	cfg := testEnv()
	cfg.Variables.GenerationEnabled = true
	cfg.Variables.LLMAPIKey = "test-key"
	return cfg
}

// TestRun_GenerationOffNeverBuildsTheConsumer pins that the kill-switch keeps the
// generation path dormant: the builder is not called and the email worker runs alone.
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

// TestRun_GenerationBuildFailureStopsBeforeConsuming pins the fail-fast boot: a
// generation that cannot be built stops the process before the broker is dialed.
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

// TestRun_GenerationRunsBesideEmailAndCleansUp pins that both consumers run in the
// same process, stop together on shutdown, and that the generation cleanup runs.
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

// TestBuildGeneration covers the boot checks of the generation wiring. The LLM
// configuration is checked before the database, so the first cases never reach Mongo.
func TestBuildGeneration(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*env.Env)
		wantErr error
	}{
		{name: "Error: a missing LLM_API_KEY fails at boot", mutate: func(c *env.Env) { c.Variables.LLMAPIKey = "" }, wantErr: openrouter.ErrMissingAPIKey},
		{name: "Error: a blank LLM_API_KEY fails at boot", mutate: func(c *env.Env) { c.Variables.LLMAPIKey = "   " }, wantErr: openrouter.ErrMissingAPIKey},
		{name: "Error: a :free model is refused at boot", mutate: func(c *env.Env) { c.Variables.LLMModel = "deepseek/deepseek-v4.1-flash:free" }, wantErr: openrouter.ErrFreeModel},
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
				assert.True(t, errors.Is(err, tc.wantErr), "got %v", err)
			}
			assert.Nil(t, runner)
			assert.Nil(t, cleanup)
		})
	}
}

// TestRunConcurrently covers the join of the runners' outcomes.
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
