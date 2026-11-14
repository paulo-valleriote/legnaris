package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"legnaris/backend/shared/logger"
)

// Account statuses written by the deletion flow. A deleting account is claimed by
// the sweeper; a dead one needs an operator and is never claimed again.
const (
	StatusDeleting     = "deleting"
	StatusDeletionDead = "deletion_dead"
)

// Field names of the deletion queue state stored on the users document.
const (
	fieldStatus              = "status"
	fieldDeletingAt          = "deleting_at"
	fieldDeletionAttempts    = "deletion_attempts"
	fieldDeletionNextAttempt = "deletion_next_attempt_at"
	fieldDeletionLockedUntil = "deletion_locked_until"
	fieldDeletionLastError   = "deletion_last_error"
)

// ErrDeletionClaimLost reports a deletion transition whose document is no longer
// in the state the caller expects, for example because the account was removed.
var ErrDeletionClaimLost = errors.New("account deletion state changed")

// DeletionStore is the persistence port of the account deletion queue. It only
// touches the deletion fields of users; the cascade that removes the account runs
// in the accountdeletion package.
type DeletionStore interface {
	// ClaimDeleting leases up to limit deleting accounts for claimer until now+lease.
	// An account whose lease has not expired, or whose next attempt is still in the
	// future, is never returned.
	ClaimDeleting(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]User, error)
	// RescheduleDeleting releases the lease, counts one failed attempt and makes the
	// account eligible again at nextAt.
	RescheduleDeleting(ctx context.Context, id string, nextAt time.Time, lastError string) error
	// MarkDeletionDead moves a deleting account to the dead status, where the
	// sweeper never claims it again.
	MarkDeletionDead(ctx context.Context, id string, lastError string) error
	// OldestDeleting returns the time the oldest deleting account was marked. The
	// bool is false when no account is deleting.
	OldestDeleting(ctx context.Context) (time.Time, bool, error)
}

// deletionMongoStore implements DeletionStore over the users collection.
type deletionMongoStore struct {
	collection *mongo.Collection
	logger     logger.StructuredLogger
}

// NewDeletionStore builds the deletion queue store over the application database.
func NewDeletionStore(db *mongo.Database, log logger.StructuredLogger) DeletionStore {
	return &deletionMongoStore{collection: db.Collection("users"), logger: log}
}

// ClaimDeleting leases accounts one at a time with a conditional findOneAndUpdate,
// so concurrent sweepers never share an account and a stopped sweeper's lease
// expires on its own.
func (s *deletionMongoStore) ClaimDeleting(ctx context.Context, _ string, now time.Time, lease time.Duration, limit int) ([]User, error) {
	leaseUntil := now.Add(lease)
	claimed := make([]User, 0, limit)
	for range limit {
		doc, ok, err := s.claimOne(ctx, now, leaseUntil)
		if err != nil {
			return claimed, err
		}
		if !ok {
			break
		}
		claimed = append(claimed, doc)
	}
	return claimed, nil
}

func (s *deletionMongoStore) claimOne(ctx context.Context, now, leaseUntil time.Time) (User, bool, error) {
	filter := bson.D{
		{Key: fieldStatus, Value: StatusDeleting},
		{Key: "$and", Value: bson.A{
			bson.D{{Key: "$or", Value: bson.A{
				bson.D{{Key: fieldDeletionLockedUntil, Value: nil}},
				bson.D{{Key: fieldDeletionLockedUntil, Value: bson.D{{Key: "$lte", Value: now}}}},
			}}},
			bson.D{{Key: "$or", Value: bson.A{
				bson.D{{Key: fieldDeletionNextAttempt, Value: nil}},
				bson.D{{Key: fieldDeletionNextAttempt, Value: bson.D{{Key: "$lte", Value: now}}}},
			}}},
		}},
	}
	update := bson.D{{Key: operatorSet, Value: bson.D{
		{Key: fieldDeletionLockedUntil, Value: leaseUntil},
	}}}

	var doc User
	err := s.collection.FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, fmt.Errorf("claim deleting account: %w", err)
	}
	return doc, true, nil
}

// RescheduleDeleting releases the lease and counts the attempt. It matches only a
// deleting account, so a transition on an account that already left the queue
// reports ErrDeletionClaimLost.
func (s *deletionMongoStore) RescheduleDeleting(ctx context.Context, id string, nextAt time.Time, lastError string) error {
	result, err := s.collection.UpdateOne(ctx,
		bson.D{{Key: fieldID, Value: id}, {Key: fieldStatus, Value: StatusDeleting}},
		bson.D{
			{Key: operatorSet, Value: bson.D{
				{Key: fieldDeletionNextAttempt, Value: nextAt},
				{Key: fieldDeletionLockedUntil, Value: nil},
				{Key: fieldDeletionLastError, Value: lastError},
			}},
			{Key: "$inc", Value: bson.D{{Key: fieldDeletionAttempts, Value: 1}}},
		})
	if err != nil {
		return fmt.Errorf("reschedule account deletion: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrDeletionClaimLost
	}
	return nil
}

// MarkDeletionDead moves a deleting account to the dead status.
func (s *deletionMongoStore) MarkDeletionDead(ctx context.Context, id string, lastError string) error {
	result, err := s.collection.UpdateOne(ctx,
		bson.D{{Key: fieldID, Value: id}, {Key: fieldStatus, Value: StatusDeleting}},
		bson.D{{Key: operatorSet, Value: bson.D{
			{Key: fieldStatus, Value: StatusDeletionDead},
			{Key: fieldDeletionLockedUntil, Value: nil},
			{Key: fieldDeletionLastError, Value: lastError},
		}}})
	if err != nil {
		return fmt.Errorf("mark account deletion dead: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrDeletionClaimLost
	}
	return nil
}

// OldestDeleting returns the oldest deleting_at among the deleting accounts.
func (s *deletionMongoStore) OldestDeleting(ctx context.Context) (time.Time, bool, error) {
	var doc struct {
		DeletingAt time.Time `bson:"deleting_at"`
	}
	opts := options.FindOne().SetSort(bson.D{{Key: fieldDeletingAt, Value: 1}}).
		SetProjection(bson.D{{Key: fieldDeletingAt, Value: 1}})
	err := s.collection.FindOne(ctx, bson.D{{Key: fieldStatus, Value: StatusDeleting}}, opts).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("oldest deleting account: %w", err)
	}
	return doc.DeletingAt, true, nil
}
