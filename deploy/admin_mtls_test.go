package deploy_test

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	mtlsScript      = "admin/mtls.sh"
	adminCAFile     = "traefik/admin-client-ca.pem"
	adminCAMount    = "/etc/traefik/admin-ca/admin-client-ca.pem"
	adminHostRule   = "Host(`{{ env \"ADMIN_DOMAIN\" }}`)"
	conflictExit    = 73
	noCAExit        = 65
	noSuchUserExit  = 66
	adminMTLSOption = "admin-mtls"
)

type adminTLSDoc struct {
	HTTP struct {
		Routers map[string]struct {
			Rule        string   `yaml:"rule"`
			Service     string   `yaml:"service"`
			EntryPoints []string `yaml:"entryPoints"`
			Middlewares []string `yaml:"middlewares"`
			TLS         *struct {
				CertResolver string `yaml:"certResolver"`
				Options      string `yaml:"options"`
			} `yaml:"tls"`
		} `yaml:"routers"`
	} `yaml:"http"`
	TLS struct {
		Options map[string]struct {
			MinVersion string `yaml:"minVersion"`
			SNIStrict  bool   `yaml:"sniStrict"`
			ClientAuth struct {
				CAFiles        []string `yaml:"caFiles"`
				ClientAuthType string   `yaml:"clientAuthType"`
			} `yaml:"clientAuth"`
		} `yaml:"options"`
	} `yaml:"tls"`
}

func readAdminTLS(t *testing.T) adminTLSDoc {
	t.Helper()
	raw, err := os.ReadFile(dynamicFile)
	require.NoError(t, err)
	var doc adminTLSDoc
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	return doc
}

func TestAdminRoutesRequireClientCertificate(t *testing.T) {
	doc := readAdminTLS(t)

	t.Run("Happy path: the admin router serves only /admin on the admin host", func(t *testing.T) {
		router, ok := doc.HTTP.Routers["admin"]
		require.True(t, ok, "the edge must define an admin router")
		assert.Equal(t, adminHostRule+" && PathPrefix(`/admin`)", router.Rule)
		assert.Equal(t, "app", router.Service)
		assert.Equal(t, []string{"websecure"}, router.EntryPoints)
		assert.Equal(t, []string{"security-headers"}, router.Middlewares)
	})

	t.Run("Happy path: the admin router uses the mTLS options and a public certificate", func(t *testing.T) {
		router := doc.HTTP.Routers["admin"]
		require.NotNil(t, router.TLS)
		assert.Equal(t, adminMTLSOption, router.TLS.Options)
		assert.Equal(t, "letsencrypt", router.TLS.CertResolver)
	})

	t.Run("Happy path: the mTLS options require a verified certificate from the mounted CA", func(t *testing.T) {
		opts, ok := doc.TLS.Options[adminMTLSOption]
		require.True(t, ok, "the edge must define the admin-mtls TLS options")
		assert.Equal(t, "RequireAndVerifyClientCert", opts.ClientAuth.ClientAuthType)
		assert.Equal(t, []string{adminCAMount}, opts.ClientAuth.CAFiles)
		assert.True(t, opts.SNIStrict)
		assert.Equal(t, "VersionTLS12", opts.MinVersion)
	})

	t.Run("Edge: the public api catch-all excludes /admin", func(t *testing.T) {
		assert.Contains(t, doc.HTTP.Routers["app"].Rule, "!PathPrefix(`/admin`)")
	})

	t.Run("Edge: no router other than the admin one names /admin or the admin host", func(t *testing.T) {
		for name, router := range doc.HTTP.Routers {
			if name == "admin" || name == "app" {
				continue
			}
			assert.NotContainsf(t, router.Rule, "/admin", "router %s must not expose /admin", name)
			assert.NotContainsf(t, router.Rule, "ADMIN_DOMAIN", "router %s must not serve the admin host", name)
		}
	})

	t.Run("Edge: every router on the admin host carries the mTLS options", func(t *testing.T) {
		for name, router := range doc.HTTP.Routers {
			if !strings.Contains(router.Rule, "ADMIN_DOMAIN") {
				continue
			}
			require.NotNilf(t, router.TLS, "router %s on the admin host must set TLS options", name)
			assert.Equalf(t, adminMTLSOption, router.TLS.Options, "router %s", name)
		}
	})
}

func TestAdminEdgeWiring(t *testing.T) {
	compose := readCompose(t)

	t.Run("Happy path: traefik mounts the committed admin CA read-only", func(t *testing.T) {
		assert.Contains(t, compose.Services["traefik"].Volumes, "./"+adminCAFile+":"+adminCAMount+":ro")
	})

	t.Run("Happy path: traefik receives the admin host with a default", func(t *testing.T) {
		assert.Equal(t, "${ADMIN_DOMAIN:-admin-api-legnaris.paulovalleriote.io}", compose.Services["traefik"].Environment["ADMIN_DOMAIN"])
	})

	t.Run("Edge: the app receives the allow-list and an unset one disables the backoffice", func(t *testing.T) {
		assert.Equal(t, "${ADMIN_USER_IDS:-}", compose.Services["app"].Environment["ADMIN_USER_IDS"])
	})

	t.Run("Edge: both admin keys stay optional in the production env example", func(t *testing.T) {
		raw, err := os.ReadFile(".env.example")
		require.NoError(t, err)
		lines := strings.Split(string(raw), "\n")
		assert.True(t, slices.Contains(lines, "# ADMIN_DOMAIN=admin-api-legnaris.paulovalleriote.io"))
		assert.True(t, slices.Contains(lines, "# ADMIN_USER_IDS="))
	})

	t.Run("Edge: the committed admin CA is a CA certificate and holds no private key", func(t *testing.T) {
		raw, err := os.ReadFile(adminCAFile)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), "PRIVATE KEY")
		block, rest := pem.Decode(raw)
		require.NotNil(t, block)
		assert.Empty(t, strings.TrimSpace(string(rest)))
		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)
		assert.True(t, cert.IsCA)
	})
}

func requireOpenSSL(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl not installed")
	}
}

func mtlsEnv(dir string) []string {
	return []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "ADMIN_CA_DIR=" + filepath.Join(dir, "ca")}
}

func verifyClient(t *testing.T, caPEM, certPEM string) bool {
	t.Helper()
	return exec.CommandContext(t.Context(), "openssl", "verify", "-purpose", "sslclient", "-CAfile", caPEM, certPEM).Run() == nil
}

func TestAdminMTLSScriptRejectsBadInput(t *testing.T) {
	requireOpenSSL(t)
	dir := t.TempDir()
	env := mtlsEnv(dir)
	caOut := filepath.Join(dir, "ca.pem")

	tests := []struct {
		name string
		args []string
		code int
	}{
		{name: "Error: no command", args: nil, code: usageExit},
		{name: "Error: unknown command", args: []string{"sign"}, code: usageExit},
		{name: "Error: init-ca without an output path", args: []string{"init-ca"}, code: usageExit},
		{name: "Error: issue without a name", args: []string{"issue"}, code: usageExit},
		{name: "Edge: uppercase name", args: []string{"issue", "Alice"}, code: usageExit},
		{name: "Edge: path traversal in the name", args: []string{"issue", "../ca"}, code: usageExit},
		{name: "Edge: name longer than 32 characters", args: []string{"issue", "a" + strings.Repeat("b", 32)}, code: usageExit},
		{name: "Edge: zero days", args: []string{"issue", "alice", "0"}, code: usageExit},
		{name: "Edge: more than a year", args: []string{"issue", "alice", "366"}, code: usageExit},
		{name: "Edge: non-numeric days", args: []string{"issue", "alice", "9x"}, code: usageExit},
		{name: "Error: issue before init-ca", args: []string{"issue", "alice"}, code: noCAExit},
		{name: "Error: revoke without an output path", args: []string{"revoke", "alice"}, code: usageExit},
		{name: "Error: revoke before init-ca", args: []string{"revoke", "alice", caOut}, code: noCAExit},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := runScript(t, mtlsScript, env, "", tc.args...)
			assert.Equal(t, tc.code, res.code, res.output)
		})
	}

	t.Run("Edge: a refused command creates no CA", func(t *testing.T) {
		_, err := os.Stat(filepath.Join(dir, "ca"))
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestAdminMTLSScriptLifecycle(t *testing.T) {
	requireOpenSSL(t)
	dir := t.TempDir()
	env := mtlsEnv(dir)
	ca := filepath.Join(dir, "ca")
	caOut := filepath.Join(dir, "published.pem")
	cert := func(name string) string { return filepath.Join(ca, "clients", name, "cert.pem") }

	res := runScript(t, mtlsScript, env, "", "init-ca", caOut)
	require.Equal(t, 0, res.code, res.output)

	t.Run("Happy path: init-ca publishes only the public certificate", func(t *testing.T) {
		raw, err := os.ReadFile(caOut)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), "PRIVATE KEY")
		info, err := os.Stat(filepath.Join(ca, "ca.key"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("Edge: a second init-ca refuses to replace the CA", func(t *testing.T) {
		res := runScript(t, mtlsScript, env, "", "init-ca", caOut)
		assert.Equal(t, conflictExit, res.code, res.output)
	})

	for _, name := range []string{"alice", "bob"} {
		res := runScript(t, mtlsScript, env, "", "issue", name, "30")
		require.Equal(t, 0, res.code, res.output)
	}

	t.Run("Happy path: issued certificates verify as clients against the published CA", func(t *testing.T) {
		assert.True(t, verifyClient(t, caOut, cert("alice")))
		assert.True(t, verifyClient(t, caOut, cert("bob")))
		info, err := os.Stat(filepath.Join(ca, "clients", "alice", "key.pem"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("Edge: issuing an existing name is refused", func(t *testing.T) {
		res := runScript(t, mtlsScript, env, "", "issue", "alice")
		assert.Equal(t, conflictExit, res.code, res.output)
	})

	t.Run("Error: revoking an unknown client is refused", func(t *testing.T) {
		res := runScript(t, mtlsScript, env, "", "revoke", "carol", caOut)
		assert.Equal(t, noSuchUserExit, res.code, res.output)
	})

	aliceOld := filepath.Join(dir, "alice-old.pem")
	raw, err := os.ReadFile(cert("alice"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(aliceOld, raw, 0o600))

	res = runScript(t, mtlsScript, env, "", "revoke", "alice", caOut)
	require.Equal(t, 0, res.code, res.output)

	t.Run("Happy path: revoke rotates the CA so the revoked certificate stops verifying", func(t *testing.T) {
		assert.False(t, verifyClient(t, caOut, aliceOld))
	})

	t.Run("Happy path: revoke reissues the remaining clients under the new CA", func(t *testing.T) {
		assert.True(t, verifyClient(t, caOut, cert("bob")))
	})

	t.Run("Edge: revoking the same client twice is refused", func(t *testing.T) {
		res := runScript(t, mtlsScript, env, "", "revoke", "alice", caOut)
		assert.Equal(t, noSuchUserExit, res.code, res.output)
	})

	t.Run("Happy path: list shows active and revoked clients", func(t *testing.T) {
		res := runScript(t, mtlsScript, env, "", "list")
		require.Equal(t, 0, res.code, res.output)
		assert.Equal(t, "clients bob\nrevoked alice\n", res.output)
	})
}
