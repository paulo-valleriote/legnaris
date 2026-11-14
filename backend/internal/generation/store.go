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

// Collection names of the generation storage.
const (
	generationsCollection = "generations"
	capCollection         = "generation_cap"
)

// Stored field names shared by the filters and updates below.
const (
	fieldID           = "_id"
	fieldUserID       = "user_id"
	fieldStatus       = "status"
	fieldActiveUser   = "active_user_id"
	fieldCreatedAt    = "created_at"
	fieldCount        = "count"
	fieldExpiresAt    = "expires_at"
	fieldPublishLease = "publish_lease_until"
	fieldErrorCode    = "error_code"
	fieldSheetID      = "sheet_id"
	operatorSet       = "$set"
	operatorUnset     = "$unset"
	operatorInc       = "$inc"
	operatorSetOnIns  = "$setOnInsert"
	operatorLte       = "$lte"
)

// capRow is the stored shape of one day's global cap counter; its _id is the UTC day.
type capRow struct {
	Count int `bson:"count"`
}

// MongoStore implements Store and PublishStore over the generations and
// generation_cap collections. Every method uses the context it is given, so
// inside a transaction the writes join it.
type MongoStore struct {
	generations *mongo.Collection
	caps        *mongo.Collection
}

// NewMongoStore returns the generation store of db.
func NewMongoStore(db *mongo.Database) *MongoStore {
	return &MongoStore{
		generations: db.Collection(generationsCollection),
		caps:        db.Collection(capCollection),
	}
}

// Insert stores a queued document. The sparse unique index on active_user_id turns a
// second in-progress document of the same player into ErrActiveExists.
func (s *MongoStore) Insert(ctx context.Context, doc Document) error {
	if _, err := s.generations.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrActiveExists
		}
		return fmt.Errorf("insert generation: %w", err)
	}
	return nil
}

// ActiveByUser returns the player's queued or running generation, if any.
func (s *MongoStore) ActiveByUser(ctx context.Context, userID string) (string, bool, error) {
	var row struct {
		ID string `bson:"_id"`
	}
	err := s.generations.FindOne(ctx, bson.D{{Key: fieldActiveUser, Value: userID}},
		options.FindOne().SetProjection(bson.D{{Key: fieldID, Value: 1}})).Decode(&row)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find active generation: %w", err)
	}
	return row.ID, true, nil
}

// FindOwned returns the document when it belongs to userID.
func (s *MongoStore) FindOwned(ctx context.Context, userID, id string) (Document, error) {
	var doc Document
	filter := bson.D{{Key: fieldID, Value: id}, {Key: fieldUserID, Value: userID}}
	err := s.generations.FindOne(ctx, filter).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Document{}, ErrNotFound
	}
	if err != nil {
		return Document{}, fmt.Errorf("find generation: %w", err)
	}
	return doc, nil
}

// MarkAccepted moves a succeeded document to accepted. The filter on status makes
// the transition happen once.
func (s *MongoStore) MarkAccepted(ctx context.Context, userID, id, sheetID string) (bool, error) {
	filter := bson.D{
		{Key: fieldID, Value: id},
		{Key: fieldUserID, Value: userID},
		{Key: fieldStatus, Value: StatusSucceeded},
	}
	update := bson.D{
		{Key: operatorSet, Value: bson.D{{Key: fieldStatus, Value: StatusAccepted}, {Key: fieldSheetID, Value: sheetID}}},
		{Key: operatorUnset, Value: bson.D{{Key: fieldActiveUser, Value: ""}}},
	}
	res, err := s.generations.UpdateOne(ctx, filter, update)
	if err != nil {
		return false, fmt.Errorf("mark generation accepted: %w", err)
	}
	return res.MatchedCount == 1, nil
}

// ReserveCap opens the day's counter when it is missing, then increments it while
// it is below limit. The open is an upsert on the day's _id, so concurrent first
// requests converge on one row.
func (s *MongoStore) ReserveCap(ctx context.Context, dayUTC string, limit int, expiresAt time.Time) (int, bool, error) {
	open := bson.D{{Key: operatorSetOnIns, Value: bson.D{{Key: fieldCount, Value: 0}, {Key: fieldExpiresAt, Value: expiresAt}}}}
	if _, err := s.caps.UpdateOne(ctx, bson.D{{Key: fieldID, Value: dayUTC}}, open, options.UpdateOne().SetUpsert(true)); err != nil {
		return 0, false, fmt.Errorf("open generation cap: %w", err)
	}

	below := bson.D{{Key: fieldID, Value: dayUTC}, {Key: fieldCount, Value: bson.D{{Key: "$lt", Value: limit}}}}
	var row capRow
	err := s.caps.FindOneAndUpdate(ctx, below,
		bson.D{{Key: operatorInc, Value: bson.D{{Key: fieldCount, Value: 1}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&row)
	if err == nil {
		return row.Count, true, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return 0, false, fmt.Errorf("increment generation cap: %w", err)
	}
	return s.readCap(ctx, dayUTC)
}

// readCap returns the day's count without changing it.
func (s *MongoStore) readCap(ctx context.Context, dayUTC string) (int, bool, error) {
	var row capRow
	err := s.caps.FindOne(ctx, bson.D{{Key: fieldID, Value: dayUTC}}).Decode(&row)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read generation cap: %w", err)
	}
	return row.Count, false, nil
}

// ClaimUnpublished leases up to limit queued documents whose publish is due. Each
// claim is one atomic find-and-update, so two sweepers never lease the same item.
func (s *MongoStore) ClaimUnpublished(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]Document, error) {
	var claimed []Document
	for range limit {
		doc, ok, err := s.claimOne(ctx, now, now.Add(lease))
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

// claimOne takes one due queued document and extends its publish lease to leaseUntil.
func (s *MongoStore) claimOne(ctx context.Context, now, leaseUntil time.Time) (Document, bool, error) {
	filter := bson.D{
		{Key: fieldStatus, Value: StatusQueued},
		{Key: fieldCreatedAt, Value: bson.D{{Key: operatorLte, Value: now.Add(-RepublishAge)}}},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: fieldPublishLease, Value: bson.D{{Key: "$exists", Value: false}}}},
			bson.D{{Key: fieldPublishLease, Value: bson.D{{Key: operatorLte, Value: now}}}},
		}},
	}
	update := bson.D{{Key: operatorSet, Value: bson.D{{Key: fieldPublishLease, Value: leaseUntil}}}}
	var doc Document
	err := s.generations.FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Document{}, false, nil
	}
	if err != nil {
		return Document{}, false, fmt.Errorf("claim unpublished generation: %w", err)
	}
	return doc, true, nil
}

// ReschedulePublish counts one failed publish and makes the document due at nextAt.
func (s *MongoStore) ReschedulePublish(ctx context.Context, id string, nextAt time.Time) error {
	filter := bson.D{{Key: fieldID, Value: id}, {Key: fieldStatus, Value: StatusQueued}}
	update := bson.D{
		{Key: operatorSet, Value: bson.D{{Key: fieldPublishLease, Value: nextAt}}},
		{Key: operatorInc, Value: bson.D{{Key: "publish_attempts", Value: 1}}},
	}
	if _, err := s.generations.UpdateOne(ctx, filter, update); err != nil {
		return fmt.Errorf("reschedule generation publish: %w", err)
	}
	return nil
}

// FailQueued moves a queued document to failed and releases its in-progress slot.
// It returns the document as it was before the move.
func (s *MongoStore) FailQueued(ctx context.Context, id, code string) (Document, bool, error) {
	filter := bson.D{{Key: fieldID, Value: id}, {Key: fieldStatus, Value: StatusQueued}}
	update := bson.D{
		{Key: operatorSet, Value: bson.D{{Key: fieldStatus, Value: StatusFailed}, {Key: fieldErrorCode, Value: code}}},
		{Key: operatorUnset, Value: bson.D{{Key: fieldActiveUser, Value: ""}}},
	}
	var before Document
	err := s.generations.FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetReturnDocument(options.Before)).Decode(&before)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Document{}, false, nil
	}
	if err != nil {
		return Document{}, false, fmt.Errorf("fail queued generation: %w", err)
	}
	return before, true, nil
}

// OldestQueued returns the creation time of the oldest queued document.
func (s *MongoStore) OldestQueued(ctx context.Context) (time.Time, bool, error) {
	var row struct {
		CreatedAt time.Time `bson:"created_at"`
	}
	opts := options.FindOne().
		SetSort(bson.D{{Key: fieldCreatedAt, Value: 1}}).
		SetProjection(bson.D{{Key: fieldCreatedAt, Value: 1}})
	err := s.generations.FindOne(ctx, bson.D{{Key: fieldStatus, Value: StatusQueued}}, opts).Decode(&row)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("find oldest queued generation: %w", err)
	}
	return row.CreatedAt, true, nil
}
