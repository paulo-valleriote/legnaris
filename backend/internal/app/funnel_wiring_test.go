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
