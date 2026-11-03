package deploy_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type hpaDoc struct {
	Spec struct {
		ScaleTargetRef struct {
			Name string `yaml:"name"`
		} `yaml:"scaleTargetRef"`
		MinReplicas int `yaml:"minReplicas"`
		MaxReplicas int `yaml:"maxReplicas"`
		Metrics     []struct {
			Resource struct {
				Name   string `yaml:"name"`
				Target struct {
					AverageUtilization int `yaml:"averageUtilization"`
				} `yaml:"target"`
			} `yaml:"resource"`
		} `yaml:"metrics"`
		Behavior struct {
			ScaleDown struct {
				StabilizationWindowSeconds int `yaml:"stabilizationWindowSeconds"`
			} `yaml:"scaleDown"`
		} `yaml:"behavior"`
	} `yaml:"spec"`
}

func TestK8sAppHPA(t *testing.T) {
	var hpa hpaDoc
	readYAMLFile(t, "k8s/base/hpa.yaml", &hpa)

	t.Run("Happy path: scales the app on CPU utilisation", func(t *testing.T) {
		assert.Equal(t, "app", hpa.Spec.ScaleTargetRef.Name)
		require.Len(t, hpa.Spec.Metrics, 1)
		assert.Equal(t, "cpu", hpa.Spec.Metrics[0].Resource.Name)
		assert.Positive(t, hpa.Spec.Metrics[0].Resource.Target.AverageUtilization)
		assert.LessOrEqual(t, hpa.Spec.Metrics[0].Resource.Target.AverageUtilization, 100)
	})

	t.Run("Edge: never drops below two replicas and can grow", func(t *testing.T) {
		assert.GreaterOrEqual(t, hpa.Spec.MinReplicas, 2)
		assert.Greater(t, hpa.Spec.MaxReplicas, hpa.Spec.MinReplicas)
	})

	t.Run("Edge: scale-down is damped by a stabilisation window", func(t *testing.T) {
		assert.Positive(t, hpa.Spec.Behavior.ScaleDown.StabilizationWindowSeconds)
	})
}

type scaledObjectDoc struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		ScaleTargetRef struct {
			Name string `yaml:"name"`
		} `yaml:"scaleTargetRef"`
		MinReplicaCount int `yaml:"minReplicaCount"`
		MaxReplicaCount int `yaml:"maxReplicaCount"`
		Triggers        []struct {
			Type     string            `yaml:"type"`
			Metadata map[string]string `yaml:"metadata"`
		} `yaml:"triggers"`
	} `yaml:"spec"`
}

func TestK8sKedaScaledObjects(t *testing.T) {
	raw, err := os.ReadFile("k8s/keda/scaledobjects.yaml")
	require.NoError(t, err)

	wantSeries := map[string]string{
		"worker": `rabbitmq_queue_messages_ready{job="legnaris-rabbitmq"`,
		"relay":  `legnaris_email_outbox_backlog{job="legnaris-relay"`,
	}
	found := map[string]bool{}
	for _, part := range strings.Split(string(raw), "\n---\n") {
		var object scaledObjectDoc
		readYAMLString(t, part, &object)
		name := object.Metadata.Name
		series, known := wantSeries[name]
		require.True(t, known, "unexpected ScaledObject %q", name)
		found[name] = true

		t.Run("Happy path: "+name+" scales on its Prometheus series", func(t *testing.T) {
			assert.Equal(t, name, object.Spec.ScaleTargetRef.Name)
			require.Len(t, object.Spec.Triggers, 1)
			assert.Equal(t, "prometheus", object.Spec.Triggers[0].Type)
			assert.Contains(t, object.Spec.Triggers[0].Metadata["query"], series)
			assert.NotEmpty(t, object.Spec.Triggers[0].Metadata["threshold"])
		})

		t.Run("Edge: "+name+" keeps one replica so the no-consumer alert stays truthful", func(t *testing.T) {
			assert.GreaterOrEqual(t, object.Spec.MinReplicaCount, 1)
			assert.Greater(t, object.Spec.MaxReplicaCount, object.Spec.MinReplicaCount)
		})
	}
	assert.Len(t, found, len(wantSeries), "every pipeline workload needs a ScaledObject")
}

func TestK8sRateLimitsAreLiftedOnlyInTheLoadtestOverlay(t *testing.T) {
	read := func(path string) string {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(raw)
	}

	t.Run("Happy path: the loadtest overlay lifts the limits", func(t *testing.T) {
		assert.Contains(t, read("k8s/overlays/loadtest/kustomization.yaml"), "AUTH_RATE_LIMIT_ATTEMPTS=")
	})

	for _, path := range []string{"k8s/base/app.yaml", "k8s/overlays/local/kustomization.yaml"} {
		t.Run("Edge: "+path+" leaves the default limits in place", func(t *testing.T) {
			assert.NotContains(t, read(path), "RATE_LIMIT")
		})
	}
}
