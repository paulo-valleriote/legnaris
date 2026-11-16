package llmconsent_test

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/internal/llmconsent"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func seedUser(t *testing.T, users *mongo.Collection, id string, extra bson.M) {
	t.Helper()
	doc := bson.M{"_id": id, "email": id + "@example.test"}
	for k, v := range extra {
		doc[k] = v
	}
	_, err := users.InsertOne(context.Background(), doc)
	require.NoError(t, err)
}

func TestConsentStore_Grant(t *testing.T) {
	first := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	later := first.Add(48 * time.Hour)

	tests := []struct {
		name       string
		userID     string
		seed       bool
		seedExtra  bson.M
		grantAt    []time.Time
		version    string
		wantErr    error
		wantAt     time.Time
		wantVerify string
	}{
		{name: "Edge: missing user is ErrUserNotFound", userID: "user-1", seed: false, grantAt: []time.Time{first}, version: llmconsent.Version, wantErr: llmconsent.ErrUserNotFound},
		{name: "Edge: empty user id is ErrUserNotFound", userID: "", seed: true, grantAt: []time.Time{first}, version: llmconsent.Version, wantErr: llmconsent.ErrUserNotFound},
		{name: "Happy path: grant stores timestamp and version", userID: "user-1", seed: true, grantAt: []time.Time{first}, version: llmconsent.Version, wantAt: first, wantVerify: llmconsent.Version},
		{name: "Edge: repeated grant keeps the original timestamp", userID: "user-1", seed: true, grantAt: []time.Time{first, later}, version: llmconsent.Version, wantAt: first, wantVerify: llmconsent.Version},
		{name: "Edge: grant over an outdated version replaces it", userID: "user-1", seed: true, seedExtra: bson.M{"llm_consent_at": first, "llm_consent_version": "0"}, grantAt: []time.Time{later}, version: llmconsent.Version, wantAt: later, wantVerify: llmconsent.Version},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb := dbtest.Database(t)
			users := mdb.Collection("users")
			if tc.seed {
				seedUser(t, users, "user-1", tc.seedExtra)
			}
			store := llmconsent.NewConsentStore(mdb)
			userID := tc.userID

			var lastErr error
			for _, at := range tc.grantAt {
				lastErr = store.Grant(context.Background(), userID, tc.version, at)
			}
			if tc.wantErr != nil {
				require.ErrorIs(t, lastErr, tc.wantErr)
				return
			}
			require.NoError(t, lastErr)

			record, err := store.Fetch(context.Background(), userID)
			require.NoError(t, err)
			require.NotNil(t, record.At)
			assert.True(t, tc.wantAt.Equal(*record.At), "stored timestamp %v, want %v", *record.At, tc.wantAt)
			assert.Equal(t, tc.wantVerify, record.Version)
		})
	}
}

func TestConsentStore_Fetch(t *testing.T) {
	at := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		userID   string
		seed     bson.M
		seeded   bool
		wantErr  error
		wantRec  llmconsent.Record
		wantNone bool
	}{
		{name: "Edge: absent user is ErrUserNotFound", userID: "ghost", seeded: false, wantErr: llmconsent.ErrUserNotFound},
		{name: "Edge: user without consent reads as empty record", userID: "user-1", seeded: true, wantNone: true},
		{name: "Happy path: consent is returned", userID: "user-1", seeded: true, seed: bson.M{"llm_consent_at": at, "llm_consent_version": llmconsent.Version}, wantRec: llmconsent.Record{At: &at, Version: llmconsent.Version}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb := dbtest.Database(t)
			if tc.seeded {
				seedUser(t, mdb.Collection("users"), tc.userID, tc.seed)
			}
			store := llmconsent.NewConsentStore(mdb)

			record, err := store.Fetch(context.Background(), tc.userID)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			if tc.wantNone {
				assert.Nil(t, record.At)
				assert.Empty(t, record.Version)
				return
			}
			require.NotNil(t, record.At)
			assert.True(t, tc.wantRec.At.Equal(*record.At))
			assert.Equal(t, tc.wantRec.Version, record.Version)
		})
	}
}

func TestConsentStore_Clear(t *testing.T) {
	at := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		userID string
		seeded bool
		seed   bson.M
	}{
		{name: "Edge: clear on an absent user is a no-op", userID: "ghost", seeded: false},
		{name: "Edge: clear on a user without consent is a no-op", userID: "user-1", seeded: true},
		{name: "Happy path: clear unsets both consent fields", userID: "user-1", seeded: true, seed: bson.M{"llm_consent_at": at, "llm_consent_version": llmconsent.Version}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb := dbtest.Database(t)
			if tc.seeded {
				seedUser(t, mdb.Collection("users"), tc.userID, tc.seed)
			}
			store := llmconsent.NewConsentStore(mdb)

			require.NoError(t, store.Clear(context.Background(), tc.userID))
			require.NoError(t, store.Clear(context.Background(), tc.userID), "a second clear must also be a no-op")

			if !tc.seeded {
				return
			}
			record, err := store.Fetch(context.Background(), tc.userID)
			require.NoError(t, err)
			assert.Nil(t, record.At)
			assert.Empty(t, record.Version)
		})
	}
}

func TestGenerationStore_DeleteByUserID(t *testing.T) {
	tests := []struct {
		name        string
		seedDocs    []bson.M
		userID      string
		wantDeleted int64
		wantLeft    int64
	}{
		{name: "Edge: absent generations collection deletes nothing and reports no error", seedDocs: nil, userID: "user-1", wantDeleted: 0, wantLeft: 0},
		{name: "Edge: empty user id matches no generation", seedDocs: []bson.M{{"user_id": "user-1"}}, userID: "", wantDeleted: 0, wantLeft: 1},
		{name: "Edge: other users' generations are kept", seedDocs: []bson.M{{"user_id": "user-2"}}, userID: "user-1", wantDeleted: 0, wantLeft: 1},
		{name: "Happy path: every generation of the user is removed", seedDocs: []bson.M{{"user_id": "user-1"}, {"user_id": "user-1"}, {"user_id": "user-2"}}, userID: "user-1", wantDeleted: 2, wantLeft: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb := dbtest.Database(t)
			generations := mdb.Collection("generations")
			for _, doc := range tc.seedDocs {
				_, err := generations.InsertOne(context.Background(), doc)
				require.NoError(t, err)
			}
			store := llmconsent.NewGenerationStore(mdb)

			deleted, err := store.DeleteByUserID(context.Background(), tc.userID)
			require.NoError(t, err)
			assert.Equal(t, tc.wantDeleted, deleted)

			left, err := generations.CountDocuments(context.Background(), bson.M{})
			require.NoError(t, err)
			assert.Equal(t, tc.wantLeft, left)
		})
	}
}

func TestGenerationStore_ListOpenByUserID(t *testing.T) {
	tests := []struct {
		name     string
		seedDocs []bson.M
		userID   string
		want     []llmconsent.OpenGeneration
	}{
		{name: "Edge: absent collection lists nothing", seedDocs: nil, userID: "user-1", want: []llmconsent.OpenGeneration{}},
		{name: "Edge: empty user id matches no generation", seedDocs: []bson.M{{"_id": "gen-1", "user_id": "user-1", "status": "queued"}}, userID: "", want: []llmconsent.OpenGeneration{}},
		{
			name: "Edge: terminal generations are not open",
			seedDocs: []bson.M{
				{"_id": "gen-1", "user_id": "user-1", "status": "succeeded", "quota": bson.M{"kind": "trial"}},
				{"_id": "gen-2", "user_id": "user-1", "status": "accepted", "quota": bson.M{"kind": "trial"}},
				{"_id": "gen-3", "user_id": "user-1", "status": "failed", "quota": bson.M{"kind": "daily", "day_utc": "2026-11-13"}},
			},
			userID: "user-1",
			want:   []llmconsent.OpenGeneration{},
		},
		{
			name: "Edge: other users' open generations are kept out",
			seedDocs: []bson.M{
				{"_id": "gen-1", "user_id": "user-2", "status": "running", "quota": bson.M{"kind": "trial"}},
			},
			userID: "user-1",
			want:   []llmconsent.OpenGeneration{},
		},
		{
			name: "Happy path: queued and running generations carry their reservation",
			seedDocs: []bson.M{
				{"_id": "gen-1", "user_id": "user-1", "status": "queued", "quota": bson.M{"kind": "trial"}},
				{"_id": "gen-2", "user_id": "user-1", "status": "running", "quota": bson.M{"kind": "daily", "day_utc": "2026-11-14"}},
				{"_id": "gen-3", "user_id": "user-1", "status": "succeeded", "quota": bson.M{"kind": "trial"}},
			},
			userID: "user-1",
			want: []llmconsent.OpenGeneration{
				{ID: "gen-1", UserID: "user-1", ReservationKind: "trial"},
				{ID: "gen-2", UserID: "user-1", ReservationKind: "daily", DayUTC: "2026-11-14"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb := dbtest.Database(t)
			generations := mdb.Collection("generations")
			for _, doc := range tc.seedDocs {
				_, err := generations.InsertOne(context.Background(), doc)
				require.NoError(t, err)
			}
			store := llmconsent.NewGenerationStore(mdb)

			open, err := store.ListOpenByUserID(context.Background(), tc.userID)
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.want, open)
		})
	}
}

type recordingRefunder struct {
	refunded []string
}

func (r *recordingRefunder) RefundOpen(_ context.Context, open llmconsent.OpenGeneration) error {
	r.refunded = append(r.refunded, open.ID)
	return nil
}

func TestService_RevokeAgainstReplicaSet(t *testing.T) {
	tests := []struct {
		name         string
		granted      bool
		generations  []bson.M
		wantLeft     int64
		wantRefunded int
	}{
		{name: "Edge: revoke without consent or generations is a no-op", granted: false, wantLeft: 0, wantRefunded: 0},
		{
			name: "Edge: terminal generations are deleted without a refund", granted: true,
			generations: []bson.M{{"_id": "gen-1", "user_id": "user-1", "status": "accepted"}}, wantLeft: 0, wantRefunded: 0,
		},
		{
			name: "Happy path: revoke refunds open generations, clears consent and removes generations", granted: true,
			generations: []bson.M{
				{"_id": "gen-1", "user_id": "user-1", "status": "queued", "quota": bson.M{"kind": "trial"}},
				{"_id": "gen-2", "user_id": "user-1", "status": "running", "quota": bson.M{"kind": "daily", "day_utc": "2026-11-14"}},
			},
			wantLeft: 0, wantRefunded: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb := dbtest.Database(t)
			ctx := context.Background()
			seedUser(t, mdb.Collection("users"), "user-1", nil)
			if tc.granted {
				require.NoError(t, llmconsent.NewConsentStore(mdb).Grant(ctx, "user-1", llmconsent.Version, time.Now()))
			}
			generations := mdb.Collection("generations")
			for _, doc := range tc.generations {
				_, err := generations.InsertOne(ctx, doc)
				require.NoError(t, err)
			}

			refunder := &recordingRefunder{}
			svc := llmconsent.NewService(
				llmconsent.NewConsentStore(mdb),
				llmconsent.NewGenerationStore(mdb),
				refunder,
				db.NewTransactor(mdb.Client(), nil),
				time.Now,
			)
			require.NoError(t, svc.Revoke(ctx, "user-1"))
			require.NoError(t, svc.Revoke(ctx, "user-1"), "a second revoke must be a no-op")

			left, err := generations.CountDocuments(ctx, bson.M{})
			require.NoError(t, err)
			assert.Equal(t, tc.wantLeft, left)
			assert.Len(t, refunder.refunded, tc.wantRefunded, "each open generation is refunded once")

			err = svc.Gate(ctx, "user-1")
			require.Error(t, err, "revoked consent must close the gate")
		})
	}
}
