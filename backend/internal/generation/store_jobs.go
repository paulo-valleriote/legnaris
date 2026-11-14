package generation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Stored field names of the worker's lease and reap state.
const (
	fieldLeaseUntil   = "lease_until"
	fieldReapLease    = "reap_lease_until"
	fieldReapAttempts = "reap_attempts"
	fieldValues       = "values"
)

// ClaimQueued moves a queued document to running under a lease that lasts until
// now+lease, and counts the attempt. A document that is not queued any more, a
// duplicate delivery, or one the account deletion removed, is (zero, false, nil):
// the claim is a no-op and the message is acknowledged.
func (s *MongoStore) ClaimQueued(ctx context.Context, id string, now time.Time, lease time.Duration) (Document, bool, error) {
	filter := bson.D{{Key: fieldID, Value: id}, {Key: fieldStatus, Value: StatusQueued}}
	update := bson.D{
		{Key: operatorSet, Value: bson.D{{Key: fieldStatus, Value: StatusRunning}, {Key: fieldLeaseUntil, Value: now.Add(lease)}}},
		{Key: operatorInc, Value: bson.D{{Key: "attempts", Value: 1}}},
	}
	var doc Document
	err := s.generations.FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Document{}, false, nil
	}
	if err != nil {
		return Document{}, false, fmt.Errorf("claim generation %s: %w", id, err)
	}
	return doc, true, nil
}

// Complete stores the validated values of a running document and moves it to
// succeeded. The filter on status=running makes the write a no-op for a document
// that is gone (account deletion, consent revocation) or no longer running (its
// lease was reaped). It reports whether the write landed.
func (s *MongoStore) Complete(ctx context.Context, id string, values map[string]any) (bool, error) {
	filter := bson.D{{Key: fieldID, Value: id}, {Key: fieldStatus, Value: StatusRunning}}
	update := bson.D{
		{Key: operatorSet, Value: bson.D{{Key: fieldStatus, Value: StatusSucceeded}, {Key: fieldValues, Value: values}}},
		{Key: operatorUnset, Value: releaseFields()},
	}
	res, err := s.generations.UpdateOne(ctx, filter, update)
	if err != nil {
		return false, fmt.Errorf("complete generation %s: %w", id, err)
	}
	return res.MatchedCount == 1, nil
}

// FailRunning moves a running document to failed with code and releases its
// in-progress slot. It returns the document as it was before the move, so the
// caller can refund its quota in the same transaction, and false when the document
// was not running (already settled, or removed).
func (s *MongoStore) FailRunning(ctx context.Context, id, code string) (Document, bool, error) {
	filter := bson.D{{Key: fieldID, Value: id}, {Key: fieldStatus, Value: StatusRunning}}
	return s.failWhere(ctx, filter, code)
}

// FailExpired fails a running document whose lease ended at or before now, with
// the lease_expired code. The lease condition makes the reap idempotent and stops
// it from failing a job that still holds a live lease.
func (s *MongoStore) FailExpired(ctx context.Context, id string, now time.Time, code string) (Document, bool, error) {
	filter := bson.D{
		{Key: fieldID, Value: id},
		{Key: fieldStatus, Value: StatusRunning},
		{Key: fieldLeaseUntil, Value: bson.D{{Key: operatorLte, Value: now}}},
	}
	return s.failWhere(ctx, filter, code)
}

// failWhere applies the running-to-failed transition to the documents filter selects.
func (s *MongoStore) failWhere(ctx context.Context, filter bson.D, code string) (Document, bool, error) {
	update := bson.D{
		{Key: operatorSet, Value: bson.D{{Key: fieldStatus, Value: StatusFailed}, {Key: fieldErrorCode, Value: code}}},
		{Key: operatorUnset, Value: releaseFields()},
	}
	var before Document
	err := s.generations.FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetReturnDocument(options.Before)).Decode(&before)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Document{}, false, nil
	}
	if err != nil {
		return Document{}, false, fmt.Errorf("fail generation: %w", err)
	}
	return before, true, nil
}

// releaseFields are the fields a terminal transition clears: the in-progress slot
// the sparse unique index guards, and the worker lease.
func releaseFields() bson.D {
	return bson.D{{Key: fieldActiveUser, Value: ""}, {Key: fieldLeaseUntil, Value: ""}}
}

// ClaimExpired leases up to limit running documents whose worker lease has ended and
// whose previous reap lease has expired, and returns them. The claim is one atomic
// find-and-update per document, so two sweepers never reap the same one.
func (s *MongoStore) ClaimExpired(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]Document, error) {
	var claimed []Document
	for range limit {
		doc, ok, err := s.claimExpiredOne(ctx, now, now.Add(lease))
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

// claimExpiredOne takes one expired running document and extends its reap lease.
func (s *MongoStore) claimExpiredOne(ctx context.Context, now, reapUntil time.Time) (Document, bool, error) {
	filter := bson.D{
		{Key: fieldStatus, Value: StatusRunning},
		{Key: fieldLeaseUntil, Value: bson.D{{Key: operatorLte, Value: now}}},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: fieldReapLease, Value: bson.D{{Key: "$exists", Value: false}}}},
			bson.D{{Key: fieldReapLease, Value: bson.D{{Key: operatorLte, Value: now}}}},
		}},
	}
	update := bson.D{{Key: operatorSet, Value: bson.D{{Key: fieldReapLease, Value: reapUntil}}}}
	var doc Document
	err := s.generations.FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Document{}, false, nil
	}
	if err != nil {
		return Document{}, false, fmt.Errorf("claim expired generation: %w", err)
	}
	return doc, true, nil
}

// RescheduleExpired makes a running document due for the reaper again at nextAt.
func (s *MongoStore) RescheduleExpired(ctx context.Context, id string, nextAt time.Time) error {
	filter := bson.D{{Key: fieldID, Value: id}, {Key: fieldStatus, Value: StatusRunning}}
	update := bson.D{
		{Key: operatorSet, Value: bson.D{{Key: fieldReapLease, Value: nextAt}}},
		{Key: operatorInc, Value: bson.D{{Key: fieldReapAttempts, Value: 1}}},
	}
	if _, err := s.generations.UpdateOne(ctx, filter, update); err != nil {
		return fmt.Errorf("reschedule expired generation %s: %w", id, err)
	}
	return nil
}

// OldestRunning returns the creation time of the oldest running document, the
// backlog age the reaper reports.
func (s *MongoStore) OldestRunning(ctx context.Context) (time.Time, bool, error) {
	var row struct {
		CreatedAt time.Time `bson:"created_at"`
	}
	opts := options.FindOne().
		SetSort(bson.D{{Key: fieldCreatedAt, Value: 1}}).
		SetProjection(bson.D{{Key: fieldCreatedAt, Value: 1}})
	err := s.generations.FindOne(ctx, bson.D{{Key: fieldStatus, Value: StatusRunning}}, opts).Decode(&row)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("find oldest running generation: %w", err)
	}
	return row.CreatedAt, true, nil
}
