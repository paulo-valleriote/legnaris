package clientip_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"legnaris/internal/platform/clientip"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// request builds the request shape the resolver consumes: the direct peer is the
// socket address the server sees, and the values are the forwarding chain in
// request order.
func request(t *testing.T, remoteAddr string, forwarded ...string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	req.RemoteAddr = remoteAddr
	for _, value := range forwarded {
		req.Header.Add("X-Forwarded-For", value)
	}
	return req
}

func newDefaultPolicy(t *testing.T) *clientip.Policy {
	t.Helper()

	policy, err := clientip.New(nil)
	require.NoError(t, err)
	return policy
}

// TestPolicy covers the trust matrix: the default set, a configured set that
// replaces the non-loopback portion, the configured-empty fail-closed case and
// an invalid entry (PRQ-03).
func TestPolicy(t *testing.T) {
	t.Run("Happy path: the default trusts loopback, private and link-local networks", func(t *testing.T) {
		policy := newDefaultPolicy(t)

		trusted := []string{
			"127.0.0.1:8080",
			"[::1]:8080",
			"10.1.2.3:1234",
			"172.16.4.5:1234",
			"192.168.1.5:1234",
			"169.254.10.20:1234",
			"[fc00::1]:1234",
			"[fe80::1]:1234",
		}
		for _, peer := range trusted {
			resolved := policy.Resolve(request(t, peer, "203.0.113.7"))
			assert.Equal(t, "203.0.113.7", resolved, "peer %s must be trusted", peer)
		}

		assert.Equal(t, "198.51.100.5", policy.Resolve(request(t, "198.51.100.5:1234", "203.0.113.7")),
			"a public peer must not be trusted")
	})

	t.Run("Edge: a configured set replaces the non-loopback portion and keeps loopback", func(t *testing.T) {
		policy, err := clientip.New([]string{"10.0.0.0/8"})
		require.NoError(t, err)

		assert.Equal(t, "203.0.113.7", policy.Resolve(request(t, "10.9.9.9:1", "203.0.113.7")),
			"the configured prefix must be trusted")
		assert.Equal(t, "203.0.113.7", policy.Resolve(request(t, "127.0.0.1:1", "203.0.113.7")),
			"loopback stays trusted and honours the chain")
		assert.Equal(t, "172.16.0.1", policy.Resolve(request(t, "172.16.0.1:1", "203.0.113.7")),
			"the default private range must be gone")
		assert.Equal(t, "169.254.1.1", policy.Resolve(request(t, "169.254.1.1:1", "203.0.113.7")),
			"the default link-local range must be gone")
	})

	t.Run("Edge: an empty configured set trusts only loopback (fail-closed)", func(t *testing.T) {
		policy, err := clientip.New([]string{})
		require.NoError(t, err)

		assert.Equal(t, "203.0.113.7", policy.Resolve(request(t, "127.0.0.1:1", "203.0.113.7")))
		assert.Equal(t, "10.0.0.1", policy.Resolve(request(t, "10.0.0.1:1", "203.0.113.7")),
			"an empty configured set must not fall back to the default")
	})

	t.Run("Edge: blank entries in a configured set are ignored", func(t *testing.T) {
		policy, err := clientip.New([]string{" ", "10.0.0.0/8", ""})
		require.NoError(t, err)

		assert.Equal(t, "203.0.113.7", policy.Resolve(request(t, "10.0.0.1:1", "203.0.113.7")))
	})

	t.Run("Error: an invalid CIDR is rejected so boot can fail fast", func(t *testing.T) {
		for _, configured := range [][]string{
			{"not-a-cidr"},
			{"10.0.0.0/8", "10.0.0.1"},
			{"10.0.0.0/33"},
		} {
			_, err := clientip.New(configured)
			require.Error(t, err, "configured %v must be rejected", configured)
		}
	})
}

// TestResolve is the normative resolution matrix: trusted peer, untrusted peer,
// absent/malformed chains, multiple headers, ports and IPv6 (PRQ-01, PRQ-02).
func TestResolve(t *testing.T) {
	policy := newDefaultPolicy(t)

	tests := []struct {
		name       string
		remoteAddr string
		forwarded  []string
		want       string
	}{
		{
			name:       "Happy path: trusted peer names one client",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"203.0.113.7"},
			want:       "203.0.113.7",
		},
		{
			name:       "Edge: the nearest untrusted hop wins over a trusted one",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"203.0.113.7, 10.0.0.2"},
			want:       "203.0.113.7",
		},
		{
			name:       "Edge: several trusted proxies still resolve the client",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"203.0.113.7, 10.0.0.2, 10.0.0.3"},
			want:       "203.0.113.7",
		},
		{
			name:       "Edge: an all-trusted chain returns the furthest hop",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"10.0.0.2, 10.0.0.3"},
			want:       "10.0.0.2",
		},
		{
			name:       "Edge: an absent header falls back to the direct peer",
			remoteAddr: "10.0.0.1:1234",
			want:       "10.0.0.1",
		},
		{
			name:       "Edge: an empty header falls back to the direct peer",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{""},
			want:       "10.0.0.1",
		},
		{
			name:       "Edge: an unparseable entry distrusts the whole header",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"203.0.113.7, not-an-address"},
			want:       "10.0.0.1",
		},
		{
			name:       "Edge: two header values are walked as one ordered chain",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"203.0.113.7", "198.51.100.9"},
			want:       "198.51.100.9",
		},
		{
			name:       "Edge: an untrusted peer ignores a forged chain",
			remoteAddr: "198.51.100.5:1234",
			forwarded:  []string{"203.0.113.7"},
			want:       "198.51.100.5",
		},
		{
			name:       "Edge: an untrusted peer ignores a syntactically valid IPv6 chain",
			remoteAddr: "198.51.100.5:1234",
			forwarded:  []string{"2001:db8::1"},
			want:       "198.51.100.5",
		},
		{
			name:       "Edge: an IPv6 entry without brackets resolves",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"2001:db8::1"},
			want:       "2001:db8::1",
		},
		{
			name:       "Edge: a bracketed IPv6 entry resolves",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"[2001:db8::2]"},
			want:       "2001:db8::2",
		},
		{
			name:       "Edge: an entry with a port resolves",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"198.51.100.7:443"},
			want:       "198.51.100.7",
		},
		{
			name:       "Edge: a bracketed loopback with a port resolves to the bare address",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"[::1]:443"},
			want:       "::1",
		},
		{
			name:       "Edge: an IPv4-mapped IPv6 entry normalizes to IPv4",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  []string{"::ffff:203.0.113.7"},
			want:       "203.0.113.7",
		},
		{
			name:       "Edge: an unparseable peer yields the sentinel and never reads the header",
			remoteAddr: "not-an-address",
			forwarded:  []string{"203.0.113.7"},
			want:       clientip.Unknown,
		},
		{
			name:       "Edge: an empty peer yields the sentinel",
			remoteAddr: "",
			want:       clientip.Unknown,
		},
		{
			name:       "Edge: a bare IPv4 peer without a port is accepted",
			remoteAddr: "10.0.0.1",
			want:       "10.0.0.1",
		},
		{
			name:       "Edge: an IPv4-mapped peer normalizes to IPv4",
			remoteAddr: "::ffff:10.0.0.1",
			want:       "10.0.0.1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := policy.Resolve(request(t, tc.remoteAddr, tc.forwarded...))
			assert.Equal(t, tc.want, got)
			assert.NotEmpty(t, got, "a request must never be keyed by an empty address")
		})
	}
}

// TestResolveSeparateAddresses proves two clients behind one proxy resolve to
// two distinct values, which is what makes their limits independent (SC-001).
func TestResolveSeparateAddresses(t *testing.T) {
	policy := newDefaultPolicy(t)

	first := policy.Resolve(request(t, "10.0.0.1:1234", "203.0.113.10"))
	second := policy.Resolve(request(t, "10.0.0.1:1234", "203.0.113.11"))

	assert.Equal(t, "203.0.113.10", first)
	assert.Equal(t, "203.0.113.11", second)
	assert.NotEqual(t, first, second)
}
