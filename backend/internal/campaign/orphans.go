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
)

type OrphanLinks interface {
	UnlinkOrphans(ctx context.Context, limit int) (int64, error)
}

type MongoOrphanLinks struct {
	sheets *mongo.Collection
}

func NewOrphanLinks(db *mongo.Database) *MongoOrphanLinks {
	return &MongoOrphanLinks{sheets: db.Collection(sheetsCollection)}
}

// UnlinkOrphans clears up to limit sheet links whose Campaign no longer exists. It returns how
// many orphaned links it examined, so a zero means nothing is left.
func (l *MongoOrphanLinks) UnlinkOrphans(ctx context.Context, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	pipeline := bson.A{
		bson.D{{Key: "$match", Value: bson.D{{Key: sheetCampaignIDField, Value: bson.D{{Key: "$ne", Value: nil}}}}}},
		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: campaignsCollection},
			{Key: "localField", Value: sheetCampaignIDField},
			{Key: "foreignField", Value: idField},
			{Key: "as", Value: "campaign"},
		}}},
		bson.D{{Key: "$match", Value: bson.D{{Key: "campaign", Value: bson.D{{Key: "$size", Value: 0}}}}}},
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
