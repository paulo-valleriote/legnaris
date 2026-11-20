package funnel_test

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/internal/funnel"
	"legnaris/backend/shared/db/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type accountFixture struct {
	ID        string
	CreatedAt time.Time
}

type sheetFixture struct {
	ID        string
	OwnerID   string
	CreatedAt time.Time
}

var accountEpoch = time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)

const withinDay = 24 * time.Hour

func TestMongoStoreStats(t *testing.T) {
	client := dbtest.Client(t)

	tests := []struct {
		name     string
		accounts []accountFixture
		sheets   []sheetFixture
		want     funnel.Stats
	}{
		{
			name:     "Happy path: an account whose first sheet was saved within the day is counted",
			accounts: []accountFixture{{ID: "a1", CreatedAt: accountEpoch}},
			sheets:   []sheetFixture{{ID: "s1", OwnerID: "a1", CreatedAt: accountEpoch.Add(2 * time.Hour)}},
			want:     funnel.Stats{Accounts: 1, FirstSheetWithin24h: 1},
		},
		{
			name:     "Edge: a sheet saved exactly 24 hours later still counts",
			accounts: []accountFixture{{ID: "a1", CreatedAt: accountEpoch}},
			sheets:   []sheetFixture{{ID: "s1", OwnerID: "a1", CreatedAt: accountEpoch.Add(withinDay)}},
			want:     funnel.Stats{Accounts: 1, FirstSheetWithin24h: 1},
		},
		{
			name:     "Edge: a sheet saved one second after the day does not count",
			accounts: []accountFixture{{ID: "a1", CreatedAt: accountEpoch}},
			sheets:   []sheetFixture{{ID: "s1", OwnerID: "a1", CreatedAt: accountEpoch.Add(withinDay + time.Second)}},
			want:     funnel.Stats{Accounts: 1},
		},
		{
			name:     "Edge: a sheet stamped before the account is not a valid milestone",
			accounts: []accountFixture{{ID: "a1", CreatedAt: accountEpoch}},
			sheets:   []sheetFixture{{ID: "s1", OwnerID: "a1", CreatedAt: accountEpoch.Add(-time.Hour)}},
			want:     funnel.Stats{Accounts: 1},
		},
		{
			name:     "Edge: an account without a sheet only raises the baseline",
			accounts: []accountFixture{{ID: "a1", CreatedAt: accountEpoch}},
			want:     funnel.Stats{Accounts: 1},
		},
		{
			name:     "Edge: the earliest of several sheets decides the milestone",
			accounts: []accountFixture{{ID: "a1", CreatedAt: accountEpoch}},
			sheets: []sheetFixture{
				{ID: "s1", OwnerID: "a1", CreatedAt: accountEpoch.Add(30 * time.Hour)},
				{ID: "s2", OwnerID: "a1", CreatedAt: accountEpoch.Add(time.Hour)},
			},
			want: funnel.Stats{Accounts: 1, FirstSheetWithin24h: 1},
		},
		{
			name: "Edge: a sheet belongs to its own account",
			accounts: []accountFixture{
				{ID: "a1", CreatedAt: accountEpoch},
				{ID: "a2", CreatedAt: accountEpoch},
			},
			sheets: []sheetFixture{
				{ID: "s1", OwnerID: "a1", CreatedAt: accountEpoch.Add(time.Hour)},
				{ID: "s2", OwnerID: "a2", CreatedAt: accountEpoch.Add(48 * time.Hour)},
			},
			want: funnel.Stats{Accounts: 2, FirstSheetWithin24h: 1},
		},
		{
			name:     "Edge: a sheet without a creation timestamp is ignored",
			accounts: []accountFixture{{ID: "a1", CreatedAt: accountEpoch}},
			sheets:   []sheetFixture{{ID: "s1", OwnerID: "a1"}},
			want:     funnel.Stats{Accounts: 1},
		},
		{
			name: "Edge: an empty database reports zeroes",
			want: funnel.Stats{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			database := dbtest.Adopt(t, client, dbtest.NewDatabaseName(t))
			seedFunnelFixtures(t, database, tc.accounts, tc.sheets)

			got, err := funnel.NewMongoStore(database).Stats(t.Context())

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func seedFunnelFixtures(t *testing.T, database *mongo.Database, accounts []accountFixture, sheets []sheetFixture) {
	t.Helper()

	if len(accounts) > 0 {
		docs := make([]any, 0, len(accounts))
		for _, account := range accounts {
			doc := bson.D{{Key: "_id", Value: account.ID}}
			if !account.CreatedAt.IsZero() {
				doc = append(doc, bson.E{Key: "created_at", Value: account.CreatedAt})
			}
			docs = append(docs, doc)
		}
		_, err := database.Collection("users").InsertMany(context.Background(), docs)
		require.NoError(t, err)
	}

	if len(sheets) > 0 {
		docs := make([]any, 0, len(sheets))
		for _, sheet := range sheets {
			doc := bson.D{
				{Key: "_id", Value: sheet.ID},
				{Key: "owner_id", Value: sheet.OwnerID},
			}
			if !sheet.CreatedAt.IsZero() {
				doc = append(doc, bson.E{Key: "created_at", Value: sheet.CreatedAt})
			}
			docs = append(docs, doc)
		}
		_, err := database.Collection("sheets").InsertMany(context.Background(), docs)
		require.NoError(t, err)
	}
}
