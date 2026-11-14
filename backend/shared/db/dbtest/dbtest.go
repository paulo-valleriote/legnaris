// Package dbtest gives integration tests a MongoDB replica set connected through
// DATABASE_URI and an isolated database per test. Tests self-skip when no
// replica set is reachable, so unit runs stay hermetic.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// databaseNamePrefix marks test databases so leftovers are easy to spot.
const databaseNamePrefix = "legnaris_test_"

// dropTimeout bounds the cleanup drop of a test database.
const dropTimeout = 10 * time.Second

// Client returns a client connected to the replica set named by DATABASE_URI.
// It skips the test when MongoDB is unreachable or runs standalone, because
// transactions and the integration suites need a replica set. The client is
// disconnected at cleanup.
func Client(t testing.TB) *mongo.Client {
	t.Helper()

	cfg := env.NewEnv(logger.NewLogger(slog.LevelError))
	database, err := db.InitMongo(cfg)
	if err != nil {
		t.Skipf("mongo unavailable — skipping integration test: %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), cfg.Variables.DatabaseCloseTimeout)
		defer cancel()
		_ = database.Close(closeCtx)
	})

	var hello bson.M
	err = database.Client.Database("admin").
		RunCommand(context.Background(), bson.D{{Key: "hello", Value: 1}}).
		Decode(&hello)
	if err != nil {
		t.Skipf("mongo hello failed — skipping integration test: %v", err)
	}
	if _, ok := hello["setName"]; !ok {
		t.Skip("mongodb is not a replica set member — skipping integration test")
	}
	return database.Client
}

// Database returns a fresh database with a unique name on a replica set client.
// The database is dropped at cleanup, before the client disconnects.
func Database(t testing.TB) *mongo.Database {
	t.Helper()
	return Adopt(t, Client(t), NewDatabaseName(t))
}

// Adopt registers cleanup that drops the named database and returns it. Use it
// when the database name must be known before the environment is built, for
// example to set DATABASE_NAME for the application modules.
func Adopt(t testing.TB, client *mongo.Client, name string) *mongo.Database {
	t.Helper()
	mdb := client.Database(name)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), dropTimeout)
		defer cancel()
		_ = mdb.Drop(ctx)
	})
	return mdb
}

// NewDatabaseName returns a unique database name for one test. The name keeps
// parallel packages and repeated runs from sharing collections.
func NewDatabaseName(t testing.TB) string {
	t.Helper()
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("generate database name: %v", err)
	}
	return databaseNamePrefix + hex.EncodeToString(buf)
}
