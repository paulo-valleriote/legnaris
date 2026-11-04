package deploy_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const k8sBaseDir = "k8s/base"

type k8sObject struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec k8sSpec `yaml:"spec"`
}

type k8sSpec struct {
	Replicas *int `yaml:"replicas"`
	Strategy struct {
		RollingUpdate struct {
			MaxUnavailable int `yaml:"maxUnavailable"`
		} `yaml:"rollingUpdate"`
	} `yaml:"strategy"`
	Template struct {
		Spec struct {
			TerminationGracePeriodSeconds int            `yaml:"terminationGracePeriodSeconds"`
			Containers                    []k8sContainer `yaml:"containers"`
		} `yaml:"spec"`
	} `yaml:"template"`
}

type k8sContainer struct {
	Name           string    `yaml:"name"`
	Image          string    `yaml:"image"`
	Command        []string  `yaml:"command"`
	LivenessProbe  *k8sProbe `yaml:"livenessProbe"`
	ReadinessProbe *k8sProbe `yaml:"readinessProbe"`
	Resources      struct {
		Requests map[string]string `yaml:"requests"`
		Limits   map[string]string `yaml:"limits"`
	} `yaml:"resources"`
	Lifecycle struct {
		PreStop *struct {
			Sleep struct {
				Seconds int `yaml:"seconds"`
			} `yaml:"sleep"`
		} `yaml:"preStop"`
	} `yaml:"lifecycle"`
}

type k8sProbe struct {
	HTTPGet *struct {
		Path string `yaml:"path"`
	} `yaml:"httpGet"`
}

func readK8sObjects(t *testing.T) []k8sObject {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(k8sBaseDir, "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "the base must define manifests")

	var objects []k8sObject
	for _, file := range files {
		if filepath.Base(file) == "kustomization.yaml" {
			continue
		}
		raw, err := os.ReadFile(file)
		require.NoError(t, err)

		decoder := yaml.NewDecoder(bytes.NewReader(raw))
		for {
			var object k8sObject
			err := decoder.Decode(&object)
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoError(t, err, "%s must be valid YAML", file)
			objects = append(objects, object)
		}
	}
	return objects
}

func findK8s(t *testing.T, objects []k8sObject, kind, name string) k8sObject {
	t.Helper()
	for _, object := range objects {
		if object.Kind == kind && object.Metadata.Name == name {
			return object
		}
	}
	require.Failf(t, "manifest missing", "%s/%s not found in %s", kind, name, k8sBaseDir)
	return k8sObject{}
}

func TestK8sAppSurvivesPodLossAndRollouts(t *testing.T) {
	objects := readK8sObjects(t)
	app := findK8s(t, objects, "Deployment", "app")
	require.Len(t, app.Spec.Template.Spec.Containers, 1)
	container := app.Spec.Template.Spec.Containers[0]

	t.Run("Happy path: at least two replicas", func(t *testing.T) {
		require.NotNil(t, app.Spec.Replicas)
		assert.GreaterOrEqual(t, *app.Spec.Replicas, 2)
	})

	t.Run("Happy path: probes target /health and /ready", func(t *testing.T) {
		require.NotNil(t, container.LivenessProbe)
		require.NotNil(t, container.LivenessProbe.HTTPGet)
		assert.Equal(t, "/health", container.LivenessProbe.HTTPGet.Path)
		require.NotNil(t, container.ReadinessProbe)
		require.NotNil(t, container.ReadinessProbe.HTTPGet)
		assert.Equal(t, "/ready", container.ReadinessProbe.HTTPGet.Path)
	})

	t.Run("Edge: a rollout never removes a ready pod before its replacement", func(t *testing.T) {
		assert.Zero(t, app.Spec.Strategy.RollingUpdate.MaxUnavailable)
	})

	t.Run("Edge: the grace period covers preStop plus the drain and DB close", func(t *testing.T) {
		require.NotNil(t, container.Lifecycle.PreStop, "app needs a preStop delay")
		const shutdownAndCloseSeconds = 10
		assert.Greater(t, app.Spec.Template.Spec.TerminationGracePeriodSeconds,
			container.Lifecycle.PreStop.Sleep.Seconds+shutdownAndCloseSeconds)
	})
}

func TestK8sWorkloadsAreBounded(t *testing.T) {
	for _, object := range readK8sObjects(t) {
		if object.Kind != "Deployment" && object.Kind != "StatefulSet" {
			continue
		}
		for _, container := range object.Spec.Template.Spec.Containers {
			name := object.Metadata.Name + "/" + container.Name
			t.Run("Edge: "+name+" sets cpu and memory requests and limits", func(t *testing.T) {
				for _, resource := range []string{"cpu", "memory"} {
					assert.NotEmpty(t, container.Resources.Requests[resource], "%s request", resource)
					assert.NotEmpty(t, container.Resources.Limits[resource], "%s limit", resource)
				}
			})
		}
	}
}

func TestK8sRelayAndWorkerOverrideTheSharedImageCommand(t *testing.T) {
	objects := readK8sObjects(t)

	for name, binary := range map[string]string{"relay": "/relay", "worker": "/worker"} {
		t.Run("Happy path: "+name+" runs "+binary+" on two replicas", func(t *testing.T) {
			deployment := findK8s(t, objects, "Deployment", name)
			require.Len(t, deployment.Spec.Template.Spec.Containers, 1)
			assert.Equal(t, []string{binary}, deployment.Spec.Template.Spec.Containers[0].Command)
			require.NotNil(t, deployment.Spec.Replicas)
			assert.GreaterOrEqual(t, *deployment.Spec.Replicas, 2)
		})
	}
}

func TestK8sStatefulSetsPersistData(t *testing.T) {
	objects := readK8sObjects(t)
	for _, name := range []string{"mongodb", "rabbitmq"} {
		t.Run("Happy path: "+name+" is a StatefulSet", func(t *testing.T) {
			findK8s(t, objects, "StatefulSet", name)
		})
	}
}

func TestK8sNoSecretIsCommitted(t *testing.T) {
	t.Run("Edge: the base declares no Secret object", func(t *testing.T) {
		for _, object := range readK8sObjects(t) {
			assert.NotEqual(t, "Secret", object.Kind, "secrets come from the gitignored secrets.env")
		}
	})

	t.Run("Edge: the committed example holds only placeholders", func(t *testing.T) {
		raw, err := os.ReadFile("k8s/overlays/local/secrets.env.example")
		require.NoError(t, err)
		for _, line := range bytes.Split(raw, []byte("\n")) {
			if len(line) == 0 || line[0] == '#' {
				continue
			}
			assert.True(t, bytes.HasSuffix(line, []byte("=replace_me")), "%s", line)
		}
	})
}
