package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"legnaris/backend/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestIntegration_Sheets_ConcurrentCreateRace proves the unique sheets.owner_id
// index, not the service pre-check, is the one-sheet guard (OWN-04): two
// concurrent creates for the same owner against real Mongo produce exactly one
// 201 and one 409 SHEET_LIMIT_REACHED, and exactly one document is persisted.
func TestIntegration_Sheets_ConcurrentCreateRace(t *testing.T) {
	h, mdb := newIntegrationRouter(t)
	token, userID := newAuthUser(t, h, "race@example.com", "race")

	const attempts = 2
	body := `{"template_id":"fantasy_v1","sheet_name":"Race"}`
	recorders := make([]*httptest.ResponseRecorder, attempts)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// The recorders slice is indexed per goroutine, so no two goroutines
			// write the same slot (race-free under -race).
			req := httptest.NewRequest(http.MethodPost, "/sheets", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			rec := httptest.NewRecorder()
			<-start
			h.ServeHTTP(rec, req)
			recorders[i] = rec
		}(i)
	}

	close(start)
	wg.Wait()

	statuses := make([]int, 0, attempts)
	conflicts := 0
	for _, rec := range recorders {
		statuses = append(statuses, rec.Code)
		if rec.Code != http.StatusConflict {
			continue
		}
		conflicts++

		var errBody response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errBody))
		assert.Equal(t, "SHEET_LIMIT_REACHED", errBody.Error.Code)
	}

	assert.ElementsMatch(t, []int{http.StatusCreated, http.StatusConflict}, statuses)
	assert.Equal(t, 1, conflicts, "exactly one create must lose the index race")

	count, err := mdb.Collection("sheets").
		CountDocuments(context.Background(), bson.D{{Key: "owner_id", Value: userID}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "exactly one sheet must persist for the owner")
}
