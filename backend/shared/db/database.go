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

// ErrMissingReplicaSet reports a DATABASE_URI without a replicaSet parameter.
// Transactions need a replica set, and a URI without one would otherwise fail
// only on the first transaction. It wraps ErrInvalidConfig.
var ErrMissingReplicaSet = fmt.Errorf("%w: DATABASE_URI must name the replica set with replicaSet=<name>", ErrInvalidConfig)

// ErrMissingCredentials reports a DATABASE_URI without a username and password.
// A credential-less URI would pass the ping and fail only on the first write,
// so it is rejected at startup. The message never echoes the URI. It wraps
// ErrInvalidConfig.
var ErrMissingCredentials = fmt.Errorf("%w: DATABASE_URI must carry a username and password", ErrInvalidConfig)

// InitMongo connects to MongoDB and verifies reachability. It fails before
// connecting when the environment holds an invalid pool configuration or a URI
// that does not name a replica set or carry credentials.
func InitMongo(env env.Env) (*Database, error) {
	if err := env.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	uri := env.Variables.DatabaseURI
	if uri == "" {
		return nil, ErrMissingURI
	}

	clientOptions := newClientOptions(env.Variables)
	if err := clientOptions.Validate(); err != nil {
		return nil, fmt.Errorf("%w: DATABASE_URI is not a valid MongoDB URI", ErrInvalidConfig)
	}
	if clientOptions.ReplicaSet == nil || *clientOptions.ReplicaSet == "" {
		return nil, ErrMissingReplicaSet
	}
	if !hasCredentials(clientOptions) {
		return nil, ErrMissingCredentials
	}

	client, err := mongo.Connect(clientOptions)
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

func hasCredentials(opts *options.ClientOptions) bool {
	return opts.Auth != nil && opts.Auth.Username != "" && opts.Auth.Password != ""
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
