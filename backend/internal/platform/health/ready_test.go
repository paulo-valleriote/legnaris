package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/platform/health"
	"legnaris/backend/internal/platform/health/mocks"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newReadyRouter(pinger health.Pinger, timeout time.Duration) *echo.Echo {
	router := echo.New()
	router.HTTPErrorHandler = app.HTTPErrorHandler
	health.RegisterReadyRoute(router.Group(""), pinger, timeout)
	return router
}

func getReady(router *echo.Echo) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	return rec
}

func TestReady(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		blockPing  bool
		wantStatus int
		wantCode   string
	}{
		{name: "Happy path: ping succeeds answers 200", wantStatus: http.StatusOK},
		{name: "Error: ping failure answers 503 NOT_READY", pingErr: errors.New("server selection error"), wantStatus: http.StatusServiceUnavailable, wantCode: "NOT_READY"},
		{name: "Edge: ping that outlives the timeout answers 503 NOT_READY", blockPing: true, wantStatus: http.StatusServiceUnavailable, wantCode: "NOT_READY"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pinger := mocks.NewMockPinger(t)
			pinger.EXPECT().Ping(mock.Anything).RunAndReturn(func(ctx context.Context) error {
				if tc.blockPing {
					<-ctx.Done()
					return ctx.Err()
				}
				return tc.pingErr
			}).Once()

			start := time.Now()
			rec := getReady(newReadyRouter(pinger, 50*time.Millisecond))

			require.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
			if tc.blockPing {
				assert.Less(t, time.Since(start), time.Second, "the probe must not hang past its timeout")
			}
			if tc.wantCode != "" {
				var body response.ErrorResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, tc.wantCode, body.Error.Code)
			}
		})
	}
}

func TestReady_PingDeadlineIsBounded(t *testing.T) {
	pinger := mocks.NewMockPinger(t)
	pinger.EXPECT().Ping(mock.Anything).RunAndReturn(func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		assert.True(t, ok, "the ping context must carry a deadline")
		assert.WithinDuration(t, time.Now().Add(time.Second), deadline, 500*time.Millisecond)
		return nil
	}).Once()

	assert.Equal(t, http.StatusOK, getReady(newReadyRouter(pinger, time.Second)).Code)
}
