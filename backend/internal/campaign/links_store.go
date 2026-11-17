package campaign

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

const (
	campaignSheetLinksCollection = "campaign_sheet_links"
	linkCampaignIDField          = "campaign_id"
	linkSheetIDField             = "sheet_id"
	linkOwnerIDField             = "owner_id"
	linkRevealedField            = "revealed"
	linkLinkedAtField            = "linked_at"
	inOperator                   = "$in"
	setOnInsertOperator          = "$setOnInsert"
)

var ErrLinkNotFound = errors.New("sheet link not found")

type SheetLink struct {
	ID         string    `bson:"_id"`
	CampaignID string    `bson:"campaign_id"`
	SheetID    string    `bson:"sheet_id"`
	OwnerID    string    `bson:"owner_id"`
	Revealed   bool      `bson:"revealed"`
	LinkedAt   time.Time `bson:"linked_at"`
}

type LinkStore interface {
	Upsert(ctx context.Context, link *SheetLink) (SheetLink, bool, error)
	Fetch(ctx context.Context, campaignID, sheetID string) (*SheetLink, error)
	Delete(ctx context.Context, campaignID, sheetID string) (bool, error)
	ListByCampaignID(ctx context.Context, campaignID string) ([]SheetLink, error)
	DeleteBySheetID(ctx context.Context, sheetID string) ([]SheetLink, error)
	DeleteByOwner(ctx context.Context, campaignID, ownerID string) ([]SheetLink, error)
	DeleteByCampaignIDBatch(ctx context.Context, campaignID string, limit int) (int64, error)
}

type MongoLinkStore struct {
	collection *mongo.Collection
}

func NewLinkStore(db *mongo.Database) *MongoLinkStore {
	return &MongoLinkStore{
		collection: db.Collection(campaignSheetLinksCollection, options.Collection().SetReadPreference(readpref.Primary())),
	}
}

func (s *MongoLinkStore) Upsert(ctx context.Context, link *SheetLink) (SheetLink, bool, error) {
	filter := bson.D{
		{Key: linkCampaignIDField, Value: link.CampaignID},
		{Key: linkSheetIDField, Value: link.SheetID},
	}
	update := bson.D{{Key: setOnInsertOperator, Value: bson.D{
		{Key: idField, Value: link.ID},
		{Key: linkCampaignIDField, Value: link.CampaignID},
		{Key: linkSheetIDField, Value: link.SheetID},
		{Key: linkOwnerIDField, Value: link.OwnerID},
		{Key: linkRevealedField, Value: link.Revealed},
		{Key: linkLinkedAtField, Value: link.LinkedAt},
	}}}
	var stored SheetLink
	err := s.collection.FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&stored)
	if err == nil {
		return stored, stored.ID == link.ID, nil
	}
	if !mongo.IsDuplicateKeyError(err) {
		return SheetLink{}, false, fmt.Errorf("upsert sheet link: %w", err)
	}
	existing, fetchErr := s.Fetch(ctx, link.CampaignID, link.SheetID)
	if fetchErr != nil {
		return SheetLink{}, false, fmt.Errorf("read concurrent sheet link: %w", fetchErr)
	}
	return *existing, false, nil
}

func (s *MongoLinkStore) Fetch(ctx context.Context, campaignID, sheetID string) (*SheetLink, error) {
	var stored SheetLink
	err := s.collection.FindOne(ctx, linkFilter(campaignID, sheetID)).Decode(&stored)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrLinkNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fetch sheet link: %w", err)
	}
	return &stored, nil
}

func (s *MongoLinkStore) Delete(ctx context.Context, campaignID, sheetID string) (bool, error) {
	result, err := s.collection.DeleteOne(ctx, linkFilter(campaignID, sheetID))
	if err != nil {
		return false, fmt.Errorf("delete sheet link: %w", err)
	}
	return result.DeletedCount > 0, nil
}

func (s *MongoLinkStore) ListByCampaignID(ctx context.Context, campaignID string) ([]SheetLink, error) {
	filter := bson.D{{Key: linkCampaignIDField, Value: campaignID}}
	opts := options.Find().SetSort(bson.D{{Key: linkLinkedAtField, Value: 1}, {Key: idField, Value: 1}})
	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("list sheet links: %w", err)
	}
	var found []SheetLink
	if err := cursor.All(ctx, &found); err != nil {
		return nil, fmt.Errorf("decode sheet links: %w", err)
	}
	return found, nil
}

func (s *MongoLinkStore) DeleteBySheetID(ctx context.Context, sheetID string) ([]SheetLink, error) {
	filter := bson.D{{Key: linkSheetIDField, Value: sheetID}}
	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list sheet links by sheet: %w", err)
	}
	var found []SheetLink
	if err := cursor.All(ctx, &found); err != nil {
		return nil, fmt.Errorf("decode sheet links by sheet: %w", err)
	}
	if len(found) == 0 {
		return nil, nil
	}
	ids := make(bson.A, 0, len(found))
	for _, link := range found {
		ids = append(ids, link.ID)
	}
	if _, err := s.collection.DeleteMany(ctx, bson.D{{Key: idField, Value: bson.D{{Key: inOperator, Value: ids}}}}); err != nil {
		return nil, fmt.Errorf("delete sheet links by sheet: %w", err)
	}
	return found, nil
}

func linkFilter(campaignID, sheetID string) bson.D {
	return bson.D{
		{Key: linkCampaignIDField, Value: campaignID},
		{Key: linkSheetIDField, Value: sheetID},
	}
}

func (s *MongoLinkStore) DeleteByOwner(ctx context.Context, campaignID, ownerID string) ([]SheetLink, error) {
	filter := bson.D{
		{Key: linkCampaignIDField, Value: campaignID},
		{Key: linkOwnerIDField, Value: ownerID},
	}
	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list sheet links by owner: %w", err)
	}
	var found []SheetLink
	if err := cursor.All(ctx, &found); err != nil {
		return nil, fmt.Errorf("decode sheet links by owner: %w", err)
	}
	if len(found) == 0 {
		return nil, nil
	}
	ids := make(bson.A, 0, len(found))
	for _, link := range found {
		ids = append(ids, link.ID)
	}
	if _, err := s.collection.DeleteMany(ctx, bson.D{{Key: idField, Value: bson.D{{Key: inOperator, Value: ids}}}}); err != nil {
		return nil, fmt.Errorf("delete sheet links by owner: %w", err)
	}
	return found, nil
}

func (s *MongoLinkStore) DeleteByCampaignIDBatch(ctx context.Context, campaignID string, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	filter := bson.D{{Key: linkCampaignIDField, Value: campaignID}}
	opts := options.Find().SetProjection(bson.D{{Key: idField, Value: 1}}).SetLimit(int64(limit))
	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return 0, fmt.Errorf("list sheet links by campaign: %w", err)
	}
	var found []struct {
		ID string `bson:"_id"`
	}
	if err := cursor.All(ctx, &found); err != nil {
		return 0, fmt.Errorf("decode sheet links by campaign: %w", err)
	}
	if len(found) == 0 {
		return 0, nil
	}
	ids := make(bson.A, 0, len(found))
	for _, doc := range found {
		ids = append(ids, doc.ID)
	}
	result, err := s.collection.DeleteMany(ctx, bson.D{{Key: idField, Value: bson.D{{Key: inOperator, Value: ids}}}})
	if err != nil {
		return 0, fmt.Errorf("delete sheet links by campaign: %w", err)
	}
	return result.DeletedCount, nil
}
