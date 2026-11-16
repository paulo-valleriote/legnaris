package app_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_SheetLimits(t *testing.T) {
	h, mdb := newIntegrationRouter(t)
	token, _ := newAuthUser(t, h, mdb, "limits@example.com", "limits-user")

	t.Run("Happy path: the first character on Free is created with its kind", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","kind":"character","sheet_name":"Hero"}`)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

		var created response.Success[sheet.SheetDTO]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
		assert.Equal(t, "character", created.Data.Kind)
	})

	t.Run("Error: a second character on Free is a 409 naming the kind and limit", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","kind":"character","sheet_name":"Second"}`)
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

		var body struct {
			Error struct {
				Code    string         `json:"code"`
				Details map[string]any `json:"details"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "SHEET_LIMIT_REACHED", body.Error.Code)
		assert.Equal(t, map[string]any{"kind": "character", "limit": float64(1)}, body.Error.Details)
	})

	t.Run("Error: a creation without a kind is a 400 INVALID_REQUEST", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","sheet_name":"NoKind"}`)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("Error: a kind the template does not accept is a 400 INVALID_REQUEST", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodPost, "/sheets", `{"template_id":"fantasy_v1","kind":"npc","sheet_name":"Goblin"}`)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("Happy path: GET /me/plan reports the plan and usage per kind", func(t *testing.T) {
		rec := doAuthRequest(t, h, token, http.MethodGet, "/me/plan", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var body response.Success[struct {
			Plan   string `json:"plan"`
			Sheets map[string]struct {
				Used  int `json:"used"`
				Limit int `json:"limit"`
			} `json:"sheets"`
		}]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "free", body.Data.Plan)
		assert.Equal(t, 1, body.Data.Sheets["character"].Used)
		assert.Equal(t, 1, body.Data.Sheets["character"].Limit)
		assert.Equal(t, 0, body.Data.Sheets["npc"].Used)
		assert.Equal(t, 2, body.Data.Sheets["npc"].Limit)
	})

	t.Run("Error: /me/plan without a token is refused", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodGet, "/me/plan", "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
