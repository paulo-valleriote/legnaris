package deploy_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	rulesFile         = "prometheus/rules.yml"
	dashboardFile     = "grafana/dashboards/email-pipeline.json"
	dashboardsDir     = "grafana/dashboards"
	runbookHeading    = "## Pipeline alerts and runbook"
	rulesMountInStack = "./prometheus/rules.yml:/etc/prometheus/rules.yml:ro"
	perObjectConf     = "prometheus.return_per_object_metrics = true"
	thresholdPrefix   = "legnaris:email_pipeline:threshold_"
	workQueueSelector = `queue="email.otp"`
	deadQueueSelector = `queue="email.dead"`
)

type rulesDoc struct {
	Groups []ruleGroup `yaml:"groups"`
}

type ruleGroup struct {
	Name  string     `yaml:"name"`
	Rules []ruleSpec `yaml:"rules"`
}

type ruleSpec struct {
	Alert       string            `yaml:"alert"`
	Record      string            `yaml:"record"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

func readRules(t *testing.T) rulesDoc {
	t.Helper()

	raw, err := os.ReadFile(rulesFile)
	require.NoErrorf(t, err, "read %s", rulesFile)

	var doc rulesDoc
	require.NoErrorf(t, yaml.Unmarshal(raw, &doc), "parse %s", rulesFile)
	return doc
}

// allRules flattens every rule of the file, in order.
func allRules(t *testing.T) []ruleSpec {
	t.Helper()

	var rules []ruleSpec
	for _, group := range readRules(t).Groups {
		rules = append(rules, group.Rules...)
	}
	return rules
}

// alertsByName returns every alert rule keyed by name and fails on a duplicate.
func alertsByName(t *testing.T) map[string]ruleSpec {
	t.Helper()

	alerts := map[string]ruleSpec{}
	for _, rule := range allRules(t) {
		if rule.Alert == "" {
			continue
		}
		_, duplicate := alerts[rule.Alert]
		require.Falsef(t, duplicate, "alert %s is defined twice", rule.Alert)
		alerts[rule.Alert] = rule
	}
	return alerts
}

// recordedThresholds returns the recording rules that carry the tunable limits.
func recordedThresholds(t *testing.T) map[string]ruleSpec {
	t.Helper()

	thresholds := map[string]ruleSpec{}
	for _, rule := range allRules(t) {
		if strings.HasPrefix(rule.Record, thresholdPrefix) {
			thresholds[rule.Record] = rule
		}
	}
	return thresholds
}

// expectedAlert is the contract of one alarm: its severity and the fragments
// its expression must carry to be reading the right fact from the right process.
type expectedAlert struct {
	name     string
	severity string
	series   []string
	reads    []string
	req      string
}

var pipelineAlerts = []expectedAlert{
	{
		name: "EmailDeadLetterQueueHoldsMessages", severity: "critical", req: "OBS-01",
		series: []string{"rabbitmq_queue_messages_ready"},
		reads:  []string{deadQueueSelector, `vhost="legnaris"`},
	},
	{
		name: "EmailWorkQueueHasNoConsumer", severity: "critical", req: "OBS-02",
		series: []string{"rabbitmq_queue_consumers"},
		reads:  []string{workQueueSelector, `vhost="legnaris"`},
	},
	{
		name: "EmailOldestMessageTooOld", severity: "high", req: "OBS-03",
		series: []string{"rabbitmq_queue_messages_ready", "rabbitmq_queue_consumers"},
		reads:  []string{workQueueSelector, `vhost="legnaris"`, "min_over_time("},
	},
	{
		name: "EmailPublicationBacklogTooLarge", severity: "high", req: "OBS-04",
		series: []string{"legnaris_email_outbox_backlog"},
		reads:  []string{`job="legnaris-relay"`},
	},
	{
		name: "EmailPublicationFailing", severity: "high", req: "OBS-05",
		series: []string{"legnaris_email_publish_total"},
		reads:  []string{`job="legnaris-relay"`},
	},
	{
		name: "EmailProviderRefusingSends", severity: "high", req: "OBS-06",
		series: []string{"legnaris_email_send_total"},
		reads:  []string{`job="legnaris-worker"`, `result="retried"`},
	},
	{
		name: "EmailDailyAllowanceReached", severity: "medium", req: "OBS-07",
		series: []string{"legnaris_auth_otp_cap_blocked_total"},
		reads:  []string{`job="legnaris-app"`},
	},
	{
		name: "EmailVerificationFailuresClimbing", severity: "medium", req: "OBS-08",
		series: []string{"legnaris_auth_otp_verify_total"},
		reads:  []string{`job="legnaris-app"`, `result!="verified"`},
	},
	{
		name: "EmailPipelineSeriesMissing", severity: "high", req: "OBS-16",
		series: []string{"rabbitmq_queue_consumers", "legnaris_email_outbox_backlog", "legnaris_email_dead_total", "up{"},
		reads:  []string{"absent(", `legnaris-app`},
	},
	{
		name: "EmailRegisterBurstTooLarge", severity: "medium", req: "LGN-159",
		series: []string{"legnaris_http_requests_total"},
		reads:  []string{`job="legnaris-app"`, `method="POST"`, `path="/auth/register"`, `status="429"`, "[24h]"},
	},
	{
		name: "EmailCapReachedEarly", severity: "medium", req: "LGN-159",
		series: []string{"legnaris_auth_otp_cap_blocked_total"},
		reads:  []string{`job="legnaris-app"`, "count_over_time(", "offset 12h", "[7d:1d]"},
	},
}

// TestAlertRulesExistAndAreComplete guards the alarm set: every way the
// pipeline stalls has its own named rule, and every rule carries a severity, a
// patience period and the words that say what broke (OBS-01…OBS-08, OBS-17,
// SC-001, SC-002).
func TestAlertRulesExistAndAreComplete(t *testing.T) {
	alerts := alertsByName(t)

	for _, want := range pipelineAlerts {
		t.Run("Happy path: "+want.name+" exists with its severity ("+want.req+")", func(t *testing.T) {
			rule, present := alerts[want.name]
			require.Truef(t, present, "the alert %s must exist", want.name)
			assert.Equal(t, want.severity, rule.Labels["severity"])
		})
	}

	t.Run("Edge: every alert carries a patience period, a summary and a runbook link", func(t *testing.T) {
		require.NotEmpty(t, alerts)
		for name, rule := range alerts {
			assert.NotEmptyf(t, rule.For, "alert %s must state its patience period", name)
			assert.NotEmptyf(t, rule.Annotations["summary"], "alert %s must say what broke", name)
			assert.NotEmptyf(t, rule.Annotations["description"], "alert %s must describe the failure", name)
			assert.Equalf(t, "deploy/README.md#"+strings.ToLower(name), rule.Annotations["runbook"], "alert %s must link its runbook entry", name)
		}
	})

	t.Run("Edge: every severity is one the runbook defines", func(t *testing.T) {
		for name, rule := range alerts {
			assert.Containsf(t, []string{"critical", "high", "medium"}, rule.Labels["severity"], "alert %s", name)
		}
	})

	t.Run("Edge: an alert name is a descriptive Email* identifier, not a bare threshold", func(t *testing.T) {
		nameShape := regexp.MustCompile(`^Email[A-Z][a-z]+([A-Z][a-z]+)+$`)
		for name := range alerts {
			assert.Regexpf(t, nameShape, name, "alert %s must name what broke in at least two words", name)
		}
	})

	t.Run("Edge: every alert has an expression", func(t *testing.T) {
		for name, rule := range alerts {
			assert.NotEmptyf(t, strings.TrimSpace(rule.Expr), "alert %s", name)
		}
	})

	t.Run("Edge: no pipeline alarm is missing from the contract table", func(t *testing.T) {
		known := map[string]bool{}
		for _, want := range pipelineAlerts {
			known[want.name] = true
		}
		for name := range alerts {
			assert.Truef(t, known[name], "alert %s must be declared in the contract table", name)
		}
	})
}

// TestAlertRulesNameTheirSeries guards each expression: it names the series
// that carry the fact, the queue it is about and the process that owns the
// counter, so the site's zero is never read for the publisher's truth (OBS-16,
// SC-003).
func TestAlertRulesNameTheirSeries(t *testing.T) {
	alerts := alertsByName(t)

	for _, want := range pipelineAlerts {
		t.Run("Happy path: "+want.name+" names its series and its source", func(t *testing.T) {
			rule, present := alerts[want.name]
			require.Truef(t, present, "the alert %s must exist", want.name)
			for _, fragment := range append(append([]string{}, want.series...), want.reads...) {
				assert.Containsf(t, rule.Expr, fragment, "alert %s must read %s", want.name, fragment)
			}
		})
	}

	t.Run("Edge: the backlog and publication rules never read the site's copy of the counter", func(t *testing.T) {
		for _, name := range []string{"EmailPublicationBacklogTooLarge", "EmailPublicationFailing"} {
			rule, present := alerts[name]
			require.Truef(t, present, "the alert %s must exist", name)
			assert.NotContains(t, rule.Expr, `job="legnaris-app"`)
			assert.NotContains(t, rule.Expr, `job=~`)
		}
	})

	t.Run("Edge: every legnaris counter is read with a job selector", func(t *testing.T) {
		for name, rule := range alerts {
			if strings.Contains(rule.Expr, "legnaris_email_") || strings.Contains(rule.Expr, "legnaris_auth_") {
				assert.Containsf(t, rule.Expr, "job=", "alert %s reads a counter two processes expose", name)
			}
		}
	})

	t.Run("Edge: the broker rules read the delivery vhost only", func(t *testing.T) {
		for name, rule := range alerts {
			if strings.Contains(rule.Expr, "rabbitmq_") {
				assert.Containsf(t, rule.Expr, `vhost="legnaris"`, "alert %s", name)
			}
		}
	})
}

// TestAlertRuleThresholdsAreConfigurable guards the tunable limits: the age
// patience, the backlog and the failure count live in one recorded block an
// operator edits, and no expression hides a number of its own (OBS-03, OBS-04,
// SC-003).
func TestAlertRuleThresholdsAreConfigurable(t *testing.T) {
	alerts := alertsByName(t)
	thresholds := recordedThresholds(t)

	tunable := map[string]string{
		"EmailPublicationBacklogTooLarge":   thresholdPrefix + "backlog_pending",
		"EmailVerificationFailuresClimbing": thresholdPrefix + "verification_failures",
		"EmailRegisterBurstTooLarge":        thresholdPrefix + "register_429s",
		"EmailCapReachedEarly":              thresholdPrefix + "early_cap_days",
	}

	for alert, record := range tunable {
		t.Run("Happy path: "+alert+" reads "+record, func(t *testing.T) {
			rule, present := alerts[alert]
			require.Truef(t, present, "the alert %s must exist", alert)
			require.Containsf(t, thresholds, record, "the threshold %s must be recorded", record)
			assert.Contains(t, rule.Expr, record)
		})
	}

	t.Run("Edge: every threshold is a constant vector so it can be tuned in one place", func(t *testing.T) {
		require.Len(t, thresholds, len(tunable))
		for record, rule := range thresholds {
			assert.Regexpf(t, `^vector\([0-9]+(\.[0-9]+)?\)$`, strings.TrimSpace(rule.Expr), "threshold %s", record)
		}
	})

	t.Run("Edge: no comparison in an alert hides a number other than zero", func(t *testing.T) {
		hidden := regexp.MustCompile(`[<>]=?\s*[1-9][0-9]*`)
		for name, rule := range alerts {
			assert.NotRegexpf(t, hidden, rule.Expr, "alert %s hard-codes a threshold", name)
		}
	})

	t.Run("Edge: the work queue rule needs a consumer and a queue that stayed non-empty, so a quiet pipeline never fires it", func(t *testing.T) {
		age := alerts["EmailOldestMessageTooOld"]
		assert.Contains(t, age.Expr, "min_over_time(", "an empty queue at any point in the window must clear the rule")
		assert.Contains(t, age.Expr, "and on(", "a missing consumer is the consumer rule's fact, not this one's")
		assert.NotEmpty(t, age.For, "the patience is the rule's own period")
	})
}

// TestMonitoringEvaluatesTheRules guards the wiring: the stack loads the rules
// file, mounts it, and the broker exposes the per-object series the queue rules
// name (OBS-02, OBS-03, OBS-09, OBS-19).
func TestMonitoringEvaluatesTheRules(t *testing.T) {
	raw, err := os.ReadFile(prometheusFile)
	require.NoError(t, err)
	var doc struct {
		RuleFiles     []string `yaml:"rule_files"`
		ScrapeConfigs []struct {
			JobName       string `yaml:"job_name"`
			MetricsPath   string `yaml:"metrics_path"`
			StaticConfigs []struct {
				Targets []string `yaml:"targets"`
			} `yaml:"static_configs"`
		} `yaml:"scrape_configs"`
		Alerting any `yaml:"alerting"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &doc))

	t.Run("Happy path: the stack loads the rules file", func(t *testing.T) {
		assert.Equal(t, []string{"/etc/prometheus/rules.yml"}, doc.RuleFiles)
	})

	t.Run("Happy path: the rules file is mounted read-only into the stack", func(t *testing.T) {
		assert.Contains(t, readCompose(t).Services["prometheus"].Volumes, rulesMountInStack)
	})

	t.Run("Edge: no rule reads the head-message timestamp, which the broker reports for classic queues only", func(t *testing.T) {
		for name, rule := range alertsByName(t) {
			assert.NotContainsf(t, rule.Expr, "head_message_timestamp", "alert %s: the work queue is a quorum queue", name)
		}
		for _, job := range doc.ScrapeConfigs {
			assert.NotEqual(t, "/metrics/detailed", job.MetricsPath, "the detailed scrape has nothing the alerts read")
		}
	})

	t.Run("Happy path: the broker serves per-object metrics", func(t *testing.T) {
		conf, err := os.ReadFile(brokerConfigFile)
		require.NoError(t, err)
		assert.Contains(t, string(conf), perObjectConf)
	})

	t.Run("Edge: no notification service is added to the stack (OBS-09)", func(t *testing.T) {
		assert.Nil(t, doc.Alerting, "the stack must not configure an alert manager")
		for name := range readCompose(t).Services {
			assert.NotContainsf(t, name, "alertmanager", "service %s", name)
		}
	})
}

// TestAlertRulesPassThePrometheusValidator runs the stack's own validator when
// it is installed, so an expression that cannot parse fails the build rather
// than the night (OBS-19, SC-007).
func TestAlertRulesPassThePrometheusValidator(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		t.Skip("promtool is not installed; run the quickstart's containerised check instead")
	}

	out, err := exec.Command(promtool, "check", "rules", rulesFile).CombinedOutput()
	assert.NoErrorf(t, err, "promtool rejected the rules:\n%s", out)
}

// TestObservabilityFilesCarryNoCredential guards the rules and the dashboard:
// neither names a secret, a token or a key (OBS-21, SC-006).
func TestObservabilityFilesCarryNoCredential(t *testing.T) {
	for _, file := range []string{rulesFile, dashboardFile} {
		t.Run("Edge: "+file+" carries no credential", func(t *testing.T) {
			raw, err := os.ReadFile(file)
			require.NoError(t, err)
			text := strings.ToLower(string(raw))
			for _, word := range []string{"password", "secret", "token", "api_key", "apikey", "bearer"} {
				assert.NotContainsf(t, text, word, "%s must not carry %q", file, word)
			}
		})
	}
}

type dashboardDoc struct {
	UID    string           `json:"uid"`
	Title  string           `json:"title"`
	Panels []dashboardPanel `json:"panels"`
}

type dashboardPanel struct {
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
}

func readDashboard(t *testing.T) dashboardDoc {
	t.Helper()

	raw, err := os.ReadFile(dashboardFile)
	require.NoErrorf(t, err, "read %s", dashboardFile)

	var doc dashboardDoc
	require.NoErrorf(t, json.Unmarshal(raw, &doc), "parse %s", dashboardFile)
	return doc
}

var seriesName = regexp.MustCompile(`\b(legnaris_[a-z_]+|rabbitmq_[a-z_]+|ALERTS)\b`)

// TestDashboardCoversThePipeline guards the dashboard: one provisioned document
// that tells every stage of the pipeline and whose every panel names the series
// it reads (OBS-10, OBS-11, OBS-12, OBS-15, SC-005).
func TestDashboardCoversThePipeline(t *testing.T) {
	dashboard := readDashboard(t)

	t.Run("Happy path: the dashboard is identified and provisioned from the repository", func(t *testing.T) {
		assert.Equal(t, "email-pipeline", dashboard.UID)
		assert.Equal(t, "Email pipeline", dashboard.Title)
		entries, err := filepath.Glob(filepath.Join(dashboardsDir, "*.json"))
		require.NoError(t, err)
		assert.Contains(t, entries, dashboardFile, "the pipeline dashboard is provisioned from the repository")
	})

	stages := []struct {
		name   string
		series string
		req    string
	}{
		{name: "codes requested", series: "legnaris_auth_otp_issued_total", req: "OBS-11"},
		{name: "codes verified", series: "legnaris_auth_otp_verify_total", req: "OBS-11"},
		{name: "backlog of unpublished codes", series: "legnaris_email_outbox_backlog", req: "OBS-11"},
		{name: "publication outcomes", series: "legnaris_email_publish_total", req: "OBS-11"},
		{name: "delivery outcomes", series: "legnaris_email_send_total", req: "OBS-11"},
		{name: "dead-lettered count", series: "legnaris_email_dead_total", req: "OBS-11"},
		{name: "expired count", series: "legnaris_email_expired_total", req: "OBS-11"},
		{name: "queue depth", series: "rabbitmq_queue_messages_ready", req: "OBS-12"},
		{name: "consumers", series: "rabbitmq_queue_consumers", req: "OBS-12"},
		{name: "work queue not draining", series: "min_over_time(rabbitmq_queue_messages_ready", req: "OBS-12"},
		{name: "provider answer time", series: "legnaris_email_send_duration_seconds", req: "OBS-12"},
		{name: "daily allowance", series: "legnaris_auth_otp_cap_blocked_total", req: "OBS-12"},
	}
	for _, stage := range stages {
		t.Run("Happy path: a panel shows the "+stage.name+" ("+stage.req+")", func(t *testing.T) {
			found := false
			for _, panel := range dashboard.Panels {
				for _, target := range panel.Targets {
					found = found || strings.Contains(target.Expr, stage.series)
				}
			}
			assert.Truef(t, found, "a panel must read %s", stage.series)
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
		for _, panel := range dashboard.Panels {
			require.NotEmptyf(t, panel.Targets, "panel %q must carry a query", panel.Title)
			for _, target := range panel.Targets {
				names := seriesName.FindAllString(target.Expr, -1)
				require.NotEmptyf(t, names, "panel %q query %q must name a series", panel.Title, target.Expr)
				for _, name := range names {
					assert.Containsf(t, panel.Description, name, "panel %q must say it reads %s", panel.Title, name)
				}
			}
			assert.Truef(t, strings.HasPrefix(panel.Description, "Reads "), "panel %q description must start with the series it reads", panel.Title)
		}
	})

	t.Run("Edge: the panels read the process that owns the counter", func(t *testing.T) {
		for _, panel := range dashboard.Panels {
			for _, target := range panel.Targets {
				if strings.Contains(target.Expr, "legnaris_email_") || strings.Contains(target.Expr, "legnaris_auth_") {
					assert.Containsf(t, target.Expr, "job=", "panel %q reads a counter two processes expose", panel.Title)
				}
			}
		}
	})
}

// TestRunbookAnswersEveryAlarm guards the runbook: one entry per rule saying
// what it means, what to do and when to escalate, plus the provider's
// allowance, the notification channel and the counter ownership (OBS-13,
// OBS-14, OBS-16, OBS-18, SC-004, SC-008).
func TestRunbookAnswersEveryAlarm(t *testing.T) {
	section := runbookSection(t, runbookHeading)
	alerts := alertsByName(t)

	entry := func(name string) string {
		heading := "### " + name
		start := strings.Index(section, heading)
		if start < 0 {
			return ""
		}
		body := section[start+len(heading):]
		if end := strings.Index(body, "\n### "); end >= 0 {
			body = body[:end]
		}
		return body
	}

	for name := range alerts {
		t.Run("Happy path: "+name+" says what it means, what to do and when to escalate", func(t *testing.T) {
			body := entry(name)
			require.NotEmptyf(t, body, "the runbook must have an entry for %s", name)
			for _, field := range []string{"**Means:**", "**Do:**", "**Escalate when:**"} {
				assert.Containsf(t, body, field, "the entry for %s", name)
			}
		})
	}

	t.Run("Edge: every field of every entry says something", func(t *testing.T) {
		for name := range alerts {
			for _, field := range []string{"**Means:**", "**Do:**", "**Escalate when:**"} {
				_, after, found := strings.Cut(entry(name), field)
				require.Truef(t, found, "the entry for %s must carry %s", name, field)
				line, _, _ := strings.Cut(strings.TrimLeft(after, " "), "\n")
				assert.GreaterOrEqualf(t, len(strings.TrimSpace(line)), 20, "the %s field of %s is empty or a stub", field, name)
			}
		}
	})

	t.Run("Edge: no entry describes an alarm that does not exist", func(t *testing.T) {
		for _, line := range strings.Split(section, "\n") {
			if name, found := strings.CutPrefix(line, "### Email"); found {
				assert.Containsf(t, alerts, "Email"+strings.TrimSpace(name), "the runbook describes an unknown alarm: %s", line)
			}
		}
	})

	t.Run("Happy path: the allowance, what happens when it is spent and the trigger for buying more", func(t *testing.T) {
		body := entry("The provider's daily allowance")
		require.NotEmpty(t, body)
		for _, fragment := range []string{"100", "80", "AUTH_OTP_DAILY_SEND_CAP", "EMAIL_UNAVAILABLE", "60", "3 days", "public launch", "campaign"} {
			assert.Containsf(t, body, fragment, "the allowance entry must carry %q", fragment)
		}
	})

	t.Run("Happy path: the notification channel is documented and absent by default", func(t *testing.T) {
		body := entry("Notification channel")
		require.NotEmpty(t, body)
		assert.Contains(t, body, "No notification channel is configured by default")
		assert.Contains(t, body, "Alertmanager")
	})

	t.Run("Happy path: the runbook says which process owns which counter", func(t *testing.T) {
		body := entry("Which process owns which counter")
		require.NotEmpty(t, body)
		for _, fragment := range []string{"legnaris-relay", "legnaris-worker", "legnaris-app", "legnaris_email_outbox_backlog", "legnaris_email_send_total", "zero"} {
			assert.Containsf(t, body, fragment, "the ownership entry must carry %q", fragment)
		}
	})

	t.Run("Edge: the broker series names are recorded with what was and was not verified", func(t *testing.T) {
		body := entry("Broker series")
		require.NotEmpty(t, body)
		for _, fragment := range []string{"rabbitmq_queue_messages_ready", "rabbitmq_queue_consumers", "quorum", "4.4.0"} {
			assert.Containsf(t, body, fragment, "the broker series entry must carry %q", fragment)
		}
	})

	t.Run("Edge: the runbook and the rules are written in English", func(t *testing.T) {
		portuguese := regexp.MustCompile(`(?i)\b(não|fila de|mensagem|alerta|então)\b`)
		assert.NotRegexp(t, portuguese, section)
		for name, rule := range alerts {
			assert.NotRegexpf(t, portuguese, rule.Annotations["summary"]+rule.Annotations["description"], "alert %s", name)
		}
	})
}
