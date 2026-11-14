package db

import (
	"context"
	"errors"
	"fmt"
	"legnaris/backend/shared/env"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Database struct {
	Client *mongo.Client
}

// ErrMissingURI reports a missing DATABASE_URI configuration at startup.
var ErrMissingURI = errors.New("DATABASE_URI is not configured")

// ErrInvalidConfig reports unusable database settings detected at startup.
var ErrInvalidConfig = errors.New("invalid database configuration")

// InitMongo connects to MongoDB and verifies reachability. It fails before
// connecting when the environment holds an invalid pool configuration.
func InitMongo(env env.Env) (*Database, error) {
	if err := env.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	uri := env.Variables.DatabaseURI
	if uri == "" {
		return nil, ErrMissingURI
	}

	client, err := mongo.Connect(newClientOptions(env.Variables))
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), env.Variables.DatabasePingTimeout)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), env.Variables.DatabaseCloseTimeout)
		defer disconnectCancel()
		_ = client.Disconnect(disconnectCtx)
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	return &Database{
		Client: client,
	}, nil
}

// newClientOptions builds the driver options for the configured URI. Retryable
// writes are forced on after the URI is applied so a URI cannot turn them off:
// the driver retries a write once on a transient network or failover error,
// which the replica set relies on.
func newClientOptions(vars env.Variables) *options.ClientOptions {
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)

	return options.Client().
		ApplyURI(vars.DatabaseURI).
		SetRetryWrites(true).
		SetServerSelectionTimeout(vars.DatabaseServerSelectionTimeout).
		SetServerAPIOptions(serverAPI).
		SetConnectTimeout(vars.DatabaseConnectTimeout).
		SetMaxPoolSize(vars.DatabaseMaxPoolSize).
		SetMinPoolSize(vars.DatabaseMinPoolSize).
		SetBSONOptions(documentBSONOptions())
}

// documentBSONOptions decodes nested BSON documents into map[string]any instead
// of the default bson.D. Sheet data is modeled as map[string]any and the
// DAG/engine helpers assert on that type.
func documentBSONOptions() *options.BSONOptions {
	return &options.BSONOptions{DefaultDocumentMap: true}
}

// Ping checks that the primary is reachable within the context deadline.
func (db *Database) Ping(ctx context.Context) error {
	if err := db.Client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("ping mongodb: %w", err)
	}
	return nil
}

func (db *Database) Close(ctx context.Context) error {
	if err := db.Client.Disconnect(ctx); err != nil {
		return fmt.Errorf("mongodb disconnection error: %w", err)
	}
	return nil
}
