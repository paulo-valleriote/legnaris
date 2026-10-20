// Package clientip resolves the address every per-client decision is keyed by
// when the API is reached only through the edge proxy. The forwarded chain is
// believed only as far as the hops are trusted, so a request from an untrusted
// peer can never choose its own key (PRQ-01, PRQ-02).
package clientip

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Unknown is the non-empty sentinel returned when the direct peer cannot be
// parsed. Such a request cannot be attributed, so it shares one fail-closed
// bucket instead of gaining an independent budget (PRQ-03).
const Unknown = "unknown"

// forwardedHeader is the only header the deployment proxy forwards the chain
// in; X-Real-IP is deliberately not a second source of truth.
const forwardedHeader = "X-Forwarded-For"

// defaultTrusted is the documented default: loopback, private (RFC 1918 and
// ULA) and link-local networks, which is where the deployment's proxy lives.
var defaultTrusted = []string{
	"127.0.0.0/8",
	"::1/128",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"fc00::/7",
	"169.254.0.0/16",
	"fe80::/10",
}

// loopbackTrusted is always trusted, even when the operator narrows the set:
// local development and same-host proxies rely on it.
var loopbackTrusted = []string{"127.0.0.0/8", "::1/128"}

// Policy is an immutable trust set built once at boot. Resolve performs no
// configuration lookup.
type Policy struct {
	prefixes []netip.Prefix
}

// New builds the trust policy from the operator's comma-separated CIDR list. A
// nil list selects the documented default; any other list trusts loopback plus
// exactly the configured prefixes, so an empty non-nil list ("," in the
// environment) trusts only loopback. An invalid entry is an error so the API
// fails to boot instead of silently trusting the wrong network (PRQ-03).
func New(configured []string) (*Policy, error) {
	sources := defaultTrusted
	if configured != nil {
		sources = loopbackTrusted
	}

	prefixes := make([]netip.Prefix, 0, len(sources)+len(configured))
	seen := make(map[netip.Prefix]struct{})
	add := func(raw string) error {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("parse trusted proxy CIDR %q: %w", raw, err)
		}
		prefix = prefix.Masked()
		if _, ok := seen[prefix]; ok {
			return nil
		}
		seen[prefix] = struct{}{}
		prefixes = append(prefixes, prefix)
		return nil
	}

	for _, raw := range sources {
		if err := add(raw); err != nil {
			return nil, err
		}
	}
	for _, raw := range configured {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if err := add(raw); err != nil {
			return nil, err
		}
	}

	return &Policy{prefixes: prefixes}, nil
}

// Resolve returns the client address for the request. The result is never
// empty: an unparseable peer yields Unknown, an untrusted peer yields itself,
// and a trusted peer yields the nearest untrusted hop or, when every hop is
// trusted, the furthest named one (PRQ-01, PRQ-02, PRQ-03).
func (p *Policy) Resolve(r *http.Request) string {
	peer, ok := parseAddress(r.RemoteAddr)
	if !ok {
		return Unknown
	}
	if !p.trusted(peer) {
		return peer.String()
	}

	chain, ok := forwardingChain(r)
	if !ok || len(chain) == 0 {
		return peer.String()
	}

	for i := len(chain) - 1; i >= 0; i-- {
		if !p.trusted(chain[i]) {
			return chain[i].String()
		}
	}
	return chain[0].String()
}

func (p *Policy) trusted(addr netip.Addr) bool {
	for _, prefix := range p.prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// forwardingChain flattens every X-Forwarded-For value, in request order, into
// one ordered chain. An entry that cannot be parsed distrusts the whole header,
// which the caller reports by falling back to the direct peer (PRQ-01).
func forwardingChain(r *http.Request) ([]netip.Addr, bool) {
	values := r.Header.Values(forwardedHeader)
	if len(values) == 0 {
		return nil, true
	}

	chain := make([]netip.Addr, 0, len(values))
	for _, value := range values {
		for _, entry := range strings.Split(value, ",") {
			addr, ok := parseAddress(entry)
			if !ok {
				return nil, false
			}
			chain = append(chain, addr)
		}
	}
	return chain, true
}

// parseAddress accepts a bare address, host:port, an IPv6 address with or
// without brackets, and an IPv6 address with a port. The result is unmapped so
// an IPv4-mapped IPv6 address and its IPv4 form share one key.
func parseAddress(raw string) (netip.Addr, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return netip.Addr{}, false
	}

	if addr, err := netip.ParseAddr(raw); err == nil {
		return addr.Unmap(), true
	}

	if host, _, err := net.SplitHostPort(raw); err == nil {
		if addr, err := netip.ParseAddr(host); err == nil {
			return addr.Unmap(), true
		}
	}

	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		if addr, err := netip.ParseAddr(raw[1 : len(raw)-1]); err == nil {
			return addr.Unmap(), true
		}
	}

	return netip.Addr{}, false
}

// Ensure the resolver keeps the signature Echo consults through
// Echo.IPExtractor (PRQ-04).
var _ func(*http.Request) string = (*Policy)(nil).Resolve
