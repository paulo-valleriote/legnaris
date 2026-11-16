package clientip

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

const Unknown = "unknown"

const forwardedHeader = "X-Forwarded-For"

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

var loopbackTrusted = []string{"127.0.0.0/8", "::1/128"}

type Policy struct {
	prefixes []netip.Prefix
}

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

var _ func(*http.Request) string = (*Policy)(nil).Resolve
