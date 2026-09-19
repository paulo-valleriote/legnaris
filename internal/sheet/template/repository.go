package template

import (
	"context"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Repository struct {
	client *mongo.Collection
}

func NewRepository(client *mongo.Database) *Repository {
	return &Repository{
		client: client.Collection("template"),
	}
}

// TODO: Filter by ID or Name
func (r *Repository) List(ctx context.Context) ([]Template, error) {
	var results []Template

	cursor, err := r.client.Find(ctx, bson.M{})
	if err != nil {
		return []Template{}, fmt.Errorf("error while listing: %w", err)
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &results); err != nil {
		log.Fatal("error decoding results: %w", err)
	}
	if len(results) == 0 {
		return []Template{}, nil
	}
	return results, nil
}

func (r *Repository) Create(ctx context.Context, template Template) error {
	result, err := r.client.InsertOne(ctx, template)
	if err != nil {
		return fmt.Errorf("error creating template: %w", err)
	}
	fmt.Println("template created with ID:", result.InsertedID)
	return nil
}
