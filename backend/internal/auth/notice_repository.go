package auth

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const EmailNoticesCollection = "email_notices"

type NoticeRepository interface {
	Claim(ctx context.Context, purpose Purpose, email string, since, at time.Time) (bool, error)
}

type NoticeRepositoryImpl struct {
	collection *mongo.Collection
}

func NewNoticeRepository(db *mongo.Database) NoticeRepository {
	return &NoticeRepositoryImpl{collection: db.Collection(EmailNoticesCollection)}
}

func (r *NoticeRepositoryImpl) Claim(ctx context.Context, purpose Purpose, email string, since, at time.Time) (bool, error) {
	key := string(purpose) + ":" + email
	_, err := r.collection.UpdateOne(
		ctx,
		bson.D{
			{Key: challengeFieldID, Value: key},
			{Key: noticeFieldClaimedAt, Value: bson.D{{Key: challengeLTOperator, Value: since}}},
		},
		bson.D{{Key: challengeSetOperator, Value: bson.D{{Key: noticeFieldClaimedAt, Value: at}}}},
		options.UpdateOne().SetUpsert(true),
	)
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim notice: %w", err)
	}
	return true, nil
}

const noticeFieldClaimedAt = "claimed_at"
