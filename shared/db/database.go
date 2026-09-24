package db

import (
	"context"
	"errors"
	"fmt"
	"legnaris/shared/env"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	mongo_context_timeout    = 1 * time.Second
	server_selection_timeout = 2 * time.Second
	connect_timeout          = 2 * time.Second
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
		SetServerSelectionTimeout(server_selection_timeout).
		SetServerAPIOptions(serverAPI).
		SetConnectTimeout(connect_timeout).
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

	ctx, cancel := context.WithTimeout(context.Background(), mongo_context_timeout)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
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
