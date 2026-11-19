package app_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorMapping_LegacyMonsterKindIsRejected(t *testing.T) {
	legacyMonsterKind := sheet.LegacyEnemyKind()

	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{
			name:   "Error: creating a sheet with the legacy monster kind is a 400 INVALID_REQUEST",
			method: http.MethodPost,
			target: "/sheets",
			body:   `{"template_id":"fantasy_v1","kind":"` + legacyMonsterKind + `","sheet_name":"Legacy"}`,
		},
		{
			name:   "Error: listing owned sheets with the legacy monster kind is a 400 INVALID_REQUEST",
			method: http.MethodGet,
			target: "/sheets/owned?kind=" + legacyMonsterKind,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router, templates := newSheetErrorRouter(t, mocks.NewMockRepository(t), newErrorTestDAG(t))

			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.target, body)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)

			var payload struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
			assert.Equal(t, string(apperr.CodeInvalidRequest), payload.Error.Code)
			templates.AssertNotCalled(t, "FetchTemplate")
		})
	}
}
