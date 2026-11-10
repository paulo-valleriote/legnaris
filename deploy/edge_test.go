package deploy_test

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const dockerSocketPath = "/var/run/docker.sock"

// requiredComposeVarPattern matches ${VAR}, ${VAR:?message} and ${VAR?message}
// (compose requires them) and captures the name. ${VAR:-default} and ${VAR-default}
// are optional and deliberately not matched.
var requiredComposeVarPattern = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)(:\?[^}]*|\?[^}]*)?\}`)

// commentedEnvPattern matches an example line documented only as a comment.
var commentedEnvPattern = regexp.MustCompile(`(?m)^#\s*([A-Z][A-Z0-9_]*)=`)

// TestEdgeProxyRunsWithoutDockerSocket guards the hardening that keeps a
// compromised proxy from driving the Docker API: Traefik routes only from the
// file provider, so the socket volume must never come back.
func TestEdgeProxyRunsWithoutDockerSocket(t *testing.T) {
	traefik := readCompose(t).Services["traefik"]
	require.NotEmpty(t, traefik.Image, "guard: the traefik service must exist")

	t.Run("Happy path: the proxy mounts no Docker socket", func(t *testing.T) {
		for _, volume := range traefik.Volumes {
			assert.NotContains(t, volume, dockerSocketPath, "the edge proxy must not mount the Docker socket")
		}
	})

	t.Run("Edge: the proxy enables no Docker provider", func(t *testing.T) {
		for _, arg := range traefik.Command {
			assert.False(t, strings.HasPrefix(arg, "--providers.docker"),
				"routing must come from the file provider only, found %q", arg)
		}
	})

	t.Run("Edge: no routing entrypoint variable remains in the proxy environment", func(t *testing.T) {
		_, present := traefik.Environment["TRAEFIK_ENTRYPOINT"]
		assert.False(t, present, "the secure entrypoint is hard-coded in dynamic.yml, not chosen by a variable")
	})
}

// TestDynamicRoutesUseSecureEntrypoint guards routing without entrypoint
// indirection: every router must enter through the TLS entrypoint literally.
func TestDynamicRoutesUseSecureEntrypoint(t *testing.T) {
	raw, err := os.ReadFile(dynamicFile)
	require.NoErrorf(t, err, "read %s", dynamicFile)

	var doc struct {
		HTTP struct {
			Routers map[string]struct {
				EntryPoints []string `yaml:"entryPoints"`
			} `yaml:"routers"`
		} `yaml:"http"`
	}
	require.NoErrorf(t, yaml.Unmarshal(raw, &doc), "parse %s", dynamicFile)
	require.NotEmpty(t, doc.HTTP.Routers, "dynamic.yml must define routers")

	t.Run("Error: no template variable selects the entrypoint", func(t *testing.T) {
		assert.NotContains(t, string(raw), "TRAEFIK_ENTRYPOINT")
	})

	t.Run("Happy path: every router uses only the websecure entrypoint", func(t *testing.T) {
		for name, router := range doc.HTTP.Routers {
			assert.Equalf(t, []string{"websecure"}, router.EntryPoints,
				"router %q must enter through the TLS entrypoint only", name)
		}
	})
}

// TestOAuthAllowlistGatesEmailDomain guards the ops gate: the wildcard e-mail
// domain lets any GitHub account through, so it may only be set where a GitHub
// user allowlist is set alongside it.
func TestOAuthAllowlistGatesEmailDomain(t *testing.T) {
	services := readCompose(t).Services

	t.Run("Happy path: the OAuth proxy restricts logins to the GitHub user allowlist", func(t *testing.T) {
		oauth, ok := services["oauth2-proxy"]
		require.True(t, ok, "guard: the oauth2-proxy service must exist")
		assert.Equal(t, "${GITHUB_USER}", oauth.Environment["OAUTH2_PROXY_GITHUB_USER"])
	})

	t.Run("Edge: a wildcard e-mail domain is only set alongside a GitHub user allowlist", func(t *testing.T) {
		for name, service := range services {
			if !hasArg(service.Command, "--email-domain=*") {
				continue
			}
			assert.NotEmptyf(t, service.Environment["OAUTH2_PROXY_GITHUB_USER"],
				"service %q accepts any e-mail domain without a GitHub user allowlist", name)
		}
	})
}

// TestExampleEnvCoversRequiredComposeVars guards the deploy template: every
// variable the compose stack requires must have a non-empty placeholder in
// .env.example, otherwise CI's interpolation and the operator's copy diverge.
func TestExampleEnvCoversRequiredComposeVars(t *testing.T) {
	composeRaw, err := os.ReadFile(composeFile)
	require.NoErrorf(t, err, "read %s", composeFile)

	required := map[string]bool{}
	for _, match := range requiredComposeVarPattern.FindAllStringSubmatch(string(composeRaw), -1) {
		required[match[1]] = true
	}
	require.NotEmpty(t, required, "guard: the compose file must reference required variables")

	raw := readEnvExample(t)
	example := parseEnvExample(raw)

	t.Run("Happy path: every required compose variable has a non-empty example value", func(t *testing.T) {
		for name := range required {
			assert.NotEmptyf(t, example[name], "required variable %s is missing or empty in %s", name, envExampleFile)
		}
	})

	t.Run("Edge: a variable documented only as a comment does not count as a value", func(t *testing.T) {
		for _, match := range commentedEnvPattern.FindAllStringSubmatch(raw, -1) {
			if required[match[1]] {
				assert.NotEmptyf(t, example[match[1]],
					"required variable %s is only commented out in %s", match[1], envExampleFile)
			}
		}
	})
}

func hasArg(args []string, want string) bool {
	return slices.Contains(args, want)
}

func readEnvExample(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(envExampleFile)
	require.NoErrorf(t, err, "read %s", envExampleFile)
	return string(raw)
}

// parseEnvExample returns the active KEY=value pairs of an env file. Comment
// lines are skipped, so an optional setting documented only as a comment reads
// as absent.
func parseEnvExample(raw string) map[string]string {
	values := map[string]string{}
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return values
}
