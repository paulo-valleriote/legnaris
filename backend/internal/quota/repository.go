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

// Collection names. The storage shapes follow ADR 0009's data model.
const (
	// TrialCollection holds one row per account that was granted a trial.
	TrialCollection = "generation_trial"
	// UsageCollection holds one counter row per account and UTC day.
	UsageCollection = "generation_usage"
)

// Stored field names shared by every filter and update in this file.
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

// TrialRow is the stored shape of a generation_trial document. The row exists
// only when the account was granted an allowance: LGN-229 inserts it with
// Allowance set to TrialAllowance, and an account without a row has none. Used
// counts lifetime generations and never goes past Allowance.
type TrialRow struct {
	UserID    string `bson:"_id"`
	Allowance int    `bson:"allowance"`
	Used      int    `bson:"used"`
}

// usageRow is the stored shape of a generation_usage document.
type usageRow struct {
	UserID    string    `bson:"user_id"`
	DayUTC    string    `bson:"day_utc"`
	Count     int       `bson:"count"`
	ExpiresAt time.Time `bson:"expires_at"`
}

// MongoStore implements Store over the generation_trial and generation_usage
// collections. Every method takes the context it is given, so inside a
// transaction the writes join it.
type MongoStore struct {
	trials *mongo.Collection
	usage  *mongo.Collection
}

// NewMongoStore returns a Store backed by the quota collections of db.
func NewMongoStore(db *mongo.Database) *MongoStore {
	return &MongoStore{
		trials: db.Collection(TrialCollection),
		usage:  db.Collection(UsageCollection),
	}
}

// ReserveTrial increments used only while it is below allowance. A missing row
// matches nothing, so an account without an allowance is refused.
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

// RefundTrial decrements used, and only while it is positive, so a refund never
// makes the counter negative.
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

// ReserveDaily opens the day's row when it is missing, then increments count
// only while it is below PaidDailyLimit. Opening is an upsert on the unique
// (user_id, day_utc) key, so two requests that open the same day converge on
// one row. It returns false when the day is full.
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

// RefundDaily decrements the counter of dayUTC, only while it is positive.
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

// FetchTrial returns the account's trial row. A missing row is the zero value,
// which reads as no allowance.
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

// FetchDaily returns the day's count. A missing row counts as zero.
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
