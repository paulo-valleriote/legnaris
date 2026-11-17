package campaign

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
	deletingAtField        = "deleting_at"
	deletionAttemptsField  = "deletion_attempts"
	deletionNextAttempt    = "deletion_next_attempt_at"
	deletionLockedUntil    = "deletion_locked_until"
	deletionLastErrorField = "deletion_last_error"
	updatedAtField         = "updated_at"
	lteOperator            = "$lte"
	orOperator             = "$or"
	andOperator            = "$and"
)

var (
	ErrNotDeleting       = errors.New("campaign is not deleting")
	ErrDeletionClaimLost = errors.New("campaign deletion state changed")
)

type DeletionStore interface {
	ClaimDeleting(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]Campaign, error)
	RescheduleDeleting(ctx context.Context, id string, nextAt time.Time, lastError string) error
	MarkDeletionDead(ctx context.Context, id string, lastError string) error
	OldestDeleting(ctx context.Context) (time.Time, bool, error)
	TouchDeleting(ctx context.Context, id string, now time.Time) (int64, error)
	RemoveDeleting(ctx context.Context, id string) (bool, error)
}

type SheetLinks interface {
	UnlinkCampaignBatch(ctx context.Context, campaignID string, limit int) (int64, error)
}

type MongoDeletionStore struct {
	campaigns *mongo.Collection
}

func NewDeletionStore(db *mongo.Database) *MongoDeletionStore {
	return &MongoDeletionStore{campaigns: db.Collection(campaignsCollection)}
}

func (s *MongoDeletionStore) ClaimDeleting(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]Campaign, error) {
	leaseUntil := now.Add(lease)
	claimed := make([]Campaign, 0, limit)
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

func (s *MongoDeletionStore) claimOne(ctx context.Context, now, leaseUntil time.Time) (Campaign, bool, error) {
	filter := bson.D{
		{Key: statusField, Value: StatusDeleting},
		{Key: andOperator, Value: bson.A{
			bson.D{{Key: orOperator, Value: bson.A{
				bson.D{{Key: deletionLockedUntil, Value: nil}},
				bson.D{{Key: deletionLockedUntil, Value: bson.D{{Key: lteOperator, Value: now}}}},
			}}},
			bson.D{{Key: orOperator, Value: bson.A{
				bson.D{{Key: deletionNextAttempt, Value: nil}},
				bson.D{{Key: deletionNextAttempt, Value: bson.D{{Key: lteOperator, Value: now}}}},
			}}},
		}},
	}
	update := bson.D{{Key: setOperator, Value: bson.D{{Key: deletionLockedUntil, Value: leaseUntil}}}}

	var doc Campaign
	err := s.campaigns.FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Campaign{}, false, nil
	}
	if err != nil {
		return Campaign{}, false, fmt.Errorf("claim deleting campaign: %w", err)
	}
	return doc, true, nil
}

func (s *MongoDeletionStore) RescheduleDeleting(ctx context.Context, id string, nextAt time.Time, lastError string) error {
	result, err := s.campaigns.UpdateOne(ctx,
		bson.D{{Key: idField, Value: id}, {Key: statusField, Value: StatusDeleting}},
		bson.D{
			{Key: setOperator, Value: bson.D{
				{Key: deletionNextAttempt, Value: nextAt},
				{Key: deletionLockedUntil, Value: nil},
				{Key: deletionLastErrorField, Value: lastError},
			}},
			{Key: incOperator, Value: bson.D{{Key: deletionAttemptsField, Value: 1}}},
		})
	if err != nil {
		return fmt.Errorf("reschedule campaign deletion: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrDeletionClaimLost
	}
	return nil
}

func (s *MongoDeletionStore) MarkDeletionDead(ctx context.Context, id string, lastError string) error {
	result, err := s.campaigns.UpdateOne(ctx,
		bson.D{{Key: idField, Value: id}, {Key: statusField, Value: StatusDeleting}},
		bson.D{{Key: setOperator, Value: bson.D{
			{Key: statusField, Value: StatusDeletionDead},
			{Key: deletionLockedUntil, Value: nil},
			{Key: deletionLastErrorField, Value: lastError},
		}}})
	if err != nil {
		return fmt.Errorf("mark campaign deletion dead: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrDeletionClaimLost
	}
	return nil
}

func (s *MongoDeletionStore) OldestDeleting(ctx context.Context) (time.Time, bool, error) {
	var doc struct {
		DeletingAt time.Time `bson:"deleting_at"`
	}
	opts := options.FindOne().SetSort(bson.D{{Key: deletingAtField, Value: 1}}).
		SetProjection(bson.D{{Key: deletingAtField, Value: 1}})
	err := s.campaigns.FindOne(ctx, bson.D{{Key: statusField, Value: StatusDeleting}}, opts).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("oldest deleting campaign: %w", err)
	}
	return doc.DeletingAt, true, nil
}

func (s *MongoDeletionStore) TouchDeleting(ctx context.Context, id string, now time.Time) (int64, error) {
	var doc struct {
		Version int64 `bson:"version"`
	}
	err := s.campaigns.FindOneAndUpdate(ctx,
		bson.D{{Key: idField, Value: id}, {Key: statusField, Value: StatusDeleting}},
		bson.D{
			{Key: incOperator, Value: bson.D{{Key: versionField, Value: 1}}},
			{Key: setOperator, Value: bson.D{{Key: updatedAtField, Value: now}}},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After).SetProjection(bson.D{{Key: versionField, Value: 1}}),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, ErrNotDeleting
	}
	if err != nil {
		return 0, fmt.Errorf("touch deleting campaign: %w", err)
	}
	return doc.Version, nil
}

func (s *MongoDeletionStore) RemoveDeleting(ctx context.Context, id string) (bool, error) {
	result, err := s.campaigns.DeleteOne(ctx, bson.D{{Key: idField, Value: id}, {Key: statusField, Value: StatusDeleting}})
	if err != nil {
		return false, fmt.Errorf("remove deleting campaign: %w", err)
	}
	return result.DeletedCount > 0, nil
}
