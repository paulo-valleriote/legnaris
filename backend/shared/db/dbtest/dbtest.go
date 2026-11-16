package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"testing"
	"time"

	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const databaseURIKey = "DATABASE_URI"

const databaseNamePrefix = "legnaris_test_"

const dropTimeout = 10 * time.Second

func Client(t testing.TB) *mongo.Client {
	t.Helper()

	if os.Getenv(databaseURIKey) == "" {
		t.Skip("DATABASE_URI is not set — skipping integration test")
	}

	cfg := env.NewEnv(logger.NewLogger(slog.LevelError))
	database, err := db.InitMongo(cfg)
	if err != nil {
		t.Fatalf("DATABASE_URI is set but mongo is unusable: %v", err)
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
		t.Fatalf("mongo hello failed: %v", err)
	}
	if _, ok := hello["setName"]; !ok {
		t.Fatal("mongodb is not a replica set member")
	}
	return database.Client
}

func Database(t testing.TB) *mongo.Database {
	t.Helper()
	return Adopt(t, Client(t), NewDatabaseName(t))
}

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

func NewDatabaseName(t testing.TB) string {
	t.Helper()
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("generate database name: %v", err)
	}
	return databaseNamePrefix + hex.EncodeToString(buf)
}
