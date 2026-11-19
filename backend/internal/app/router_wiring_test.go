package app_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	authmocks "legnaris/backend/internal/auth/mocks"
	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/platform/metrics"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func newWiringRouter(t *testing.T) (http.Handler, *app.Modules) {
	t.Helper()
	return newWiringRouterWithEnv(t, nil)
}

func newWiringRouterWithEnv(t *testing.T, extra map[string]string) (http.Handler, *app.Modules) {
	t.Helper()

	router, modules := newWiringEcho(t, extra)
	return router, modules
}

func newWiringEcho(t *testing.T, extra map[string]string) (*echo.Echo, *app.Modules) {
	t.Helper()
	return newWiringEchoWithChecker(t, extra, nil)
}

func newWiringEchoWithChecker(t *testing.T, extra map[string]string, checker auth.ActiveAccountChecker) (*echo.Echo, *app.Modules) {
	t.Helper()

	t.Setenv("JWT_SECRET", "wiring-test-secret")
	t.Setenv("AUTH_BCRYPT_COST", "12")
	t.Setenv("EMAIL_CODE_SECRET", "wiring-test-code-secret-0123456789")
	for key, value := range extra {
		t.Setenv(key, value)
	}

	log := logger.NewLogger(slog.LevelError)
	env := env.NewEnv(log)

	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=50"))
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Disconnect(ctx)
	})

	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)

	modules, err := app.NewModules(&db.Database{Client: client}, registry, env, auth.NewNopObserver(), auth.NewNopChallengeObserver(), events.NoOp{})
	require.NoError(t, err)

	if checker == nil {
		checker = activeAccountsStub(t)
	}
	modules.ActiveAccounts = checker
	return app.NewRouter(env, modules, metrics.New()), modules
}

func activeAccountsStub(t *testing.T) auth.ActiveAccountChecker {
	t.Helper()
	checker := authmocks.NewMockService(t)
	checker.EXPECT().EnsureActive(mock.Anything, mock.Anything).Return(nil).Maybe()
	return checker
}

func TestRouterWiring_ActiveGateGuardsProtectedRoutes(t *testing.T) {
	deleting := apperr.NewError(apperr.CodeForbidden, auth.MsgAccountDeleting, nil)
	checker := authmocks.NewMockService(t)
	checker.EXPECT().EnsureActive(mock.Anything, mock.Anything).Return(deleting).Maybe()

	h, modules := newWiringEchoWithChecker(t, nil, checker)
	access, err := modules.Auth.TokenManager.Issue("deleting-user")
	require.NoError(t, err)

	protected := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/sheets"},
		{http.MethodGet, "/campaigns"},
		{http.MethodGet, "/templates"},
		{http.MethodPost, "/generations"},
		{http.MethodGet, "/me/plan"},
		{http.MethodPut, "/me/llm-consent"},
	}
	for _, tc := range protected {
		t.Run("Error: "+tc.method+" "+tc.path+" is forbidden for a deleting account", func(t *testing.T) {
			rec := doAuthRequest(t, h, access.Value, tc.method, tc.path, "")
			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

			var body response.ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, "FORBIDDEN", body.Error.Code)
		})
	}

	t.Run("Error: the deletion code request is behind the gate", func(t *testing.T) {
		rec := doAuthRequest(t, h, access.Value, http.MethodPost, "/auth/me/deletion", "")
		assert.Equal(t, http.StatusForbidden, rec.Code, "a deleting account must not receive another code")
	})

	t.Run("Happy path: the deletion confirmation is not behind the gate", func(t *testing.T) {
		rec := doAuthRequest(t, h, access.Value, http.MethodDelete, "/auth/me", `{"challenge_id":"x","code":"1"}`)
		assert.NotEqual(t, http.StatusForbidden, rec.Code, "a repeat deletion confirmation must stay idempotent")
	})
}

func TestRouterWiring_AuthRouteInventory(t *testing.T) {
	router, _ := newWiringEcho(t, nil)

	var got []string
	for _, route := range router.Router().Routes() {
		if route.Path == "/auth" || strings.HasPrefix(route.Path, "/auth/") {
			got = append(got, route.Method+" "+route.Path)
		}
	}

	assert.ElementsMatch(t, []string{
		http.MethodPost + " /auth/register",
		http.MethodPost + " /auth/login",
		http.MethodPost + " /auth/me/deletion",
		http.MethodDelete + " /auth/me",
		http.MethodPost + " /auth/refresh",
		http.MethodPost + " /auth/logout",
		http.MethodGet + " /auth/csrf",
		http.MethodPost + " /auth/challenge/verify",
		http.MethodPost + " /auth/challenge/resend",
		http.MethodGet + " /auth/me",
	}, got, "the auth surface must be exactly the session routes plus the two challenge routes plus the identity read")

	for _, route := range got {
		assert.NotContains(t, route, "purpose",
			"the challenge purpose must never be part of the route contract (OTP-09)")
	}
}

func TestRouterWiring_MeRequiresBearer(t *testing.T) {
	h, modules := newWiringRouter(t)

	t.Run("Edge: no credential is 401 with no account data", func(t *testing.T) {
		rec := doAuthRequest(t, h, "", http.MethodGet, "/auth/me", "")
		require.Equal(t, http.StatusUnauthorized, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "UNAUTHORIZED", body.Error.Code)
		assert.NotContains(t, rec.Body.String(), "@")
	})

	t.Run("Edge: a forged credential is 401", func(t *testing.T) {
		rec := doAuthRequest(t, h, "not-a-jwt", http.MethodGet, "/auth/me", "")
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Happy path: a valid credential reaches the handler", func(t *testing.T) {
		access, err := modules.Auth.TokenManager.Issue("01900000-0000-7000-8000-000000000001")
		require.NoError(t, err)

		rec := doAuthRequest(t, h, access.Value, http.MethodGet, "/auth/me", "")
		require.Equal(t, http.StatusInternalServerError, rec.Code)

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "INTERNAL_ERROR", body.Error.Code)
		assert.NotContains(t, rec.Body.String(), "@")
	})
}

func TestRouterWiring_RequireAuthGuardsProtectedRoutes(t *testing.T) {
	h, modules := newWiringRouter(t)

	protected := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "Edge: POST /sheets without token", method: http.MethodPost, path: "/sheets", body: `{"template_id":"fantasy_v1","kind":"character","sheet_name":"x"}`},
		{name: "Edge: GET /sheets without token", method: http.MethodGet, path: "/sheets"},
		{name: "Edge: GET /sheets/:id without token", method: http.MethodGet, path: "/sheets/" + missingSheetID},
		{name: "Edge: PATCH /sheets/:id without token", method: http.MethodPatch, path: "/sheets/" + missingSheetID, body: `{"data":{"stats":{"dexterity":12}}}`},
		{name: "Edge: DELETE /sheets/:id without token", method: http.MethodDelete, path: "/sheets/" + missingSheetID},
		{name: "Edge: POST /sheets/:id/preview without token", method: http.MethodPost, path: "/sheets/" + missingSheetID + "/preview", body: `{"data":{"stats":{"dexterity":12}}}`},
		{name: "Edge: GET /campaigns without token", method: http.MethodGet, path: "/campaigns"},
		{name: "Edge: POST /campaigns without token", method: http.MethodPost, path: "/campaigns", body: `{"name":"x","template_id":"dnd5e"}`},
		{name: "Edge: POST /campaigns/:id/leave without token", method: http.MethodPost, path: "/campaigns/" + missingSheetID + "/leave"},
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

		rrec := doRequest(t, h, http.MethodPost, "/auth/register", `{}`)
		require.Equal(t, http.StatusBadRequest, rrec.Code, "register must stay public")

		crec := doRequest(t, h, http.MethodPost, "/csp-report",
			`{"csp-report":{"effective-directive":"script-src","disposition":"enforce"}}`)
		require.Equal(t, http.StatusNoContent, crec.Code, "csp-report must stay public")
	})
}

const wiringAdminID = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e0f"

func TestRouterWiring_AdminRoutesMaskMethodsAndUnknownPaths(t *testing.T) {
	h, modules := newWiringRouterWithEnv(t, map[string]string{"ADMIN_USER_IDS": wiringAdminID})
	access, err := modules.Auth.TokenManager.Issue(wiringAdminID)
	require.NoError(t, err)

	requests := []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/admin/users/" + wiringAdminID},
		{http.MethodDelete, "/admin/users/" + wiringAdminID + "/comp"},
		{http.MethodGet, "/admin/users/" + wiringAdminID + "/comp/revoke"},
		{http.MethodPatch, "/admin/users/" + wiringAdminID + "/quota/reset"},
		{http.MethodPost, "/admin/unknown"},
		{http.MethodGet, "/admin/unknown/deeper"},
	}
	for _, tc := range requests {
		t.Run("Edge: "+tc.method+" "+tc.path+" is a 404 mask", func(t *testing.T) {
			rec := doAuthRequest(t, h, access.Value, tc.method, tc.path, "")
			require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

			var body response.ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, "NOT_FOUND", body.Error.Code)
		})
	}
}

func TestRouterWiring_AdminActorMustBeActive(t *testing.T) {
	deleting := apperr.NewError(apperr.CodeForbidden, auth.MsgAccountDeleting, nil)
	checker := authmocks.NewMockService(t)
	checker.EXPECT().EnsureActive(mock.Anything, wiringAdminID).Return(deleting).Maybe()

	h, modules := newWiringEchoWithChecker(t, map[string]string{"ADMIN_USER_IDS": wiringAdminID}, checker)
	access, err := modules.Auth.TokenManager.Issue(wiringAdminID)
	require.NoError(t, err)

	rec := doAuthRequest(t, h, access.Value, http.MethodGet, "/admin/users/"+wiringAdminID, "")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	var body response.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "NOT_FOUND", body.Error.Code)
}
