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

type loadtestPanel struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	FieldConfig struct {
		Defaults struct {
			Unit string `json:"unit"`
		} `json:"defaults"`
	} `json:"fieldConfig"`
	Options struct {
		ReduceOptions struct {
			Calcs []string `json:"calcs"`
		} `json:"reduceOptions"`
	} `json:"options"`
	Targets []struct {
		Expr string `json:"expr"`
	} `json:"targets"`
}

type loadtestDashboard struct {
	Panels     []loadtestPanel `json:"panels"`
	Templating struct {
		List []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"list"`
	} `json:"templating"`
}

func readLoadtestDashboard(t *testing.T) loadtestDashboard {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(dashboardsDir, "legnaris-loadtest.json"))
	require.NoError(t, err)

	var dash loadtestDashboard
	require.NoError(t, json.Unmarshal(raw, &dash))
	return dash
}

func (d loadtestDashboard) panel(t *testing.T, title string) loadtestPanel {
	t.Helper()

	for _, panel := range d.Panels {
		if panel.Title == title {
			return panel
		}
	}
	require.Failf(t, "panel not found", "no panel titled %q", title)
	return loadtestPanel{}
}

func (p loadtestPanel) exprs() string {
	var all []string
	for _, target := range p.Targets {
		all = append(all, target.Expr)
	}
	return strings.Join(all, "\n")
}

func TestLoadtestDashboard(t *testing.T) {
	dash := readLoadtestDashboard(t)

	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{"Happy path: k6 latency panels are in seconds, the unit k6 reports", func(t *testing.T) {
			checked := 0
			for _, panel := range dash.Panels {
				if strings.Contains(panel.exprs(), "k6_http_req_duration") {
					assert.Equal(t, "s", panel.FieldConfig.Defaults.Unit, "panel %q", panel.Title)
					checked++
				}
			}
			assert.Positive(t, checked, "guard: at least one k6 latency panel")
		}},
		{"Edge: dropped iterations shows 0 instead of No data when nothing was dropped", func(t *testing.T) {
			assert.Contains(t, dash.panel(t, "Dropped iterations").exprs(), "or vector(0)")
		}},
		{"Edge: checks use the Rate series k6 exports, not a non-existent counter", func(t *testing.T) {
			exprs := dash.panel(t, "Checks passing").exprs()
			assert.Contains(t, exprs, "k6_checks_rate")
			assert.NotContains(t, exprs, "k6_checks_total")
		}},
		{"Edge: stat panels summarise the whole range, never the last (ramp-down) sample", func(t *testing.T) {
			stats := 0
			for _, panel := range dash.Panels {
				if panel.Type != "stat" {
					continue
				}
				stats++
				assert.NotContains(t, panel.Options.ReduceOptions.Calcs, "lastNotNull", "panel %q", panel.Title)
			}
			assert.Positive(t, stats, "guard: at least one stat panel")
		}},
		{"Edge: server panels exclude the scrape endpoint from route metrics", func(t *testing.T) {
			for _, title := range []string{"Throughput by status", "Server latency p50/p95/p99", "Server p95 by route"} {
				assert.Contains(t, dash.panel(t, title).exprs(), `path!~"/metrics|unmatched"`, "panel %q", title)
			}
		}},
		{"Edge: saturation is observable through the in-flight gauge", func(t *testing.T) {
			assert.Contains(t, dash.panel(t, "In-flight requests").exprs(), "legnaris_http_in_flight_requests")
		}},
		{"Edge: client and server p95 share one panel so the gap is visible", func(t *testing.T) {
			exprs := dash.panel(t, "Client p95 vs server p95").exprs()
			assert.Contains(t, exprs, "k6_http_req_duration_p95")
			assert.Contains(t, exprs, "legnaris_http_request_duration_seconds_bucket")
		}},
		{"Edge: CPU and goroutines are separate panels (different scales)", func(t *testing.T) {
			assert.NotContains(t, dash.panel(t, "Server CPU").exprs(), "go_goroutines")
			assert.NotContains(t, dash.panel(t, "Goroutines").exprs(), "process_cpu_seconds_total")
		}},
		{"Edge: the target rate is a variable, not a hard-coded 100", func(t *testing.T) {
			assert.Contains(t, dash.panel(t, "Arrival rate vs target").exprs(), "vector($target_rate)")
			names := map[string]string{}
			for _, variable := range dash.Templating.List {
				names[variable.Name] = variable.Type
			}
			assert.Equal(t, "textbox", names["target_rate"])
			assert.Contains(t, names, "testid")
			assert.Contains(t, names, "name")
		}},
		{"Edge: no panel keeps the old single-operation title", func(t *testing.T) {
			for _, panel := range dash.Panels {
				assert.NotEqual(t, "PATCH p95", panel.Title, "stale panel title from the single-operation dashboard")
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, tc.check)
	}
}

func TestGrafanaDatasourceMatchesPrometheusRoutePrefix(t *testing.T) {
	compose, err := os.ReadFile("docker-compose.yml")
	require.NoError(t, err)
	require.Contains(t, string(compose), "--web.route-prefix=/prometheus", "guard: the stack serves Prometheus under a prefix")

	raw, err := os.ReadFile("grafana/provisioning/datasources/datasources.yml")
	require.NoError(t, err)

	var doc struct {
		Datasources []struct {
			UID string `yaml:"uid"`
			URL string `yaml:"url"`
		} `yaml:"datasources"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &doc))

	urls := map[string]string{}
	for _, datasource := range doc.Datasources {
		urls[datasource.UID] = datasource.URL
	}
	assert.Equal(t, "http://prometheus:9090/prometheus", urls["prometheus"],
		"without the prefix every Grafana query hits a 404 on Prometheus")
}
