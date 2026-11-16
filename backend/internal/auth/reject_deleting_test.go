package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/auth/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/requestcache"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type accountReadKey struct{}

func TestRejectDeleting(t *testing.T) {
	gateErr := apperr.NewError(apperr.CodeForbidden, auth.MsgAccountDeleting, nil)

	tests := []struct {
		name       string
		authed     bool
		gateErr    error
		wantCode   apperr.Code
		wantLoads  int
		wantHandle bool
	}{
		{name: "Edge: a request without an authenticated user is refused before the gate", authed: false, wantCode: apperr.CodeUnauthorized, wantLoads: 0},
		{name: "Happy path: the gate and the handler share one account read", authed: true, wantLoads: 1, wantHandle: true},
		{name: "Error: a deleting account is refused and the handler does not run", authed: true, gateErr: gateErr, wantCode: apperr.CodeForbidden, wantLoads: 1},
		{name: "Error: a gate failure is returned and the handler does not run", authed: true, gateErr: errors.New("db down"), wantLoads: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			loads := 0
			load := func(context.Context) (string, error) {
				loads++
				return "account", nil
			}
			checker := mocks.NewMockService(t)
			if tc.authed {
				checker.EXPECT().EnsureActive(mock.Anything, "u1").RunAndReturn(
					func(ctx context.Context, _ string) error {
						_, err := requestcache.Memo(ctx, accountReadKey{}, load)
						return errOr(err, tc.gateErr)
					},
				).Once()
			}

			handled := false
			next := func(c *echo.Context) error {
				handled = true
				_, err := requestcache.Memo(c.Request().Context(), accountReadKey{}, load)
				return err
			}

			e := echo.New()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
			if tc.authed {
				c.Set(auth.UserIDContextKey, "u1")
			}

			err := auth.RejectDeleting(checker)(next)(c)

			if tc.wantCode != "" || tc.gateErr != nil {
				require.Error(t, err)
			}
			if tc.wantCode != "" {
				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, tc.wantCode, appErr.Code)
			}
			assert.Equal(t, tc.wantHandle, handled)
			assert.Equal(t, tc.wantLoads, loads, "the account is read once per request")
		})
	}
}

func errOr(primary, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}
