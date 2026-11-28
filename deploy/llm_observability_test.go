package deploy_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const llmDashboardFile = "grafana/dashboards/llm-generation.json"

func readLLMDashboard(t *testing.T) dashboardDoc {
	t.Helper()

	raw, err := os.ReadFile(llmDashboardFile)
	require.NoErrorf(t, err, "read %s", llmDashboardFile)

	var doc dashboardDoc
	require.NoErrorf(t, json.Unmarshal(raw, &doc), "parse %s", llmDashboardFile)
	return doc
}

func TestLLMDashboardCoversGenerationMetrics(t *testing.T) {
	dashboard := readLLMDashboard(t)

	t.Run("Happy path: the dashboard is identified and provisioned from the repository", func(t *testing.T) {
		assert.Equal(t, "llm-generation", dashboard.UID)
		assert.Equal(t, "LLM generation", dashboard.Title)
		entries, err := filepath.Glob(filepath.Join(dashboardsDir, "*.json"))
		require.NoError(t, err)
		assert.Contains(t, entries, llmDashboardFile)
	})

	series := []string{
		"legnaris_gen_ai_client_operation_duration_seconds",
		"legnaris_gen_ai_client_token_usage",
		"legnaris_gen_ai_cost_usd_total",
		"legnaris_gen_ai_cost_unknown_total",
		"legnaris_gen_ai_invalid_output_total",
		"legnaris_gen_ai_schema_retry_total",
		"legnaris_gen_ai_breaker_open",
		"legnaris_gen_ai_breaker_opened_total",
	}
	for _, want := range series {
		t.Run("Happy path: a panel shows the "+want, func(t *testing.T) {
			found := false
			for _, panel := range dashboard.Panels {
				for _, target := range panel.Targets {
					found = found || strings.Contains(target.Expr, want)
				}
			}
			assert.Truef(t, found, "a panel must read %s", want)
		})
	}

	t.Run("Edge: every panel is titled, typed, uniquely numbered and reads the monitoring datasource", func(t *testing.T) {
		require.NotEmpty(t, dashboard.Panels)
		ids := map[int]bool{}
		for _, panel := range dashboard.Panels {
			assert.NotEmptyf(t, panel.Title, "panel %d", panel.ID)
			assert.NotEmptyf(t, panel.Type, "panel %q", panel.Title)
			assert.Equalf(t, "prometheus", panel.Datasource.UID, "panel %q", panel.Title)
			assert.Falsef(t, ids[panel.ID], "panel id %d is used twice", panel.ID)
			ids[panel.ID] = true
		}
	})

	t.Run("Edge: every panel names the series it reads, in its query and in its description", func(t *testing.T) {
		names := regexp.MustCompile(`\b(legnaris_[a-z_]+)\b`)
		for _, panel := range dashboard.Panels {
			require.NotEmptyf(t, panel.Targets, "panel %q must carry a query", panel.Title)
			assert.Truef(t, strings.HasPrefix(panel.Description, "Reads "), "panel %q description must start with the series it reads", panel.Title)
			for _, target := range panel.Targets {
				for _, name := range names.FindAllString(target.Expr, -1) {
					assert.Containsf(t, panel.Description, name, "panel %q must say it reads %s", panel.Title, name)
				}
			}
		}
	})

	t.Run("Edge: the dashboard reads only the worker series and carries no credential", func(t *testing.T) {
		raw, err := os.ReadFile(llmDashboardFile)
		require.NoError(t, err)
		text := strings.ToLower(string(raw))
		for _, word := range []string{"password", "secret", "api_key", "apikey", "bearer"} {
			assert.NotContainsf(t, text, word, "%s must not carry %q", llmDashboardFile, word)
		}
		for _, panel := range dashboard.Panels {
			for _, target := range panel.Targets {
				if strings.Contains(target.Expr, "legnaris_gen_ai_") {
					assert.Containsf(t, target.Expr, `job="legnaris-worker"`, "panel %q must read the process that owns the series", panel.Title)
				}
			}
		}
	})
}
