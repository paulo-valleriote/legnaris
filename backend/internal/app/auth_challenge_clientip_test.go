package app_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	authmocks "legnaris/backend/internal/auth/mocks"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestChallengeController_VerifyCarriesClientIP(t *testing.T) {
	tests := []struct {
		name     string
		resolved string
		want     string
	}{
		{name: "Happy path: the resolved client address reaches the service", resolved: "203.0.113.9", want: "203.0.113.9"},
		{name: "Edge: an unresolvable address is passed through empty", resolved: "", want: ""},
		{name: "Edge: the IPv6 form is passed through unchanged", resolved: "2001:db8::9", want: "2001:db8::9"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := authmocks.NewMockService(t)
			service.EXPECT().
				VerifyChallenge(mock.Anything, auth.VerifyChallengeCommand{ChallengeID: challengeID, Code: "042917", ClientIP: tc.want}).
				Return(session(), nil).
				Once()

			e := echo.New()
			e.IPExtractor = func(*http.Request) string { return tc.resolved }
			controller := app.NewChallengeController(service, &countingObserver{})
			e.POST(verifyRoute, controller.Verify)

			req := httptest.NewRequest(http.MethodPost, verifyRoute,
				strings.NewReader(`{"challenge_id":"`+challengeID+`","code":"042917"}`))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		})
	}
}
