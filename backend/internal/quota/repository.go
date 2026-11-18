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
	TrialCollection = "generation_trial"
	UsageCollection = "generation_usage"
)

const (
	fieldID        = "_id"
	fieldUsed      = "used"
	fieldAllowance = "allowance"
	fieldUserID    = "user_id"
	fieldDayUTC    = "day_utc"
	fieldCount     = "count"
	fieldExpiresAt = "expires_at"
	operatorInc    = "$inc"
	operatorSet    = "$setOnInsert"
)

type TrialRow struct {
	UserID    string `bson:"_id"`
	Allowance int    `bson:"allowance"`
	Used      int    `bson:"used"`
}

type usageRow struct {
	UserID    string    `bson:"user_id"`
	DayUTC    string    `bson:"day_utc"`
	Count     int       `bson:"count"`
	ExpiresAt time.Time `bson:"expires_at"`
}

type MongoStore struct {
	trials *mongo.Collection
	usage  *mongo.Collection
}

func NewMongoStore(db *mongo.Database) *MongoStore {
	return &MongoStore{
		trials: db.Collection(TrialCollection),
		usage:  db.Collection(UsageCollection),
	}
}

func (s *MongoStore) ReserveTrial(ctx context.Context, userID string) (bool, error) {
	filter := bson.D{
		{Key: fieldID, Value: userID},
		{Key: "$expr", Value: bson.D{{Key: "$lt", Value: bson.A{"$" + fieldUsed, "$" + fieldAllowance}}}},
	}
	res, err := s.trials.UpdateOne(ctx, filter, bson.D{{Key: operatorInc, Value: bson.D{{Key: fieldUsed, Value: 1}}}})
	if err != nil {
		return false, fmt.Errorf("increment trial: %w", err)
	}
	return res.MatchedCount == 1, nil
}

func (s *MongoStore) GrantTrial(ctx context.Context, userID string, allowance int) error {
	filter := bson.D{{Key: fieldID, Value: userID}}
	grant := bson.D{{Key: operatorSet, Value: bson.D{
		{Key: fieldAllowance, Value: allowance},
		{Key: fieldUsed, Value: 0},
	}}}
	_, err := s.trials.UpdateOne(ctx, filter, grant, options.UpdateOne().SetUpsert(true))
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		return fmt.Errorf("grant trial: %w", err)
	}
	return nil
}

func (s *MongoStore) RefundTrial(ctx context.Context, userID string) error {
	filter := bson.D{
		{Key: fieldID, Value: userID},
		{Key: fieldUsed, Value: bson.D{{Key: "$gt", Value: 0}}},
	}
	if _, err := s.trials.UpdateOne(ctx, filter, bson.D{{Key: operatorInc, Value: bson.D{{Key: fieldUsed, Value: -1}}}}); err != nil {
		return fmt.Errorf("decrement trial: %w", err)
	}
	return nil
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

func (s *MongoStore) DeleteDaily(ctx context.Context, userID, dayUTC string) error {
	filter := bson.D{{Key: fieldUserID, Value: userID}, {Key: fieldDayUTC, Value: dayUTC}}
	if _, err := s.usage.DeleteOne(ctx, filter); err != nil {
		return fmt.Errorf("delete daily counter: %w", err)
	}
	return nil
}

func (s *MongoStore) FetchTrial(ctx context.Context, userID string) (TrialRow, error) {
	var row TrialRow
	err := s.trials.FindOne(ctx, bson.D{{Key: fieldID, Value: userID}}).Decode(&row)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return TrialRow{}, nil
	}
	if err != nil {
		return TrialRow{}, fmt.Errorf("find trial: %w", err)
	}
	return row, nil
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
