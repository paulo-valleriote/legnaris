package campaign

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

const (
	campaignsCollection = "campaigns"
	idField             = "_id"
	versionField        = "version"
	statusField         = "status"
	membersUserIDField  = "members.user_id"
	setOperator         = "$set"
	incOperator         = "$inc"
)

var ErrNotFound = errors.New("campaign not found")

var ErrVersionConflict = errors.New("campaign version conflict")

type Repository interface {
	Insert(ctx context.Context, c *Campaign) error
	FetchByID(ctx context.Context, id string) (*Campaign, error)
	ListActiveByMember(ctx context.Context, userID string) ([]Campaign, error)
	Save(ctx context.Context, c *Campaign, expectedVersion int64) error
}

type MongoRepository struct {
	collection *mongo.Collection
}

func NewRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		collection: db.Collection(campaignsCollection, options.Collection().SetReadPreference(readpref.Primary())),
	}
}

func (r *MongoRepository) Insert(ctx context.Context, c *Campaign) error {
	if _, err := r.collection.InsertOne(ctx, c); err != nil {
		return fmt.Errorf("insert campaign: %w", err)
	}
	return nil
}

func (r *MongoRepository) FetchByID(ctx context.Context, id string) (*Campaign, error) {
	var stored Campaign
	err := r.collection.FindOne(ctx, bson.D{{Key: idField, Value: id}}).Decode(&stored)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fetch campaign: %w", err)
	}
	return &stored, nil
}

func (r *MongoRepository) ListActiveByMember(ctx context.Context, userID string) ([]Campaign, error) {
	filter := bson.D{
		{Key: membersUserIDField, Value: userID},
		{Key: statusField, Value: StatusActive},
	}
	cursor, err := r.collection.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	var found []Campaign
	if err := cursor.All(ctx, &found); err != nil {
		return nil, fmt.Errorf("decode campaigns: %w", err)
	}
	return found, nil
}

func (r *MongoRepository) Save(ctx context.Context, c *Campaign, expectedVersion int64) error {
	filter := bson.D{
		{Key: idField, Value: c.ID},
		{Key: versionField, Value: expectedVersion},
		{Key: statusField, Value: StatusActive},
	}
	update := bson.D{
		{Key: setOperator, Value: bson.D{
			{Key: "members", Value: c.Members},
			{Key: "gm_count", Value: c.GMCount},
			{Key: "player_count", Value: c.PlayerCount},
			{Key: "updated_at", Value: c.UpdatedAt},
		}},
		{Key: incOperator, Value: bson.D{{Key: versionField, Value: 1}}},
	}
	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("save campaign: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrVersionConflict
	}
	return nil
}
