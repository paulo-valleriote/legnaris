package template

import (
	"context"
	"errors"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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

func (r *Repository) FetchByID(ctx context.Context, ID string) (*Template, error) {
	var result Template

	filter := bson.D{{Key: "_id", Value: ID}}
	opts := options.FindOne()
	err := r.client.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("template not found")
		} else {
			return nil, err
		}
	}

	return &result, nil
}

func (r *Repository) Create(ctx context.Context, t Template) (string, error) {
	result, err := r.client.InsertOne(ctx, t)
	if err != nil {
		return "", fmt.Errorf("error creating template: %w", err)
	}
	return result.InsertedID.(string), nil
}

func (r *Repository) CreateMany(ctx context.Context, templates []Template) ([]string, error) {
	if len(templates) == 0 {
		return []string{}, nil
	}

	models := make([]mongo.WriteModel, 0, len(templates))
	for i := range templates {
		template := templates[i]

		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": template.ID}).
			SetUpdate(bson.M{
				"$setOnInsert": template,
			}).
			SetUpsert(true),
		)
	}

	result, err := r.client.BulkWrite(ctx, models)
	if err != nil {
		return []string{}, fmt.Errorf("error creating template: %w", err)
	}

	ids := make([]string, len(result.UpsertedIDs))
	for _, id := range result.UpsertedIDs {
		ids = append(ids, id.(string))
	}
	return ids, nil
}
