package admin

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

const AuditCollection = "admin_audit"

type MongoStore struct {
	audit *mongo.Collection
}

func NewMongoStore(database *mongo.Database) *MongoStore {
	return &MongoStore{audit: database.Collection(AuditCollection)}
}

func (s *MongoStore) Insert(ctx context.Context, entry Entry) error {
	if _, err := s.audit.InsertOne(ctx, entry); err != nil {
		return fmt.Errorf("insert admin audit: %w", err)
	}
	return nil
}
