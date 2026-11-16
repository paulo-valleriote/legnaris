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

var ErrRefused = errors.New("e-mail domain cannot receive mail")

const mxTimeout = 2 * time.Second

//go:embed blocklist.txt
var blocklistSource []byte

type LookupMXFunc func(ctx context.Context, name string) ([]*net.MX, error)

type Checker struct {
	blocked map[string]struct{}
	lookup  LookupMXFunc
}

func NewChecker(lookup LookupMXFunc) *Checker {
	if lookup == nil {
		lookup = net.DefaultResolver.LookupMX
	}
	return &Checker{blocked: parseBlocklist(blocklistSource), lookup: lookup}
}

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

func domainOf(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(email[at+1:]), "."))
}

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
