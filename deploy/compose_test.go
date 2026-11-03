package deploy_test

import (
	"cmp"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// composeFile is the base stack definition. The local override only tweaks
// entrypoints/ports and never removes the app's environment, so validating the
// base is sufficient.
const composeFile = "docker-compose.yml"

// envExampleFile is the deployment template the operator copies into deploy/.env.
const envExampleFile = ".env.example"

// composeDoc is the minimal shape needed to inspect service environments.
type composeDoc struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Image           string              `yaml:"image"`
	Command         []string            `yaml:"command"`
	Restart         string              `yaml:"restart"`
	StopGracePeriod string              `yaml:"stop_grace_period"`
	Entrypoint      []string            `yaml:"entrypoint"`
	Networks        []string            `yaml:"networks"`
	Volumes         []string            `yaml:"volumes"`
	Environment     map[string]string   `yaml:"environment"`
	Build           *composeBuild       `yaml:"build"`
	Ports           []string            `yaml:"ports"`
	Healthcheck     *composeHealthcheck `yaml:"healthcheck"`
	DependsOn       composeDependsOn    `yaml:"depends_on"`
}

type composeBuild struct {
	Context    string            `yaml:"context"`
	Dockerfile string            `yaml:"dockerfile"`
	Args       map[string]string `yaml:"args"`
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

// TestAppServiceEmailVerificationGate guards the rollout gate's deployment
// wiring: it is passed through to the app container, it is optional, and it
// does not join the required-variable set (TFA-07, TFA-16).
func TestAppServiceEmailVerificationGate(t *testing.T) {
	doc := readCompose(t)

	app, ok := doc.Services["app"]
	require.True(t, ok, "the stack must define an app service")

	t.Run("Happy path: the gate is an optional interpolation with a disabled default", func(t *testing.T) {
		value, present := app.Environment["AUTH_REQUIRE_EMAIL_VERIFICATION"]
		require.True(t, present, "app.environment must pass AUTH_REQUIRE_EMAIL_VERIFICATION through")
		assert.Contains(t, value, "${AUTH_REQUIRE_EMAIL_VERIFICATION:-false}",
			"the gate must be optional and default to the disabled state")
	})

	t.Run("Edge: the required-variable list is unchanged", func(t *testing.T) {
		require.Equal(t, []string{"DATABASE_URI", "JWT_SECRET", "EMAIL_CODE_SECRET"}, env.RequiredKeys())
	})

	t.Run("Happy path: the deployment template documents the gate", func(t *testing.T) {
		raw, err := os.ReadFile(envExampleFile)
		require.NoErrorf(t, err, "read %s", envExampleFile)

		assert.Contains(t, string(raw), "AUTH_REQUIRE_EMAIL_VERIFICATION",
			"the gate must be documented in the deployment template")
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

// workerSettings are the variables the delivery worker reads (BRK-04); the
// request-serving process must receive none of the broker or worker ones.
var workerSettings = []string{
	"RABBITMQ_URL",
	"EMAIL_PROVIDER",
	"RESEND_API_KEY",
	"EMAIL_FROM",
	"EMAIL_SEND_TIMEOUT",
	"EMAIL_WORKER_PREFETCH",
	"EMAIL_WORKER_MAX_ATTEMPTS",
	"EMAIL_WORKER_METRICS_ADDR",
}

// brokerOnlySettings are the worker settings the app never receives; the
// provider settings stay shared because the app already carries them.
var brokerOnlySettings = []string{
	"RABBITMQ_URL",
	"EMAIL_WORKER_PREFETCH",
	"EMAIL_WORKER_MAX_ATTEMPTS",
	"EMAIL_WORKER_METRICS_ADDR",
}

const (
	workerImage       = "legnaris-app:local"
	brokerConfigFile  = "rabbitmq/rabbitmq.conf"
	brokerDockerfile  = "rabbitmq/Dockerfile"
	consolePrefixConf = "management.path_prefix = /rabbitmq"
	metricsPortConf   = "prometheus.tcp.port = 15692"
	runbookFile       = "README.md"
	prometheusFile    = "prometheus/prometheus.yml"
	deliveryVhost     = "legnaris"
	operatorAccount   = "operator"
	provisionScript   = "rabbitmq/provision.sh"
	entrypointScript  = "rabbitmq/entrypoint.sh"
	opsPasswordKey    = "RABBITMQ_OPS_PASSWORD"
)

var exactBrokerImage = regexp.MustCompile(`^rabbitmq:4\.\d+\.\d+-management(-alpine)?$`)

// TestWorkerIsASeparateService guards the worker's place in the stack: its own
// service on the shared image, started on its own entrypoint, restarted on
// failure, internal only, waiting for the broker and never for the database
// (BRK-01, BRK-02, BRK-03, BRK-20, SC-001).
func TestWorkerIsASeparateService(t *testing.T) {
	doc := readCompose(t)

	worker, ok := doc.Services["worker"]
	require.True(t, ok, "the stack must define a worker service")
	app, ok := doc.Services["app"]
	require.True(t, ok, "the stack must define an app service")

	t.Run("Happy path: the worker shares the image and starts on its own entrypoint", func(t *testing.T) {
		assert.Equal(t, workerImage, worker.Image)
		assert.Equal(t, doc.Services["relay"].Build, worker.Build, "the worker must build the same image as the relay")
		assert.Equal(t, []string{"/worker"}, worker.Entrypoint)
	})

	t.Run("Happy path: the worker restarts after a failure", func(t *testing.T) {
		assert.Equal(t, "unless-stopped", worker.Restart)
	})

	t.Run("Happy path: the worker waits for a healthy broker", func(t *testing.T) {
		condition, present := worker.DependsOn["rabbitmq"]
		require.True(t, present, "the worker must wait for the broker")
		assert.Equal(t, map[string]any{"condition": "service_healthy"}, condition)
	})

	t.Run("Edge: the worker publishes no host port", func(t *testing.T) {
		assert.Empty(t, worker.Ports, "the worker must be reachable only inside the machine")
	})

	t.Run("Edge: the worker is on the internal network only", func(t *testing.T) {
		assert.Equal(t, []string{"internal"}, worker.Networks)
	})

	t.Run("Edge: the worker never depends on the database", func(t *testing.T) {
		_, present := worker.DependsOn["mongodb"]
		assert.False(t, present, "the worker needs no store; a dependency it does not need is a failure mode")
	})

	t.Run("Edge: the site depends on neither the broker nor the worker", func(t *testing.T) {
		_, onBroker := app.DependsOn["rabbitmq"]
		_, onWorker := app.DependsOn["worker"]
		assert.False(t, onBroker, "the site must start without the broker")
		assert.False(t, onWorker, "the site must start without the worker")
	})

	t.Run("Edge: no service depends on the worker", func(t *testing.T) {
		for name, service := range doc.Services {
			_, present := service.DependsOn["worker"]
			assert.Falsef(t, present, "service %s must not depend on the worker", name)
		}
	})
}

// TestWorkerEnvironmentIsSeparatedFromTheApp guards the configuration split:
// the worker carries the broker address, the provider settings and its own
// knobs; the request-serving process carries the broker's and the worker's
// settings never, and the required list stays as it was (BRK-04, BRK-21, SC-005).
func TestWorkerEnvironmentIsSeparatedFromTheApp(t *testing.T) {
	doc := readCompose(t)

	worker, ok := doc.Services["worker"]
	require.True(t, ok, "the stack must define a worker service")
	app, ok := doc.Services["app"]
	require.True(t, ok, "the stack must define an app service")

	t.Run("Happy path: the worker receives every setting it reads as an interpolation", func(t *testing.T) {
		for _, key := range workerSettings {
			value, present := worker.Environment[key]
			require.Truef(t, present, "worker.environment must pass %s through", key)
			assert.Containsf(t, value, "${", "worker.environment[%s] must be an interpolation, never a committed literal", key)
		}
	})

	t.Run("Edge: the app receives neither the broker address nor a worker knob", func(t *testing.T) {
		for _, key := range brokerOnlySettings {
			_, present := app.Environment[key]
			assert.Falsef(t, present, "app.environment must not carry %s", key)
		}
	})

	t.Run("Edge: no app value points at the broker", func(t *testing.T) {
		for key, value := range app.Environment {
			assert.NotContainsf(t, value, "rabbitmq", "app.environment[%s] must not name the broker", key)
		}
	})

	t.Run("Edge: the worker receives no database setting and no operator credential", func(t *testing.T) {
		for _, key := range []string{"DATABASE_URI", "DATABASE_NAME", "JWT_SECRET", "EMAIL_CODE_SECRET", "RABBITMQ_OPS_PASSWORD"} {
			_, present := worker.Environment[key]
			assert.Falsef(t, present, "worker.environment must not carry %s", key)
		}
	})

	t.Run("Edge: the worker and the relay name the same delivery vhost", func(t *testing.T) {
		relay := doc.Services["relay"].Environment["RABBITMQ_URL"]
		assert.Equal(t, relay, worker.Environment["RABBITMQ_URL"])
		assert.Contains(t, worker.Environment["RABBITMQ_URL"], "@rabbitmq:5672/"+deliveryVhost+"}")
	})

	t.Run("Edge: the required-variable list is unchanged", func(t *testing.T) {
		assert.Equal(t, []string{"DATABASE_URI", "JWT_SECRET", "EMAIL_CODE_SECRET"}, env.RequiredKeys())
	})
}

// TestBrokerBasePinIsExact guards the broker's version across both places the
// pin may sit: it must name an exact 4.x release, never a floating tag
// (BRK-06, SC-006).
func TestBrokerBasePinIsExact(t *testing.T) {
	tests := []struct {
		name  string
		image string
		valid bool
	}{
		{name: "Happy path: an exact release is accepted", image: "rabbitmq:4.4.0-management", valid: true},
		{name: "Happy path: an exact alpine release is accepted", image: "rabbitmq:4.4.0-management-alpine", valid: true},
		{name: "Error: the floating major tag is rejected", image: "rabbitmq:4-management"},
		{name: "Error: the floating minor tag is rejected", image: "rabbitmq:4.4-management"},
		{name: "Error: latest is rejected", image: "rabbitmq:latest"},
		{name: "Error: an untagged image is rejected", image: "rabbitmq"},
		{name: "Error: another major series is rejected", image: "rabbitmq:3.13.7-management"},
		{name: "Error: an image without the console is rejected", image: "rabbitmq:4.4.0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.valid, exactBrokerImage.MatchString(tc.image))
		})
	}
}

// brokerBaseImage returns the image the broker really runs on: the derived
// image's base when the service builds one, the service's own tag otherwise.
func brokerBaseImage(t *testing.T, service composeService) string {
	t.Helper()

	if service.Build == nil {
		return service.Image
	}
	dockerfile := filepath.Join(service.Build.Context, cmp.Or(service.Build.Dockerfile, "Dockerfile"))
	raw, err := os.ReadFile(dockerfile)
	require.NoErrorf(t, err, "read %s", dockerfile)

	for _, line := range strings.Split(string(raw), "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 && strings.EqualFold(fields[0], "FROM") {
			return fields[1]
		}
	}
	require.Failf(t, "no FROM instruction", "%s has no FROM instruction", dockerfile)
	return ""
}

// TestBrokerServiceShape guards the broker's exposure, version and durability:
// internal only, health-checked, pinned, building the derived image and keeping
// its volume. It joins no network beyond the internal one because the edge
// router already reaches the internal network, and every extra network widens
// who can talk to a broker that holds the codes in the clear (BRK-05, BRK-06,
// BRK-07, BRK-12, BRK-14, BRK-18, SC-002).
func TestBrokerServiceShape(t *testing.T) {
	doc := readCompose(t)

	rabbitmq, ok := doc.Services["rabbitmq"]
	require.True(t, ok, "the stack must define a rabbitmq service")

	t.Run("Happy path: the broker runs on an exact 4.x base wherever the pin sits", func(t *testing.T) {
		assert.Regexp(t, exactBrokerImage, brokerBaseImage(t, rabbitmq))
	})

	t.Run("Edge: the broker publishes no host port, console and metrics included", func(t *testing.T) {
		assert.Empty(t, rabbitmq.Ports)
	})

	t.Run("Edge: the broker is on the internal network only", func(t *testing.T) {
		assert.Equal(t, []string{"internal"}, rabbitmq.Networks)
	})

	t.Run("Happy path: the broker is built from the derived image", func(t *testing.T) {
		require.NotNil(t, rabbitmq.Build, "the broker must build the derived image that enables the metrics plugin")
		assert.Equal(t, "./rabbitmq", rabbitmq.Build.Context)
	})

	t.Run("Edge: the durable volume survives a broker replacement", func(t *testing.T) {
		assert.Contains(t, rabbitmq.Volumes, "rabbitmq_data:/var/lib/rabbitmq")
	})

	t.Run("Happy path: the broker configuration is mounted read-only", func(t *testing.T) {
		assert.Contains(t, rabbitmq.Volumes, "./rabbitmq/rabbitmq.conf:/etc/rabbitmq/conf.d/20-legnaris.conf:ro")
	})

	t.Run("Edge: healthy means provisioned, so the relay and the worker never meet a bare broker", func(t *testing.T) {
		require.NotNil(t, rabbitmq.Healthcheck)
		check := strings.Join(rabbitmq.Healthcheck.Test, " ")
		assert.Contains(t, check, "rabbitmq-diagnostics -q ping")
		assert.Contains(t, check, "list_permissions -p "+deliveryVhost)
		for _, name := range []string{"relay", "worker"} {
			assert.Equalf(t, map[string]any{"condition": "service_healthy"}, doc.Services[name].DependsOn["rabbitmq"],
				"%s must wait for the broker to be healthy", name)
		}
	})

	t.Run("Edge: the operator's password is an optional interpolation that only the broker receives", func(t *testing.T) {
		assert.Equal(t, "${"+opsPasswordKey+":-}", rabbitmq.Environment[opsPasswordKey])
		for name, service := range doc.Services {
			if name == "rabbitmq" {
				continue
			}
			_, present := service.Environment[opsPasswordKey]
			assert.Falsef(t, present, "%s must not receive the operator's password", name)
		}
	})
}

// TestDerivedBrokerImageEnablesMetricsAtBuild guards the build-time enablement:
// the plugin is on before the broker ever starts, using the broker's own
// documented command and a plugin the pinned version ships (BRK-14).
func TestDerivedBrokerImageEnablesMetricsAtBuild(t *testing.T) {
	raw, err := os.ReadFile(brokerDockerfile)
	require.NoErrorf(t, err, "read %s", brokerDockerfile)
	dockerfile := string(raw)

	t.Run("Happy path: the plugin is enabled offline at build time", func(t *testing.T) {
		assert.Contains(t, dockerfile, "RUN rabbitmq-plugins enable --offline rabbitmq_prometheus")
	})

	t.Run("Edge: nothing enables plugins when the broker starts", func(t *testing.T) {
		for _, file := range []string{entrypointScript, provisionScript} {
			script, err := os.ReadFile(file)
			require.NoErrorf(t, err, "read %s", file)
			assert.NotContainsf(t, string(script), "rabbitmq-plugins", "%s must not touch plugins", file)
		}
	})

	t.Run("Happy path: the image starts through the provisioning entrypoint and still runs the broker", func(t *testing.T) {
		assert.Contains(t, dockerfile, `ENTRYPOINT ["legnaris-entrypoint"]`)
		assert.Contains(t, dockerfile, `CMD ["rabbitmq-server"]`)
	})

	t.Run("Edge: both scripts are copied into the image and executable", func(t *testing.T) {
		assert.Contains(t, dockerfile, "COPY provision.sh /usr/local/bin/legnaris-provision")
		assert.Contains(t, dockerfile, "COPY entrypoint.sh /usr/local/bin/legnaris-entrypoint")
		for _, file := range []string{entrypointScript, provisionScript} {
			info, err := os.Stat(file)
			require.NoErrorf(t, err, "stat %s", file)
			assert.NotZerof(t, info.Mode()&0o111, "%s must be executable", file)
		}
	})
}

// TestBrokerConfigurationServesConsoleUnderItsPrefix guards the broker's
// configuration: the console lives under the prefix the edge route matches and
// the metrics listener has its own port (BRK-07, BRK-12).
func TestBrokerConfigurationServesConsoleUnderItsPrefix(t *testing.T) {
	raw, err := os.ReadFile(brokerConfigFile)
	require.NoErrorf(t, err, "read %s", brokerConfigFile)
	conf := string(raw)

	t.Run("Happy path: the console is served under its own path prefix", func(t *testing.T) {
		assert.Contains(t, conf, consolePrefixConf)
	})

	t.Run("Happy path: the metrics listener has an explicit port", func(t *testing.T) {
		assert.Contains(t, conf, metricsPortConf)
	})

	t.Run("Edge: no credential is written in the configuration", func(t *testing.T) {
		assert.NotContains(t, strings.ToLower(conf), "password")
		assert.NotContains(t, strings.ToLower(conf), "default_pass")
	})
}

// runbookSection returns the runbook text from heading to the next heading of
// the same level, so the provisioning checks read the section and nothing else.
func runbookSection(t *testing.T, heading string) string {
	t.Helper()

	raw, err := os.ReadFile(runbookFile)
	require.NoErrorf(t, err, "read %s", runbookFile)
	text := string(raw)

	start := strings.Index(text, heading)
	require.GreaterOrEqualf(t, start, 0, "the runbook must carry the section %q", heading)
	section := text[start+len(heading):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return section
}

// TestProvisioningScriptConfiguresTheBroker guards the script the broker runs
// beside itself: the steps are there, in the contract's order, the operator's
// password comes from the environment, and the application's account ends with
// no console right and the health check's signal as the last step (BRK-09,
// BRK-10, BRK-11, SC-003).
func TestProvisioningScriptConfiguresTheBroker(t *testing.T) {
	raw, err := os.ReadFile(provisionScript)
	require.NoErrorf(t, err, "read %s", provisionScript)
	script := string(raw)

	t.Run("Happy path: the steps appear in order, the application's rights last", func(t *testing.T) {
		steps := []string{
			"rabbitmqctl await_startup",
			`add_vhost "$vhost"`,
			`add_user "$operator"`,
			`set_user_tags "$operator" management`,
			`set_permissions -p "$vhost" "$operator"`,
			`set_user_tags "$vhost"`,
			`clear_permissions -p / "$vhost"`,
			`set_permissions -p "$vhost" "$vhost"`,
		}
		cursor := 0
		for _, step := range steps {
			offset := strings.Index(script[cursor:], step)
			require.GreaterOrEqualf(t, offset, 0, "the step %q must appear after the previous one", step)
			cursor += offset + len(step)
		}
		assert.Equal(t, `".*" ".*" ".*"`, strings.TrimSpace(script[cursor:]), "the application's rights must be the last step")
	})

	t.Run("Edge: the names are the contract's", func(t *testing.T) {
		assert.Contains(t, script, "vhost="+deliveryVhost)
		assert.Contains(t, script, "operator="+operatorAccount)
	})

	t.Run("Edge: the application's account is never given a tag", func(t *testing.T) {
		for _, line := range strings.Split(script, "\n") {
			if strings.Contains(line, `set_user_tags "$vhost"`) {
				assert.NotContains(t, line, "management")
				assert.NotContains(t, line, "administrator")
			}
		}
	})

	t.Run("Edge: the operator's password is read from the environment, never written in the script", func(t *testing.T) {
		assert.Contains(t, script, `"${`+opsPasswordKey+`:-}"`)
		assert.NotRegexp(t, opsPasswordKey+`=\S`, script)
	})

	t.Run("Edge: an empty operator password skips the operator and still provisions the application", func(t *testing.T) {
		guardAt := strings.Index(script, `if [ -n "${`+opsPasswordKey+`:-}" ]`)
		require.GreaterOrEqual(t, guardAt, 0, "the operator steps must be guarded")
		assert.Less(t, guardAt, strings.Index(script, `add_user "$operator"`))
		assert.Less(t, strings.Index(script, "\nfi\n"), strings.Index(script, `clear_permissions -p / "$vhost"`),
			"the application's steps must sit outside the guard")
	})

	t.Run("Edge: every step is repeatable", func(t *testing.T) {
		assert.Contains(t, script, "set -eu")
		assert.Contains(t, script, `list_vhosts | grep -qx "$vhost" \`, "add_vhost must be guarded by an existence check")
		assert.Contains(t, script, "change_password", "an existing operator account is rotated, not re-added")
	})

	t.Run("Edge: a broker that never comes up fails the script instead of waiting forever", func(t *testing.T) {
		assert.Contains(t, script, "exit 1")
	})
}

// TestEntrypointProvisionsBesideTheBroker guards the wrapper: the broker keeps
// the image's own entrypoint, a failed provisioning stops it so the restart
// policy retries, and a stop signal reaches the broker (BRK-09).
func TestEntrypointProvisionsBesideTheBroker(t *testing.T) {
	raw, err := os.ReadFile(entrypointScript)
	require.NoErrorf(t, err, "read %s", entrypointScript)
	script := string(raw)

	t.Run("Happy path: the broker starts through the image's entrypoint and the script runs as its user", func(t *testing.T) {
		assert.Contains(t, script, `docker-entrypoint.sh "$@" &`)
		assert.Contains(t, script, "gosu rabbitmq legnaris-provision")
	})

	t.Run("Edge: a stop signal is forwarded to the broker", func(t *testing.T) {
		assert.Contains(t, script, "trap 'kill -TERM")
	})

	t.Run("Edge: a failed provisioning stops the broker and the container exits with the broker's status", func(t *testing.T) {
		failure := strings.Index(script, `if ! wait "$provision"`)
		require.GreaterOrEqual(t, failure, 0)
		assert.Contains(t, script[failure:], `kill -TERM "$broker"`)
		assert.Contains(t, script, `exit "$status"`)
	})
}

// TestRunbookCarriesTheBrokerProvisioning guards the operator's runbook: it
// says the broker provisions itself, what the operator sets, and what the
// application's account ends with (BRK-09, BRK-10, BRK-11, BRK-16, SC-003).
func TestRunbookCarriesTheBrokerProvisioning(t *testing.T) {
	section := runbookSection(t, "## RabbitMQ provisioning")
	lower := strings.ToLower(section)

	t.Run("Happy path: the runbook names the script and the variable the operator sets", func(t *testing.T) {
		assert.Contains(t, section, "rabbitmq/provision.sh")
		assert.Contains(t, section, opsPasswordKey)
	})

	t.Run("Edge: the section says the provisioning is automatic, re-run on every start and gates the pipeline", func(t *testing.T) {
		assert.Contains(t, lower, "idempotent")
		assert.Contains(t, lower, "every start")
		assert.Contains(t, lower, "healthy")
	})

	t.Run("Edge: the section says what happens with an empty password and when it is rotated", func(t *testing.T) {
		assert.Contains(t, lower, "empty")
		assert.Contains(t, lower, "rotate")
	})

	t.Run("Edge: the runbook no longer asks for a manual provisioning command", func(t *testing.T) {
		assert.NotContains(t, section, "rabbitmqctl add_vhost")
		assert.NotContains(t, section, "read -rs")
	})

	t.Run("Edge: the section keeps the volume warning", func(t *testing.T) {
		assert.Contains(t, lower, "rabbitmq_data")
	})
}

// TestRunbookDocumentsTheConsole guards the console's documentation: where it
// is, what it shows and how the pipeline is inspected when it stalls (BRK-16,
// BRK-17).
func TestRunbookDocumentsTheConsole(t *testing.T) {
	raw, err := os.ReadFile(runbookFile)
	require.NoErrorf(t, err, "read %s", runbookFile)
	runbook := string(raw)

	t.Run("Happy path: the console's address is in the access table", func(t *testing.T) {
		assert.Contains(t, runbook, "https://ops.legnaris.com.br/rabbitmq/")
	})

	t.Run("Edge: the runbook says the console shows the queued codes in the clear", func(t *testing.T) {
		assert.Contains(t, strings.ToLower(runbook), "in the clear")
	})

	t.Run("Happy path: the worker's variables and the stall procedure are documented", func(t *testing.T) {
		for _, term := range []string{"EMAIL_WORKER_PREFETCH", "EMAIL_WORKER_MAX_ATTEMPTS", "EMAIL_WORKER_METRICS_ADDR", "### Email worker", "pipeline stalls"} {
			assert.Containsf(t, runbook, term, "the runbook must mention %q", term)
		}
	})
}

// credentialInURL matches a broker or database address with an inline password.
var credentialInURL = regexp.MustCompile(`(?:amqps?|mongodb)://[^:/\s@]+:([^@\s]+)@`)

// TestNoBrokerCredentialIsCommitted guards the repository's deploy tree: the
// operator's password reaches no service, is not an interpolation of the stack
// and no address carries a literal password (BRK-11, BRK-21, SC-004, SC-005).
func TestNoBrokerCredentialIsCommitted(t *testing.T) {
	t.Run("Edge: the operator's password appears in the stack only as the broker's optional interpolation", func(t *testing.T) {
		raw, err := os.ReadFile(composeFile)
		require.NoError(t, err)
		assert.Equal(t, 2, strings.Count(string(raw), opsPasswordKey), "one comment-free mention for the key and one for the interpolation")
		assert.Contains(t, string(raw), opsPasswordKey+": ${"+opsPasswordKey+":-}")
	})

	t.Run("Edge: the example environment carries the operator's password only as a comment", func(t *testing.T) {
		raw, err := os.ReadFile(envExampleFile)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "# RABBITMQ_OPS_PASSWORD=replace_me")
		assert.NotRegexp(t, `(?m)^RABBITMQ_OPS_PASSWORD=`, string(raw))
	})

	t.Run("Edge: no address under deploy/ carries a literal password", func(t *testing.T) {
		scanned := 0
		err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || strings.HasSuffix(path, "_test.go") {
				return walkErr
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			for _, match := range credentialInURL.FindAllStringSubmatch(string(raw), -1) {
				password := match[1]
				if strings.HasPrefix(password, "$") || strings.HasPrefix(password, "replace_me") {
					continue
				}
				assert.Failf(t, "literal credential", "%s carries a literal password in %q", path, match[0])
			}
			return nil
		})
		require.NoError(t, err)
		require.Positive(t, scanned, "guard: the scan must read at least one file")
	})
}

type prometheusDoc struct {
	ScrapeConfigs []struct {
		JobName       string `yaml:"job_name"`
		MetricsPath   string `yaml:"metrics_path"`
		StaticConfigs []struct {
			Targets []string `yaml:"targets"`
		} `yaml:"static_configs"`
	} `yaml:"scrape_configs"`
}

// scrapeTargets maps every job to its targets.
func scrapeTargets(t *testing.T) map[string][]string {
	t.Helper()

	raw, err := os.ReadFile(prometheusFile)
	require.NoErrorf(t, err, "read %s", prometheusFile)
	var doc prometheusDoc
	require.NoErrorf(t, yaml.Unmarshal(raw, &doc), "parse %s", prometheusFile)

	jobs := map[string][]string{}
	for _, job := range doc.ScrapeConfigs {
		for _, static := range job.StaticConfigs {
			jobs[job.JobName] = append(jobs[job.JobName], static.Targets...)
		}
	}
	return jobs
}

// TestMonitoringScrapesTheBrokerAndTheWorker guards the monitoring targets:
// the broker's metrics and the worker's are scraped beside the existing ones,
// each on the internal network at a service that publishes no host port
// (BRK-12, BRK-13, SC-007).
func TestMonitoringScrapesTheBrokerAndTheWorker(t *testing.T) {
	jobs := scrapeTargets(t)
	doc := readCompose(t)

	t.Run("Happy path: the broker and the worker are scraped", func(t *testing.T) {
		assert.Equal(t, []string{"rabbitmq:15692"}, jobs["legnaris-rabbitmq"])
		assert.Equal(t, []string{"worker:9101"}, jobs["legnaris-worker"])
	})

	t.Run("Edge: the four existing targets are still scraped", func(t *testing.T) {
		for _, job := range []string{"legnaris-app", "legnaris-relay", "traefik", "mongodb"} {
			assert.NotEmptyf(t, jobs[job], "the job %s must stay", job)
		}
	})

	t.Run("Edge: every target is a stack service that publishes no host port", func(t *testing.T) {
		for job, targets := range jobs {
			for _, target := range targets {
				host, _, found := strings.Cut(target, ":")
				require.Truef(t, found, "job %s target %q must carry a port", job, target)
				service, present := doc.Services[host]
				require.Truef(t, present, "job %s target %q must name a stack service", job, target)
				if host != "traefik" {
					assert.Emptyf(t, service.Ports, "job %s target %s must publish no host port", job, host)
				}
			}
		}
	})

	t.Run("Edge: the scraped ports are the ones the processes listen on", func(t *testing.T) {
		raw, err := os.ReadFile(brokerConfigFile)
		require.NoError(t, err)
		assert.Contains(t, string(raw), metricsPortConf)
		assert.Contains(t, doc.Services["worker"].Environment["EMAIL_WORKER_METRICS_ADDR"], ":-:9101}")
	})
}

// TestAppHealthcheckProbesReadiness guards the app container's healthcheck: the
// distroless image has no shell or curl, so the binary probes itself, and it
// must hit /ready (database reachable) rather than the bare liveness route.
func TestAppHealthcheckProbesReadiness(t *testing.T) {
	doc := readCompose(t)

	app, ok := doc.Services["app"]
	require.True(t, ok, "the stack must define an app service")
	require.NotNil(t, app.Healthcheck, "the app must declare a healthcheck")

	t.Run("Happy path: the healthcheck runs the binary against /ready on the app port", func(t *testing.T) {
		assert.Equal(t,
			[]string{"CMD", "/app", "healthcheck", "http://127.0.0.1:8080/ready"},
			app.Healthcheck.Test)
	})

	t.Run("Edge: the probe URL targets the port the app listens on", func(t *testing.T) {
		assert.Equal(t, "8080", app.Environment["PORT"])
	})
}

// TestAppStopGracePeriodCoversGracefulShutdown guards the shutdown budget: on
// SIGTERM the app drains for SHUTDOWN_TIMEOUT and then closes MongoDB for up to
// DATABASE_CLOSE_TIMEOUT, so a shorter Docker grace period would SIGKILL it
// mid-drain.
func TestAppStopGracePeriodCoversGracefulShutdown(t *testing.T) {
	doc := readCompose(t)

	app, ok := doc.Services["app"]
	require.True(t, ok, "the stack must define an app service")

	t.Run("Edge: the app declares an explicit stop_grace_period", func(t *testing.T) {
		require.NotEmpty(t, app.StopGracePeriod,
			"the Docker default of 10s leaves no margin over the shutdown budget")
	})

	t.Run("Happy path: the grace period exceeds the shutdown and database close budgets", func(t *testing.T) {
		grace, err := time.ParseDuration(app.StopGracePeriod)
		require.NoError(t, err)

		vars := env.NewEnv(logger.NewLogger(slog.LevelError)).Variables
		budget := vars.ShutdownTimeout + vars.DatabaseCloseTimeout
		assert.Greater(t, grace, budget)
	})

	t.Run("Edge: SHUTDOWN_TIMEOUT is passed through with the app default", func(t *testing.T) {
		value, present := app.Environment["SHUTDOWN_TIMEOUT"]
		require.True(t, present)
		assert.Contains(t, value, "SHUTDOWN_TIMEOUT")
	})
}

// TestDatabasePoolConfiguration guards the Mongo pool knobs: app and relay pass
// them through as optional interpolations with the safe defaults, and
// .env.example documents them (LGN-126).
func TestDatabasePoolConfiguration(t *testing.T) {
	doc := readCompose(t)

	for _, name := range []string{"app", "relay"} {
		service, ok := doc.Services[name]
		require.Truef(t, ok, "the stack must define a %s service", name)

		t.Run("Happy path: "+name+" passes the pool sizes with safe defaults", func(t *testing.T) {
			assert.Equal(t, "${DATABASE_MAX_POOL_SIZE:-20}", service.Environment["DATABASE_MAX_POOL_SIZE"])
			assert.Equal(t, "${DATABASE_MIN_POOL_SIZE:-0}", service.Environment["DATABASE_MIN_POOL_SIZE"])
		})
	}

	t.Run("Edge: .env.example documents the pool sizes as optional comments", func(t *testing.T) {
		raw, err := os.ReadFile(envExampleFile)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "# DATABASE_MAX_POOL_SIZE=20")
		assert.Contains(t, string(raw), "# DATABASE_MIN_POOL_SIZE=0")
		assert.NotRegexp(t, `(?m)^DATABASE_M(AX|IN)_POOL_SIZE=`, string(raw))
	})
}

// TestSharedRateLimitStore guards the replica-consistent rate limit wiring:
// Redis is internal-only and ephemeral, the app reaches it through an optional
// REDIS_URL and never depends on it to start (the limiters fail open), and the
// k8s guidance for TRUSTED_PROXY_CIDRS is documented (LGN-128).
func TestSharedRateLimitStore(t *testing.T) {
	doc := readCompose(t)

	redis, ok := doc.Services["redis"]
	require.True(t, ok, "the stack must define a redis service")

	t.Run("Happy path: redis publishes no host port and joins only the internal network", func(t *testing.T) {
		assert.Empty(t, redis.Ports)
		assert.Equal(t, []string{"internal"}, redis.Networks)
	})

	t.Run("Happy path: redis has a healthcheck and restarts", func(t *testing.T) {
		require.NotNil(t, redis.Healthcheck)
		assert.NotEmpty(t, redis.Healthcheck.Test)
		assert.Equal(t, "unless-stopped", redis.Restart)
	})

	t.Run("Edge: redis keeps no volume, the counters are ephemeral", func(t *testing.T) {
		assert.Empty(t, redis.Volumes)
	})

	t.Run("Happy path: the app points at redis through an optional override", func(t *testing.T) {
		app := doc.Services["app"]
		assert.Equal(t, "${REDIS_URL-redis://redis:6379/0}", app.Environment["REDIS_URL"])
	})

	t.Run("Edge: redis never evicts live counters silently", func(t *testing.T) {
		assert.Contains(t, redis.Command, "noeviction")
	})

	t.Run("Edge: the app does not depend on redis to start", func(t *testing.T) {
		assert.NotContains(t, doc.Services["app"].DependsOn, "redis")
	})

	t.Run("Edge: REDIS_URL stays out of the required variables", func(t *testing.T) {
		assert.NotContains(t, env.RequiredKeys(), "REDIS_URL")
	})

	t.Run("Happy path: the runbook documents TRUSTED_PROXY_CIDRS for Kubernetes", func(t *testing.T) {
		raw, err := os.ReadFile("README.md")
		require.NoError(t, err)
		section := string(raw)
		assert.Contains(t, section, "Kubernetes")
		assert.Contains(t, section, "REDIS_URL")
		assert.Regexp(t, `(?s)Kubernetes.*TRUSTED_PROXY_CIDRS`, section)
	})

	t.Run("Edge: .env.example documents REDIS_URL as optional", func(t *testing.T) {
		raw, err := os.ReadFile(envExampleFile)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "# REDIS_URL=")
		assert.NotRegexp(t, `(?m)^REDIS_URL=`, string(raw))
	})
}
