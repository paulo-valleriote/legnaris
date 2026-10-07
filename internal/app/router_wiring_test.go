package app_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"legnaris/internal/app"
	"legnaris/internal/auth"
	"legnaris/internal/engine"
	"legnaris/internal/platform/metrics"
	"legnaris/internal/sheet/template"
	"legnaris/shared/db"
	"legnaris/shared/env"
	"legnaris/shared/logger"
	"legnaris/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// newWiringRouter builds the production router over a mongo client that never
// reaches a server: mongo.Connect seeds the topology without dialing, so the
// JWT wiring is observable with no database. This is the route-wiring guard
// that runs in CI, where the integration suite self-skips without a database.
func newWiringRouter(t *testing.T) (http.Handler, *app.Modules) {
	t.Helper()

	t.Setenv("JWT_SECRET", "wiring-test-secret")
	t.Setenv("AUTH_BCRYPT_COST", "12")

	log := logger.NewLogger(slog.LevelError)
	env := env.NewEnv(log)

	// Port 1 refuses instantly; the short server-selection timeout only comes
	// into play for a request that wrongly reaches a database-backed handler.
	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=50"))
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Disconnect(ctx)
	})

	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)

	modules, err := app.NewModules(&db.Database{Client: client}, registry, env, auth.NewNopObserver())
	require.NoError(t, err)

	return app.NewRouter(env, modules, metrics.New()), modules
}

// TestRouterWiring_RequireAuthGuardsProtectedRoutes covers OWN-15…OWN-18
// without a database: every /sheets* and /templates* route rejects a tokenless
// request with 401 UNAUTHORIZED, health/metrics/auth stay public, and a valid
// token reaches the normal handler.
func TestRouterWiring_RequireAuthGuardsProtectedRoutes(t *testing.T) {
	h, modules := newWiringRouter(t)

	protected := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "Edge: POST /sheets without token", method: http.MethodPost, path: "/sheets", body: `{"template_id":"fantasy_v1","sheet_name":"x"}`},
		{name: "Edge: GET /sheets without token", method: http.MethodGet, path: "/sheets"},
		{name: "Edge: GET /sheets/:id without token", method: http.MethodGet, path: "/sheets/" + missingSheetID},
		{name: "Edge: PATCH /sheets/:id without token", method: http.MethodPatch, path: "/sheets/" + missingSheetID, body: `{"data":{"stats":{"dexterity":12}}}`},
		{name: "Edge: DELETE /sheets/:id without token", method: http.MethodDelete, path: "/sheets/" + missingSheetID},
		{name: "Edge: POST /sheets/:id/preview without token", method: http.MethodPost, path: "/sheets/" + missingSheetID + "/preview", body: `{"data":{"stats":{"dexterity":12}}}`},
		{name: "Edge: GET /templates without token", method: http.MethodGet, path: "/templates"},
		{name: "Edge: GET /templates/:id without token", method: http.MethodGet, path: "/templates/fantasy_v1"},
		{name: "Edge: POST /templates/:id/preview without token", method: http.MethodPost, path: "/templates/fantasy_v1/preview", body: `{"data":{"stats":{"dexterity":12}}}`},
	}

	for _, tc := range protected {
		t.Run(tc.name, func(t *testing.T) {
			rec := doAuthRequest(t, h, "", tc.method, tc.path, tc.body)
			require.Equal(t, http.StatusUnauthorized, rec.Code, "%s %s must be 401 without a token", tc.method, tc.path)

			var body response.ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, "UNAUTHORIZED", body.Error.Code)
		})
	}

	t.Run("Edge: an invalid token is 401", func(t *testing.T) {
		rec := doAuthRequest(t, h, "not-a-jwt", http.MethodGet, "/sheets", "")
		require.Equal(t, http.StatusUnauthorized, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "UNAUTHORIZED", body.Error.Code)
	})

	t.Run("Happy path: a valid token reaches the normal handler", func(t *testing.T) {
		access, err := modules.Auth.TokenManager.Issue("wiring-user")
		require.NoError(t, err)

		rec := doAuthRequest(t, h, access.Value, http.MethodGet, "/templates", "")
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Edge: public routes stay reachable without a token", func(t *testing.T) {
		health := doRequest(t, h, http.MethodGet, "/health", "")
		require.True(t, health.Code == http.StatusOK || health.Code == http.StatusNoContent, "health must stay public")

		mrec := doRequest(t, h, http.MethodGet, "/metrics", "")
		require.Equal(t, http.StatusOK, mrec.Code)

		// An invalid email stops register at service validation, before any
		// database access, so a 400 here proves the route is not JWT-gated.
		rrec := doRequest(t, h, http.MethodPost, "/auth/register", `{}`)
		require.Equal(t, http.StatusBadRequest, rrec.Code, "register must stay public")
	})
}
