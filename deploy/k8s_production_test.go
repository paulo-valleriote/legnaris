package deploy_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const productionOverlay = "k8s/overlays/production"

func readText(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(raw)
}

func TestK8sProductionImagesArePinnedToImmutableTags(t *testing.T) {
	kustomization := readText(t, productionOverlay+"/kustomization.yaml")
	tags := regexp.MustCompile(`newTag: (\S+)`).FindAllStringSubmatch(kustomization, -1)

	t.Run("Happy path: app, web and rabbitmq each carry a commit-SHA tag", func(t *testing.T) {
		require.Len(t, tags, 3)
		for _, tag := range tags {
			assert.Regexp(t, `^sha-[0-9a-f]{40}$`, tag[1])
		}
	})

	t.Run("Edge: no local or floating tag reaches production", func(t *testing.T) {
		assert.NotContains(t, kustomization, ":local")
		assert.NotContains(t, kustomization, "newTag: latest")
		assert.NotContains(t, kustomization, "newTag: local")
	})

	t.Run("Happy path: the bump workflow rewrites exactly the tags the overlay uses", func(t *testing.T) {
		workflow := readText(t, "../.github/workflows/gitops-bump.yml")
		assert.Contains(t, workflow, "newTag: sha-[0-9a-f]{40}")
		assert.Contains(t, workflow, productionOverlay+"/kustomization.yaml")
	})
}

func TestK8sProductionServesOverTLSWithBrowserPolicy(t *testing.T) {
	kustomization := readText(t, productionOverlay+"/kustomization.yaml")
	middlewares := readText(t, productionOverlay+"/middlewares.yaml")

	t.Run("Happy path: api and app hosts get Let's Encrypt certificates", func(t *testing.T) {
		for _, host := range []string{"api.legnaris.com.br", "app.legnaris.com.br"} {
			assert.Contains(t, kustomization, host)
		}
		assert.Contains(t, kustomization, "cert-manager.io/cluster-issuer: letsencrypt-prod")
		assert.Contains(t, kustomization, "router.tls")
	})

	t.Run("Happy path: the web router applies the CSP middleware", func(t *testing.T) {
		assert.Contains(t, kustomization, "default-web-security-headers@kubernetescrd")
		assert.Contains(t, middlewares, "name: web-security-headers")
	})

	t.Run("Edge: the CSP allows the configured API origin, which is also the web client's API", func(t *testing.T) {
		assert.Contains(t, middlewares, "connect-src 'self' https://api.legnaris.com.br")
		assert.Contains(t, kustomization, "API_BASE_URL=https://api.legnaris.com.br")
		assert.Contains(t, kustomization, "CORS_ALLOWED_ORIGINS=https://app.legnaris.com.br")
	})

	t.Run("Edge: the ClusterIssuer targets the production ACME endpoint", func(t *testing.T) {
		assert.Contains(t, readText(t, "k8s/platform/clusterissuer.yaml"),
			"https://acme-v02.api.letsencrypt.org/directory")
	})
}

func TestK8sProductionBackup(t *testing.T) {
	backup := readText(t, productionOverlay+"/backup.yaml")

	t.Run("Happy path: a scheduled CronJob dumps the database and uploads it", func(t *testing.T) {
		assert.Contains(t, backup, "kind: CronJob")
		assert.Contains(t, backup, "mongodump")
		assert.Contains(t, backup, "aws s3 cp")
		assert.Regexp(t, `schedule: "[^"]+"`, backup)
	})

	t.Run("Edge: overlapping runs are forbidden and credentials come from a Secret", func(t *testing.T) {
		assert.Contains(t, backup, "concurrencyPolicy: Forbid")
		assert.Contains(t, backup, "name: backup-s3")
		assert.NotRegexp(t, `AWS_SECRET_ACCESS_KEY\s*[:=]\s*\S`, backup)
	})

	t.Run("Happy path: a restore check targets a scratch database, never production", func(t *testing.T) {
		restore := readText(t, "k8s/backup/restore-verify.yaml")
		assert.Contains(t, restore, "--nsTo='legnaris_restore_check.*'")
		assert.Contains(t, restore, "dropDatabase")
		assert.NotContains(t, restore, "--drop")
	})
}

func TestK8sGitOpsApplications(t *testing.T) {
	applications := readText(t, "k8s/gitops/applications.yaml")

	t.Run("Happy path: Argo CD syncs automatically, prunes and reverts drift", func(t *testing.T) {
		assert.Equal(t, 2, strings.Count(applications, "kind: Application"))
		assert.Equal(t, 2, strings.Count(applications, "selfHeal: true"))
		assert.Equal(t, 2, strings.Count(applications, "prune: true"))
		assert.Contains(t, applications, "targetRevision: main")
	})

	t.Run("Edge: every Application path exists in the repository", func(t *testing.T) {
		for _, match := range regexp.MustCompile(`path: (deploy/\S+)`).FindAllStringSubmatch(applications, -1) {
			_, err := os.Stat("../" + match[1] + "/kustomization.yaml")
			assert.NoError(t, err, match[1])
		}
	})

	t.Run("Edge: Argo CD is allowed to load the shared files outside each overlay", func(t *testing.T) {
		assert.Contains(t, readText(t, "k8s/gitops/argocd-cm-patch.yaml"), "LoadRestrictionsNone")
	})
}

func TestK8sProductionSecretsNeverCommitted(t *testing.T) {
	t.Run("Edge: secret sources are gitignored and no plain Secret manifest is committed", func(t *testing.T) {
		gitignore := readText(t, "../.gitignore")
		assert.Contains(t, gitignore, "deploy/k8s/overlays/production/*.env")
		assert.Contains(t, gitignore, "deploy/k8s/overlays/production-ops/*.env")

		for _, dir := range []string{productionOverlay, "k8s/overlays/production-ops"} {
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".env") {
					continue
				}
				content := readText(t, dir+"/"+entry.Name())
				assert.NotRegexp(t, `(?m)^kind: Secret$`, content, "%s/%s", dir, entry.Name())
			}
		}
	})

	t.Run("Happy path: the seal script covers every secret the workloads read", func(t *testing.T) {
		script := readText(t, "../scripts/k8s-seal-secrets.sh")
		for _, name := range []string{"legnaris-secrets", "backup-s3", "oauth2-proxy", "grafana-admin"} {
			assert.Contains(t, script, name)
		}
	})
}

type productionKustomization struct {
	Patches []struct {
		Target struct {
			Kind string `yaml:"kind"`
			Name string `yaml:"name"`
		} `yaml:"target"`
		Patch string `yaml:"patch"`
	} `yaml:"patches"`
}

type workerPatch struct {
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Name string `yaml:"name"`
					Env  []struct {
						Name      string `yaml:"name"`
						Value     string `yaml:"value"`
						ValueFrom struct {
							SecretKeyRef struct {
								Name string `yaml:"name"`
								Key  string `yaml:"key"`
							} `yaml:"secretKeyRef"`
						} `yaml:"valueFrom"`
					} `yaml:"env"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

func TestK8sProductionWorkerSendsEmailWithResend(t *testing.T) {
	var kustomization productionKustomization
	require.NoError(t, yaml.Unmarshal([]byte(readText(t, productionOverlay+"/kustomization.yaml")), &kustomization))

	var patch string
	for _, p := range kustomization.Patches {
		if p.Target.Kind == "Deployment" && p.Target.Name == "worker" {
			patch = p.Patch
		}
	}
	require.NotEmpty(t, patch, "production overlay must patch the worker Deployment")

	var rendered workerPatch
	require.NoError(t, yaml.Unmarshal([]byte(patch), &rendered))
	require.Len(t, rendered.Spec.Template.Spec.Containers, 1)
	env := map[string]struct{ value, secretName, secretKey string }{}
	for _, e := range rendered.Spec.Template.Spec.Containers[0].Env {
		env[e.Name] = struct{ value, secretName, secretKey string }{
			e.Value, e.ValueFrom.SecretKeyRef.Name, e.ValueFrom.SecretKeyRef.Key,
		}
	}

	t.Run("Happy path: the worker selects the Resend adapter with the production sender", func(t *testing.T) {
		assert.Equal(t, "resend", env["EMAIL_PROVIDER"].value)
		assert.Equal(t, "no-reply@mail.legnaris.com.br", env["EMAIL_FROM"].value)
	})

	t.Run("Edge: the API key is read from the sealed secret and never embedded as a value", func(t *testing.T) {
		key, ok := env["RESEND_API_KEY"]
		require.True(t, ok, "RESEND_API_KEY must be defined")
		assert.Empty(t, key.value)
		assert.Equal(t, "legnaris-secrets", key.secretName)
		assert.Equal(t, "RESEND_API_KEY", key.secretKey)
	})

	t.Run("Edge: the base worker keeps the log provider so local clusters need no Resend key", func(t *testing.T) {
		raw := readText(t, "k8s/base/worker.yaml")
		assert.Contains(t, raw, "EMAIL_PROVIDER\n              value: log")
		assert.NotContains(t, raw, "RESEND_API_KEY")
	})
}
