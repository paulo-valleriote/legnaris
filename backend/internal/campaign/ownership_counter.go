package campaign

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

const (
	ownershipCountsCollection = "campaign_owner_counts"
	ownershipCountField       = "count"
)

type MongoOwnershipCounter struct {
	collection *mongo.Collection
}

func NewOwnershipCounter(db *mongo.Database) *MongoOwnershipCounter {
	return &MongoOwnershipCounter{
		collection: db.Collection(ownershipCountsCollection, options.Collection().SetReadPreference(readpref.Primary())),
	}
}

func (c *MongoOwnershipCounter) Reserve(ctx context.Context, ownerID string, limit int) error {
	key := bson.D{{Key: idField, Value: ownerID}}
	open := bson.D{{Key: "$setOnInsert", Value: bson.D{{Key: ownershipCountField, Value: 0}}}}
	if _, err := c.collection.UpdateOne(ctx, key, open, options.UpdateOne().SetUpsert(true)); err != nil {
		return fmt.Errorf("open owned campaign counter: %w", err)
	}

	below := bson.D{
		{Key: idField, Value: ownerID},
		{Key: ownershipCountField, Value: bson.D{{Key: "$lt", Value: limit}}},
	}
	increment := bson.D{{Key: incOperator, Value: bson.D{{Key: ownershipCountField, Value: 1}}}}
	result, err := c.collection.UpdateOne(ctx, below, increment)
	if err != nil {
		return fmt.Errorf("reserve owned campaign slot: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrOwnershipLimitReached
	}
	return nil
}

func (c *MongoOwnershipCounter) Release(ctx context.Context, ownerID string) error {
	positive := bson.D{
		{Key: idField, Value: ownerID},
		{Key: ownershipCountField, Value: bson.D{{Key: "$gt", Value: 0}}},
	}
	decrement := bson.D{{Key: incOperator, Value: bson.D{{Key: ownershipCountField, Value: -1}}}}
	if _, err := c.collection.UpdateOne(ctx, positive, decrement); err != nil {
		return fmt.Errorf("release owned campaign slot: %w", err)
	}
	return nil
}

func (c *MongoOwnershipCounter) Count(ctx context.Context, ownerID string) (int, error) {
	var doc struct {
		Count int `bson:"count"`
	}
	err := c.collection.FindOne(ctx, bson.D{{Key: idField, Value: ownerID}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("count owned campaigns: %w", err)
	}
	return doc.Count, nil
}
