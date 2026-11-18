package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"legnaris/backend/internal/admin"
	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/quota"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const adminIntegrationID = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e0f"

type adminUserView struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type adminSubscriptionView struct {
	ID               string     `json:"id"`
	Provider         string     `json:"provider"`
	PlanID           string     `json:"plan_id"`
	Status           string     `json:"status"`
	CurrentPeriodEnd *time.Time `json:"current_period_end"`
	RefundState      string     `json:"refund_state"`
}

type adminQuotaView struct {
	Limit     int `json:"limit"`
	Used      int `json:"used"`
	Remaining int `json:"remaining"`
}

type adminTrialView struct {
	Allowance int `json:"allowance"`
	Used      int `json:"used"`
	Remaining int `json:"remaining"`
}

type adminView struct {
	User         adminUserView          `json:"user"`
	Plan         string                 `json:"plan"`
	Subscription *adminSubscriptionView `json:"subscription"`
	DailyQuota   adminQuotaView         `json:"daily_quota"`
	Trial        adminTrialView         `json:"trial"`
}

func newAdminRouter(t *testing.T) (http.Handler, *mongo.Database, string) {
	t.Helper()

	h, mdb, modules := newIntegrationStack(t, map[string]string{"ADMIN_USER_IDS": adminIntegrationID})
	now := time.Now().UTC()
	_, err := mdb.Collection("users").InsertOne(context.Background(), bson.M{
		"_id":        adminIntegrationID,
		"email":      "support-admin@example.com",
		"username":   "support-admin",
		"created_at": now,
		"updated_at": now,
	})
	require.NoError(t, err)

	token, err := modules.Auth.TokenManager.Issue(adminIntegrationID)
	require.NoError(t, err)
	return h, mdb, token.Value
}

func adminRequest(t *testing.T, h http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return doAuthRequest(t, h, token, method, path, body)
}

func adminViewOf(t *testing.T, rec *httptest.ResponseRecorder) adminView {
	t.Helper()
	var body response.Success[adminView]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Data
}

func errorBodyOf(t *testing.T, rec *httptest.ResponseRecorder) response.ErrorResponse {
	t.Helper()
	var body response.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func TestIntegration_AdminRoutesAreHidden(t *testing.T) {
	h, mdb, _ := newIntegrationStack(t, map[string]string{"ADMIN_USER_IDS": ""})
	token, userID := newAuthUser(t, h, mdb, "not-admin@example.com", "not-admin")

	requests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/admin/users/" + userID, ""},
		{http.MethodGet, "/admin/users?email=not-admin@example.com", ""},
		{http.MethodPost, "/admin/users/" + userID + "/comp", `{"planId":"paid","reason":"x"}`},
		{http.MethodDelete, "/admin/users/" + userID + "/comp", `{"reason":"x"}`},
		{http.MethodPost, "/admin/users/" + userID + "/quota/reset", `{"reason":"x"}`},
		{http.MethodPost, "/admin/users/" + userID + "/refund", `{"override":true,"reason":"x"}`},
	}

	for _, tc := range requests {
		t.Run("Edge: unauthenticated "+tc.method+" "+tc.path+" is 404", func(t *testing.T) {
			rec := adminRequest(t, h, "", tc.method, tc.path, tc.body)
			require.Equal(t, http.StatusNotFound, rec.Code)
			assert.Equal(t, "NOT_FOUND", errorBodyOf(t, rec).Error.Code)
		})

		t.Run("Edge: an authenticated non-admin "+tc.method+" "+tc.path+" is 404", func(t *testing.T) {
			rec := adminRequest(t, h, token, tc.method, tc.path, tc.body)
			require.Equal(t, http.StatusNotFound, rec.Code)
			assert.Equal(t, "NOT_FOUND", errorBodyOf(t, rec).Error.Code)
		})
	}
}

func TestIntegration_AdminFlow(t *testing.T) {
	h, mdb, adminToken := newAdminRouter(t)
	_, targetID := newAuthUser(t, h, mdb, "admin-target@example.com", "admin-target")
	ctx := context.Background()

	t.Run("Happy path: GET by id consolidates account, plan and quotas", func(t *testing.T) {
		rec := adminRequest(t, h, adminToken, http.MethodGet, "/admin/users/"+targetID, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		view := adminViewOf(t, rec)
		assert.Equal(t, targetID, view.User.ID)
		assert.Equal(t, "admin-target@example.com", view.User.Email)
		assert.Equal(t, plan.IDFree, view.Plan)
		assert.Nil(t, view.Subscription)
		assert.Equal(t, quota.PaidDailyLimit, view.DailyQuota.Limit)
	})

	t.Run("Happy path: GET by email accepts a display-name spelling", func(t *testing.T) {
		rec := adminRequest(t, h, adminToken, http.MethodGet, "/admin/users?email=Admin+Target+%3Cadmin-target@example.com%3E", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, targetID, adminViewOf(t, rec).User.ID)
	})

	t.Run("Edge: missing email is 400", func(t *testing.T) {
		rec := adminRequest(t, h, adminToken, http.MethodGet, "/admin/users", "")
		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, "INVALID_REQUEST", errorBodyOf(t, rec).Error.Code)
	})

	t.Run("Edge: unknown email is 404", func(t *testing.T) {
		rec := adminRequest(t, h, adminToken, http.MethodGet, "/admin/users?email=missing@example.com", "")
		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, "NOT_FOUND", errorBodyOf(t, rec).Error.Code)
	})

	t.Run("Edge: a malformed id is 400", func(t *testing.T) {
		rec := adminRequest(t, h, adminToken, http.MethodGet, "/admin/users/not-a-uuid", "")
		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, "INVALID_REQUEST", errorBodyOf(t, rec).Error.Code)
	})

	t.Run("Edge: a valid but unknown id is 404", func(t *testing.T) {
		rec := adminRequest(t, h, adminToken, http.MethodGet, "/admin/users/0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9eff", "")
		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, "NOT_FOUND", errorBodyOf(t, rec).Error.Code)
	})

	expires := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)

	t.Run("Happy path: a comp grant makes the effective plan paid and is audited", func(t *testing.T) {
		body := fmt.Sprintf(`{"planId":"paid","expiresAt":%q,"reason":"support"}`, expires)
		rec := adminRequest(t, h, adminToken, http.MethodPost, "/admin/users/"+targetID+"/comp", body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

		var created response.Success[adminSubscriptionView]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
		assert.Equal(t, billing.ProviderComp, created.Data.Provider)
		assert.Equal(t, plan.IDPaid, created.Data.PlanID)
		assert.Equal(t, string(billing.StatusActive), created.Data.Status)
		require.NotNil(t, created.Data.CurrentPeriodEnd)

		view := adminViewOf(t, adminRequest(t, h, adminToken, http.MethodGet, "/admin/users/"+targetID, ""))
		assert.Equal(t, plan.IDPaid, view.Plan)
		require.NotNil(t, view.Subscription)
		assert.Equal(t, billing.ProviderComp, view.Subscription.Provider)

		var audit bson.M
		require.NoError(t, mdb.Collection(admin.AuditCollection).FindOne(ctx, bson.D{
			{Key: "target_user_id", Value: targetID},
			{Key: "action", Value: admin.ActionCompGrant},
		}).Decode(&audit))
		assert.Equal(t, adminIntegrationID, audit["actor_user_id"])
		assert.Equal(t, "support", audit["reason"])
	})

	t.Run("Edge: a second comp grant is 409", func(t *testing.T) {
		body := fmt.Sprintf(`{"planId":"paid","expiresAt":%q,"reason":"again"}`, expires)
		rec := adminRequest(t, h, adminToken, http.MethodPost, "/admin/users/"+targetID+"/comp", body)
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "CONFLICT", errorBodyOf(t, rec).Error.Code)
	})

	t.Run("Edge: invalid comp bodies are 400", func(t *testing.T) {
		bodies := []string{
			`{"planId":"paid"}`,
			`{"planId":"free","reason":"x"}`,
			`{"planId":"enterprise","reason":"x"}`,
			fmt.Sprintf(`{"planId":"paid","expiresAt":%q,"reason":"x"}`, time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)),
			fmt.Sprintf(`{"planId":"paid","reason":%q}`, strings.Repeat("x", admin.MaxReasonLength+1)),
		}
		for _, body := range bodies {
			rec := adminRequest(t, h, adminToken, http.MethodPost, "/admin/users/"+targetID+"/comp", body)
			require.Equal(t, http.StatusBadRequest, rec.Code, body)
			assert.Equal(t, "INVALID_REQUEST", errorBodyOf(t, rec).Error.Code)
		}
	})

	t.Run("Edge: a comp for an unknown user is 404", func(t *testing.T) {
		body := `{"planId":"paid","reason":"x"}`
		rec := adminRequest(t, h, adminToken, http.MethodPost, "/admin/users/0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9eff/comp", body)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("Edge: revoking a comp needs a reason and the comp", func(t *testing.T) {
		rec := adminRequest(t, h, adminToken, http.MethodDelete, "/admin/users/"+targetID+"/comp", `{}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		rec = adminRequest(t, h, adminToken, http.MethodDelete, "/admin/users/"+targetID+"/comp", `{"reason":"cleanup"}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		view := adminViewOf(t, adminRequest(t, h, adminToken, http.MethodGet, "/admin/users/"+targetID, ""))
		assert.Equal(t, plan.IDFree, view.Plan)
		assert.Nil(t, view.Subscription)

		rec = adminRequest(t, h, adminToken, http.MethodDelete, "/admin/users/"+targetID+"/comp", `{"reason":"cleanup"}`)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("Happy path: quota reset clears today and is idempotent", func(t *testing.T) {
		day := time.Now().UTC().Format("2006-01-02")
		_, err := mdb.Collection(quota.UsageCollection).InsertOne(ctx, bson.M{
			"user_id":    targetID,
			"day_utc":    day,
			"count":      9,
			"expires_at": time.Now().UTC().Add(48 * time.Hour),
		})
		require.NoError(t, err)

		rec := adminRequest(t, h, adminToken, http.MethodPost, "/admin/users/"+targetID+"/quota/reset", `{"reason":"support"}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		count, err := mdb.Collection(quota.UsageCollection).CountDocuments(ctx, bson.D{
			{Key: "user_id", Value: targetID},
			{Key: "day_utc", Value: day},
		})
		require.NoError(t, err)
		assert.Zero(t, count)

		rec = adminRequest(t, h, adminToken, http.MethodPost, "/admin/users/"+targetID+"/quota/reset", `{"reason":"support"}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		count, err = mdb.Collection(admin.AuditCollection).CountDocuments(ctx, bson.D{
			{Key: "target_user_id", Value: targetID},
			{Key: "action", Value: admin.ActionQuotaReset},
		})
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)

		rec = adminRequest(t, h, adminToken, http.MethodPost, "/admin/users/"+targetID+"/quota/reset", `{}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Edge: a free user without a paid subscription cannot be refunded", func(t *testing.T) {
		rec := adminRequest(t, h, adminToken, http.MethodPost, "/admin/users/"+targetID+"/refund", `{"override":true,"reason":"x"}`)
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "REFUND_NOT_ELIGIBLE", errorBodyOf(t, rec).Error.Code)
	})

	t.Run("Happy path: an in-window refund is queued once and audited", func(t *testing.T) {
		subID := mustAdminID(t)
		paidAt := time.Now().UTC().Add(-time.Hour)
		now := time.Now().UTC()
		_, err := mdb.Collection(billing.SubscriptionsCollection).InsertOne(ctx, bson.M{
			"_id":           subID,
			"user_id":       targetID,
			"plan_id":       plan.IDPaid,
			"provider":      billing.ProviderStripe,
			"status":        billing.StatusActive,
			"first_paid_at": paidAt,
			"created_at":    now,
			"updated_at":    now,
		})
		require.NoError(t, err)

		rec := adminRequest(t, h, adminToken, http.MethodPost, "/admin/users/"+targetID+"/refund", `{"override":false,"reason":"goodwill"}`)
		require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())

		var stored bson.M
		require.NoError(t, mdb.Collection(billing.SubscriptionsCollection).
			FindOne(ctx, bson.D{{Key: "_id", Value: subID}}).Decode(&stored))
		assert.Equal(t, string(billing.RefundRequested), stored["refund_state"])
		assert.Contains(t, stored, "refund_used_at")

		rec = adminRequest(t, h, adminToken, http.MethodPost, "/admin/users/"+targetID+"/refund", `{"override":true,"reason":"again"}`)
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "REFUND_NOT_ELIGIBLE", errorBodyOf(t, rec).Error.Code)

		count, err := mdb.Collection(admin.AuditCollection).CountDocuments(ctx, bson.D{
			{Key: "target_user_id", Value: targetID},
			{Key: "action", Value: admin.ActionRefundRequest},
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("Edge: override skips the window while the plain request refuses it", func(t *testing.T) {
		_, windowUserID := newAuthUser(t, h, mdb, "admin-window@example.com", "admin-window")
		subID := mustAdminID(t)
		paidAt := time.Now().UTC().Add(-billing.RefundWindow - time.Hour)
		now := time.Now().UTC()
		_, err := mdb.Collection(billing.SubscriptionsCollection).InsertOne(ctx, bson.M{
			"_id":           subID,
			"user_id":       windowUserID,
			"plan_id":       plan.IDPaid,
			"provider":      billing.ProviderStripe,
			"status":        billing.StatusActive,
			"first_paid_at": paidAt,
			"created_at":    now,
			"updated_at":    now,
		})
		require.NoError(t, err)

		rec := adminRequest(t, h, adminToken, http.MethodPost, "/admin/users/"+windowUserID+"/refund", `{"override":false,"reason":"x"}`)
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, billing.ReasonWindowExpired, errorBodyOf(t, rec).Error.Details["reason"])

		rec = adminRequest(t, h, adminToken, http.MethodPost, "/admin/users/"+windowUserID+"/refund", `{"override":true,"reason":"exception"}`)
		require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	})

	t.Run("Edge: a comp is never refundable", func(t *testing.T) {
		_, compUserID := newAuthUser(t, h, mdb, "admin-comp@example.com", "admin-comp")
		body := fmt.Sprintf(`{"planId":"paid","expiresAt":%q,"reason":"comp"}`, expires)
		rec := adminRequest(t, h, adminToken, http.MethodPost, "/admin/users/"+compUserID+"/comp", body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

		rec = adminRequest(t, h, adminToken, http.MethodPost, "/admin/users/"+compUserID+"/refund", `{"override":true,"reason":"x"}`)
		require.Equal(t, http.StatusConflict, rec.Code)
		bodyErr := errorBodyOf(t, rec)
		assert.Equal(t, "REFUND_NOT_ELIGIBLE", bodyErr.Error.Code)
		assert.Equal(t, billing.ReasonCompNotRefundable, bodyErr.Error.Details["reason"])

		count, err := mdb.Collection(admin.AuditCollection).CountDocuments(ctx, bson.D{
			{Key: "target_user_id", Value: compUserID},
			{Key: "action", Value: admin.ActionRefundRequest},
		})
		require.NoError(t, err)
		assert.Zero(t, count)
	})
}

func mustAdminID(t *testing.T) string {
	t.Helper()
	id, err := idgen.New()
	require.NoError(t, err)
	return id
}
