package deploy_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const dynamicFile = "traefik/dynamic.yml"

// supportedHeaderOptions is the set of `headers` middleware options accepted by
// the Traefik release pinned in docker-compose.yml (v3.3). Traefik validates the
// dynamic file as a whole, so one unsupported key makes it discard the entire
// configuration: every router disappears, the edge answers 404 without CORS
// headers, and the browser reports the failure as a CORS error. Keep this list
// in sync when bumping the Traefik image.
var supportedHeaderOptions = []string{
	"accessControlAllowCredentials",
	"accessControlAllowHeaders",
	"accessControlAllowMethods",
	"accessControlAllowOriginList",
	"accessControlAllowOriginListRegex",
	"accessControlExposeHeaders",
	"accessControlMaxAge",
	"addVaryHeader",
	"allowedHosts",
	"browserXssFilter",
	"contentSecurityPolicy",
	"contentSecurityPolicyReportOnly",
	"contentTypeNosniff",
	"customBrowserXSSValue",
	"customFrameOptionsValue",
	"customRequestHeaders",
	"customResponseHeaders",
	"featurePolicy",
	"forceSTSHeader",
	"frameDeny",
	"hostsProxyHeaders",
	"isDevelopment",
	"permissionsPolicy",
	"publicKey",
	"referrerPolicy",
	"sslForceHost",
	"sslHost",
	"sslProxyHeaders",
	"sslRedirect",
	"sslTemporaryRedirect",
	"stsIncludeSubdomains",
	"stsPreload",
	"stsSeconds",
}

type dynamicDoc struct {
	HTTP struct {
		Middlewares map[string]struct {
			Headers map[string]any `yaml:"headers"`
		} `yaml:"middlewares"`
	} `yaml:"http"`
}

func readDynamic(t *testing.T) dynamicDoc {
	t.Helper()

	raw, err := os.ReadFile(dynamicFile)
	require.NoErrorf(t, err, "read %s", dynamicFile)

	var doc dynamicDoc
	require.NoErrorf(t, yaml.Unmarshal(raw, &doc), "parse %s", dynamicFile)
	return doc
}

// TestTraefikHeadersMiddlewareOptions guards the regression that took the whole
// stack down: `crossOriginOpenerPolicy` is not a Traefik v3.3 header option, so
// the file provider rejected the configuration and every public route returned
// 404, which the browser surfaced as a CORS failure on login and registration.
func TestTraefikHeadersMiddlewareOptions(t *testing.T) {
	doc := readDynamic(t)
	require.NotEmpty(t, doc.HTTP.Middlewares, "dynamic.yml must define middlewares")

	t.Run("Happy path: every headers option is supported by the pinned Traefik version", func(t *testing.T) {
		for name, middleware := range doc.HTTP.Middlewares {
			for option := range middleware.Headers {
				assert.Containsf(t, supportedHeaderOptions, option,
					"middleware %q uses the unsupported headers option %q; Traefik rejects the whole dynamic configuration and every router starts returning 404", name, option)
			}
		}
	})

	t.Run("Edge: COOP and CORP are delivered as custom response headers", func(t *testing.T) {
		headers, ok := doc.HTTP.Middlewares["web-security-headers"]
		require.True(t, ok, "the web client must keep a dedicated web-security-headers middleware")

		_, unsupportedCOOP := headers.Headers["crossOriginOpenerPolicy"]
		_, unsupportedCORP := headers.Headers["crossOriginResourcePolicy"]
		assert.False(t, unsupportedCOOP, "COOP must not use a dedicated option (absent in Traefik v3.3)")
		assert.False(t, unsupportedCORP, "CORP must not use a dedicated option (absent in Traefik v3.3)")

		responseHeaders, ok := headers.Headers["customResponseHeaders"].(map[string]any)
		require.True(t, ok, "customResponseHeaders must be a map")
		assert.Equal(t, "same-origin", responseHeaders["Cross-Origin-Opener-Policy"])
		assert.Equal(t, "same-origin", responseHeaders["Cross-Origin-Resource-Policy"])
	})
}

// TestWebSecurityHeadersKeepContract pins the headers the web client contract
// requires (headers.contract.md), including the two that moved to
// customResponseHeaders.
func TestWebSecurityHeadersKeepContract(t *testing.T) {
	doc := readDynamic(t)
	middleware, ok := doc.HTTP.Middlewares["web-security-headers"]
	require.True(t, ok, "web-security-headers must be defined")
	headers := middleware.Headers

	responseHeaders, ok := headers["customResponseHeaders"].(map[string]any)
	require.True(t, ok, "customResponseHeaders must be a map")

	tests := []struct {
		name   string
		option string
		value  any
	}{
		{name: "Reporting-Endpoints", option: "Reporting-Endpoints", value: responseHeaders["Reporting-Endpoints"]},
		{name: "Cross-Origin-Opener-Policy", option: "Cross-Origin-Opener-Policy", value: responseHeaders["Cross-Origin-Opener-Policy"]},
		{name: "Cross-Origin-Resource-Policy", option: "Cross-Origin-Resource-Policy", value: responseHeaders["Cross-Origin-Resource-Policy"]},
		{name: "Content-Security-Policy", option: "contentSecurityPolicy", value: headers["contentSecurityPolicy"]},
		{name: "Strict-Transport-Security", option: "stsSeconds", value: headers["stsSeconds"]},
		{name: "Referrer-Policy", option: "referrerPolicy", value: headers["referrerPolicy"]},
		{name: "Permissions-Policy", option: "permissionsPolicy", value: headers["permissionsPolicy"]},
	}
	for _, tc := range tests {
		t.Run("Happy path: "+tc.name+" is delivered", func(t *testing.T) {
			assert.NotNilf(t, tc.value, "%s must be configured", tc.option)
			assert.NotEmptyf(t, tc.value, "%s must not be empty", tc.option)
		})
	}

	t.Run("Happy path: framing and sniffing protections stay enabled", func(t *testing.T) {
		assert.Equal(t, true, headers["frameDeny"], "frameDeny must stay enabled")
		assert.Equal(t, true, headers["contentTypeNosniff"], "contentTypeNosniff must stay enabled")
	})

	t.Run("Edge: HSTS flags are pinned", func(t *testing.T) {
		assert.Equal(t, 63072000, headers["stsSeconds"])
		assert.Equal(t, true, headers["stsIncludeSubdomains"])
		assert.Equal(t, true, headers["stsPreload"])
	})
}

// TestSupportedHeaderOptionsHasNoDuplicates keeps the allowlist trustworthy:
// a duplicated entry would hide a misspelled option during review.
func TestSupportedHeaderOptionsHasNoDuplicates(t *testing.T) {
	seen := make(map[string]struct{}, len(supportedHeaderOptions))
	for _, option := range supportedHeaderOptions {
		if _, exists := seen[option]; exists {
			t.Errorf("duplicate option %q in supportedHeaderOptions", option)
		}
		seen[option] = struct{}{}
	}

	assert.True(t, slices.IsSorted(supportedHeaderOptions),
		"supportedHeaderOptions must stay sorted for reviewable diffs")
}

type dynamicRoutes struct {
	HTTP struct {
		Routers map[string]struct {
			Rule        string   `yaml:"rule"`
			Service     string   `yaml:"service"`
			Middlewares []string `yaml:"middlewares"`
		} `yaml:"routers"`
		Services map[string]struct {
			LoadBalancer struct {
				Servers []struct {
					URL string `yaml:"url"`
				} `yaml:"servers"`
			} `yaml:"loadBalancer"`
		} `yaml:"services"`
	} `yaml:"http"`
}

func readRoutes(t *testing.T) dynamicRoutes {
	t.Helper()

	raw, err := os.ReadFile(dynamicFile)
	require.NoErrorf(t, err, "read %s", dynamicFile)

	var doc dynamicRoutes
	require.NoErrorf(t, yaml.Unmarshal(raw, &doc), "parse %s", dynamicFile)
	return doc
}

const opsHostRule = "Host(`{{ env \"OPS_DOMAIN\" }}`)"

// TestBrokerConsoleRouteIsTheOperators guards the console's route: the
// operator's domain, its own path prefix, the broker's console, and the
// sign-in gate and security headers in that order (BRK-07, BRK-08, SC-002).
func TestBrokerConsoleRouteIsTheOperators(t *testing.T) {
	doc := readRoutes(t)

	router, ok := doc.HTTP.Routers["rabbitmq"]
	require.True(t, ok, "the edge must define a rabbitmq router")

	t.Run("Happy path: the rule is the operator's domain and the console's prefix", func(t *testing.T) {
		assert.Equal(t, opsHostRule+" && PathPrefix(`/rabbitmq`)", router.Rule)
	})

	t.Run("Happy path: the gate answers first, then the security headers", func(t *testing.T) {
		assert.Equal(t, []string{"oauth", "security-headers"}, router.Middlewares)
	})

	t.Run("Happy path: the router reaches the broker's console on its internal port", func(t *testing.T) {
		assert.Equal(t, "rabbitmq", router.Service)
		service, present := doc.HTTP.Services["rabbitmq"]
		require.True(t, present, "the edge must define a rabbitmq service")
		require.Len(t, service.LoadBalancer.Servers, 1)
		assert.Equal(t, "http://rabbitmq:15672", service.LoadBalancer.Servers[0].URL)
	})

	t.Run("Edge: the prefix is the one the broker serves its console under", func(t *testing.T) {
		raw, err := os.ReadFile(brokerConfigFile)
		require.NoError(t, err)
		assert.Contains(t, string(raw), consolePrefixConf)
	})

	t.Run("Edge: no other router reaches the broker's console", func(t *testing.T) {
		for name, other := range doc.HTTP.Routers {
			if name != "rabbitmq" {
				assert.NotEqualf(t, "rabbitmq", other.Service, "router %s must not route to the broker", name)
			}
		}
	})
}

// TestOperatorRoutesKeepTheirGate guards every route on the operator's domain:
// all but the OAuth callback carry the sign-in gate, and the three consoles
// already there are unchanged (BRK-08, SC-009).
func TestOperatorRoutesKeepTheirGate(t *testing.T) {
	doc := readRoutes(t)

	t.Run("Edge: every operator route but the OAuth callback carries the gate first", func(t *testing.T) {
		checked := 0
		for name, router := range doc.HTTP.Routers {
			if !strings.HasPrefix(router.Rule, opsHostRule) || name == "oauth2-proxy" {
				continue
			}
			checked++
			require.NotEmptyf(t, router.Middlewares, "router %s must carry the sign-in gate", name)
			assert.Equalf(t, "oauth", router.Middlewares[0], "router %s must be gated before anything else", name)
			assert.Containsf(t, router.Middlewares, "security-headers", "router %s must carry the security headers", name)
		}
		require.GreaterOrEqual(t, checked, 5, "guard: the four consoles and the home must be checked")
	})

	tests := []struct {
		name    string
		router  string
		rule    string
		service string
	}{
		{name: "Happy path: grafana is unchanged", router: "grafana", rule: opsHostRule + " && PathPrefix(`/grafana`)", service: "grafana"},
		{name: "Happy path: prometheus is unchanged", router: "prometheus", rule: opsHostRule + " && PathPrefix(`/prometheus`)", service: "prometheus"},
		{name: "Happy path: mongo-express is unchanged", router: "mongo-express", rule: opsHostRule + " && PathPrefix(`/mongo`)", service: "mongo-express"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router, ok := doc.HTTP.Routers[tc.router]
			require.True(t, ok)
			assert.Equal(t, tc.rule, router.Rule)
			assert.Equal(t, tc.service, router.Service)
			assert.Equal(t, []string{"oauth", "security-headers"}, router.Middlewares)
		})
	}
}

// TestPipelineDashboardStaysBehindTheOperatorsRoute guards the dashboard's
// reach: it is served by the monitoring dashboard host, which is routed only on
// the operator's domain behind the sign-in gate, and the provisioned directory
// is mounted read-only so nothing edited in a browser outlives a restart
// (OBS-10, OBS-21, SC-006).
func TestPipelineDashboardStaysBehindTheOperatorsRoute(t *testing.T) {
	doc := readRoutes(t)
	compose := readCompose(t)

	t.Run("Happy path: the dashboard host is on the operator's domain behind the gate", func(t *testing.T) {
		router, ok := doc.HTTP.Routers["grafana"]
		require.True(t, ok)
		assert.Contains(t, router.Rule, opsHostRule)
		assert.Equal(t, []string{"oauth", "security-headers"}, router.Middlewares)
	})

	t.Run("Edge: every router that reaches the dashboard host carries the gate first", func(t *testing.T) {
		reached := 0
		for name, router := range doc.HTTP.Routers {
			if router.Service != "grafana" {
				continue
			}
			reached++
			require.NotEmptyf(t, router.Middlewares, "router %s reaches the dashboard host and must carry the gate", name)
			assert.Equalf(t, "oauth", router.Middlewares[0], "router %s must gate before anything else", name)
		}
		assert.GreaterOrEqual(t, reached, 1, "guard: at least the dashboard's own router must be checked")
	})

	t.Run("Edge: the dashboard host publishes no port of its own", func(t *testing.T) {
		assert.Empty(t, compose.Services["grafana"].Ports)
	})

	t.Run("Happy path: the pipeline's dashboard directory is provisioned read-only", func(t *testing.T) {
		assert.Contains(t, compose.Services["grafana"].Volumes, "./grafana/dashboards:/var/lib/grafana/dashboards:ro")
	})
}

const apiHostRule = "Host(`{{ env \"API_DOMAIN\" }}`)"

// TestPublicProbeExposure guards which probes the public API domain reaches:
// /ready pings the database and stays internal, /metrics stays internal, and
// /health is public behind a per-client rate limit.
func TestPublicProbeExposure(t *testing.T) {
	routes := readRoutes(t)

	t.Run("Happy path: the public api router excludes /metrics and /ready", func(t *testing.T) {
		router, ok := routes.HTTP.Routers["app"]
		require.True(t, ok)
		assert.Equal(t, apiHostRule+" && !PathPrefix(`/metrics`) && !PathPrefix(`/ready`)", router.Rule)
	})

	t.Run("Edge: no router other than the api catch-all matches /ready", func(t *testing.T) {
		for name, router := range routes.HTTP.Routers {
			if name == "app" || !strings.Contains(router.Rule, "API_DOMAIN") {
				continue
			}
			assert.NotContainsf(t, router.Rule, "/ready", "router %s must not expose /ready", name)
		}
	})

	t.Run("Happy path: /health is public behind the probe rate limit", func(t *testing.T) {
		router, ok := routes.HTTP.Routers["app-health"]
		require.True(t, ok, "the edge must define an app-health router")
		assert.Equal(t, apiHostRule+" && Path(`/health`)", router.Rule)
		assert.Equal(t, "app", router.Service)
		assert.Equal(t, []string{"probe-rate-limit"}, router.Middlewares)
	})

	t.Run("Edge: the health router outranks the api catch-all", func(t *testing.T) {
		raw, err := os.ReadFile(dynamicFile)
		require.NoError(t, err)
		var doc struct {
			HTTP struct {
				Routers map[string]struct {
					Priority int `yaml:"priority"`
				} `yaml:"routers"`
				Middlewares map[string]struct {
					RateLimit struct {
						Average int    `yaml:"average"`
						Period  string `yaml:"period"`
						Burst   int    `yaml:"burst"`
					} `yaml:"rateLimit"`
				} `yaml:"middlewares"`
			} `yaml:"http"`
		}
		require.NoError(t, yaml.Unmarshal(raw, &doc))

		assert.Greater(t, doc.HTTP.Routers["app-health"].Priority, doc.HTTP.Routers["app"].Priority)

		limit := doc.HTTP.Middlewares["probe-rate-limit"].RateLimit
		assert.Equal(t, 100, limit.Average)
		assert.Equal(t, "1s", limit.Period)
		assert.Equal(t, 200, limit.Burst)
	})
}
