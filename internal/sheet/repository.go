package sheet

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Repository interface {
	Create(ctx context.Context, sheet *Sheet) error
}

type RepositoryImpl struct {
	client *mongo.Collection
}

func NewRepository(client *mongo.Database) Repository {
	return &RepositoryImpl{
		client: client.Collection("sheets"),
	}
}

// TODO: Filter by ID or Name
func (r *RepositoryImpl) List(ctx context.Context) ([]Sheet, error) {
	var results []Sheet

	cursor, err := r.client.Find(ctx, bson.M{})
	if err != nil {
		return []Sheet{}, fmt.Errorf("error while listing: %w", err)
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &results); err != nil {
		log.Fatal("error decoding results: %w", err)
	}
	if len(results) == 0 {
		return []Sheet{}, nil
	}
	return results, nil
}

func (r *RepositoryImpl) Create(ctx context.Context, sheet *Sheet) error {
	result, err := r.client.InsertOne(ctx, sheet)
	if err != nil {
		return fmt.Errorf("error creating sheet: %w", err)
	}
	slog.Info("sheet created with ID:", result.InsertedID)
	return nil
}
