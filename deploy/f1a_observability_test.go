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
	f1aRulesFile       = "prometheus/rules-f1a.yml"
	f1aRulesMount      = "./prometheus/rules-f1a.yml:/etc/prometheus/rules-f1a.yml:ro"
	f1aRulesPath       = "/etc/prometheus/rules-f1a.yml"
	f1aDashboardFile   = "grafana/dashboards/f1a-generation.json"
	f1aRunbook         = "docs/runbook-sweeper-dead-items.md"
	f1aThresholdPrefix = "legnaris:f1a:threshold_"
	f1aOldestThreshold = f1aThresholdPrefix + "oldest_pending_seconds"
	f1aCapThreshold    = f1aThresholdPrefix + "generation_cap_ratio"
	f1aMetricsSource   = "../backend/internal/platform/metrics/metrics.go"
)

type f1aAlert struct {
	name     string
	severity string
	series   []string
	reads    []string
}

var f1aAlerts = []f1aAlert{
	{
		name: "LegnarisSweeperOldestPendingTooOld", severity: "high",
		series: []string{"legnaris_sweeper_oldest_pending_seconds"},
		reads:  []string{`job="legnaris-relay"`, "max by (queue)", f1aOldestThreshold},
	},
	{
		name: "LegnarisSweeperHandlerFailing", severity: "medium",
		series: []string{"legnaris_sweeper_failed_total"},
		reads:  []string{`job="legnaris-relay"`, "increase(", "by (queue)"},
	},
	{
		name: "LegnarisEventsUnroutedNotEmpty", severity: "high",
		series: []string{"rabbitmq_queue_messages_ready"},
		reads:  []string{`job="legnaris-rabbitmq"`, `vhost="legnaris"`, `queue="events.unrouted"`},
	},
	{
		name: "LegnarisGenerationCapAt80Percent", severity: "high",
		series: []string{"legnaris_generation_cap_used", "legnaris_generation_cap_limit"},
		reads:  []string{`job="legnaris-app"`, f1aCapThreshold, "> 0"},
	},
}

type f1aRulesDoc struct {
	Groups []struct {
		Name  string     `yaml:"name"`
		Rules []ruleSpec `yaml:"rules"`
	} `yaml:"groups"`
}

func readF1aRules(t *testing.T) []ruleSpec {
	t.Helper()

	raw, err := os.ReadFile(f1aRulesFile)
	require.NoErrorf(t, err, "read %s", f1aRulesFile)

	var doc f1aRulesDoc
	require.NoErrorf(t, yaml.Unmarshal(raw, &doc), "parse %s", f1aRulesFile)

	var rules []ruleSpec
	for _, group := range doc.Groups {
		rules = append(rules, group.Rules...)
	}
	return rules
}

func f1aAlertsByName(t *testing.T) map[string]ruleSpec {
	t.Helper()

	alerts := map[string]ruleSpec{}
	for _, rule := range readF1aRules(t) {
		if rule.Alert == "" {
			continue
		}
		_, duplicate := alerts[rule.Alert]
		require.Falsef(t, duplicate, "alert %s is defined twice", rule.Alert)
		alerts[rule.Alert] = rule
	}
	return alerts
}

func TestF1aAlertRulesExistAndAreComplete(t *testing.T) {
	alerts := f1aAlertsByName(t)

	for _, want := range f1aAlerts {
		t.Run("Happy path: "+want.name+" exists with its severity", func(t *testing.T) {
			rule, present := alerts[want.name]
			require.Truef(t, present, "the alert %s must exist", want.name)
			assert.Equal(t, want.severity, rule.Labels["severity"])
		})
	}

	t.Run("Edge: every alert states its patience, what broke and links the dead-item runbook", func(t *testing.T) {
		require.Len(t, alerts, len(f1aAlerts), "no F1a alarm may be missing or extra")
		for name, rule := range alerts {
			assert.NotEmptyf(t, rule.For, "alert %s must state its patience period", name)
			assert.NotEmptyf(t, rule.Annotations["summary"], "alert %s must say what broke", name)
			assert.NotEmptyf(t, rule.Annotations["description"], "alert %s must describe the failure", name)
			assert.Equalf(t, f1aRunbook, rule.Annotations["runbook"], "alert %s must link the dead-item runbook", name)
		}
	})

	t.Run("Edge: every severity is one the runbook defines", func(t *testing.T) {
		for name, rule := range alerts {
			assert.Containsf(t, []string{"critical", "high", "medium"}, rule.Labels["severity"], "alert %s", name)
		}
	})

	t.Run("Edge: every alert has an expression that is not blank", func(t *testing.T) {
		for name, rule := range alerts {
			assert.NotEmptyf(t, strings.TrimSpace(rule.Expr), "alert %s", name)
		}
	})

	t.Run("Edge: the rules of this file never reuse an email alert name", func(t *testing.T) {
		email := alertsByName(t)
		for name := range alerts {
			assert.NotContainsf(t, email, name, "alert %s collides with the email pipeline", name)
		}
	})
}

func TestF1aAlertRulesNameTheirSeriesAndProcess(t *testing.T) {
	alerts := f1aAlertsByName(t)

	for _, want := range f1aAlerts {
		t.Run("Happy path: "+want.name+" names its series and its source", func(t *testing.T) {
			rule, present := alerts[want.name]
			require.Truef(t, present, "the alert %s must exist", want.name)
			for _, fragment := range append(append([]string{}, want.series...), want.reads...) {
				assert.Containsf(t, rule.Expr, fragment, "alert %s must read %s", want.name, fragment)
			}
		})
	}

	t.Run("Edge: every legnaris counter in these rules is read with a job selector", func(t *testing.T) {
		for name, rule := range alerts {
			if strings.Contains(rule.Expr, "legnaris_") {
				assert.Containsf(t, rule.Expr, "job=", "alert %s", name)
			}
		}
	})

	t.Run("Edge: the cap rule guards against a zero limit so an unset cap never divides by zero", func(t *testing.T) {
		rule := alerts["LegnarisGenerationCapAt80Percent"]
		assert.Contains(t, rule.Expr, `legnaris_generation_cap_limit{job="legnaris-app"} > 0`)
	})

	t.Run("Edge: the cap rule reads used over limit, never the ratio gauge that only refreshes on reservation", func(t *testing.T) {
		rule := alerts["LegnarisGenerationCapAt80Percent"]
		assert.NotContains(t, rule.Expr, "legnaris_generation_cap_used_ratio")
	})

	t.Run("Edge: the cap alert says the value can be stale after UTC midnight", func(t *testing.T) {
		rule := alerts["LegnarisGenerationCapAt80Percent"]
		assert.Contains(t, rule.Annotations["description"], "midnight")
	})
}

func TestF1aAlertThresholdsAreConfigurable(t *testing.T) {
	alerts := f1aAlertsByName(t)

	thresholds := map[string]ruleSpec{}
	for _, rule := range readF1aRules(t) {
		if strings.HasPrefix(rule.Record, f1aThresholdPrefix) {
			thresholds[rule.Record] = rule
		}
	}

	t.Run("Happy path: the oldest-pending alert and the cap alert read their recorded thresholds", func(t *testing.T) {
		require.Contains(t, thresholds, f1aOldestThreshold)
		require.Contains(t, thresholds, f1aCapThreshold)
		assert.Contains(t, alerts["LegnarisSweeperOldestPendingTooOld"].Expr, f1aOldestThreshold)
		assert.Contains(t, alerts["LegnarisGenerationCapAt80Percent"].Expr, f1aCapThreshold)
	})

	t.Run("Edge: the thresholds are constants with the documented values (one hour, 80%)", func(t *testing.T) {
		require.Len(t, thresholds, 2)
		assert.Equal(t, "vector(3600)", strings.TrimSpace(thresholds[f1aOldestThreshold].Expr))
		assert.Equal(t, "vector(0.8)", strings.TrimSpace(thresholds[f1aCapThreshold].Expr))
	})

	t.Run("Edge: no comparison in an F1a alert hides a number other than zero", func(t *testing.T) {
		hidden := regexp.MustCompile(`[<>]=?\s*[1-9][0-9]*(\.[0-9]+)?`)
		for name, rule := range alerts {
			assert.NotRegexpf(t, hidden, rule.Expr, "alert %s hard-codes a threshold", name)
		}
	})
}

func TestF1aSweeperMetricsExistInSource(t *testing.T) {
	raw, err := os.ReadFile(f1aMetricsSource)
	require.NoError(t, err)
	source := string(raw)

	cases := []struct {
		name string
		want []string
	}{
		{name: "sweeper oldest pending", want: []string{"sweeperSubsystem", `"oldest_pending_seconds"`}},
		{name: "sweeper failed", want: []string{"sweeperSubsystem", `"failed_total"`}},
		{name: "events dropped", want: []string{`"events_dropped_total"`, `[]string{"type", "reason"}`}},
		{name: "generation cap used", want: []string{`"generation_cap_used"`}},
		{name: "generation cap limit", want: []string{`"generation_cap_limit"`}},
	}
	for _, tc := range cases {
		t.Run("Happy path: "+tc.name+" is registered by the metrics package", func(t *testing.T) {
			for _, fragment := range tc.want {
				assert.Containsf(t, source, fragment, "metrics.go must register %s", tc.name)
			}
		})
	}

	t.Run("Edge: the sweeper series are labelled by queue only", func(t *testing.T) {
		assert.Contains(t, source, `const queueLabel = "queue"`)
	})
}

func TestF1aScrapeCarriesTheSweeperAndTheBroker(t *testing.T) {
	jobs := scrapeTargets(t)

	t.Run("Happy path: the legnaris-relay job scrapes the relay metrics listener", func(t *testing.T) {
		assert.Equal(t, []string{"relay:9100"}, jobs["legnaris-relay"])
	})

	t.Run("Edge: the relay's metrics listener defaults to the port the scrape reads", func(t *testing.T) {
		raw, err := os.ReadFile(composeFile)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "EMAIL_RELAY_METRICS_ADDR: ${EMAIL_RELAY_METRICS_ADDR:-:9100}")
	})

	t.Run("Edge: the broker scrape is the plugin's port, so events.unrouted depth is readable", func(t *testing.T) {
		assert.Equal(t, []string{"rabbitmq:15692"}, jobs["legnaris-rabbitmq"])
	})
}

func TestF1aRulesAreLoadedAndMounted(t *testing.T) {
	raw, err := os.ReadFile(prometheusFile)
	require.NoError(t, err)
	var doc struct {
		RuleFiles []string `yaml:"rule_files"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &doc))

	t.Run("Happy path: the stack loads the F1a rules file", func(t *testing.T) {
		assert.Contains(t, doc.RuleFiles, f1aRulesPath)
	})

	t.Run("Happy path: the F1a rules file is mounted read-only into the stack", func(t *testing.T) {
		assert.Contains(t, readCompose(t).Services["prometheus"].Volumes, f1aRulesMount)
	})
}

type f1aDashboardDoc struct {
	UID    string `json:"uid"`
	Title  string `json:"title"`
	Panels []struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Datasource  struct {
			UID string `json:"uid"`
		} `json:"datasource"`
		Targets []struct {
			Expr string `json:"expr"`
		} `json:"targets"`
	} `json:"panels"`
}

func readF1aDashboard(t *testing.T) f1aDashboardDoc {
	t.Helper()

	raw, err := os.ReadFile(f1aDashboardFile)
	require.NoErrorf(t, err, "read %s", f1aDashboardFile)

	var doc f1aDashboardDoc
	require.NoErrorf(t, json.Unmarshal(raw, &doc), "parse %s", f1aDashboardFile)
	return doc
}

func TestF1aDashboardCoversTheSweeperAndCap(t *testing.T) {
	dashboard := readF1aDashboard(t)

	t.Run("Happy path: the dashboard is identified and provisioned from the repository", func(t *testing.T) {
		assert.Equal(t, "f1a-generation", dashboard.UID)
		assert.NotEmpty(t, dashboard.Title)
		entries, err := filepath.Glob(filepath.Join(dashboardsDir, "*.json"))
		require.NoError(t, err)
		assert.Contains(t, entries, f1aDashboardFile)
	})

	panels := []struct {
		name   string
		series string
	}{
		{name: "sweeper backlog age per queue", series: "legnaris_sweeper_oldest_pending_seconds"},
		{name: "sweeper failed attempts per queue", series: "legnaris_sweeper_failed_total"},
		{name: "events.unrouted depth", series: "rabbitmq_queue_messages_ready"},
		{name: "dropped events by reason", series: "legnaris_events_dropped_total"},
		{name: "generation jobs versus the cap", series: "legnaris_generation_cap_used"},
		{name: "generation cap limit", series: "legnaris_generation_cap_limit"},
	}
	for _, want := range panels {
		t.Run("Happy path: a panel shows the "+want.name, func(t *testing.T) {
			found := false
			for _, panel := range dashboard.Panels {
				for _, target := range panel.Targets {
					found = found || strings.Contains(target.Expr, want.series)
				}
			}
			assert.Truef(t, found, "a panel must read %s", want.series)
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
		names := regexp.MustCompile(`\b(legnaris_[a-z_]+|rabbitmq_[a-z_]+)\b`)
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

	t.Run("Edge: the dashboard carries no credential", func(t *testing.T) {
		raw, err := os.ReadFile(f1aDashboardFile)
		require.NoError(t, err)
		text := strings.ToLower(string(raw))
		for _, word := range []string{"password", "secret", "token", "api_key", "apikey", "bearer"} {
			assert.NotContainsf(t, text, word, "%s must not carry %q", f1aDashboardFile, word)
		}
	})
}
