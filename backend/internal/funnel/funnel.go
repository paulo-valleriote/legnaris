package funnel

import (
	"context"
	"fmt"
	"time"

	"legnaris/backend/internal/user"
	"legnaris/backend/shared/logger"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	RefreshInterval   = 15 * time.Minute
	firstSheetWindow  = 24 * time.Hour
	usersCollection   = "users"
	sheetsCollection  = "sheets"
	firstSheetKey     = "first_sheet_at"
	firstSheetPath    = "$first_sheet_at"
	firstSheetLookup  = "first_sheet"
	statusField       = "status"
	createdAtField    = "created_at"
	ownerIDField      = "owner_id"
	firstSheetElement = "$first_sheet.created_at"
)

type Stats struct {
	Accounts            int
	FirstSheetWithin24h int
}

type countRow struct {
	Count int64 `bson:"n"`
}

type Store interface {
	Stats(ctx context.Context) (Stats, error)
}

type Observer interface {
	FunnelStats(stats Stats)
}

type MongoStore struct {
	users *mongo.Collection
}

func NewMongoStore(database *mongo.Database) *MongoStore {
	return &MongoStore{users: database.Collection(usersCollection)}
}

func (s *MongoStore) Stats(ctx context.Context) (Stats, error) {
	cursor, err := s.users.Aggregate(ctx, milestonePipeline())
	if err != nil {
		return Stats{}, fmt.Errorf("aggregate funnel milestones: %w", err)
	}

	var rows []struct {
		Accounts []countRow `bson:"accounts"`
		Within   []countRow `bson:"within"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return Stats{}, fmt.Errorf("decode funnel milestones: %w", err)
	}
	if len(rows) == 0 {
		return Stats{}, nil
	}
	return Stats{
		Accounts:            int(firstCount(rows[0].Accounts)),
		FirstSheetWithin24h: int(firstCount(rows[0].Within)),
	}, nil
}

func firstCount(rows []countRow) int64 {
	if len(rows) == 0 {
		return 0
	}
	return rows[0].Count
}

func milestonePipeline() mongo.Pipeline {
	windowMillis := firstSheetWindow.Milliseconds()

	return mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: statusField, Value: bson.D{{Key: "$nin", Value: bson.A{
				user.StatusDeleting, user.StatusDeletionDead,
			}}}},
		}}},
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: sheetsCollection},
			{Key: "localField", Value: "_id"},
			{Key: "foreignField", Value: ownerIDField},
			{Key: "pipeline", Value: mongo.Pipeline{
				{{Key: "$sort", Value: bson.D{{Key: createdAtField, Value: 1}}}},
				{{Key: "$limit", Value: 1}},
				{{Key: "$project", Value: bson.D{
					{Key: createdAtField, Value: 1},
					{Key: "_id", Value: 0},
				}}},
			}},
			{Key: "as", Value: firstSheetLookup},
		}}},
		{{Key: "$project", Value: bson.D{
			{Key: createdAtField, Value: 1},
			{Key: firstSheetKey, Value: bson.D{{Key: "$arrayElemAt", Value: bson.A{firstSheetElement, 0}}}},
		}}},
		{{Key: "$addFields", Value: bson.D{
			{Key: "first_sheet_delay", Value: bson.D{{Key: "$subtract", Value: bson.A{firstSheetPath, "$created_at"}}}},
		}}},
		{{Key: "$facet", Value: bson.D{
			{Key: "accounts", Value: mongo.Pipeline{
				{{Key: "$count", Value: "n"}},
			}},
			{Key: "within", Value: mongo.Pipeline{
				{{Key: "$match", Value: bson.D{{Key: "$expr", Value: bson.D{{Key: "$and", Value: bson.A{
					bson.D{{Key: "$gte", Value: bson.A{"$first_sheet_delay", 0}}},
					bson.D{{Key: "$lte", Value: bson.A{"$first_sheet_delay", windowMillis}}},
				}}}}}}},
				{{Key: "$count", Value: "n"}},
			}},
		}}},
	}
}

type Refresher struct {
	store    Store
	observer Observer
	log      logger.StructuredLogger
	interval time.Duration
}

func NewRefresher(store Store, observer Observer, log logger.StructuredLogger, interval time.Duration) *Refresher {
	return &Refresher{store: store, observer: observer, log: log, interval: interval}
}

func (r *Refresher) RunOnce(ctx context.Context) error {
	stats, err := r.store.Stats(ctx)
	if err != nil {
		return fmt.Errorf("load funnel stats: %w", err)
	}
	r.observer.FunnelStats(stats)
	return nil
}

func (r *Refresher) Run(ctx context.Context) error {
	for {
		if err := r.RunOnce(ctx); err != nil {
			r.log.Error(ctx, "funnel stats refresh failed", logger.Attribute{Key: "error", Value: err.Error()})
		}
		if ctx.Err() != nil {
			return nil
		}

		timer := time.NewTimer(r.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
