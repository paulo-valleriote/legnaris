package generation_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/generation/mocks"
	"legnaris/backend/internal/llm"
	llmmocks "legnaris/backend/internal/llm/mocks"
	"legnaris/backend/internal/platform/breaker"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	jobID         = "0190a1b2-0000-7000-8000-0000000000a1"
	secretConcept = "SECRET-CONCEPT a scout who walks the northern road"
	secretName    = "SECRET-NAME"
	invalidOutput = `{"identity":{"name":"` + secretName + `"}}`
	extraKeyJSON  = `{"identity":{"name":"A","title":"B"},"stats":{"strength":"floor(1)","dexterity":1,"hero":false},"config":{"armorClassAbility":"acUsesDex"},"rules":{}}`
)

type workerFixture struct {
	jobs    *mocks.MockJobStore
	catalog *mocks.MockCatalog
	client  *llmmocks.MockClient
	quota   *mocks.MockQuota
	tx      *mocks.MockTransactor
	breaker *breaker.Breaker
	worker  *generation.Worker
}

func newWorkerFixture(t *testing.T, threshold int) workerFixture {
	t.Helper()
	f := workerFixture{
		jobs:    mocks.NewMockJobStore(t),
		catalog: mocks.NewMockCatalog(t),
		client:  llmmocks.NewMockClient(t),
		quota:   mocks.NewMockQuota(t),
		tx:      mocks.NewMockTransactor(t),
		breaker: breaker.New(threshold, time.Hour, time.Now),
	}
	fixedNow := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	worker, err := generation.NewWorker(generation.WorkerDeps{
		Jobs:    f.jobs,
		Catalog: f.catalog,
		Client:  f.client,
		Breaker: f.breaker,
		Quota:   f.quota,
		Tx:      f.tx,
		Now:     func() time.Time { return fixedNow },
		Logger:  logger.NewLogger(slog.LevelDebug),
	})
	require.NoError(t, err)
	f.worker = worker
	return f
}

func (f workerFixture) expectTemplate(t *testing.T) {
	t.Helper()
	tmpl := fixtureTemplate()
	dag := fixtureDAG(t, tmpl)
	f.catalog.EXPECT().FetchByID(fixtureID).Return(tmpl, true).Maybe()
	f.catalog.EXPECT().DAG(fixtureID).Return(dag, true).Maybe()
}

func (f workerFixture) expectClaim(reservation generation.Reservation) {
	doc := generation.Document{
		ID: jobID, UserID: userID, TemplateID: fixtureID, Kind: "character",
		Concept: secretConcept, Status: generation.StatusRunning, Quota: reservation,
	}
	f.jobs.EXPECT().ClaimQueued(mock.Anything, jobID, mock.Anything, generation.LeaseDuration).
		Return(doc, true, nil).Once()
}

func (f workerFixture) expectTerminalTx(code string, settled bool, reservation generation.Reservation) {
	f.tx.EXPECT().WithTransaction(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Once()
	before := generation.Document{ID: jobID, UserID: userID, Quota: reservation}
	f.jobs.EXPECT().FailRunning(mock.Anything, jobID, code).Return(before, settled, nil).Once()
}

func (f workerFixture) expectTrialRefund() {
	f.quota.EXPECT().RefundTrial(mock.Anything, userID).Return(nil).Once()
}

func (f workerFixture) completeWith(landed bool) {
	f.jobs.EXPECT().Complete(mock.Anything, jobID, mock.Anything).Return(landed, nil).Once()
}

func (f workerFixture) generateReturns(out string, err error) {
	f.client.EXPECT().Generate(mock.Anything, mock.Anything).
		Return(llm.Response{Output: json.RawMessage(out)}, err).Once()
}

var trialReservation = generation.Reservation{Kind: generation.ReservationTrial}

func TestWorkerProcess_Happy(t *testing.T) {
	t.Run("Happy path: queued, running, succeeded with validated values and no refund", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.expectTemplate(t)
		f.expectClaim(trialReservation)
		f.generateReturns(validOutput, nil)
		f.jobs.EXPECT().Complete(mock.Anything, jobID, mock.MatchedBy(func(values map[string]any) bool {
			stats := values["stats"].(map[string]any)
			return stats["dexterity"] == 15.0 && values["identity"].(map[string]any)["name"] == "Aria"
		})).Return(true, nil).Once()

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})

	t.Run("Happy path: the request carries the schema and the delimited concept", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.expectTemplate(t)
		f.expectClaim(trialReservation)
		f.client.EXPECT().Generate(mock.Anything, mock.MatchedBy(func(req llm.Request) bool {
			return req.MaxTokens == generation.MaxOutputTokens &&
				strings.Contains(req.Prompt, "<concept>") &&
				strings.Contains(req.Prompt, secretConcept) &&
				strings.Contains(req.System, "identity.name") &&
				json.Valid(req.Schema)
		})).Return(llm.Response{Output: json.RawMessage(validOutput)}, nil).Once()
		f.completeWith(true)

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})
}

func TestWorkerProcess_Edge(t *testing.T) {
	t.Run("Edge: a duplicate delivery is a no-op that calls nothing", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.jobs.EXPECT().ClaimQueued(mock.Anything, jobID, mock.Anything, generation.LeaseDuration).
			Return(generation.Document{}, false, nil).Once()

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})

	t.Run("Error: a claim that fails returns the error so the message stays unacknowledged", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.jobs.EXPECT().ClaimQueued(mock.Anything, jobID, mock.Anything, generation.LeaseDuration).
			Return(generation.Document{}, false, errors.New("mongo down")).Once()

		require.Error(t, f.worker.Process(context.Background(), jobID))
	})

	t.Run("Edge: a job whose document was removed by consent revocation writes nothing and refunds nothing", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.expectTemplate(t)
		f.expectClaim(trialReservation)
		f.generateReturns(validOutput, nil)
		f.completeWith(false)

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})

	t.Run("Error: a store failure on the success write is returned for redelivery", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.expectTemplate(t)
		f.expectClaim(trialReservation)
		f.generateReturns(validOutput, nil)
		f.jobs.EXPECT().Complete(mock.Anything, jobID, mock.Anything).Return(false, errors.New("write failed")).Once()

		require.Error(t, f.worker.Process(context.Background(), jobID))
	})

	t.Run("Error: a store failure on the failure write rolls back and returns the error", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.expectTemplate(t)
		f.expectClaim(trialReservation)
		f.generateReturns("", llm.ErrPermanent)
		f.tx.EXPECT().WithTransaction(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Once()
		f.jobs.EXPECT().FailRunning(mock.Anything, jobID, generation.ErrorCodeProviderFailed).
			Return(generation.Document{}, false, errors.New("write failed")).Once()

		require.Error(t, f.worker.Process(context.Background(), jobID))
	})

	t.Run("Edge: a failure write on a job no longer running refunds nothing", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.expectTemplate(t)
		f.expectClaim(trialReservation)
		f.generateReturns("", llm.ErrPermanent)
		f.expectTerminalTx(generation.ErrorCodeProviderFailed, false, trialReservation)

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})
}

func TestWorkerProcess_SchemaRetries(t *testing.T) {
	t.Run("Happy path: a rejected output is retried with its problems and then succeeds", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.expectTemplate(t)
		f.expectClaim(trialReservation)
		f.generateReturns(invalidOutput, nil)
		f.client.EXPECT().Generate(mock.Anything, mock.MatchedBy(func(req llm.Request) bool {
			return strings.Contains(req.Prompt, "rejected") && strings.Contains(req.Prompt, "identity.title: is required")
		})).Return(llm.Response{Output: json.RawMessage(validOutput)}, nil).Once()
		f.completeWith(true)

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})

	tests := []struct {
		name    string
		outputs []string
	}{
		{name: "Error: three rejected outputs exhaust the retries and fail the job", outputs: []string{invalidOutput, extraKeyJSON, "not json"}},
		{name: "Edge: an output with extra keys and a formula string is rejected every time", outputs: []string{extraKeyJSON, extraKeyJSON, extraKeyJSON}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newWorkerFixture(t, 5)
			f.expectTemplate(t)
			f.expectClaim(trialReservation)
			for _, out := range tc.outputs {
				f.generateReturns(out, nil)
			}
			f.expectTerminalTx(generation.ErrorCodeInvalidOutput, true, trialReservation)
			f.expectTrialRefund()

			require.NoError(t, f.worker.Process(context.Background(), jobID))
		})
	}

	t.Run("Edge: the exhausted retries stop after the configured number of model calls", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.expectTemplate(t)
		f.expectClaim(trialReservation)
		for range generation.MaxSchemaRetries + 1 {
			f.generateReturns(invalidOutput, nil)
		}
		f.expectTerminalTx(generation.ErrorCodeInvalidOutput, true, trialReservation)
		f.expectTrialRefund()

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})
}

func TestWorkerProcess_Transport(t *testing.T) {
	tests := []struct {
		name        string
		outcomes    []error
		wantCode    string
		wantSuccess bool
	}{
		{name: "Edge: a transient failure is retried once and the job then succeeds", outcomes: []error{llm.ErrTransient, nil}, wantSuccess: true},
		{name: "Error: two transient failures fail the job as provider_failed", outcomes: []error{llm.ErrTransient, llm.ErrTransient}, wantCode: generation.ErrorCodeProviderFailed},
		{name: "Error: a permanent failure is not retried", outcomes: []error{llm.ErrPermanent}, wantCode: generation.ErrorCodeProviderFailed},
		{name: "Error: no eligible provider is not retried and fails as provider_unavailable", outcomes: []error{llm.ErrProviderUnavailable}, wantCode: generation.ErrorCodeProviderUnavailable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newWorkerFixture(t, 5)
			f.expectTemplate(t)
			f.expectClaim(trialReservation)
			for _, outcome := range tc.outcomes {
				if outcome == nil {
					f.generateReturns(validOutput, nil)
					continue
				}
				f.generateReturns("", outcome)
			}
			if tc.wantSuccess {
				f.completeWith(true)
			} else {
				f.expectTerminalTx(tc.wantCode, true, trialReservation)
				f.expectTrialRefund()
			}

			require.NoError(t, f.worker.Process(context.Background(), jobID))
		})
	}

	t.Run("Error: a job fails fast as provider_unavailable while the breaker is open", func(t *testing.T) {
		f := newWorkerFixture(t, 1)
		f.expectTemplate(t)

		f.expectClaim(trialReservation)
		f.generateReturns("", llm.ErrProviderUnavailable)
		f.expectTerminalTx(generation.ErrorCodeProviderUnavailable, true, trialReservation)
		f.expectTrialRefund()
		require.NoError(t, f.worker.Process(context.Background(), jobID))

		secondID := "0190a1b2-0000-7000-8000-0000000000a2"
		f.jobs.EXPECT().ClaimQueued(mock.Anything, secondID, mock.Anything, generation.LeaseDuration).
			Return(generation.Document{ID: secondID, UserID: userID, TemplateID: fixtureID,
				Concept: secretConcept, Quota: trialReservation}, true, nil).Once()
		f.tx.EXPECT().WithTransaction(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Once()
		f.jobs.EXPECT().FailRunning(mock.Anything, secondID, generation.ErrorCodeProviderUnavailable).
			Return(generation.Document{ID: secondID, UserID: userID, Quota: trialReservation}, true, nil).Once()
		f.expectTrialRefund()

		require.NoError(t, f.worker.Process(context.Background(), secondID))
	})

	t.Run("Error: a transient failure that opens the breaker is not retried", func(t *testing.T) {
		f := newWorkerFixture(t, 1)
		f.expectTemplate(t)
		f.expectClaim(trialReservation)
		f.generateReturns("", llm.ErrTransient)
		f.expectTerminalTx(generation.ErrorCodeProviderUnavailable, true, trialReservation)
		f.expectTrialRefund()

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})

	t.Run("Edge: a permanent failure never opens the breaker", func(t *testing.T) {
		f := newWorkerFixture(t, 1)
		f.expectTemplate(t)
		f.expectClaim(trialReservation)
		f.generateReturns("", llm.ErrPermanent)
		f.expectTerminalTx(generation.ErrorCodeProviderFailed, true, trialReservation)
		f.expectTrialRefund()
		require.NoError(t, f.worker.Process(context.Background(), jobID))

		f.expectClaim(trialReservation)
		f.generateReturns(validOutput, nil)
		f.completeWith(true)
		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})
}

func TestWorkerProcess_Refunds(t *testing.T) {
	day := "2026-11-14"
	t.Run("Happy path: a failed paid job refunds its daily unit for its own day", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.expectTemplate(t)
		reservation := generation.Reservation{Kind: generation.ReservationDaily, DayUTC: day}
		f.expectClaim(reservation)
		f.generateReturns("", llm.ErrPermanent)
		f.expectTerminalTx(generation.ErrorCodeProviderFailed, true, reservation)
		f.quota.EXPECT().RefundDaily(mock.Anything, userID, day).Return(nil).Once()

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})

	t.Run("Error: a refund that fails returns the error so the transaction rolls back", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.expectTemplate(t)
		f.expectClaim(trialReservation)
		f.generateReturns("", llm.ErrPermanent)
		f.tx.EXPECT().WithTransaction(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Once()
		f.jobs.EXPECT().FailRunning(mock.Anything, jobID, generation.ErrorCodeProviderFailed).
			Return(generation.Document{ID: jobID, UserID: userID, Quota: trialReservation}, true, nil).Once()
		f.quota.EXPECT().RefundTrial(mock.Anything, userID).Return(errors.New("quota down")).Once()

		require.Error(t, f.worker.Process(context.Background(), jobID))
	})

	t.Run("Error: a reservation of an unknown kind is refused instead of guessing", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.expectTemplate(t)
		f.expectClaim(generation.Reservation{Kind: "mystery"})
		f.generateReturns("", llm.ErrPermanent)
		f.tx.EXPECT().WithTransaction(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Once()
		f.jobs.EXPECT().FailRunning(mock.Anything, jobID, generation.ErrorCodeProviderFailed).
			Return(generation.Document{ID: jobID, UserID: userID, Quota: generation.Reservation{Kind: "mystery"}}, true, nil).Once()

		require.Error(t, f.worker.Process(context.Background(), jobID))
	})
}

func TestWorkerProcess_Template(t *testing.T) {
	t.Run("Error: a missing template fails the job as template_unavailable without calling the gateway", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.catalog.EXPECT().FetchByID(fixtureID).Return(nil, false).Once()
		f.expectClaim(trialReservation)
		f.expectTerminalTx(generation.ErrorCodeTemplateUnavailable, true, trialReservation)
		f.expectTrialRefund()

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})

	t.Run("Error: a template without rules fails the job as template_unavailable", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		f.catalog.EXPECT().FetchByID(fixtureID).Return(fixtureTemplate(), true).Once()
		f.catalog.EXPECT().DAG(fixtureID).Return(nil, false).Once()
		f.expectClaim(trialReservation)
		f.expectTerminalTx(generation.ErrorCodeTemplateUnavailable, true, trialReservation)
		f.expectTrialRefund()

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})

	t.Run("Error: a template with nothing to generate fails the job as template_unavailable", func(t *testing.T) {
		f := newWorkerFixture(t, 5)
		empty := &template.Template{ID: fixtureID, Fields: template.FieldGroup{
			"stats": {Children: template.FieldGroup{"luck": {Schema: &template.FieldSchema{Type: "number", IsEditable: true}}}},
		}}
		f.catalog.EXPECT().FetchByID(fixtureID).Return(empty, true).Once()
		f.catalog.EXPECT().DAG(fixtureID).Return(fixtureDAG(t, fixtureTemplate()), true).Once()
		f.expectClaim(trialReservation)
		f.expectTerminalTx(generation.ErrorCodeTemplateUnavailable, true, trialReservation)
		f.expectTrialRefund()

		require.NoError(t, f.worker.Process(context.Background(), jobID))
	})
}

func TestWorkerProcess_LogsNeverCarryContent(t *testing.T) {
	tests := []struct {
		name    string
		outputs []string
		err     error
	}{
		{name: "Happy path: a succeeded job logs its identifiers only", outputs: []string{validOutput}},
		{name: "Error: a rejected output logs its rule, never its value", outputs: []string{invalidOutput, invalidOutput, invalidOutput}},
		{name: "Error: a gateway failure logs its code, never the body", err: llm.ErrPermanent},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureStdout(t, func() {
				f := newWorkerFixture(t, 5)
				f.expectTemplate(t)
				f.expectClaim(trialReservation)
				if tc.err != nil {
					f.generateReturns("", tc.err)
					f.expectTerminalTx(generation.ErrorCodeProviderFailed, true, trialReservation)
					f.expectTrialRefund()
				}
				for _, out := range tc.outputs {
					f.generateReturns(out, nil)
				}
				if len(tc.outputs) == 1 {
					f.completeWith(true)
				}
				if len(tc.outputs) == 3 {
					f.expectTerminalTx(generation.ErrorCodeInvalidOutput, true, trialReservation)
					f.expectTrialRefund()
				}
				require.NoError(t, f.worker.Process(context.Background(), jobID))
			})
			assert.NotContains(t, logs, "SECRET-CONCEPT")
			assert.NotContains(t, logs, secretName)
			assert.NotContains(t, logs, "northern road")
			assert.Contains(t, logs, jobID, "the identifier is logged")
		})
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original }()

	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(reader)
		done <- string(data)
	}()
	fn()
	require.NoError(t, writer.Close())
	os.Stdout = original
	return <-done
}

func TestNewWorker(t *testing.T) {
	base := func(t *testing.T) generation.WorkerDeps {
		f := newWorkerFixture(t, 5)
		return generation.WorkerDeps{
			Jobs: f.jobs, Catalog: f.catalog, Client: f.client, Breaker: f.breaker,
			Quota: f.quota, Tx: f.tx, Logger: logger.NewLogger(slog.LevelInfo),
		}
	}
	tests := []struct {
		name   string
		mutate func(*generation.WorkerDeps)
	}{
		{name: "Error: a nil job store is refused", mutate: func(d *generation.WorkerDeps) { d.Jobs = nil }},
		{name: "Error: a nil catalog is refused", mutate: func(d *generation.WorkerDeps) { d.Catalog = nil }},
		{name: "Error: a nil client is refused", mutate: func(d *generation.WorkerDeps) { d.Client = nil }},
		{name: "Error: a nil breaker is refused", mutate: func(d *generation.WorkerDeps) { d.Breaker = nil }},
		{name: "Error: a nil quota is refused", mutate: func(d *generation.WorkerDeps) { d.Quota = nil }},
		{name: "Error: a nil transactor is refused", mutate: func(d *generation.WorkerDeps) { d.Tx = nil }},
		{name: "Error: a nil logger is refused", mutate: func(d *generation.WorkerDeps) { d.Logger = nil }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := base(t)
			tc.mutate(&deps)
			w, err := generation.NewWorker(deps)
			assert.Error(t, err)
			assert.Nil(t, w)
		})
	}

	t.Run("Happy path: a nil clock defaults to the wall clock", func(t *testing.T) {
		w, err := generation.NewWorker(base(t))
		require.NoError(t, err)
		assert.NotNil(t, w)
	})
}
