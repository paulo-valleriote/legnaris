package db

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

// ErrNoClient reports a Transactor built without a MongoDB client.
var ErrNoClient = errors.New("transactor has no mongodb client")

// ErrNilCallback reports a WithTransaction call without a function to run.
var ErrNilCallback = errors.New("transaction callback is nil")

// ErrTransactionAborted reports that the callback, or a step hook inside it,
// returned an error and the transaction was rolled back. The callback error is
// wrapped, so errors.Is matches both this sentinel and the callback's own error.
var ErrTransactionAborted = errors.New("transaction aborted")

// ErrTransactionFailed reports that the transaction could not be started or
// committed, for example because the replica set is unreachable or the context
// ended. The callback may not have run.
var ErrTransactionFailed = errors.New("transaction failed")

// Step names a point inside a transaction where crash-injection tests can
// interrupt the flow.
type Step string

const (
	// StepAfterEntityWrite runs after the entity document is written.
	StepAfterEntityWrite Step = "after_entity_write"
	// StepAfterOutboxInsert runs after the event is inserted into the outbox.
	StepAfterOutboxInsert Step = "after_outbox_insert"
	// StepAfterProviderRead runs after a payment provider state is read.
	StepAfterProviderRead Step = "after_provider_read"
)

// StepHook is called at a named Step. An error returned by the hook aborts the
// enclosing transaction. Production code passes a nil hook.
type StepHook func(ctx context.Context, step Step) error

// Transactor runs multi-document writes inside a MongoDB transaction on a
// replica set.
type Transactor struct {
	client *mongo.Client
	hook   StepHook
}

// NewTransactor returns a Transactor bound to client. A nil hook disables step
// interruption, which is the production configuration.
func NewTransactor(client *mongo.Client, hook StepHook) *Transactor {
	return &Transactor{client: client, hook: hook}
}

// WithTransaction runs fn inside a transaction and commits when fn returns nil.
// fn receives a session-bound context; every repository call inside fn must use
// it, or the write falls outside the transaction. The driver re-runs fn on
// TransientTransactionError and UnknownTransactionCommitResult, so fn must not
// have side effects outside the database.
func (t *Transactor) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if t == nil || t.client == nil {
		return ErrNoClient
	}
	if fn == nil {
		return ErrNilCallback
	}

	session, err := t.client.StartSession()
	if err != nil {
		return fmt.Errorf("%w: start session: %w", ErrTransactionFailed, err)
	}
	defer session.EndSession(ctx)

	var callbackErr error
	_, err = session.WithTransaction(ctx, func(sessCtx context.Context) (any, error) {
		callbackErr = fn(sessCtx)
		return nil, callbackErr
	}, transactionOptions())

	if callbackErr != nil {
		return fmt.Errorf("%w: %w", ErrTransactionAborted, callbackErr)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTransactionFailed, err)
	}
	return nil
}

// Step invokes the hook for step. It is a no-op when the Transactor has no hook.
func (t *Transactor) Step(ctx context.Context, step Step) error {
	if t == nil || t.hook == nil {
		return nil
	}
	if err := t.hook(ctx, step); err != nil {
		return fmt.Errorf("step %s: %w", step, err)
	}
	return nil
}

// transactionOptions writes with majority acknowledgement, as ADR 0013 requires,
// and reads from the primary so authorization decisions see the latest state.
func transactionOptions() *options.TransactionOptionsBuilder {
	return options.Transaction().
		SetWriteConcern(writeconcern.Majority()).
		SetReadPreference(readpref.Primary())
}
