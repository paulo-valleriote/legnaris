package billing

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
	fieldProvider         = "provider"
	fieldCurrentPeriodEnd = "current_period_end"
	operatorNe            = "$ne"
)

func (s *MongoStore) FindByID(ctx context.Context, id string) (Subscription, bool, error) {
	var sub Subscription
	err := s.subscriptions.FindOne(ctx, bson.D{{Key: fieldID, Value: id}}).Decode(&sub)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Subscription{}, false, nil
	}
	if err != nil {
		return Subscription{}, false, fmt.Errorf("find subscription: %w", err)
	}
	return sub, true, nil
}

func (s *MongoStore) CancelComp(ctx context.Context, id string, now time.Time) (bool, error) {
	res, err := s.subscriptions.UpdateOne(ctx,
		bson.D{
			{Key: fieldID, Value: id},
			{Key: fieldProvider, Value: ProviderComp},
			{Key: fieldStatus, Value: StatusActive},
		},
		bson.D{{Key: operatorSetLedger, Value: bson.D{
			{Key: fieldStatus, Value: StatusCanceled},
			{Key: fieldUpdatedAt, Value: now},
		}}},
	)
	if err != nil {
		return false, fmt.Errorf("cancel comp subscription: %w", err)
	}
	return res.MatchedCount > 0, nil
}

func (s *MongoStore) ExpiredComps(ctx context.Context, now time.Time, limit int) ([]Subscription, error) {
	cursor, err := s.subscriptions.Find(ctx,
		bson.D{
			{Key: fieldProvider, Value: ProviderComp},
			{Key: fieldStatus, Value: StatusActive},
			{Key: fieldCurrentPeriodEnd, Value: bson.D{
				{Key: operatorNe, Value: nil},
				{Key: operatorLte, Value: now},
			}},
		},
		options.Find().
			SetSort(bson.D{{Key: fieldCurrentPeriodEnd, Value: 1}}).
			SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, fmt.Errorf("load expired comps: %w", err)
	}
	var subs []Subscription
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, fmt.Errorf("decode expired comps: %w", err)
	}
	return subs, nil
}
