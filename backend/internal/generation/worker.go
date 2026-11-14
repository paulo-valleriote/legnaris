package generation

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"legnaris/backend/internal/llm"
	"legnaris/backend/internal/platform/breaker"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/logger"
)

// settleTimeout bounds the terminal write. It runs on a context detached from the
// job, so a shutdown does not leave a finished job running.
const settleTimeout = 10 * time.Second

// Log attribute keys. The worker logs identifiers, states and counters only: never
// the concept or the output (ADR 0007, ADR 0008).
const (
	attrGenerationID = "generation_id"
	attrStatus       = "status"
	attrAttempts     = "attempts"
	attrLatency      = "latency_ms"
	attrErrorCode    = "error_code"
)

// WorkerDeps are the collaborators of the generation worker.
type WorkerDeps struct {
	Jobs    JobStore
	Catalog Catalog
	Client  llm.Client
	Breaker *breaker.Breaker
	Quota   Quota
	Tx      Transactor
	Now     func() time.Time
	Logger  logger.StructuredLogger
}

// Worker runs one claimed generation job. It is safe for concurrent use: each Process
// call owns its job, and the breaker and the stores are shared.
type Worker struct {
	jobs    JobStore
	catalog Catalog
	client  llm.Client
	breaker *breaker.Breaker
	quota   Quota
	tx      Transactor
	now     func() time.Time
	backoff time.Duration
	logger  logger.StructuredLogger
}

// NewWorker validates deps and builds the worker. A missing collaborator is a boot
// error, so the consumer never starts without one.
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
	case deps.Logger == nil:
		return nil, errors.New("generation worker: nil logger")
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return &Worker{
		jobs:    deps.Jobs,
		catalog: deps.Catalog,
		client:  deps.Client,
		breaker: deps.Breaker,
		quota:   deps.Quota,
		tx:      deps.Tx,
		now:     now,
		backoff: TransportBackoff,
		logger:  deps.Logger,
	}, nil
}

// Process runs the job named by id from claim to terminal state. It returns nil when
// the message is settled: the job completed, failed, or was skipped because it was
// not queued (duplicate delivery, removed document). It returns an error only when
// the store fails, and the consumer then leaves the message unacknowledged.
func (w *Worker) Process(ctx context.Context, id string) error {
	claimedAt := w.now()
	doc, claimed, err := w.jobs.ClaimQueued(ctx, id, claimedAt, LeaseDuration)
	if err != nil {
		return err
	}
	if !claimed {
		w.logger.Info(ctx, "generation skipped: not queued",
			logger.Attribute{Key: attrGenerationID, Value: id})
		return nil
	}

	jobCtx, cancel := context.WithTimeout(ctx, LeaseDuration)
	defer cancel()
	values, code := w.generate(jobCtx, doc)

	settleCtx, settle := context.WithTimeout(context.WithoutCancel(ctx), settleTimeout)
	defer settle()
	if code == "" {
		return w.succeed(settleCtx, doc, values, claimedAt)
	}
	return w.fail(settleCtx, doc, code, claimedAt)
}

// generate asks the model for values until they validate, or returns the error code
// that ends the job. Schema retries feed the rejection back and cost nothing.
func (w *Worker) generate(ctx context.Context, doc Document) (map[string]any, string) {
	tmpl, dag, code := w.resolve(doc.TemplateID)
	if code != "" {
		return nil, code
	}
	schema, err := OutputSchema(tmpl)
	if err != nil {
		return nil, ErrorCodeTemplateUnavailable
	}
	system, err := SystemPrompt(tmpl)
	if err != nil {
		return nil, ErrorCodeTemplateUnavailable
	}

	var problems []string
	for attempt := 0; attempt <= MaxSchemaRetries; attempt++ {
		req := llm.Request{
			System:    system,
			Prompt:    UserPrompt(doc.Concept, problems),
			Schema:    schema,
			MaxTokens: MaxOutputTokens,
		}
		resp, code := w.call(ctx, req)
		if code != "" {
			return nil, code
		}
		values, err := parseAndAssemble(tmpl, dag, resp.Output)
		if err == nil {
			return values, ""
		}
		problems = problemsOf(err)
		w.logger.Info(ctx, "generation output rejected",
			logger.Attribute{Key: attrGenerationID, Value: doc.ID},
			logger.Attribute{Key: attrAttempts, Value: strconv.Itoa(attempt + 1)},
			logger.Attribute{Key: attrErrorCode, Value: ErrorCodeInvalidOutput})
	}
	return nil, ErrorCodeInvalidOutput
}

// resolve loads the Template and its rules, or returns the code of the failure.
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

// call performs one completion under the provider breaker. It fails fast while the
// breaker is open, retries a transient failure once, and never retries a
// no-eligible-provider or permanent failure.
func (w *Worker) call(ctx context.Context, req llm.Request) (llm.Response, string) {
	admission := w.breaker.Admit()
	if !admission.Proceed {
		return llm.Response{}, ErrorCodeProviderUnavailable
	}
	probe := admission.Probe
	for transport := 0; ; transport++ {
		resp, err := w.client.Generate(ctx, req)
		if err == nil {
			w.report(ctx, probe, breaker.OutcomeSuccess)
			return resp, ""
		}
		w.report(ctx, probe, classifyLLM(err))
		probe = 0

		switch {
		case errors.Is(err, llm.ErrProviderUnavailable):
			return llm.Response{}, ErrorCodeProviderUnavailable
		case errors.Is(err, llm.ErrTransient) && transport < TransportRetries:
			if !pause(ctx, w.backoff) {
				return llm.Response{}, ErrorCodeProviderFailed
			}
		default:
			return llm.Response{}, ErrorCodeProviderFailed
		}
	}
}

// report feeds one outcome to the breaker and logs the breaker transitions.
func (w *Worker) report(ctx context.Context, probe uint64, outcome breaker.Outcome) {
	switch w.breaker.Record(probe, outcome) {
	case breaker.TransitionOpened:
		w.logger.Warn(ctx, "generation provider breaker opened")
	case breaker.TransitionClosed:
		w.logger.Info(ctx, "generation provider breaker closed")
	case breaker.TransitionNone:
	}
}

// classifyLLM maps one gateway error to its breaker outcome. A gateway that is
// down, or has no eligible provider, counts toward opening the breaker; a permanent
// failure proves nothing about the provider.
func classifyLLM(err error) breaker.Outcome {
	if errors.Is(err, llm.ErrTransient) || errors.Is(err, llm.ErrProviderUnavailable) {
		return breaker.OutcomeOutage
	}
	return breaker.OutcomeNeutral
}

// parseAndAssemble runs the validation pipeline of one output: strict parse, type
// and key checks with the exclusive expansion, then the seeded Template with the
// values applied through the DAG.
func parseAndAssemble(tmpl *template.Template, dag *template.DAG, raw []byte) (map[string]any, error) {
	object, err := ParseOutput(raw)
	if err != nil {
		return nil, err
	}
	expanded, err := Expand(tmpl, object)
	if err != nil {
		return nil, err
	}
	values, err := Assemble(tmpl, dag, expanded)
	if err != nil {
		return nil, &OutputError{Problems: []string{"values break a template rule"}}
	}
	return values, nil
}

// problemsOf returns the problems to feed back for a rejected output.
func problemsOf(err error) []string {
	var outErr *OutputError
	if errors.As(err, &outErr) && len(outErr.Problems) > 0 {
		return outErr.Problems
	}
	return []string{"output does not match the schema"}
}

// succeed stores the validated values and logs the completion.
func (w *Worker) succeed(ctx context.Context, doc Document, values map[string]any, claimedAt time.Time) error {
	landed, err := w.jobs.Complete(ctx, doc.ID, values)
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

// fail moves the job to failed and refunds its quota in the same transaction, so a
// failed job never keeps its allowance (ADR 0007).
func (w *Worker) fail(ctx context.Context, doc Document, code string, claimedAt time.Time) error {
	err := w.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		before, settled, err := w.jobs.FailRunning(txCtx, doc.ID, code)
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

// attrs builds the metadata every terminal log carries: identifiers, state, attempt
// count and latency. Neither the concept nor the output is ever included.
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

// pause waits d or until ctx ends. It reports whether the full pause elapsed.
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
