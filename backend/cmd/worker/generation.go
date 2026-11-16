package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/generation"
	generationamqp "legnaris/backend/internal/generation/amqp"
	llmfactory "legnaris/backend/internal/llm/factory"
	"legnaris/backend/internal/platform/breaker"
	"legnaris/backend/internal/quota"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
)

const generationShutdownTimeout = 5 * time.Second

type generationRunner interface {
	Run(ctx context.Context) error
}

func buildGeneration(cfg env.Env) (generationRunner, func(), error) {
	vars := cfg.Variables
	client, err := llmfactory.New(llmfactory.Config{
		Provider: vars.LLMProvider,
		BaseURL:  vars.LLMBaseURL,
		APIKey:   vars.LLMAPIKey,
		Model:    vars.LLMModel,
		Timeout:  vars.LLMTimeout,
		Logger:   cfg.Logger,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("build llm client: %w", err)
	}

	database, err := db.InitMongo(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("open mongo: %w", err)
	}
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), generationShutdownTimeout)
		defer cancel()
		_ = database.Close(ctx)
	}

	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("build template registry: %w", err)
	}

	mdb := database.Client.Database(vars.DatabaseName)
	worker, err := generation.NewWorker(generation.WorkerDeps{
		Jobs:    generation.NewMongoStore(mdb),
		Catalog: registry,
		Client:  client,
		Breaker: breaker.New(breaker.DefaultBreakerThreshold, breaker.DefaultBreakerProbeInterval, time.Now),
		Quota:   quota.NewService(quota.NewMongoStore(mdb), nil),
		Tx:      db.NewTransactor(database.Client, nil),
		Now:     time.Now,
		Logger:  cfg.Logger,
	})
	if err != nil {
		cleanup()
		return nil, nil, err
	}

	consumer := generationamqp.NewConsumer(generationamqp.ConsumerConfig{URL: vars.RabbitMQURL},
		func(ctx context.Context, msg generation.Message) error {
			return worker.Process(ctx, msg.ID)
		}, cfg.Logger)
	return consumer, cleanup, nil
}

func runConcurrently(ctx context.Context, runners ...func(context.Context) error) error {
	var wg sync.WaitGroup
	errs := make([]error, len(runners))
	for i, run := range runners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = run(ctx)
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}
