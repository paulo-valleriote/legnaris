package auth

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EmailNoticesCollection holds one claim per notice purpose and address, so a
// stranger registering with someone else's address cannot flood that mailbox.
// Claims expire through a TTL index on claimed_at.
const EmailNoticesCollection = "email_notices"

// NoticeRepository paces the code-free notices a registration can trigger.
type NoticeRepository interface {
	// Claim reserves the right to send the purpose's notice to the address. It
	// reports true for exactly one caller per window: when no claim exists or
	// the last one is older than since. It reports false when the notice is
	// throttled.
	Claim(ctx context.Context, purpose Purpose, email string, since, at time.Time) (bool, error)
}

// NoticeRepositoryImpl is the MongoDB-backed NoticeRepository.
type NoticeRepositoryImpl struct {
	collection *mongo.Collection
}

// NewNoticeRepository builds the notice store over the application database.
func NewNoticeRepository(db *mongo.Database) NoticeRepository {
	return &NoticeRepositoryImpl{collection: db.Collection(EmailNoticesCollection)}
}

// Claim upserts the (purpose, address) document only when its last claim is
// older than since. The document identity is the key, so a throttled caller
// hits a duplicate-key error on the upsert and concurrent callers cannot both
// win.
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
