package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/response"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const fantasyCampaignFixtureName = "The Ember Crown"

func insertFantasyCampaign(t *testing.T, mdb *mongo.Database, ownerID string, playerIDs ...string) string {
	t.Helper()
	campaignID, err := idgen.New()
	require.NoError(t, err)
	now := time.Now().UTC()
	members := make([]bson.M, 0, 1+len(playerIDs))
	members = append(members, bson.M{
		"user_id": ownerID, "role": string(campaign.RoleGM), "owner": true, "joined_at": now,
	})
	for _, playerID := range playerIDs {
		members = append(members, bson.M{
			"user_id": playerID, "role": string(campaign.RolePlayer), "joined_at": now,
		})
	}
	_, err = mdb.Collection("campaigns").InsertOne(context.Background(), bson.M{
		"_id":          campaignID,
		"name":         fantasyCampaignFixtureName,
		"template_id":  "fantasy_v1",
		"status":       string(campaign.StatusActive),
		"invite_code":  "FANTASY1",
		"version":      int64(0),
		"gm_count":     1,
		"player_count": len(playerIDs),
		"members":      members,
		"created_at":   now,
		"updated_at":   now,
	})
	require.NoError(t, err)
	return campaignID
}

func createFantasySheet(t *testing.T, h http.Handler, mdb *mongo.Database, token, ownerID, kind, name string) sheet.SheetDTO {
	t.Helper()
	campaignID := insertFantasyCampaign(t, mdb, ownerID)
	body := fmt.Sprintf(`{"template_id":"fantasy_v1","kind":%q,"sheet_name":%q,"campaign_id":%q}`,
		kind, name, campaignID)
	rec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var created response.Success[sheet.SheetDTO]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Equal(t, "fantasy_v1", created.Data.TemplateID)
	return created.Data
}
