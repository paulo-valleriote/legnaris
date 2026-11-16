package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"legnaris/backend/internal/user"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	usersCollection            = "users"
	candidateScanFactor        = 10
	fieldEventStatus           = "status"
	fieldEventNextAttempt      = "next_attempt_at"
	fieldEventReceivedAt       = "received_at"
	fieldLockedUntil           = "locked_until"
	fieldLockedBy              = "locked_by"
	fieldProviderSyncedAt      = "provider_synced_at"
	fieldProviderSubID         = "provider_subscription_id"
	fieldProviderCustomerID    = "provider_customer_id"
	fieldUserPlan              = "plan"
	fieldUserPlanVersion       = "plan_version"
	fieldUserStatus            = "status"
	fieldAttempts              = "attempts"
	operatorSetLedger          = "$set"
	operatorIncLedger          = "$inc"
	operatorUnsetLedger        = "$unset"
	statusReceivedLedgerString = string(EventReceived)
)

var errEventNotReceived = errors.New("billing event is not received")

func liveStatuses() bson.A {
	return bson.A{StatusPending, StatusActive, StatusPastDue}
}

func (s *MongoStore) InsertEvent(ctx context.Context, ev Event) error {
	if _, err := s.events.InsertOne(ctx, ev); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrDuplicateEvent
		}
		return fmt.Errorf("insert billing event: %w", err)
	}
	return nil
}

func (s *MongoStore) ResolveUser(ctx context.Context, providerSubscriptionID, providerCustomerID string) (string, bool, error) {
	filters := []bson.D{}
	if providerSubscriptionID != "" {
		filters = append(filters, bson.D{{Key: fieldProviderSubID, Value: providerSubscriptionID}})
	}
	if providerCustomerID != "" {
		filters = append(filters, bson.D{{Key: fieldProviderCustomerID, Value: providerCustomerID}})
	}
	for _, filter := range filters {
		var doc struct {
			UserID string `bson:"user_id"`
		}
		err := s.subscriptions.FindOne(ctx, filter, options.FindOne().SetProjection(bson.D{{Key: fieldUserID, Value: 1}})).Decode(&doc)
		if errors.Is(err, mongo.ErrNoDocuments) {
			continue
		}
		if err != nil {
			return "", false, fmt.Errorf("resolve billing user: %w", err)
		}
		return doc.UserID, true, nil
	}
	return "", false, nil
}

func (s *MongoStore) ClaimEvents(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]Event, error) {
	cursor, err := s.events.Find(ctx,
		bson.D{
			{Key: fieldEventStatus, Value: statusReceivedLedgerString},
			{Key: fieldEventNextAttempt, Value: bson.D{{Key: "$lte", Value: now}}},
		},
		options.Find().SetSort(bson.D{{Key: fieldEventReceivedAt, Value: 1}}).SetLimit(int64(limit*candidateScanFactor)),
	)
	if err != nil {
		return nil, fmt.Errorf("scan billing events: %w", err)
	}
	var candidates []Event
	if err := cursor.All(ctx, &candidates); err != nil {
		return nil, fmt.Errorf("decode billing events: %w", err)
	}

	claimed := make([]Event, 0, limit)
	seen := make(map[string]bool, len(candidates))
	for _, ev := range candidates {
		if len(claimed) == limit {
			break
		}
		if seen[ev.UserID] {
			continue
		}
		seen[ev.UserID] = true
		ok, err := s.acquireUser(ctx, ev.UserID, claimer, now, now.Add(lease))
		if err != nil {
			return claimed, err
		}
		if ok {
			ev.Claimer = claimer
			claimed = append(claimed, ev)
		}
	}
	return claimed, nil
}

func (s *MongoStore) acquireUser(ctx context.Context, userID, claimer string, now, leaseUntil time.Time) (bool, error) {
	filter := bson.D{
		{Key: fieldID, Value: userID},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: fieldLockedUntil, Value: nil}},
			bson.D{{Key: fieldLockedUntil, Value: bson.D{{Key: "$lte", Value: now}}}},
		}},
	}
	update := bson.D{{Key: operatorSetLedger, Value: bson.D{
		{Key: fieldLockedUntil, Value: leaseUntil},
		{Key: fieldLockedBy, Value: claimer},
	}}}
	err := s.locks.FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetUpsert(true)).Err()
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("acquire billing user lease: %w", err)
	}
	return true, nil
}

func (s *MongoStore) ReleaseUser(ctx context.Context, userID, claimer string) error {
	_, err := s.locks.UpdateOne(ctx,
		bson.D{{Key: fieldID, Value: userID}, {Key: fieldLockedBy, Value: claimer}},
		bson.D{{Key: operatorUnsetLedger, Value: bson.D{{Key: fieldLockedUntil, Value: ""}, {Key: fieldLockedBy, Value: ""}}}},
	)
	if err != nil {
		return fmt.Errorf("release billing user lease: %w", err)
	}
	return nil
}

func (s *MongoStore) RescheduleEvent(ctx context.Context, id string, nextAt time.Time, reason string) error {
	res, err := s.events.UpdateOne(ctx,
		bson.D{{Key: fieldID, Value: id}, {Key: fieldEventStatus, Value: statusReceivedLedgerString}},
		bson.D{
			{Key: operatorSetLedger, Value: bson.D{{Key: fieldEventNextAttempt, Value: nextAt}, {Key: "last_error", Value: reason}}},
			{Key: operatorIncLedger, Value: bson.D{{Key: fieldAttempts, Value: 1}}},
		})
	if err != nil {
		return fmt.Errorf("reschedule billing event: %w", err)
	}
	if res.MatchedCount == 0 {
		return errEventNotReceived
	}
	return nil
}

func (s *MongoStore) FailEvent(ctx context.Context, id string, reason string) error {
	res, err := s.events.UpdateOne(ctx,
		bson.D{{Key: fieldID, Value: id}, {Key: fieldEventStatus, Value: statusReceivedLedgerString}},
		bson.D{{Key: operatorSetLedger, Value: bson.D{{Key: fieldEventStatus, Value: EventFailed}, {Key: "last_error", Value: reason}}}})
	if err != nil {
		return fmt.Errorf("fail billing event: %w", err)
	}
	if res.MatchedCount == 0 {
		return errEventNotReceived
	}
	return nil
}

func (s *MongoStore) MarkEventProcessed(ctx context.Context, id, reason string, at time.Time) error {
	set := bson.D{{Key: fieldEventStatus, Value: EventProcessed}, {Key: "processed_at", Value: at}}
	if reason != "" {
		set = append(set, bson.E{Key: "reason", Value: reason})
	}
	_, err := s.events.UpdateOne(ctx, bson.D{{Key: fieldID, Value: id}}, bson.D{{Key: operatorSetLedger, Value: set}})
	if err != nil {
		return fmt.Errorf("mark billing event processed: %w", err)
	}
	return nil
}

func (s *MongoStore) OldestReceived(ctx context.Context) (time.Time, bool, error) {
	var doc struct {
		ReceivedAt time.Time `bson:"received_at"`
	}
	err := s.events.FindOne(ctx,
		bson.D{{Key: fieldEventStatus, Value: statusReceivedLedgerString}},
		options.FindOne().SetSort(bson.D{{Key: fieldEventReceivedAt, Value: 1}}),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("oldest billing event: %w", err)
	}
	return doc.ReceivedAt, true, nil
}

func (s *MongoStore) AccountPlan(ctx context.Context, userID string) (AccountPlan, error) {
	var doc struct {
		Status      string `bson:"status"`
		Plan        string `bson:"plan"`
		PlanVersion int64  `bson:"plan_version"`
	}
	err := s.users.FindOne(ctx, bson.D{{Key: fieldID, Value: userID}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return AccountPlan{}, nil
	}
	if err != nil {
		return AccountPlan{}, fmt.Errorf("load account plan: %w", err)
	}
	deleting := doc.Status == user.StatusDeleting || doc.Status == user.StatusDeletionDead
	return AccountPlan{Exists: true, Deleting: deleting, Plan: doc.Plan, PlanVersion: doc.PlanVersion}, nil
}

func (s *MongoStore) SetPlan(ctx context.Context, userID, plan string) (int64, error) {
	var doc struct {
		PlanVersion int64 `bson:"plan_version"`
	}
	err := s.users.FindOneAndUpdate(ctx,
		bson.D{{Key: fieldID, Value: userID}},
		bson.D{
			{Key: operatorSetLedger, Value: bson.D{{Key: fieldUserPlan, Value: plan}}},
			{Key: operatorIncLedger, Value: bson.D{{Key: fieldUserPlanVersion, Value: 1}}},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After).SetProjection(bson.D{{Key: fieldUserPlanVersion, Value: 1}}),
	).Decode(&doc)
	if err != nil {
		return 0, fmt.Errorf("set user plan: %w", err)
	}
	return doc.PlanVersion, nil
}

func (s *MongoStore) SubscriptionsOf(ctx context.Context, userID string) ([]Subscription, error) {
	cursor, err := s.subscriptions.Find(ctx, bson.D{{Key: fieldUserID, Value: userID}})
	if err != nil {
		return nil, fmt.Errorf("load subscriptions: %w", err)
	}
	var subs []Subscription
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, fmt.Errorf("decode subscriptions: %w", err)
	}
	return subs, nil
}

func (s *MongoStore) FindTarget(ctx context.Context, userID, providerSubscriptionID string) (Subscription, bool, error) {
	if providerSubscriptionID != "" {
		var sub Subscription
		err := s.subscriptions.FindOne(ctx, bson.D{{Key: fieldUserID, Value: userID}, {Key: fieldProviderSubID, Value: providerSubscriptionID}}).Decode(&sub)
		if err == nil {
			return sub, true, nil
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return Subscription{}, false, fmt.Errorf("find subscription by provider id: %w", err)
		}
	}
	var sub Subscription
	err := s.subscriptions.FindOne(ctx, bson.D{
		{Key: fieldUserID, Value: userID},
		{Key: fieldProviderSubID, Value: bson.D{{Key: "$exists", Value: false}}},
		{Key: fieldStatus, Value: bson.D{{Key: "$in", Value: liveStatuses()}}},
	}).Decode(&sub)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Subscription{}, false, nil
	}
	if err != nil {
		return Subscription{}, false, fmt.Errorf("find pending subscription: %w", err)
	}
	return sub, true, nil
}

func (s *MongoStore) ApplySubscription(ctx context.Context, sub Subscription, readAt time.Time) (bool, error) {
	res, err := s.subscriptions.UpdateOne(ctx,
		bson.D{
			{Key: fieldID, Value: sub.ID},
			{Key: "$or", Value: bson.A{
				bson.D{{Key: fieldProviderSyncedAt, Value: nil}},
				bson.D{{Key: fieldProviderSyncedAt, Value: bson.D{{Key: "$lt", Value: readAt}}}},
			}},
		},
		bson.D{{Key: operatorSetLedger, Value: bson.D{
			{Key: fieldStatus, Value: sub.Status},
			{Key: fieldProviderSubID, Value: sub.ProviderSubID},
			{Key: fieldProviderCustomerID, Value: sub.ProviderCustomerID},
			{Key: "current_period_end", Value: sub.CurrentPeriodEnd},
			{Key: "cancel_at_period_end", Value: sub.CancelAtPeriodEnd},
			{Key: fieldProviderSyncedAt, Value: sub.ProviderSyncedAt},
			{Key: "updated_at", Value: sub.UpdatedAt},
		}}},
	)
	if err != nil {
		return false, fmt.Errorf("apply subscription: %w", err)
	}
	return res.MatchedCount > 0, nil
}

func (s *MongoStore) InsertSubscription(ctx context.Context, sub Subscription) error {
	if _, err := s.subscriptions.InsertOne(ctx, sub); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrLiveExists
		}
		return fmt.Errorf("insert subscription: %w", err)
	}
	return nil
}
