package quota

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	UsageCollection = "generation_usage"
)

const (
	fieldUserID    = "user_id"
	fieldDayUTC    = "day_utc"
	fieldCount     = "count"
	fieldExpiresAt = "expires_at"
	operatorInc    = "$inc"
	operatorSet    = "$setOnInsert"
)

type usageRow struct {
	UserID    string    `bson:"user_id"`
	DayUTC    string    `bson:"day_utc"`
	Count     int       `bson:"count"`
	ExpiresAt time.Time `bson:"expires_at"`
}

type MongoStore struct {
	usage *mongo.Collection
}

func NewMongoStore(db *mongo.Database) *MongoStore {
	return &MongoStore{
		usage: db.Collection(UsageCollection),
	}
}

func (s *MongoStore) ReserveDaily(ctx context.Context, userID, dayUTC string, expiresAt time.Time) (bool, error) {
	key := bson.D{{Key: fieldUserID, Value: userID}, {Key: fieldDayUTC, Value: dayUTC}}
	open := bson.D{{Key: operatorSet, Value: bson.D{
		{Key: fieldCount, Value: 0},
		{Key: fieldExpiresAt, Value: expiresAt},
	}}}
	if _, err := s.usage.UpdateOne(ctx, key, open, options.UpdateOne().SetUpsert(true)); err != nil {
		return false, fmt.Errorf("open daily counter: %w", err)
	}

	filter := append(bson.D{}, key...)
	filter = append(filter, bson.E{Key: fieldCount, Value: bson.D{{Key: "$lt", Value: PaidDailyLimit}}})
	res, err := s.usage.UpdateOne(ctx, filter, bson.D{{Key: operatorInc, Value: bson.D{{Key: fieldCount, Value: 1}}}})
	if err != nil {
		return false, fmt.Errorf("increment daily counter: %w", err)
	}
	return res.MatchedCount == 1, nil
}

func (s *MongoStore) RefundDaily(ctx context.Context, userID, dayUTC string) error {
	filter := bson.D{
		{Key: fieldUserID, Value: userID},
		{Key: fieldDayUTC, Value: dayUTC},
		{Key: fieldCount, Value: bson.D{{Key: "$gt", Value: 0}}},
	}
	if _, err := s.usage.UpdateOne(ctx, filter, bson.D{{Key: operatorInc, Value: bson.D{{Key: fieldCount, Value: -1}}}}); err != nil {
		return fmt.Errorf("decrement daily counter: %w", err)
	}
	return nil
}

func (s *MongoStore) FetchDaily(ctx context.Context, userID, dayUTC string) (int, error) {
	filter := bson.D{{Key: fieldUserID, Value: userID}, {Key: fieldDayUTC, Value: dayUTC}}
	var row usageRow
	err := s.usage.FindOne(ctx, filter).Decode(&row)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("find daily counter: %w", err)
	}
	return row.Count, nil
}
