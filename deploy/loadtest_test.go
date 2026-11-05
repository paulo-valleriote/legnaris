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
			for _, key := range []string{"AUTH_RATE_LIMIT_ATTEMPTS", "USER_RATE_LIMIT_ATTEMPTS"} {
				assert.NotEmpty(t, env[key], "%s must be raised or 100 req/s trips the limiter", key)
			}
		}},
		{"Happy path: the register limits are lifted and the disposable check is off", func(t *testing.T) {
			env := doc.Services["app"].Environment
			assert.Equal(t, "100000", env["AUTH_REGISTER_IP_RATE_LIMIT_ATTEMPTS"])
			assert.Equal(t, "100000", env["AUTH_REGISTER_EMAIL_RATE_LIMIT_ATTEMPTS"])
			assert.Equal(t, "false", env["AUTH_DISPOSABLE_EMAIL_CHECK"])
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
		{"Edge: the script is selectable and defaults to the acceptance test", func(t *testing.T) {
			assert.Equal(t, []string{"/scripts/${LOADTEST_SCRIPT:-patch-sheet.js}"}, doc.Services["k6"].Command)
			assert.FileExists(t, filepath.Join(k6ScriptsDir, "patch-sheet.js"))
			assert.FileExists(t, filepath.Join(k6ScriptsDir, "capacity.js"))
		}},
		{"Edge: the testid tag comes from the script, so one service runs every scenario", func(t *testing.T) {
			raw, err := os.ReadFile(loadtestOverlay)
			require.NoError(t, err)
			assert.NotContains(t, string(raw), "testid=")
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
			Name     string          `json:"name"`
			Type     string          `json:"type"`
			AllValue string          `json:"allValue"`
			Query    json.RawMessage `json:"query"`
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
		{"Edge: leak trends are slopes over a window, not raw values, so a leak shows as a persistent positive line", func(t *testing.T) {
			for _, title := range []string{"RSS growth (MiB/h)", "Heap growth (MiB/h)", "Goroutine growth (per hour)"} {
				assert.Contains(t, dash.panel(t, title).exprs(), "deriv(", "panel %q", title)
			}
		}},
		{"Edge: All operations excludes setup traffic, whose 401s and bcrypt latency would read as the run's errors and p95", func(t *testing.T) {
			var all, query string
			for _, variable := range dash.Templating.List {
				if variable.Name == "name" {
					var spec struct {
						Query string `json:"query"`
					}
					require.NoError(t, json.Unmarshal(variable.Query, &spec))
					all, query = variable.AllValue, spec.Query
				}
			}
			require.NotEmpty(t, all, "guard: the operation variable has an All value")
			assert.NotEqual(t, ".*", all, "a bare .* matches the setup series")
			assert.True(t, regexp.MustCompile("^(?:"+all+")$").MatchString("GET /sheets/:id"))
			assert.True(t, regexp.MustCompile("^(?:"+all+")$").MatchString("PATCH /sheets/:id cascade"))
			for _, setup := range []string{"setup /auth/login", "setup POST /sheets", "verify GET /sheets/:id", "teardown DELETE /sheets/:id"} {
				assert.False(t, regexp.MustCompile("^(?:"+all+")$").MatchString(setup), setup)
			}
			assert.Contains(t, query, "verify.*")
		}},
		{"Edge: each ramp step has its own p95 and error series", func(t *testing.T) {
			assert.Contains(t, dash.panel(t, "Client p95 by step").exprs(), "by (step)")
			assert.Contains(t, dash.panel(t, "Error rate by step").exprs(), "by (step)")
		}},
		{"Edge: the auth storm panel compares login with the background traffic", func(t *testing.T) {
			exprs := dash.panel(t, "Auth storm: login vs background p95").exprs()
			assert.Contains(t, exprs, `name="POST /auth/login"`)
			assert.Contains(t, exprs, `group="background"`)
		}},
		{"Edge: soak latency drift is read from the phase tag", func(t *testing.T) {
			assert.Contains(t, dash.panel(t, "Client p95 by phase").exprs(), "by (phase)")
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

func TestK6Scripts(t *testing.T) {
	read := func(t *testing.T, name string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(k6ScriptsDir, name))
		require.NoError(t, err)
		return string(raw)
	}

	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{"Happy path: the seed registers through the shared idempotent helper", func(t *testing.T) {
			seed := read(t, "seed.js")
			assert.Contains(t, seed, "ensureAccount")
			assert.Contains(t, seed, "shared-iterations")
		}},
		{"Edge: accounts are idempotent, an existing account is logged into before registering", func(t *testing.T) {
			assert.Contains(t, read(t, "lib/accounts.js"), "login(index) || register(index)")
		}},
		{"Edge: the API's one-sheet-per-user limit is honoured by fetching before creating", func(t *testing.T) {
			lib := read(t, "lib/accounts.js")
			fetch := strings.Index(lib, "GET /sheets")
			create := strings.Index(lib, "post('/sheets'")
			require.NotEqual(t, -1, fetch)
			require.NotEqual(t, -1, create)
			assert.Less(t, fetch, create)
		}},
		{"Edge: the PATCH test refreshes tokens, since a run outlives the access-token TTL", func(t *testing.T) {
			patch := read(t, "patch-sheet.js")
			assert.Contains(t, patch, "liveToken(session)")
			assert.Contains(t, read(t, "lib/accounts.js"), "expiresAt")
		}},
		{"Edge: the PATCH test no longer deletes the seeded sheets", func(t *testing.T) {
			assert.NotContains(t, read(t, "patch-sheet.js"), "teardown")
		}},
		{"Happy path: the capacity test ramps the arrival rate over a weighted mix", func(t *testing.T) {
			assert.Contains(t, read(t, "capacity.js"), "ramping-arrival-rate")
			mix := read(t, "lib/mix.js")
			for _, operation := range []string{"GET /sheets/:id", "PATCH /sheets/:id", "PATCH /sheets/:id cascade"} {
				assert.Contains(t, mix, operation)
			}
			assert.Contains(t, mix, "MIX_GET")
		}},
		{"Edge: every step has its own abort threshold, since a run-wide aggregate hides a collapse at the top step", func(t *testing.T) {
			ramp := read(t, "lib/ramp.js")
			assert.Contains(t, ramp, "abortOnFail: true")
			assert.Contains(t, ramp, "http_req_duration{${step}}")
			assert.Contains(t, ramp, "http_req_failed{${step}}")
			assert.Contains(t, read(t, "capacity.js"), "stepThresholds(STEPS")
		}},
		{"Edge: the p95 budget is tunable so thresholds can be tightened after the first run", func(t *testing.T) {
			assert.Contains(t, read(t, "capacity.js"), "P95_BUDGET_MS")
		}},
		{"Edge: each script tags its own testid", func(t *testing.T) {
			assert.Contains(t, read(t, "capacity.js"), "testid: 'capacity'")
			assert.Contains(t, read(t, "patch-sheet.js"), "testid: 'patch-sheet'")
		}},
		{"Edge: the PATCH mix includes a cascading write (identity.proficiencyBonus feeds every skill and save)", func(t *testing.T) {
			assert.Contains(t, read(t, "lib/mix.js"), "identity.proficiencyBonus")
		}},
		{"Happy path: the concurrency test fails on a lost update instead of only reporting it", func(t *testing.T) {
			concurrency := read(t, "concurrency.js")
			assert.Contains(t, concurrency, "legnaris_lost_updates: ['count==0']")
			assert.Contains(t, concurrency, "legnaris_inconsistent_derived: ['count==0']")
		}},
		{"Edge: both concurrency variants exist and an unknown one is refused", func(t *testing.T) {
			concurrency := read(t, "concurrency.js")
			assert.Contains(t, concurrency, "VARIANT !== 'distinct' && VARIANT !== 'same'")
			assert.Contains(t, concurrency, "SAME_FIELD")
			assert.Contains(t, concurrency, "DISTINCT_FIELDS")
		}},
		{"Edge: the storm duplicates writes in parallel, like a retry after a timeout", func(t *testing.T) {
			assert.Contains(t, read(t, "concurrency.js"), "http.batch")
		}},
		{"Edge: the storm never shares a sheet with the ramp (own accounts)", func(t *testing.T) {
			assert.Contains(t, read(t, "concurrency.js"), "CONCURRENCY_ACCOUNT_BASE")
		}},
		{"Edge: derived values are recomputed from stored inputs to catch a stale cascade", func(t *testing.T) {
			concurrency := read(t, "concurrency.js")
			assert.Contains(t, concurrency, "derived.armorClass")
			assert.Contains(t, concurrency, "modifiers.")
		}},
		{"Edge: onboarding is measured apart, register and create separately", func(t *testing.T) {
			onboarding := read(t, "onboarding.js")
			assert.Contains(t, onboarding, "POST /auth/register")
			assert.Contains(t, onboarding, "POST /sheets")
			assert.Contains(t, onboarding, "constant-arrival-rate")
		}},
		{"Edge: the one-sheet-per-user rule is honoured, each onboarding iteration uses a new account", func(t *testing.T) {
			assert.Contains(t, read(t, "onboarding.js"), "iterationInTest")
		}},
		{"Edge: ramp and soak share one operation mix, so their numbers are comparable", func(t *testing.T) {
			for _, script := range []string{"capacity.js", "soak.js"} {
				assert.Contains(t, read(t, script), "lib/mix.js", script)
			}
		}},
		{"Happy path: the soak holds one constant arrival rate for the configured time", func(t *testing.T) {
			soak := read(t, "soak.js")
			assert.Contains(t, soak, "constant-arrival-rate")
			assert.Contains(t, soak, "SOAK_SECONDS")
			assert.Contains(t, soak, "SOAK_RATE")
		}},
		{"Edge: soak latency drift is visible, the start and the end of the run are separate series", func(t *testing.T) {
			soak := read(t, "soak.js")
			for _, phase := range []string{"phase:start", "phase:middle", "phase:end"} {
				assert.Contains(t, soak, phase)
			}
		}},
		{"Edge: soak thresholds tolerate no drops and no failures", func(t *testing.T) {
			soak := read(t, "soak.js")
			assert.Contains(t, soak, "dropped_iterations: ['count==0']")
			for _, phase := range []string{"start", "middle", "end"} {
				assert.Contains(t, soak, "'http_req_failed{phase:"+phase+"}': ['rate<0.001']")
			}
			assert.NotContains(t, soak, "    http_req_failed: ", "the run-wide rate counts setup 401s")
		}},
		{"Happy path: the stress run goes past the capacity ramp and ends with a recovery step", func(t *testing.T) {
			stress := read(t, "stress.js")
			assert.Contains(t, stress, "ramping-arrival-rate")
			assert.Contains(t, stress, "2000,3000,4500,6000,300")
			assert.Contains(t, stress, "abort: false")
		}},
		{"Edge: stress only stops on a meltdown, a breached budget is a finding, not an abort", func(t *testing.T) {
			ramp := read(t, "lib/ramp.js")
			assert.Contains(t, ramp, "MELTDOWN_FAILED_RATE")
			assert.Contains(t, ramp, "abort ?")
			assert.Contains(t, read(t, "capacity.js"), "abort: true")
		}},
		{"Edge: capacity and stress share the ramp and mix so their steps are comparable", func(t *testing.T) {
			for _, script := range []string{"capacity.js", "stress.js"} {
				content := read(t, script)
				assert.Contains(t, content, "lib/ramp.js", script)
				assert.Contains(t, content, "lib/mix.js", script)
			}
		}},
		{"Happy path: the auth storm runs logins on top of background sheet traffic", func(t *testing.T) {
			storm := read(t, "auth-storm.js")
			assert.Contains(t, storm, "background")
			assert.Contains(t, storm, "loginStorm")
			assert.Contains(t, storm, "group:background")
		}},
		{"Edge: the auth storm tags every request with the login-rate step, so each step has its own background p95", func(t *testing.T) {
			storm := read(t, "auth-storm.js")
			assert.Equal(t, 2, strings.Count(storm, "step: currentStep(STEPS, STEP_SECONDS)"))
		}},
		{"Edge: setup may outlive k6's 60s default when accounts are created on the fly", func(t *testing.T) {
			assert.Contains(t, read(t, "patch-sheet.js"), "setupTimeout")
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, tc.check)
	}
}
