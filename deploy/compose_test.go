package deploy_test

import (
	"os"
	"testing"

	"legnaris/shared/env"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// composeFile is the base stack definition. The local override only tweaks
// entrypoints/ports and never removes the app's environment, so validating the
// base is sufficient.
const composeFile = "docker-compose.yml"

// composeDoc is the minimal shape needed to inspect service environments.
type composeDoc struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Environment map[string]string `yaml:"environment"`
	Build       *composeBuild     `yaml:"build"`
}

type composeBuild struct {
	Args map[string]string `yaml:"args"`
}

func readCompose(t *testing.T) composeDoc {
	t.Helper()

	raw, err := os.ReadFile(composeFile)
	require.NoErrorf(t, err, "read %s", composeFile)

	var doc composeDoc
	require.NoErrorf(t, yaml.Unmarshal(raw, &doc), "parse %s", composeFile)
	return doc
}

// TestAppServiceProvidesRequiredEnv guards the regression that took the stack
// down: the backend started requiring JWT_SECRET at boot, but the compose app
// service never passed it, so the container crash-looped on
// "JWT secret is not configured". The check is anchored to env.RequiredKeys,
// the same set the application enforces at startup.
func TestAppServiceProvidesRequiredEnv(t *testing.T) {
	doc := readCompose(t)

	app, ok := doc.Services["app"]
	require.True(t, ok, "the stack must define an app service")

	required := env.RequiredKeys()
	require.NotEmpty(t, required, "guard: the required set must not be empty")

	t.Run("Happy path: every required variable is passed to the app container", func(t *testing.T) {
		for _, key := range required {
			value, present := app.Environment[key]
			require.Truef(t, present,
				"app.environment must pass the required variable %s; the app refuses to boot without it", key)
			require.NotEmptyf(t, value,
				"app.environment[%s] must not be empty", key)
		}
	})

	t.Run("Edge: JWT_SECRET is interpolated from the deployment env, not hardcoded", func(t *testing.T) {
		value, present := app.Environment["JWT_SECRET"]
		require.True(t, present, "app.environment must define JWT_SECRET")
		require.Contains(t, value, "JWT_SECRET",
			"JWT_SECRET must be sourced from ${JWT_SECRET...}, never a committed literal")
	})
}

// TestWebServiceBakesApiOrigin guards the web client's only build-time setting:
// Vite inlines the API origin into the bundle, so a committed literal would
// freeze every deployment to one environment's API and a browser on the web
// domain would call the wrong host. The value must stay an environment
// interpolation, and the service must be buildable from web/Dockerfile.
func TestWebServiceBakesApiOrigin(t *testing.T) {
	doc := readCompose(t)

	web, ok := doc.Services["web"]
	require.True(t, ok, "the stack must define a web service")

	require.NotNil(t, web.Build, "web must be built (context ../web, dockerfile Dockerfile)")

	t.Run("Happy path: the API origin comes from the deployment env", func(t *testing.T) {
		value, present := web.Build.Args["VITE_API_BASE_URL"]
		require.True(t, present, "web.build.args must define VITE_API_BASE_URL")
		require.Contains(t, value, "WEB_API_BASE_URL",
			"VITE_API_BASE_URL must be sourced from ${WEB_API_BASE_URL...}, never a committed literal")
	})
}
