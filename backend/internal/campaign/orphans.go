package campaign

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"legnaris/backend/internal/sweeper"
)

const (
	sheetsCollection     = "sheets"
	sheetCampaignIDField = "campaign_id"
	OrphanQueueName      = "campaigns_orphan_links"
	OrphanInterval       = 24 * time.Hour
	orphanJobID          = "orphan-links"
	lookupStage          = "$lookup"
	matchStage           = "$match"
	sizeOperator         = "$size"
	lookupFromKey        = "from"
	lookupLocalKey       = "localField"
	lookupForeignKey     = "foreignField"
	lookupAsKey          = "as"
	campaignLookupField  = "campaign"
	sheetLookupField     = "sheet"
)

type OrphanLinks interface {
	UnlinkOrphans(ctx context.Context, limit int) (int64, error)
}

type MongoOrphanLinks struct {
	sheets *mongo.Collection
	links  *mongo.Collection
}

func NewOrphanLinks(db *mongo.Database) *MongoOrphanLinks {
	return &MongoOrphanLinks{
		sheets: db.Collection(sheetsCollection),
		links:  db.Collection(campaignSheetLinksCollection),
	}
}

// UnlinkOrphans clears up to limit orphaned character links and up to limit orphaned NPC and
// Monster links. It returns how many orphans it examined, so a zero means nothing is left.
func (l *MongoOrphanLinks) UnlinkOrphans(ctx context.Context, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	cleared, err := l.unlinkOrphanSheets(ctx, limit)
	if err != nil {
		return 0, err
	}
	deleted, err := l.deleteOrphanLinks(ctx, limit)
	if err != nil {
		return 0, err
	}
	return cleared + deleted, nil
}

func (l *MongoOrphanLinks) unlinkOrphanSheets(ctx context.Context, limit int) (int64, error) {
	pipeline := bson.A{
		bson.D{{Key: matchStage, Value: bson.D{{Key: sheetCampaignIDField, Value: bson.D{{Key: "$ne", Value: nil}}}}}},
		bson.D{{Key: lookupStage, Value: bson.D{
			{Key: lookupFromKey, Value: campaignsCollection},
			{Key: lookupLocalKey, Value: sheetCampaignIDField},
			{Key: lookupForeignKey, Value: idField},
			{Key: lookupAsKey, Value: campaignLookupField},
		}}},
		bson.D{{Key: matchStage, Value: bson.D{{Key: campaignLookupField, Value: bson.D{{Key: sizeOperator, Value: 0}}}}}},
		bson.D{{Key: "$limit", Value: int64(limit)}},
		bson.D{{Key: "$project", Value: bson.D{{Key: idField, Value: 1}, {Key: sheetCampaignIDField, Value: 1}}}},
	}
	cursor, err := l.sheets.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, fmt.Errorf("find orphan sheet links: %w", err)
	}
	var orphans []struct {
		ID         string `bson:"_id"`
		CampaignID string `bson:"campaign_id"`
	}
	if err := cursor.All(ctx, &orphans); err != nil {
		return 0, fmt.Errorf("decode orphan sheet links: %w", err)
	}
	for _, o := range orphans {
		// Conditional on the orphaned campaign_id: a sheet relinked meanwhile is left alone.
		if _, err := l.sheets.UpdateOne(ctx,
			bson.D{{Key: idField, Value: o.ID}, {Key: sheetCampaignIDField, Value: o.CampaignID}},
			bson.D{{Key: setOperator, Value: bson.D{{Key: sheetCampaignIDField, Value: nil}}}},
		); err != nil {
			return 0, fmt.Errorf("clear orphan sheet link: %w", err)
		}
	}
	return int64(len(orphans)), nil
}

func (l *MongoOrphanLinks) deleteOrphanLinks(ctx context.Context, limit int) (int64, error) {
	pipeline := bson.A{
		bson.D{{Key: lookupStage, Value: bson.D{
			{Key: lookupFromKey, Value: campaignsCollection},
			{Key: lookupLocalKey, Value: linkCampaignIDField},
			{Key: lookupForeignKey, Value: idField},
			{Key: lookupAsKey, Value: campaignLookupField},
		}}},
		bson.D{{Key: lookupStage, Value: bson.D{
			{Key: lookupFromKey, Value: sheetsCollection},
			{Key: lookupLocalKey, Value: linkSheetIDField},
			{Key: lookupForeignKey, Value: idField},
			{Key: lookupAsKey, Value: sheetLookupField},
		}}},
		bson.D{{Key: matchStage, Value: bson.D{{Key: orOperator, Value: bson.A{
			bson.D{{Key: campaignLookupField, Value: bson.D{{Key: sizeOperator, Value: 0}}}},
			bson.D{{Key: sheetLookupField, Value: bson.D{{Key: sizeOperator, Value: 0}}}},
		}}}}},
		bson.D{{Key: "$limit", Value: int64(limit)}},
		bson.D{{Key: "$project", Value: bson.D{{Key: idField, Value: 1}}}},
	}
	cursor, err := l.links.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, fmt.Errorf("find orphan npc links: %w", err)
	}
	var orphans []struct {
		ID string `bson:"_id"`
	}
	if err := cursor.All(ctx, &orphans); err != nil {
		return 0, fmt.Errorf("decode orphan npc links: %w", err)
	}
	if len(orphans) == 0 {
		return 0, nil
	}
	ids := make(bson.A, 0, len(orphans))
	for _, o := range orphans {
		ids = append(ids, o.ID)
	}
	result, err := l.links.DeleteMany(ctx, bson.D{{Key: idField, Value: bson.D{{Key: inOperator, Value: ids}}}})
	if err != nil {
		return 0, fmt.Errorf("delete orphan npc links: %w", err)
	}
	return result.DeletedCount, nil
}

// orphanQueueStore has no backlog state: the sweeper's daily interval is the only schedule, and
// each pass re-derives the orphans from the sheets collection.
type orphanQueueStore struct{}

func (orphanQueueStore) Claim(context.Context, string, time.Time, time.Duration, int) ([]sweeper.Job[struct{}], error) {
	return []sweeper.Job[struct{}]{{ID: orphanJobID}}, nil
}

func (orphanQueueStore) Reschedule(context.Context, string, time.Time, time.Time, string) error {
	return nil
}

func (orphanQueueStore) MarkDead(context.Context, string, time.Time, string) error {
	return nil
}

func (orphanQueueStore) OldestPending(context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func NewOrphanLinksQueue(links OrphanLinks, batch int) (sweeper.Queue, error) {
	if links == nil {
		return sweeper.Queue{}, fmt.Errorf("orphan link reconciliation requires sheet links")
	}
	return sweeper.NewQueue(sweeper.QueueSpec[struct{}]{
		Name:       OrphanQueueName,
		Interval:   OrphanInterval,
		AttemptCap: 1,
		Batch:      batch,
		Store:      orphanQueueStore{},
		Handle: func(ctx context.Context, _ sweeper.Job[struct{}], _ time.Time) error {
			return ReconcileOrphans(ctx, links, batch)
		},
	})
}

func ReconcileOrphans(ctx context.Context, links OrphanLinks, batch int) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := links.UnlinkOrphans(ctx, batch)
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
	}
}
