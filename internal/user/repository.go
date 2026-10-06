package user

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"legnaris/shared/logger"
)

type Repository interface {
	Create(ctx context.Context, user *User) error
	FetchByEmail(ctx context.Context, email string) (*User, error)
	FetchByID(ctx context.Context, id string) (*User, error)
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
	return r.fetchOne(ctx, bson.D{{Key: "email", Value: email}})
}

func (r *RepositoryImpl) FetchByID(ctx context.Context, id string) (*User, error) {
	return r.fetchOne(ctx, bson.D{{Key: "_id", Value: id}})
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
