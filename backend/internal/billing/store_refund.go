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
	fieldFirstPaidAt       = "first_paid_at"
	fieldRefundState       = "refund_state"
	fieldRefundRequestedAt = "refund_requested_at"
	fieldRefundUsedAt      = "refund_used_at"
	fieldRefundNextAttempt = "refund_next_attempt_at"
	fieldRefundAttempts    = "refund_attempts"
	fieldRefundDead        = "refund_dead"
	fieldRefundLastError   = "refund_last_error"
)

func (s *MongoStore) RequestRefund(ctx context.Context, id string, now time.Time) (bool, error) {
	res, err := s.subscriptions.UpdateOne(ctx,
		bson.D{
			{Key: fieldID, Value: id},
			{Key: fieldStatus, Value: StatusActive},
			{Key: fieldRefundState, Value: bson.D{{Key: "$exists", Value: false}}},
		},
		bson.D{{Key: operatorSetLedger, Value: bson.D{
			{Key: fieldRefundState, Value: RefundRequested},
			{Key: fieldRefundRequestedAt, Value: now},
			{Key: fieldRefundUsedAt, Value: now},
			{Key: fieldRefundNextAttempt, Value: now},
			{Key: fieldUpdatedAt, Value: now},
		}}},
	)
	if mongo.IsDuplicateKeyError(err) {
		return false, ErrRefundAlreadyUsed
	}
	if err != nil {
		return false, fmt.Errorf("request refund: %w", err)
	}
	return res.MatchedCount > 0, nil
}

func (s *MongoStore) ClaimRefunds(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]Subscription, error) {
	claimed := make([]Subscription, 0, limit)
	for len(claimed) < limit {
		var sub Subscription
		err := s.subscriptions.FindOneAndUpdate(ctx,
			bson.D{
				{Key: fieldRefundState, Value: bson.D{{Key: operatorIn, Value: bson.A{RefundRequested, RefundRefunded}}}},
				{Key: fieldRefundDead, Value: bson.D{{Key: "$ne", Value: true}}},
				{Key: fieldRefundNextAttempt, Value: bson.D{{Key: operatorLte, Value: now}}},
			},
			bson.D{{Key: operatorSetLedger, Value: bson.D{{Key: fieldRefundNextAttempt, Value: now.Add(lease)}}}},
			options.FindOneAndUpdate().
				SetSort(bson.D{{Key: fieldRefundRequestedAt, Value: 1}}).
				SetReturnDocument(options.After),
		).Decode(&sub)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return claimed, nil
		}
		if err != nil {
			return claimed, fmt.Errorf("claim refund: %w", err)
		}
		claimed = append(claimed, sub)
	}
	return claimed, nil
}

func (s *MongoStore) RescheduleRefund(ctx context.Context, id string, nextAt time.Time, reason string) error {
	if _, err := s.subscriptions.UpdateOne(ctx,
		bson.D{{Key: fieldID, Value: id}},
		bson.D{
			{Key: operatorSetLedger, Value: bson.D{
				{Key: fieldRefundNextAttempt, Value: nextAt},
				{Key: fieldRefundLastError, Value: reason},
			}},
			{Key: operatorIncLedger, Value: bson.D{{Key: fieldRefundAttempts, Value: 1}}},
		},
	); err != nil {
		return fmt.Errorf("reschedule refund: %w", err)
	}
	return nil
}

func (s *MongoStore) SetRefundState(ctx context.Context, id string, from, to RefundState) error {
	res, err := s.subscriptions.UpdateOne(ctx,
		bson.D{{Key: fieldID, Value: id}, {Key: fieldRefundState, Value: from}},
		bson.D{{Key: operatorSetLedger, Value: bson.D{{Key: fieldRefundState, Value: to}}}},
	)
	if err != nil {
		return fmt.Errorf("set refund state: %w", err)
	}
	if res.MatchedCount == 0 {
		return ErrRefundStateChanged
	}
	return nil
}

func (s *MongoStore) FailRefund(ctx context.Context, id string, reason string) error {
	res, err := s.subscriptions.UpdateOne(ctx,
		bson.D{{Key: fieldID, Value: id}, {Key: fieldRefundState, Value: RefundRequested}},
		bson.D{
			{Key: operatorSetLedger, Value: bson.D{
				{Key: fieldRefundState, Value: RefundFailed},
				{Key: fieldRefundDead, Value: true},
				{Key: fieldRefundLastError, Value: reason},
			}},
			{Key: operatorUnsetLedger, Value: bson.D{{Key: fieldRefundUsedAt, Value: ""}}},
		},
	)
	if err != nil {
		return fmt.Errorf("fail refund: %w", err)
	}
	if res.MatchedCount > 0 {
		return nil
	}
	_, err = s.subscriptions.UpdateOne(ctx,
		bson.D{{Key: fieldID, Value: id}, {Key: fieldRefundState, Value: RefundRefunded}},
		bson.D{{Key: operatorSetLedger, Value: bson.D{
			{Key: fieldRefundDead, Value: true},
			{Key: fieldRefundLastError, Value: reason},
		}}},
	)
	if err != nil {
		return fmt.Errorf("stop refunded cancel: %w", err)
	}
	return nil
}

func (s *MongoStore) OldestRefundRequested(ctx context.Context) (time.Time, bool, error) {
	var doc struct {
		RequestedAt time.Time `bson:"refund_requested_at"`
	}
	err := s.subscriptions.FindOne(ctx,
		bson.D{{Key: fieldRefundState, Value: RefundRequested}},
		options.FindOne().SetSort(bson.D{{Key: fieldRefundRequestedAt, Value: 1}}),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("oldest refund requested: %w", err)
	}
	return doc.RequestedAt, true, nil
}
