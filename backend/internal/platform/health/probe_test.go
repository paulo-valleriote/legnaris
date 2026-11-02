package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"legnaris/backend/internal/platform/health"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbe(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{name: "Happy path: 200 passes", handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }},
		{name: "Error: 503 fails", handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }, wantErr: true},
		{name: "Error: 404 fails", handler: http.NotFound, wantErr: true},
		{name: "Edge: 204 is not the readiness contract and fails", handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			t.Cleanup(server.Close)

			err := health.Probe(context.Background(), server.URL, time.Second)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}

	t.Run("Edge: a server slower than the timeout fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(2 * time.Second):
			}
		}))
		t.Cleanup(server.Close)

		start := time.Now()
		require.Error(t, health.Probe(context.Background(), server.URL, 50*time.Millisecond))
		assert.Less(t, time.Since(start), time.Second)
	})

	t.Run("Error: an unreachable address fails", func(t *testing.T) {
		assert.Error(t, health.Probe(context.Background(), "http://127.0.0.1:1/ready", time.Second))
	})

	t.Run("Edge: a malformed URL fails", func(t *testing.T) {
		assert.Error(t, health.Probe(context.Background(), "://bad", time.Second))
	})
}
