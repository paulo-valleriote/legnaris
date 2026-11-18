package admin

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
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

func (s *MongoStore) Entry(ctx context.Context, id string) (Entry, bool, error) {
	var entry Entry
	err := s.audit.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&entry)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Entry{}, false, nil
	}
	if err != nil {
		return Entry{}, false, fmt.Errorf("find admin audit: %w", err)
	}
	return entry, true, nil
}
