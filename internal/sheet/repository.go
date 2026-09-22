package sheet

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Repository interface {
	Create(ctx context.Context, sheet *Sheet) error
	FetchByID(ctx context.Context, id string) (*Sheet, error)
	PartialUpdate(ctx context.Context, id string, dbUpdates map[string]any) error
}

type RepositoryImpl struct {
	client *mongo.Collection
}

func NewRepository(client *mongo.Database) Repository {
	return &RepositoryImpl{
		client: client.Collection("sheets"),
	}
}

func (r *RepositoryImpl) FetchByID(ctx context.Context, id string) (*Sheet, error) {
	var result Sheet

	filter := bson.D{{Key: "_id", Value: id}}
	opts := options.FindOne()
	err := r.client.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("sheet not found")
		} else {
			return nil, err
		}
	}

	return &result, nil
}

func (r *RepositoryImpl) Create(ctx context.Context, sheet *Sheet) error {
	result, err := r.client.InsertOne(ctx, sheet)
	if err != nil {
		return fmt.Errorf("error creating sheet: %w", err)
	}
	slog.Info("sheet created with ID:", result.InsertedID)
	return nil
}

func (r *RepositoryImpl) PartialUpdate(ctx context.Context, id string, dbUpdates map[string]any) error {
	filter := bson.D{{Key: "_id", Value: id}}
	update := bson.M{"$set": dbUpdates}

	_, err := r.client.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("error creating sheet: %w", err)
	}
	return nil
}
