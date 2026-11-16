package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	SubscriptionsCollection = "subscriptions"
	fieldStatus             = "status"
	fieldUserID             = "user_id"
	fieldUpdatedAt          = "updated_at"
	fieldID                 = "_id"
)

type MongoStore struct {
	subscriptions *mongo.Collection
	events        *mongo.Collection
	locks         *mongo.Collection
	users         *mongo.Collection
}

func NewMongoStore(database *mongo.Database) *MongoStore {
	return &MongoStore{
		subscriptions: database.Collection(SubscriptionsCollection),
		events:        database.Collection(EventsCollection),
		locks:         database.Collection(UserLocksCollection),
		users:         database.Collection(usersCollection),
	}
}

func liveFilter(userID string) bson.D {
	return bson.D{
		{Key: fieldUserID, Value: userID},
		{Key: fieldStatus, Value: bson.D{{Key: "$in", Value: liveStatuses()}}},
	}
}

func (s *MongoStore) FindLive(ctx context.Context, userID string) (Subscription, bool, error) {
	var sub Subscription
	err := s.subscriptions.FindOne(ctx, liveFilter(userID)).Decode(&sub)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Subscription{}, false, nil
	}
	if err != nil {
		return Subscription{}, false, fmt.Errorf("find live subscription: %w", err)
	}
	return sub, true, nil
}

func (s *MongoStore) InsertPending(ctx context.Context, sub Subscription) error {
	if _, err := s.subscriptions.InsertOne(ctx, sub); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrLiveExists
		}
		return fmt.Errorf("insert pending subscription: %w", err)
	}
	return nil
}

func (s *MongoStore) Cancel(ctx context.Context, id string) error {
	_, err := s.subscriptions.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: fieldStatus, Value: StatusPending}},
		bson.D{{Key: "$set", Value: bson.D{{Key: fieldStatus, Value: StatusCanceled}, {Key: fieldUpdatedAt, Value: time.Now().UTC()}}}},
	)
	if err != nil {
		return fmt.Errorf("cancel pending subscription: %w", err)
	}
	return nil
}

func (s *MongoStore) AttachCheckout(ctx context.Context, id, customerID, url string, expiresAt time.Time) (bool, error) {
	res, err := s.subscriptions.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: fieldStatus, Value: StatusPending}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "provider_customer_id", Value: customerID},
			{Key: "checkout_url", Value: url},
			{Key: "checkout_expires_at", Value: expiresAt},
			{Key: fieldUpdatedAt, Value: time.Now().UTC()},
		}}},
	)
	if err != nil {
		return false, fmt.Errorf("attach checkout session: %w", err)
	}
	return res.MatchedCount > 0, nil
}
