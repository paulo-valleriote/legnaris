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

var ErrMissingURI = errors.New("DATABASE_URI is not configured")

var ErrInvalidConfig = errors.New("invalid database configuration")

var ErrMissingReplicaSet = fmt.Errorf("%w: DATABASE_URI must name the replica set with replicaSet=<name>", ErrInvalidConfig)

var ErrMissingCredentials = fmt.Errorf("%w: DATABASE_URI must carry a username and password", ErrInvalidConfig)

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

func documentBSONOptions() *options.BSONOptions {
	return &options.BSONOptions{DefaultDocumentMap: true}
}

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
