package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"legnaris/shared/logger"
)

// Stored field names shared by every filter and projection in this file.
const (
	fieldID    = "_id"
	fieldEmail = "email"
)

type Repository interface {
	Create(ctx context.Context, user *User) error
	FetchByEmail(ctx context.Context, email string) (*User, error)
	FetchByID(ctx context.Context, id string) (*User, error)
	MarkEmailVerified(ctx context.Context, id string, at time.Time) error
}

type RepositoryImpl struct {
	collection *mongo.Collection
	logger     logger.StructuredLogger
}

// ErrUserNotFound reports a lookup that matched no user document. Services
// translate it to apperr.CodeNotFound (or a generic 401) at the boundary.
var ErrUserNotFound = errors.New("user not found")

// ErrEmailAlreadyExists reports a create rejected by the unique email index.
// Services translate it to apperr.CodeConflict at the boundary.
var ErrEmailAlreadyExists = errors.New("email already exists")

func NewRepository(db *mongo.Database, log logger.StructuredLogger) Repository {
	return &RepositoryImpl{
		collection: db.Collection("users"),
		logger:     log,
	}
}

func (r *RepositoryImpl) Create(ctx context.Context, user *User) error {
	if _, err := r.collection.InsertOne(ctx, user); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrEmailAlreadyExists
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *RepositoryImpl) FetchByEmail(ctx context.Context, email string) (*User, error) {
	return r.fetchOne(ctx, bson.D{{Key: fieldEmail, Value: email}})
}

func (r *RepositoryImpl) FetchByID(ctx context.Context, id string) (*User, error) {
	return r.fetchOne(ctx, bson.D{{Key: fieldID, Value: id}})
}

// MarkEmailVerified records the first verification instant for an existing
// account. The conditional filter makes the write idempotent and keeps the first
// value under concurrency; only email_verified_at and updated_at change, and an
// unknown account is never created (PRQ-13).
func (r *RepositoryImpl) MarkEmailVerified(ctx context.Context, id string, at time.Time) error {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.D{
			{Key: fieldID, Value: id},
			{Key: "email_verified_at", Value: nil},
		},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "email_verified_at", Value: at},
			{Key: "updated_at", Value: at},
		}}},
	)
	if err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}
	if result.MatchedCount == 1 {
		return nil
	}

	// Nothing matched: either the account is already verified (the field is
	// non-nil) or it does not exist. A read distinguishes the two without
	// creating anything.
	if _, err := r.FetchByID(ctx, id); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("mark email verified: %w", err)
	}
	return nil
}

func (r *RepositoryImpl) fetchOne(ctx context.Context, filter bson.D) (*User, error) {
	var result User
	if err := r.collection.FindOne(ctx, filter).Decode(&result); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("fetch user: %w", err)
	}
	return &result, nil
}

// ListEmailIdentities reads every account's identifier and stored address,
// projecting nothing else, so the migration never loads credential material
// (PRQ-11).
func (r *RepositoryImpl) ListEmailIdentities(ctx context.Context) ([]EmailIdentity, error) {
	cursor, err := r.collection.Find(ctx, bson.D{}, options.Find().SetProjection(bson.D{
		{Key: fieldID, Value: 1},
		{Key: fieldEmail, Value: 1},
	}))
	if err != nil {
		return nil, fmt.Errorf("list email identities: %w", err)
	}

	var documents []struct {
		ID    string `bson:"_id"`
		Email string `bson:"email"`
	}
	if err := cursor.All(ctx, &documents); err != nil {
		return nil, fmt.Errorf("list email identities: %w", err)
	}

	identities := make([]EmailIdentity, 0, len(documents))
	for _, document := range documents {
		identities = append(identities, EmailIdentity{ID: document.ID, Email: document.Email})
	}
	return identities, nil
}

// UpdateEmail rewrites one row's stored address and updated_at, matching by
// identifier so no other field changes and no document is ever created
// (PRQ-11).
func (r *RepositoryImpl) UpdateEmail(ctx context.Context, id string, email string, updatedAt time.Time) error {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.D{{Key: fieldID, Value: id}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: fieldEmail, Value: email},
			{Key: "updated_at", Value: updatedAt},
		}}},
	)
	if err != nil {
		return fmt.Errorf("update email: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrUserNotFound
	}
	return nil
}

// NewEmailMigrationStore exposes the migration seam without widening the
// Repository contract consumed by the API (PRQ-11).
func NewEmailMigrationStore(db *mongo.Database, log logger.StructuredLogger) EmailMigrationStore {
	return &RepositoryImpl{
		collection: db.Collection("users"),
		logger:     log,
	}
}
