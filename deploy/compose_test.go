package deploy_test

import (
	"os"
	"testing"

	"legnaris/backend/shared/env"

	"github.com/stretchr/testify/assert"
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
	Environment map[string]string   `yaml:"environment"`
	Build       *composeBuild       `yaml:"build"`
	Ports       []string            `yaml:"ports"`
	Healthcheck *composeHealthcheck `yaml:"healthcheck"`
	DependsOn   composeDependsOn    `yaml:"depends_on"`
}

type composeBuild struct {
	Args map[string]string `yaml:"args"`
}

type composeHealthcheck struct {
	Test []string `yaml:"test"`
}

// composeDependsOn accepts both the short (sequence) and long (mapping)
// depends_on forms, so every service's dependencies can be inspected.
type composeDependsOn map[string]any

func (d *composeDependsOn) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.MappingNode:
		parsed := map[string]any{}
		if err := value.Decode(&parsed); err != nil {
			return err
		}
		*d = parsed
	case yaml.SequenceNode:
		var names []string
		if err := value.Decode(&names); err != nil {
			return err
		}
		parsed := make(map[string]any, len(names))
		for _, name := range names {
			parsed[name] = nil
		}
		*d = parsed
	default:
		*d = composeDependsOn{}
	}
	return nil
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

// TestAppServiceTrustedProxyCIDRs guards the optional proxy-set pass-through:
// the operator can narrow or widen the trust boundary from the deployment env,
// and leaving it unset keeps the app's documented default. The required-variable
// contract is deliberately unchanged (PRQ-03, PRQ-18).
func TestAppServiceTrustedProxyCIDRs(t *testing.T) {
	doc := readCompose(t)

	app, ok := doc.Services["app"]
	require.True(t, ok, "the stack must define an app service")

	t.Run("Happy path: the proxy set is an optional interpolation", func(t *testing.T) {
		value, present := app.Environment["TRUSTED_PROXY_CIDRS"]
		require.True(t, present, "app.environment must pass TRUSTED_PROXY_CIDRS through")
		require.Contains(t, value, "${TRUSTED_PROXY_CIDRS:-}",
			"the set must be optional and sourced from the deployment env")
	})

	t.Run("Edge: the required-variable list gains only the digest secret", func(t *testing.T) {
		require.Equal(t, []string{"DATABASE_URI", "JWT_SECRET", "EMAIL_CODE_SECRET"}, env.RequiredKeys())
	})
}

// TestRabbitMQServiceIsInternalOnly guards the broker's exposure: it is a
// separate service reachable only on the internal network, with no published
// host port and a healthcheck (EML-16, SC-007).
func TestRabbitMQServiceIsInternalOnly(t *testing.T) {
	doc := readCompose(t)

	rabbitmq, ok := doc.Services["rabbitmq"]
	require.True(t, ok, "the stack must define a rabbitmq service")

	t.Run("Happy path: the broker publishes no host port", func(t *testing.T) {
		assert.Empty(t, rabbitmq.Ports, "the broker must not be reachable from the host")
	})

	t.Run("Happy path: the broker has a healthcheck", func(t *testing.T) {
		require.NotNil(t, rabbitmq.Healthcheck, "the broker must declare a healthcheck")
		assert.NotEmpty(t, rabbitmq.Healthcheck.Test)
	})
}

// TestRelayIsASeparateService guards the relay topology: it is a separate
// service reachable only internally, the only one that depends on the broker,
// while the API never depends on the broker to start (EML-16).
func TestRelayIsASeparateService(t *testing.T) {
	doc := readCompose(t)

	relay, ok := doc.Services["relay"]
	require.True(t, ok, "the stack must define a relay service")
	app, ok := doc.Services["app"]
	require.True(t, ok, "the stack must define an app service")

	t.Run("Happy path: the relay depends on the broker", func(t *testing.T) {
		_, present := relay.DependsOn["rabbitmq"]
		assert.True(t, present, "the relay must wait for the broker")
	})

	t.Run("Edge: the API does not depend on the broker", func(t *testing.T) {
		_, present := app.DependsOn["rabbitmq"]
		assert.False(t, present, "the API must start without the broker")
	})

	t.Run("Happy path: the relay publishes no host port", func(t *testing.T) {
		assert.Empty(t, relay.Ports, "the relay must be reachable only internally")
	})
}

// TestEmailConfigurationIsOptional guards the configuration surface: every new
// email/broker setting is an optional interpolation, the API never receives the
// broker URL, and the required-variable list is unchanged (EML-14, EML-16).
func TestEmailConfigurationIsOptional(t *testing.T) {
	doc := readCompose(t)

	app, ok := doc.Services["app"]
	require.True(t, ok, "the stack must define an app service")
	relay, ok := doc.Services["relay"]
	require.True(t, ok, "the stack must define a relay service")

	t.Run("Happy path: the API passes the optional email settings through", func(t *testing.T) {
		for _, key := range []string{"EMAIL_PROVIDER", "RESEND_API_KEY", "EMAIL_FROM", "EMAIL_SEND_TIMEOUT", "EMAIL_OUTBOX_RETENTION"} {
			value, present := app.Environment[key]
			require.Truef(t, present, "app.environment must pass %s through", key)
			assert.Containsf(t, value, "${", "app.environment[%s] must be an optional interpolation", key)
		}
	})

	t.Run("Edge: the API never receives the broker URL", func(t *testing.T) {
		_, present := app.Environment["RABBITMQ_URL"]
		assert.False(t, present, "the request-serving process must never read RABBITMQ_URL")
	})

	t.Run("Happy path: the relay receives the broker and relay settings", func(t *testing.T) {
		for _, key := range []string{"RABBITMQ_URL", "EMAIL_RELAY_POLL_INTERVAL", "EMAIL_RELAY_LEASE", "EMAIL_RELAY_BATCH", "EMAIL_RELAY_CONFIRM_TIMEOUT", "EMAIL_RELAY_MAX_ATTEMPTS", "EMAIL_RELAY_METRICS_ADDR"} {
			value, present := relay.Environment[key]
			require.Truef(t, present, "relay.environment must pass %s through", key)
			assert.Containsf(t, value, "${", "relay.environment[%s] must be an optional interpolation", key)
		}
	})

	t.Run("Edge: the required-variable list gains only the digest secret", func(t *testing.T) {
		assert.Equal(t, []string{"DATABASE_URI", "JWT_SECRET", "EMAIL_CODE_SECRET"}, env.RequiredKeys())
	})
}

// TestRabbitMQPasswordIsRequired guards the broker credential: a forgotten
// RABBITMQ_PASSWORD must fail `docker compose config` (and therefore `up`)
// instead of starting the broker with an empty password, and .env.example must
// carry the placeholder the CI compose gate interpolates (EML-16).
func TestRabbitMQPasswordIsRequired(t *testing.T) {
	doc := readCompose(t)

	rabbitmq, ok := doc.Services["rabbitmq"]
	require.True(t, ok, "the stack must define a rabbitmq service")

	t.Run("Happy path: the broker password is a required interpolation", func(t *testing.T) {
		value, present := rabbitmq.Environment["RABBITMQ_DEFAULT_PASS"]
		require.True(t, present, "rabbitmq.environment must define RABBITMQ_DEFAULT_PASS")
		assert.Contains(t, value, "${RABBITMQ_PASSWORD:?",
			"a missing password must fail the stack, never start an empty one")
	})

	t.Run("Happy path: .env.example provides the placeholder", func(t *testing.T) {
		raw, err := os.ReadFile(".env.example")
		require.NoError(t, err)
		assert.Contains(t, string(raw), "RABBITMQ_PASSWORD=",
			"the CI compose gate interpolates .env.example, so the placeholder must be present")
		assert.NotContains(t, string(raw), "# RABBITMQ_PASSWORD=",
			"the placeholder must not be commented out")
	})
}

// TestChallengeConfigurationIsRequiredAndDocumented guards the code-step
// settings: the digest secret is a required interpolation the CI compose gate
// can satisfy from .env.example, and every optional setting is passed through
// as an interpolation with the app's documented default (OTP-18, SC-008).
func TestChallengeConfigurationIsRequiredAndDocumented(t *testing.T) {
	doc := readCompose(t)

	app, ok := doc.Services["app"]
	require.True(t, ok, "the stack must define an app service")

	t.Run("Happy path: the digest secret is a required interpolation", func(t *testing.T) {
		value, present := app.Environment["EMAIL_CODE_SECRET"]
		require.True(t, present, "app.environment must define EMAIL_CODE_SECRET")
		assert.Contains(t, value, "${EMAIL_CODE_SECRET:?",
			"a missing digest secret must fail the stack, never boot a weak one")
	})

	t.Run("Happy path: the optional challenge settings are interpolations", func(t *testing.T) {
		for _, key := range []string{
			"AUTH_OTP_TTL", "AUTH_OTP_CEILING", "AUTH_OTP_RESEND_COOLDOWN",
			"AUTH_OTP_MAX_ATTEMPTS", "AUTH_OTP_DAILY_SEND_CAP",
			"AUTH_OTP_USER_WINDOW", "AUTH_OTP_USER_LIMIT",
		} {
			value, present := app.Environment[key]
			require.Truef(t, present, "app.environment must pass %s through", key)
			assert.Containsf(t, value, "${", "app.environment[%s] must be an interpolation", key)
		}
	})

	t.Run("Happy path: .env.example provides the required placeholder", func(t *testing.T) {
		raw, err := os.ReadFile(".env.example")
		require.NoError(t, err)
		assert.Contains(t, string(raw), "EMAIL_CODE_SECRET=",
			"the CI compose gate interpolates .env.example, so the placeholder must be present")
		assert.NotContains(t, string(raw), "# EMAIL_CODE_SECRET=",
			"the placeholder must not be commented out")
	})
}
