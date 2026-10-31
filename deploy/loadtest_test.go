package deploy_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	loadtestOverlay = "docker-compose.loadtest.yml"
	k6ScriptsDir    = "../loadtest/k6"
)

type loadtestDoc struct {
	Services map[string]struct {
		Environment map[string]string `yaml:"environment"`
		Command     []string          `yaml:"command"`
		Profiles    []string          `yaml:"profiles"`
		Volumes     []string          `yaml:"volumes"`
	} `yaml:"services"`
}

func readLoadtestOverlay(t *testing.T) loadtestDoc {
	t.Helper()

	raw, err := os.ReadFile(loadtestOverlay)
	require.NoError(t, err)

	var doc loadtestDoc
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	return doc
}

func TestLoadtestOverlay(t *testing.T) {
	doc := readLoadtestOverlay(t)

	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{"Happy path: every rate limiter is lifted on the app", func(t *testing.T) {
			env := doc.Services["app"].Environment
			for _, key := range []string{"AUTH_RATE_LIMIT_ATTEMPTS", "USER_RATE_LIMIT_ATTEMPTS", "PUBLIC_RATE_LIMIT_ATTEMPTS"} {
				assert.NotEmpty(t, env[key], "%s must be raised or 100 req/s trips the limiter", key)
			}
		}},
		{"Edge: prometheus keeps the base flags and accepts remote writes", func(t *testing.T) {
			command := strings.Join(doc.Services["prometheus"].Command, " ")
			assert.Contains(t, command, "--web.enable-remote-write-receiver")
			assert.Contains(t, command, "--web.route-prefix=/prometheus")
			assert.Contains(t, command, "--config.file=/etc/prometheus/prometheus.yml")
		}},
		{"Edge: k6 only starts under the loadtest profile", func(t *testing.T) {
			assert.Equal(t, []string{"loadtest"}, doc.Services["k6"].Profiles)
		}},
		{"Edge: k6 pushes to the prometheus route prefix", func(t *testing.T) {
			url := doc.Services["k6"].Environment["K6_PROMETHEUS_RW_SERVER_URL"]
			assert.Equal(t, "http://prometheus:9090/prometheus/api/v1/write", url)
		}},
		{"Edge: the mounted script exists", func(t *testing.T) {
			assert.Contains(t, doc.Services["k6"].Command, "/scripts/patch-sheet.js")
			assert.FileExists(t, filepath.Join(k6ScriptsDir, "patch-sheet.js"))
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.check)
	}
}

func TestDashboardsAreProvisionable(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(dashboardsDir, "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, paths, "guard: at least one dashboard must be provisioned")

	seen := map[string]struct{}{}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			require.NoError(t, err)

			var dash struct {
				UID    string `json:"uid"`
				Title  string `json:"title"`
				Panels []struct {
					Type    string `json:"type"`
					Title   string `json:"title"`
					Targets []struct {
						Expr string `json:"expr"`
					} `json:"targets"`
				} `json:"panels"`
			}
			require.NoError(t, json.Unmarshal(raw, &dash), "dashboard must be valid JSON")

			require.NotEmpty(t, dash.UID)
			assert.NotEmpty(t, dash.Title)
			assert.NotContains(t, seen, dash.UID, "dashboard uid must be unique")
			seen[dash.UID] = struct{}{}

			for _, panel := range dash.Panels {
				if panel.Type == "row" {
					continue
				}
				require.NotEmpty(t, panel.Targets, "panel %q has no query", panel.Title)
				for _, target := range panel.Targets {
					assert.NotEmpty(t, target.Expr, "panel %q has an empty query", panel.Title)
				}
			}
		})
	}
}

func TestDevObservabilityStack(t *testing.T) {
	raw, err := os.ReadFile("docker-compose.dev-observability.yml")
	require.NoError(t, err)

	var doc struct {
		Services map[string]struct {
			Ports   []string `yaml:"ports"`
			Command []string `yaml:"command"`
		} `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &doc))

	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{"Happy path: prometheus accepts k6 remote writes", func(t *testing.T) {
			assert.Contains(t, doc.Services["prometheus"].Command, "--web.enable-remote-write-receiver")
		}},
		{"Edge: anonymous Grafana and Prometheus bind to loopback only", func(t *testing.T) {
			for _, name := range []string{"prometheus", "grafana"} {
				require.NotEmpty(t, doc.Services[name].Ports, name)
				for _, port := range doc.Services[name].Ports {
					assert.True(t, strings.HasPrefix(port, "127.0.0.1:"), "%s port %q must not be exposed", name, port)
				}
			}
		}},
		{"Edge: the scrape target is the host API", func(t *testing.T) {
			scrape, err := os.ReadFile("dev-observability/prometheus.yml")
			require.NoError(t, err)
			assert.Contains(t, string(scrape), "host.docker.internal:8080")
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, tc.check)
	}
}
