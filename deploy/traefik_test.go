package deploy_test

import (
	"os"
	"slices"
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
