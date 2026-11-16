package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"legnaris/backend/shared/logger"
)

const (
	fieldID              = "_id"
	fieldEmail           = "email"
	fieldUsername        = "username"
	fieldPasswordHash    = "password_hash"
	fieldEmailVerifiedAt = "email_verified_at"
	fieldUpdatedAt       = "updated_at"
	operatorSet          = "$set"
)

type Repository interface {
	Create(ctx context.Context, user *User) error
	FetchByEmail(ctx context.Context, email string) (*User, error)
	FetchByID(ctx context.Context, id string) (*User, error)
	FetchByUsername(ctx context.Context, username string) (*User, error)
	MarkEmailVerified(ctx context.Context, id string, at time.Time) error
	MarkDeleting(ctx context.Context, id string, at time.Time) error
	ReplaceUnverifiedCredentials(ctx context.Context, id, username, passwordHash string, at time.Time) (bool, error)
}

type RepositoryImpl struct {
	collection *mongo.Collection
	logger     logger.StructuredLogger
}

var ErrUserNotFound = errors.New("user not found")

var ErrEmailAlreadyExists = errors.New("email already exists")

var ErrUsernameAlreadyExists = errors.New("username already exists")

const UsernameUniqueIndex = "users_username_unique"

func translateDuplicateKey(err error) error {
	if strings.Contains(err.Error(), UsernameUniqueIndex) {
		return ErrUsernameAlreadyExists
	}
	return ErrEmailAlreadyExists
}

func NewRepository(db *mongo.Database, log logger.StructuredLogger) Repository {
	return &RepositoryImpl{
		collection: db.Collection("users"),
		logger:     log,
	}
}

func (r *RepositoryImpl) Create(ctx context.Context, user *User) error {
	if _, err := r.collection.InsertOne(ctx, user); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return translateDuplicateKey(err)
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

func (r *RepositoryImpl) FetchByUsername(ctx context.Context, username string) (*User, error) {
	return r.fetchOne(ctx, bson.D{{Key: fieldUsername, Value: username}})
}

func (r *RepositoryImpl) ReplaceUnverifiedCredentials(ctx context.Context, id, username, passwordHash string, at time.Time) (bool, error) {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.D{
			{Key: fieldID, Value: id},
			{Key: fieldEmailVerifiedAt, Value: nil},
		},
		bson.D{{Key: operatorSet, Value: bson.D{
			{Key: fieldUsername, Value: username},
			{Key: fieldPasswordHash, Value: passwordHash},
			{Key: fieldUpdatedAt, Value: at},
		}}},
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return false, translateDuplicateKey(err)
		}
		return false, fmt.Errorf("replace unverified credentials: %w", err)
	}
	return result.MatchedCount == 1, nil
}

func (r *RepositoryImpl) MarkEmailVerified(ctx context.Context, id string, at time.Time) error {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.D{
			{Key: fieldID, Value: id},
			{Key: fieldEmailVerifiedAt, Value: nil},
		},
		bson.D{{Key: operatorSet, Value: bson.D{
			{Key: fieldEmailVerifiedAt, Value: at},
			{Key: fieldUpdatedAt, Value: at},
		}}},
	)
	if err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}
	if result.MatchedCount == 1 {
		return nil
	}

	if _, err := r.FetchByID(ctx, id); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("mark email verified: %w", err)
	}
	return nil
}

func (r *RepositoryImpl) MarkDeleting(ctx context.Context, id string, at time.Time) error {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.D{
			{Key: fieldID, Value: id},
			{Key: fieldStatus, Value: bson.D{{Key: "$in", Value: bson.A{nil, ""}}}},
		},
		bson.D{{Key: operatorSet, Value: bson.D{
			{Key: fieldStatus, Value: StatusDeleting},
			{Key: fieldDeletingAt, Value: at},
			{Key: fieldUpdatedAt, Value: at},
		}}},
	)
	if err != nil {
		return fmt.Errorf("mark account deleting: %w", err)
	}
	if result.MatchedCount == 1 {
		return nil
	}

	if _, err := r.FetchByID(ctx, id); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("mark account deleting: %w", err)
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

func (r *RepositoryImpl) UpdateEmail(ctx context.Context, id string, email string, updatedAt time.Time) error {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.D{{Key: fieldID, Value: id}},
		bson.D{{Key: operatorSet, Value: bson.D{
			{Key: fieldEmail, Value: email},
			{Key: fieldUpdatedAt, Value: updatedAt},
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

func NewEmailMigrationStore(db *mongo.Database, log logger.StructuredLogger) EmailMigrationStore {
	return &RepositoryImpl{
		collection: db.Collection("users"),
		logger:     log,
	}
}
