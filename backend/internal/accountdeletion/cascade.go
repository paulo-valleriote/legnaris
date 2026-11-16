package accountdeletion

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/events/outbox"
	"legnaris/backend/internal/quota"
)

const EventUserDeleted = "user.deleted"

const (
	sheetsCollection      = "sheets"
	sheetCountsCollection = "sheet_counts"
	generationsCollection = "generations"
	usersCollection       = "users"
	tombstonesCollection  = "user_tombstones"
	challengesCollection  = "auth_challenges"
	emailOutboxCollection = "email_outbox"
	noticesCollection     = "email_notices"
	ownerIDField          = "owner_id"
	userIDField           = "user_id"
	idField               = "_id"
	emailField            = "email"
	toField               = "to"
	deletedAtField        = "deleted_at"
	expiresAtField        = "expires_at"
)

type Tombstone struct {
	ID        string    `bson:"_id"`
	DeletedAt time.Time `bson:"deleted_at"`
}

type Transactor interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type Cascade struct {
	database *mongo.Database
	outbox   outbox.Repository
	now      func() time.Time
}

func NewCascade(database *mongo.Database, outboxRepo outbox.Repository, now func() time.Time) *Cascade {
	return &Cascade{database: database, outbox: outboxRepo, now: now}
}

func (c *Cascade) Remove(ctx context.Context, userID string) error {
	steps := []struct {
		collection, field string
	}{
		{sheetsCollection, ownerIDField},
		{sheetCountsCollection, ownerIDField},
		{generationsCollection, userIDField},
		{quota.TrialCollection, idField},
		{quota.UsageCollection, userIDField},
	}
	for _, step := range steps {
		if err := c.deleteOwned(ctx, step.collection, step.field, userID); err != nil {
			return err
		}
	}

	account, removed, err := c.deleteUser(ctx, userID)
	if err != nil {
		return err
	}
	if !removed {
		return nil
	}

	if err := c.deleteOwned(ctx, challengesCollection, userIDField, userID); err != nil {
		return err
	}
	if err := c.deleteAddressRows(ctx, account.Email); err != nil {
		return err
	}

	now := c.now()
	if err := c.retireBilling(ctx, userID, now); err != nil {
		return err
	}
	if err := c.writeTombstone(ctx, userID, now); err != nil {
		return err
	}
	return c.insertUserDeleted(ctx, userID, now)
}

func (c *Cascade) deleteOwned(ctx context.Context, collection, field, userID string) error {
	if _, err := c.database.Collection(collection).DeleteMany(ctx, bson.D{{Key: field, Value: userID}}); err != nil {
		return fmt.Errorf("delete %s for account: %w", collection, err)
	}
	return nil
}

func (c *Cascade) retireBilling(ctx context.Context, userID string, at time.Time) error {
	if _, err := c.database.Collection(billing.EventsCollection).DeleteMany(ctx, bson.D{{Key: userIDField, Value: userID}}); err != nil {
		return fmt.Errorf("delete %s for account: %w", billing.EventsCollection, err)
	}
	if _, err := c.database.Collection(billing.UserLocksCollection).DeleteOne(ctx, bson.D{{Key: idField, Value: userID}}); err != nil {
		return fmt.Errorf("delete %s for account: %w", billing.UserLocksCollection, err)
	}
	_, err := c.database.Collection(billing.SubscriptionsCollection).UpdateMany(ctx,
		bson.D{{Key: userIDField, Value: userID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "status", Value: billing.StatusCanceled},
			{Key: expiresAtField, Value: billing.RetentionUntil(at)},
		}}})
	if err != nil {
		return fmt.Errorf("retain %s for account: %w", billing.SubscriptionsCollection, err)
	}
	return nil
}

func (c *Cascade) writeTombstone(ctx context.Context, userID string, at time.Time) error {
	_, err := c.database.Collection(tombstonesCollection).UpdateOne(ctx,
		bson.D{{Key: idField, Value: userID}},
		bson.D{{Key: "$setOnInsert", Value: bson.D{{Key: deletedAtField, Value: at}}}},
		options.UpdateOne().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("write account tombstone: %w", err)
	}
	return nil
}

func (c *Cascade) insertUserDeleted(ctx context.Context, userID string, at time.Time) error {
	entry, err := outbox.NewEntry(events.Event{
		Type:          EventUserDeleted,
		AggregateType: "user",
		AggregateID:   userID,
		Data:          map[string]any{},
	}, at)
	if err != nil {
		return fmt.Errorf("build user.deleted entry: %w", err)
	}
	return c.outbox.Insert(ctx, entry)
}

type accountRecord struct {
	Email string `bson:"email"`
}

func (c *Cascade) deleteUser(ctx context.Context, userID string) (accountRecord, bool, error) {
	var account accountRecord
	err := c.database.Collection(usersCollection).FindOneAndDelete(ctx, bson.D{{Key: idField, Value: userID}}).Decode(&account)
	if err == nil {
		return account, true, nil
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return accountRecord{}, false, nil
	}
	return accountRecord{}, false, fmt.Errorf("delete account record: %w", err)
}

func (c *Cascade) deleteAddressRows(ctx context.Context, email string) error {
	if email == "" {
		return nil
	}
	if _, err := c.database.Collection(emailOutboxCollection).DeleteMany(ctx, bson.D{{Key: toField, Value: email}}); err != nil {
		return fmt.Errorf("delete %s for account: %w", emailOutboxCollection, err)
	}
	suffix := bson.D{{Key: "$regex", Value: ":" + regexp.QuoteMeta(email) + "$"}}
	if _, err := c.database.Collection(noticesCollection).DeleteMany(ctx, bson.D{{Key: idField, Value: suffix}}); err != nil {
		return fmt.Errorf("delete %s for account: %w", noticesCollection, err)
	}
	return nil
}
