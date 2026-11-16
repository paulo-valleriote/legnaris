package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var ErrNoClient = errors.New("transactor has no mongodb client")

var ErrNilCallback = errors.New("transaction callback is nil")

var ErrTransactionAborted = errors.New("transaction aborted")

var ErrTransactionFailed = errors.New("transaction failed")

type Step string

const (
	StepAfterEntityWrite  Step = "after_entity_write"
	StepAfterOutboxInsert Step = "after_outbox_insert"
	StepAfterProviderRead Step = "after_provider_read"
)

type StepHook func(ctx context.Context, step Step) error

const DefaultTransactionTimeout = 10 * time.Second

type Transactor struct {
	client  *mongo.Client
	hook    StepHook
	timeout time.Duration
}

func NewTransactor(client *mongo.Client, hook StepHook) *Transactor {
	return &Transactor{client: client, hook: hook, timeout: DefaultTransactionTimeout}
}

func (t *Transactor) WithTimeout(d time.Duration) *Transactor {
	if t == nil {
		return nil
	}
	copied := *t
	if d > 0 {
		copied.timeout = d
	}
	return &copied
}

func (t *Transactor) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if t == nil || t.client == nil {
		return ErrNoClient
	}
	if fn == nil {
		return ErrNilCallback
	}

	ctx, cancel := context.WithTimeout(ctx, t.effectiveTimeout())
	defer cancel()

	var callbackErr error
	err := t.client.UseSessionWithOptions(ctx, options.Session(), func(sessionCtx context.Context) error {
		_, txErr := mongo.SessionFromContext(sessionCtx).WithTransaction(sessionCtx, func(txCtx context.Context) (any, error) {
			callbackErr = fn(txCtx)
			if callbackErr != nil && txCtx.Err() != nil {
				return nil, unlabeledError{callbackErr.Error()}
			}
			return nil, callbackErr
		}, TransactionOptions())
		return txErr
	})

	if callbackErr != nil {
		return fmt.Errorf("%w: %w", ErrTransactionAborted, callbackErr)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTransactionFailed, err)
	}
	return nil
}

type unlabeledError struct{ message string }

func (e unlabeledError) Error() string { return e.message }

func (t *Transactor) effectiveTimeout() time.Duration {
	if t.timeout > 0 {
		return t.timeout
	}
	return DefaultTransactionTimeout
}

func (t *Transactor) Step(ctx context.Context, step Step) error {
	if t == nil || t.hook == nil {
		return nil
	}
	if err := t.hook(ctx, step); err != nil {
		return fmt.Errorf("step %s: %w", step, err)
	}
	return nil
}
