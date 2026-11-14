package llmconsent

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
	usersCollection       = "users"
	idField               = "_id"
	generationsCollection = "generations"
	generationOwnerField  = "user_id"
	generationStatusField = "status"
	consentAtField        = "llm_consent_at"
	consentVersionField   = "llm_consent_version"
)

type consentDocument struct {
	At      *time.Time `bson:"llm_consent_at"`
	Version string     `bson:"llm_consent_version"`
}

type mongoConsentStore struct {
	users *mongo.Collection
}

// NewConsentStore reads and writes the consent fields of the users collection.
func NewConsentStore(mdb *mongo.Database) ConsentStore {
	return &mongoConsentStore{users: mdb.Collection(usersCollection)}
}

// Fetch returns the stored consent, or ErrUserNotFound when the user is absent.
func (s *mongoConsentStore) Fetch(ctx context.Context, userID string) (Record, error) {
	var doc consentDocument
	opts := options.FindOne().SetProjection(bson.M{consentAtField: 1, consentVersionField: 1})
	err := s.users.FindOne(ctx, bson.M{idField: userID}, opts).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Record{}, ErrUserNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("fetch llm consent: %w", err)
	}
	return Record(doc), nil
}

// Grant sets the consent fields unless the user already holds this version.
// A missing user is ErrUserNotFound; an already-granted version is a no-op.
func (s *mongoConsentStore) Grant(ctx context.Context, userID, version string, at time.Time) error {
	filter := bson.M{idField: userID, consentVersionField: bson.M{"$ne": version}}
	update := bson.M{"$set": bson.M{consentAtField: at, consentVersionField: version}}
	res, err := s.users.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("grant llm consent: %w", err)
	}
	if res.MatchedCount > 0 {
		return nil
	}
	return s.requireUser(ctx, userID)
}

// Clear unsets both consent fields. A missing user or an absent consent is a
// no-op.
func (s *mongoConsentStore) Clear(ctx context.Context, userID string) error {
	update := bson.M{"$unset": bson.M{consentAtField: "", consentVersionField: ""}}
	if _, err := s.users.UpdateOne(ctx, bson.M{idField: userID}, update); err != nil {
		return fmt.Errorf("clear llm consent: %w", err)
	}
	return nil
}

func (s *mongoConsentStore) requireUser(ctx context.Context, userID string) error {
	count, err := s.users.CountDocuments(ctx, bson.M{idField: userID})
	if err != nil {
		return fmt.Errorf("check user for llm consent: %w", err)
	}
	if count == 0 {
		return ErrUserNotFound
	}
	return nil
}

type mongoGenerationStore struct {
	generations *mongo.Collection
}

// NewGenerationStore deletes generation documents by owner. An absent
// collection holds no documents, so the delete reports zero and no error.
func NewGenerationStore(mdb *mongo.Database) GenerationStore {
	return &mongoGenerationStore{generations: mdb.Collection(generationsCollection)}
}

// openStatuses are the generation statuses whose quota is still reserved: a
// queued or running job has not reached a terminal state yet.
var openStatuses = []string{"queued", "running"}

type openGenerationDocument struct {
	ID    string `bson:"_id"`
	Quota struct {
		Kind   string `bson:"kind"`
		DayUTC string `bson:"day_utc"`
	} `bson:"quota"`
}

// ListOpenByUserID returns the queued and running generations of userID with
// the quota reservation each one holds. An absent collection holds none.
func (s *mongoGenerationStore) ListOpenByUserID(ctx context.Context, userID string) ([]OpenGeneration, error) {
	filter := bson.M{generationOwnerField: userID, generationStatusField: bson.M{"$in": openStatuses}}
	cursor, err := s.generations.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("find open generations: %w", err)
	}
	var docs []openGenerationDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode open generations: %w", err)
	}
	open := make([]OpenGeneration, 0, len(docs))
	for _, doc := range docs {
		open = append(open, OpenGeneration{
			ID:              doc.ID,
			UserID:          userID,
			ReservationKind: doc.Quota.Kind,
			DayUTC:          doc.Quota.DayUTC,
		})
	}
	return open, nil
}

// DeleteByUserID removes every generation document owned by userID and returns
// how many it removed.
func (s *mongoGenerationStore) DeleteByUserID(ctx context.Context, userID string) (int64, error) {
	res, err := s.generations.DeleteMany(ctx, bson.M{generationOwnerField: userID})
	if err != nil {
		return 0, fmt.Errorf("delete generations: %w", err)
	}
	return res.DeletedCount, nil
}
