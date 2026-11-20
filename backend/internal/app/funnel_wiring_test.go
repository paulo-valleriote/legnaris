package app_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouterWiring_FunnelEndpointIsPublicAndCounts(t *testing.T) {
	h, _ := newWiringRouter(t)

	rec := doRequest(t, h, http.MethodPost, "/funnel/home", `{"event":"visit"}`)

	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	metricsRec := doRequest(t, h, http.MethodGet, "/metrics", "")
	require.Equal(t, http.StatusOK, metricsRec.Code, metricsRec.Body.String())
	assert.Contains(t, metricsRec.Body.String(), "legnaris_funnel_home_visits_total 1")
	assert.Contains(t, metricsRec.Body.String(), "legnaris_funnel_home_cta_clicks_total 0")
}

func TestRouterWiring_FunnelEndpointRefusesTheOverflowPerAddress(t *testing.T) {
	h, _ := newWiringRouterWithEnv(t, map[string]string{
		"CSP_REPORT_RATE_LIMIT_ATTEMPTS": "2",
		"CSP_REPORT_RATE_LIMIT_WINDOW":   "1h",
	})
	const address = "198.51.100.9:4000"

	for i := 1; i <= 2; i++ {
		rec := routeRequest(t, h, http.MethodPost, "/funnel/home", `{"event":"visit"}`, "", address)
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code, "request %d must be admitted", i)
	}

	requireRateLimited(t, routeRequest(t, h, http.MethodPost, "/funnel/home", `{"event":"visit"}`, "", address))

	t.Run("Edge: another address keeps its own budget", func(t *testing.T) {
		rec := routeRequest(t, h, http.MethodPost, "/funnel/home", `{"event":"visit"}`, "", "198.51.100.10:4000")
		assert.NotEqual(t, http.StatusTooManyRequests, rec.Code)
	})
}
