package app_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"legnaris/backend/internal/app"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type funnelObserverStub struct {
	visits int
	clicks int
}

func (o *funnelObserverStub) HomeVisited() { o.visits++ }

func (o *funnelObserverStub) HomeCTAClicked() { o.clicks++ }

func newFunnelRouter(observer app.FunnelObserver) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	e.POST("/funnel/home", app.NewFunnelController(observer).Home)
	return e
}

func TestFunnelController_Home(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantCode   int
		wantVisits int
		wantClicks int
		wantError  string
	}{
		{
			name:       "Happy path: a visit increments the visit counter",
			body:       `{"event":"visit"}`,
			wantCode:   http.StatusNoContent,
			wantVisits: 1,
		},
		{
			name:       "Happy path: a hero CTA click increments the click counter",
			body:       `{"event":"cta_click"}`,
			wantCode:   http.StatusNoContent,
			wantClicks: 1,
		},
		{
			name:       "Happy path: surrounding whitespace in the body is accepted",
			body:       "  {\"event\":\"visit\"} \n",
			wantCode:   http.StatusNoContent,
			wantVisits: 1,
		},
		{
			name:      "Error: an unknown event is refused",
			body:      `{"event":"signup"}`,
			wantCode:  http.StatusBadRequest,
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "Error: an empty event is refused",
			body:      `{"event":""}`,
			wantCode:  http.StatusBadRequest,
			wantError: "INVALID_REQUEST",
		},
		{
			name:      "Error: an empty body is refused",
			body:      "",
			wantCode:  http.StatusBadRequest,
			wantError: "INVALID_JSON",
		},
		{
			name:      "Error: malformed JSON is refused",
			body:      `{"event":`,
			wantCode:  http.StatusBadRequest,
			wantError: "INVALID_JSON",
		},
		{
			name:      "Error: an unknown field is refused",
			body:      `{"event":"visit","source":"hero"}`,
			wantCode:  http.StatusBadRequest,
			wantError: "INVALID_JSON",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			observer := &funnelObserverStub{}

			rec := doRequest(t, newFunnelRouter(observer), http.MethodPost, "/funnel/home", tc.body)

			require.Equal(t, tc.wantCode, rec.Code, rec.Body.String())
			assert.Equal(t, tc.wantVisits, observer.visits)
			assert.Equal(t, tc.wantClicks, observer.clicks)
			if tc.wantError != "" {
				var body response.ErrorResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, tc.wantError, body.Error.Code)
			}
		})
	}
}
