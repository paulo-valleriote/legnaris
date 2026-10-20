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

func InitMongo(env env.Env) (*Database, error) {
	uri := env.Variables.DatabaseURI
	if uri == "" {
		return nil, ErrMissingURI
	}

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)

	opts := options.Client().
		ApplyURI(uri).
		SetServerSelectionTimeout(env.Variables.DatabaseServerSelectionTimeout).
		SetServerAPIOptions(serverAPI).
		SetConnectTimeout(env.Variables.DatabaseConnectTimeout).
		SetBSONOptions(&options.BSONOptions{
			// Decode nested BSON documents into map[string]any instead of the
			// default bson.D. Sheet data is modeled as map[string]any and the
			// DAG/engine helpers assert on that type.
			DefaultDocumentMap: true,
		})

	client, err := mongo.Connect(opts)
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

func (db *Database) Close(ctx context.Context) error {
	if err := db.Client.Disconnect(ctx); err != nil {
		return fmt.Errorf("mongodb disconnection error: %w", err)
	}
	return nil
}
