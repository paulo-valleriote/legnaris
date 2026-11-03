package deploy_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type ruleGroupsDoc struct {
	Groups []any `yaml:"groups"`
}

type prometheusRuleDoc struct {
	Kind string        `yaml:"kind"`
	Spec ruleGroupsDoc `yaml:"spec"`
}

type podMonitorDoc struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		NamespaceSelector struct {
			MatchNames []string `yaml:"matchNames"`
		} `yaml:"namespaceSelector"`
		Endpoints []struct {
			Relabelings []struct {
				TargetLabel string `yaml:"targetLabel"`
				Replacement string `yaml:"replacement"`
			} `yaml:"relabelings"`
		} `yaml:"podMetricsEndpoints"`
	} `yaml:"spec"`
}

func readYAMLFile(t *testing.T, path string, out any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(raw, out), "%s must be valid YAML", path)
}

func TestK8sPrometheusRuleMatchesRulesFile(t *testing.T) {
	var source ruleGroupsDoc
	readYAMLFile(t, "prometheus/rules.yml", &source)
	var generated prometheusRuleDoc
	readYAMLFile(t, "k8s/observability/prometheusrule.yaml", &generated)

	t.Run("Happy path: the PrometheusRule carries exactly the compose rule groups", func(t *testing.T) {
		assert.Equal(t, "PrometheusRule", generated.Kind)
		require.NotEmpty(t, source.Groups)
		assert.Equal(t, source.Groups, generated.Spec.Groups,
			"run `make k8s-rules` after editing deploy/prometheus/rules.yml")
	})
}

func TestK8sPodMonitorsKeepComposeJobNames(t *testing.T) {
	raw, err := os.ReadFile("k8s/observability/podmonitors.yaml")
	require.NoError(t, err)

	jobs := map[string]string{}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	for {
		var monitor podMonitorDoc
		err := decoder.Decode(&monitor)
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)

		t.Run("Edge: "+monitor.Metadata.Name+" looks in the workload namespace", func(t *testing.T) {
			assert.Equal(t, []string{"default"}, monitor.Spec.NamespaceSelector.MatchNames)
		})
		require.NotEmpty(t, monitor.Spec.Endpoints)
		for _, relabeling := range monitor.Spec.Endpoints[0].Relabelings {
			if relabeling.TargetLabel == "job" {
				jobs[monitor.Metadata.Name] = relabeling.Replacement
			}
		}
	}

	t.Run("Happy path: every scraped workload keeps the job the alert rules select on", func(t *testing.T) {
		assert.Equal(t, map[string]string{
			"legnaris-app":      "legnaris-app",
			"legnaris-relay":    "legnaris-relay",
			"legnaris-worker":   "legnaris-worker",
			"legnaris-rabbitmq": "legnaris-rabbitmq",
		}, jobs)
	})
}
