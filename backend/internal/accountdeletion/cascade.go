// Package accountdeletion removes an account and its data (LGN-190, ADR 0013).
// The request only marks the user as deleting; the sweeper queue users_deleting
// then removes the Sheets, generations, consent and quota rows, the account's
// challenges and the e-mail rows that carry its address, writes a tombstone and
// the user.deleted event, and deletes the user record, all in one transaction.
// A failed run leaves the account unchanged, so a retry is safe.
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
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/events/outbox"
	"legnaris/backend/internal/quota"
)

// EventUserDeleted is the domain event announcing that an account was removed (ADR 0010).
const EventUserDeleted = "user.deleted"

// Collections and fields the cascade touches. They are named here because the
// cascade must run inside one transaction across the stores that own them.
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
)

// Tombstone is the record kept after an account is removed. It holds only the
// identifier and the deletion instant: no e-mail, username or other PII.
type Tombstone struct {
	ID        string    `bson:"_id"`
	DeletedAt time.Time `bson:"deleted_at"`
}

// Transactor runs a function inside a transaction; db.Transactor satisfies it.
type Transactor interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// Cascade removes one account's data. Every write uses the context it is given,
// so the caller decides the transaction boundary.
type Cascade struct {
	database *mongo.Database
	outbox   outbox.Repository
	now      func() time.Time
}

// NewCascade builds the cascade over the application database. The outbox entry
// is written through the outbox repository, so it joins the caller's transaction.
func NewCascade(database *mongo.Database, outboxRepo outbox.Repository, now func() time.Time) *Cascade {
	return &Cascade{database: database, outbox: outboxRepo, now: now}
}

// Remove deletes everything owned by userID and the user record, then the
// challenges and the e-mail rows that carry the account's address, and records
// the tombstone and the user.deleted event. An account whose record is already gone was
// removed by an earlier committed run, so Remove then returns without a second
// event. Callers run it in one transaction, which makes a failed run leave nothing behind.
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

// accountRecord is the part of the user document the cascade needs after the
// record is gone: the address the e-mail rows are keyed by.
type accountRecord struct {
	Email string `bson:"email"`
}

// deleteUser removes the user record and returns it, so the address rows can be
// erased after the record is gone. It reports false when no record existed.
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

// deleteAddressRows erases the rows keyed by the account's address rather than
// its identifier: the e-mail outbox messages sent to it and the notice claims,
// whose identity is "<purpose>:<address>". An account without an address has
// no such rows.
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
