package generation

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"legnaris/backend/internal/llm"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/platform/breaker"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/logger"
)

const settleTimeout = 10 * time.Second

const (
	attrGenerationID = "generation_id"
	attrStatus       = "status"
	attrAttempts     = "attempts"
	attrLatency      = "latency_ms"
	attrErrorCode    = "error_code"
	attrProblems     = "problems"
	attrFinishReason = "finish_reason"
	attrOutputBytes  = "output_bytes"
	attrError        = "error"
)

const maxProblemsBytes = 500

type WorkerDeps struct {
	Jobs       JobStore
	Catalog    Catalog
	Client     llm.Client
	Breaker    *breaker.Breaker
	Quota      Quota
	Tx         Transactor
	LLMTimeout time.Duration
	Now        func() time.Time
	Logger     logger.StructuredLogger
	Observer   WorkerObserver
}

type Worker struct {
	jobs     JobStore
	catalog  Catalog
	client   llm.Client
	breaker  *breaker.Breaker
	quota    Quota
	tx       Transactor
	lease    time.Duration
	budget   time.Duration
	now      func() time.Time
	backoff  time.Duration
	logger   logger.StructuredLogger
	observer WorkerObserver
}

func NewWorker(deps WorkerDeps) (*Worker, error) {
	switch {
	case deps.Jobs == nil:
		return nil, errors.New("generation worker: nil job store")
	case deps.Catalog == nil:
		return nil, errors.New("generation worker: nil catalog")
	case deps.Client == nil:
		return nil, errors.New("generation worker: nil llm client")
	case deps.Breaker == nil:
		return nil, errors.New("generation worker: nil breaker")
	case deps.Quota == nil:
		return nil, errors.New("generation worker: nil quota")
	case deps.Tx == nil:
		return nil, errors.New("generation worker: nil transactor")
	case deps.LLMTimeout <= 0:
		return nil, errors.New("generation worker: non-positive LLM timeout")
	case deps.Logger == nil:
		return nil, errors.New("generation worker: nil logger")
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return &Worker{
		jobs:     deps.Jobs,
		catalog:  deps.Catalog,
		client:   deps.Client,
		breaker:  deps.Breaker,
		quota:    deps.Quota,
		tx:       deps.Tx,
		lease:    LeaseDurationFor(deps.LLMTimeout),
		budget:   jobBudgetFor(deps.LLMTimeout),
		now:      now,
		backoff:  TransportBackoff,
		logger:   deps.Logger,
		observer: workerObserver(deps.Observer),
	}, nil
}

func (w *Worker) Process(ctx context.Context, id string) error {
	claimedAt := w.now()
	doc, claimed, err := w.jobs.ClaimQueued(ctx, id, claimedAt, w.lease)
	if err != nil {
		return err
	}
	if !claimed {
		w.logger.Info(ctx, "generation skipped: not queued",
			logger.Attribute{Key: attrGenerationID, Value: id})
		return nil
	}

	jobCtx, cancel := context.WithTimeout(ctx, w.budget)
	defer cancel()
	values, code, meta := w.generate(jobCtx, doc)

	settleCtx, settle := context.WithTimeout(context.WithoutCancel(ctx), settleTimeout)
	defer settle()
	if code == "" {
		return w.succeed(settleCtx, doc, values, meta, claimedAt)
	}
	return w.fail(settleCtx, doc, code, meta, claimedAt)
}

func (w *Worker) generate(ctx context.Context, doc Document) (map[string]any, string, Metadata) {
	ctx = llm.WithGenerationID(ctx, doc.ID)
	tmpl, dag, code := w.resolve(doc.TemplateID)
	if code != "" {
		return nil, code, Metadata{}
	}
	kind := plan.SheetKind(documentKind(doc.Kind))
	schema, err := OutputSchema(tmpl, kind)
	if err != nil {
		return nil, ErrorCodeTemplateUnavailable, Metadata{}
	}
	system, err := SystemPrompt(tmpl, kind)
	if err != nil {
		return nil, ErrorCodeTemplateUnavailable, Metadata{}
	}
	meta := Metadata{PromptVersion: PromptVersion(system)}

	var problems []string
	for attempt := 0; attempt <= MaxSchemaRetries; attempt++ {
		req := llm.Request{
			System:    system,
			Prompt:    UserPrompt(kind, doc.Concept, problems),
			Schema:    schema,
			MaxTokens: MaxOutputTokens,
		}
		resp, code := w.call(ctx, req, meta.PromptVersion)
		if code != "" {
			return nil, code, meta
		}
		if resp.Model != "" {
			meta.Model = resp.Model
		}
		values, err := parseAndAssemble(tmpl, dag, kind, resp.Output)
		if err == nil {
			return values, "", meta
		}
		var outputErr *OutputError
		if !errors.As(err, &outputErr) {
			w.logger.Error(ctx, "generation assemble failed",
				logger.Attribute{Key: attrGenerationID, Value: doc.ID},
				logger.Attribute{Key: attrAttempts, Value: strconv.Itoa(attempt + 1)},
				logger.Attribute{Key: attrError, Value: logger.Truncate(err.Error(), maxProblemsBytes)},
				logger.Attribute{Key: attrFinishReason, Value: resp.FinishReason},
				logger.Attribute{Key: attrOutputBytes, Value: strconv.Itoa(len(resp.Output))},
				logger.Attribute{Key: attrErrorCode, Value: ErrorCodeTemplateUnavailable})
			return nil, ErrorCodeTemplateUnavailable, meta
		}
		problems = problemsOf(err)
		w.observer.GenerationInvalidOutput(resp.Model, meta.PromptVersion)
		if attempt < MaxSchemaRetries {
			w.observer.GenerationSchemaRetry(resp.Model, meta.PromptVersion)
		}
		w.logger.Info(ctx, "generation output rejected",
			logger.Attribute{Key: attrGenerationID, Value: doc.ID},
			logger.Attribute{Key: attrAttempts, Value: strconv.Itoa(attempt + 1)},
			logger.Attribute{Key: attrProblems, Value: logger.Truncate(strings.Join(problems, "; "), maxProblemsBytes)},
			logger.Attribute{Key: attrFinishReason, Value: resp.FinishReason},
			logger.Attribute{Key: attrOutputBytes, Value: strconv.Itoa(len(resp.Output))},
			logger.Attribute{Key: attrErrorCode, Value: ErrorCodeInvalidOutput})
	}
	return nil, ErrorCodeInvalidOutput, meta
}

func (w *Worker) resolve(templateID string) (*template.Template, *template.DAG, string) {
	tmpl, ok := w.catalog.FetchByID(templateID)
	if !ok {
		return nil, nil, ErrorCodeTemplateUnavailable
	}
	dag, ok := w.catalog.DAG(templateID)
	if !ok {
		return nil, nil, ErrorCodeTemplateUnavailable
	}
	return tmpl, dag, ""
}

func (w *Worker) call(ctx context.Context, req llm.Request, promptVersion string) (llm.Response, string) {
	probe, admitted := w.admit()
	if !admitted {
		return llm.Response{}, ErrorCodeProviderUnavailable
	}
	for transport := 0; ; transport++ {
		started := time.Now()
		resp, err := w.client.Generate(ctx, req)
		duration := time.Since(started)
		if err == nil {
			w.observer.GenerationAttempt(resp.Model, promptVersion, duration, resp.Usage, AttemptResultSuccess)
			w.report(ctx, probe, breaker.OutcomeSuccess)
			return resp, ""
		}
		w.observer.GenerationAttempt("", promptVersion, duration, llm.Usage{}, AttemptResultError)
		w.report(ctx, probe, classifyLLM(err))

		switch {
		case errors.Is(err, llm.ErrProviderUnavailable):
			return llm.Response{}, ErrorCodeProviderUnavailable
		case errors.Is(err, llm.ErrTransient) && transport < TransportRetries:
			if !pause(ctx, w.backoff) {
				return llm.Response{}, ErrorCodeProviderFailed
			}
			if probe, admitted = w.admit(); !admitted {
				return llm.Response{}, ErrorCodeProviderUnavailable
			}
		default:
			return llm.Response{}, ErrorCodeProviderFailed
		}
	}
}

func (w *Worker) admit() (uint64, bool) {
	admission := w.breaker.Admit()
	return admission.Probe, admission.Proceed
}

func (w *Worker) report(ctx context.Context, probe uint64, outcome breaker.Outcome) {
	transition := w.breaker.Record(probe, outcome)
	w.observer.GenerationBreakerState(w.breaker.IsOpen())
	switch transition {
	case breaker.TransitionOpened:
		w.observer.GenerationBreakerOpened()
		w.logger.Warn(ctx, "generation provider breaker opened")
	case breaker.TransitionClosed:
		w.logger.Info(ctx, "generation provider breaker closed")
	case breaker.TransitionNone:
	}
}

func classifyLLM(err error) breaker.Outcome {
	if errors.Is(err, llm.ErrTransient) || errors.Is(err, llm.ErrProviderUnavailable) {
		return breaker.OutcomeOutage
	}
	return breaker.OutcomeNeutral
}

func parseAndAssemble(tmpl *template.Template, dag *template.DAG, kind plan.SheetKind, raw []byte) (map[string]any, error) {
	object, err := ParseOutput(raw)
	if err != nil {
		return nil, err
	}
	expanded, err := Expand(tmpl, kind, object)
	if err != nil {
		return nil, err
	}
	return Assemble(tmpl, dag, kind, expanded)
}

func problemsOf(err error) []string {
	var outErr *OutputError
	if errors.As(err, &outErr) && len(outErr.Problems) > 0 {
		return outErr.Problems
	}
	return []string{"output does not match the schema"}
}

func (w *Worker) succeed(ctx context.Context, doc Document, values map[string]any, meta Metadata, claimedAt time.Time) error {
	landed, err := w.jobs.Complete(ctx, doc.ID, values, meta)
	if err != nil {
		return fmt.Errorf("complete generation %s: %w", doc.ID, err)
	}
	if !landed {
		w.logger.Info(ctx, "generation discarded: no longer running",
			logger.Attribute{Key: attrGenerationID, Value: doc.ID})
		return nil
	}
	w.logger.Info(ctx, "generation succeeded", w.attrs(doc, StatusSucceeded, claimedAt, "")...)
	return nil
}

func (w *Worker) fail(ctx context.Context, doc Document, code string, meta Metadata, claimedAt time.Time) error {
	err := w.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		before, settled, err := w.jobs.FailRunning(txCtx, doc.ID, code, meta)
		if err != nil {
			return err
		}
		if !settled {
			return nil
		}
		return RefundQuota(txCtx, w.quota, before)
	})
	if err != nil {
		return fmt.Errorf("fail generation %s: %w", doc.ID, err)
	}
	w.logger.Warn(ctx, "generation failed", w.attrs(doc, StatusFailed, claimedAt, code)...)
	return nil
}

func (w *Worker) attrs(doc Document, status Status, claimedAt time.Time, code string) []logger.Attribute {
	attrs := []logger.Attribute{
		{Key: attrGenerationID, Value: doc.ID},
		{Key: attrStatus, Value: string(status)},
		{Key: attrAttempts, Value: strconv.Itoa(doc.Attempts)},
		{Key: attrLatency, Value: strconv.FormatInt(w.now().Sub(claimedAt).Milliseconds(), 10)},
	}
	if code != "" {
		attrs = append(attrs, logger.Attribute{Key: attrErrorCode, Value: code})
	}
	return attrs
}

func pause(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

type noopWorkerObserver struct{}

func (noopWorkerObserver) GenerationAttempt(string, string, time.Duration, llm.Usage, string) {}

func (noopWorkerObserver) GenerationInvalidOutput(string, string) {}

func (noopWorkerObserver) GenerationSchemaRetry(string, string) {}

func (noopWorkerObserver) GenerationBreakerState(bool) {}

func (noopWorkerObserver) GenerationBreakerOpened() {}

func workerObserver(observer WorkerObserver) WorkerObserver {
	if observer == nil {
		return noopWorkerObserver{}
	}
	return observer
}
