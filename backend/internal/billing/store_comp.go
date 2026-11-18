package billing

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	fieldProvider         = "provider"
	fieldCurrentPeriodEnd = "current_period_end"
	operatorNe            = "$ne"
)

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

func (s *MongoStore) CancelExpiredComps(ctx context.Context, now time.Time) (int, error) {
	res, err := s.subscriptions.UpdateMany(ctx,
		bson.D{
			{Key: fieldProvider, Value: ProviderComp},
			{Key: fieldStatus, Value: StatusActive},
			{Key: fieldCurrentPeriodEnd, Value: bson.D{
				{Key: operatorNe, Value: nil},
				{Key: operatorLte, Value: now},
			}},
		},
		bson.D{{Key: operatorSetLedger, Value: bson.D{
			{Key: fieldStatus, Value: StatusCanceled},
			{Key: fieldUpdatedAt, Value: now},
		}}},
	)
	if err != nil {
		return 0, fmt.Errorf("cancel expired comps: %w", err)
	}
	return int(res.ModifiedCount), nil
}
