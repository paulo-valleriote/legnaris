package disposable

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"errors"
	"net"
	"strings"
	"time"
)

// ErrRefused is the cause a Checker returns when the address's domain is
// blocklisted or has no mail exchange. Callers map it to a single user-facing
// refusal.
var ErrRefused = errors.New("e-mail domain cannot receive mail")

// mxTimeout bounds the MX lookup. A slow resolver must not hold up registration.
const mxTimeout = 2 * time.Second

//go:embed blocklist.txt
var blocklistSource []byte

// LookupMXFunc resolves the mail exchanges of a domain. net.Resolver.LookupMX
// satisfies it; tests inject a fake.
type LookupMXFunc func(ctx context.Context, name string) ([]*net.MX, error)

// Checker applies the blocklist and then the MX check. The MX check fails open:
// a lookup error or timeout accepts the address, because a DNS outage must not
// block signups.
type Checker struct {
	blocked map[string]struct{}
	lookup  LookupMXFunc
}

// NewChecker returns a Checker that resolves MX records with lookup. A nil
// lookup uses the process's default resolver.
func NewChecker(lookup LookupMXFunc) *Checker {
	if lookup == nil {
		lookup = net.DefaultResolver.LookupMX
	}
	return &Checker{blocked: parseBlocklist(blocklistSource), lookup: lookup}
}

// Check returns ErrRefused when the address's domain is blocklisted or has no
// mail exchange, and nil otherwise. An address without a domain returns nil:
// format validation belongs to the caller, not to this check.
func (c *Checker) Check(ctx context.Context, email string) error {
	domain := domainOf(email)
	if domain == "" {
		return nil
	}
	if c.isBlocked(domain) {
		return ErrRefused
	}
	return c.checkMX(ctx, domain)
}

func (c *Checker) checkMX(ctx context.Context, domain string) error {
	ctx, cancel := context.WithTimeout(ctx, mxTimeout)
	defer cancel()

	records, err := c.lookup(ctx, domain)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return ErrRefused
		}
		return nil
	}
	if !hasMailExchange(records) {
		return ErrRefused
	}
	return nil
}

// hasMailExchange reports whether any record names a real host. The null MX
// (RFC 7505, host ".") declares that the domain accepts no mail.
func hasMailExchange(records []*net.MX) bool {
	for _, record := range records {
		if record == nil {
			continue
		}
		host := strings.TrimSuffix(record.Host, ".")
		if host != "" {
			return true
		}
	}
	return false
}

// isBlocked matches the domain and each of its parent domains, so a listed
// provider also refuses its subdomains.
func (c *Checker) isBlocked(domain string) bool {
	for candidate := domain; candidate != ""; {
		if _, found := c.blocked[candidate]; found {
			return true
		}
		_, rest, found := strings.Cut(candidate, ".")
		if !found {
			return false
		}
		candidate = rest
	}
	return false
}

// domainOf returns the lowercased domain after the last '@', without a
// trailing root dot. It returns "" when there is no '@' or the domain is empty.
func domainOf(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(email[at+1:]), "."))
}

// parseBlocklist reads one domain per line, skipping blank lines and comments.
func parseBlocklist(source []byte) map[string]struct{} {
	blocked := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(source))
	for scanner.Scan() {
		line := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		blocked[line] = struct{}{}
	}
	return blocked
}
