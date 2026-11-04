package disposable_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/email/disposable"
)

// fakeLookup answers every MX lookup with the same outcome and records how
// many lookups ran, so tests can assert the blocklist short-circuits DNS.
type fakeLookup struct {
	records []*net.MX
	err     error
	calls   int
}

func (f *fakeLookup) lookup(_ context.Context, _ string) ([]*net.MX, error) {
	f.calls++
	return f.records, f.err
}

func mxRecords(hosts ...string) []*net.MX {
	records := make([]*net.MX, 0, len(hosts))
	for _, host := range hosts {
		records = append(records, &net.MX{Host: host, Pref: 10})
	}
	return records
}

func TestChecker_Check(t *testing.T) {
	ctx := context.Background()
	notFound := &net.DNSError{Err: "no such host", Name: "x", IsNotFound: true}
	timeout := &net.DNSError{Err: "i/o timeout", Name: "x", IsTimeout: true}

	tests := []struct {
		name      string
		email     string
		lookup    *fakeLookup
		wantErr   error
		wantCalls int
	}{
		{
			name:      "Happy path: legitimate domain with a mail exchange is accepted",
			email:     "player@example.com",
			lookup:    &fakeLookup{records: mxRecords("mx.example.com.")},
			wantCalls: 1,
		},
		{
			name:      "Happy path: blocklisted domain is refused without a DNS lookup",
			email:     "player@mailinator.com",
			lookup:    &fakeLookup{records: mxRecords("mx.mailinator.com.")},
			wantErr:   disposable.ErrRefused,
			wantCalls: 0,
		},
		{
			name:      "Edge: upper-case blocklisted domain is refused",
			email:     "Player@MAILINATOR.COM",
			lookup:    &fakeLookup{records: mxRecords("mx.mailinator.com.")},
			wantErr:   disposable.ErrRefused,
			wantCalls: 0,
		},
		{
			name:      "Edge: subdomain of a blocklisted domain is refused",
			email:     "player@inbox.guerrillamail.com",
			lookup:    &fakeLookup{records: mxRecords("mx.example.com.")},
			wantErr:   disposable.ErrRefused,
			wantCalls: 0,
		},
		{
			name:      "Edge: domain that only shares a suffix with a blocklisted one is accepted",
			email:     "player@notmailinator.com",
			lookup:    &fakeLookup{records: mxRecords("mx.example.com.")},
			wantCalls: 1,
		},
		{
			name:      "Edge: trailing root dot on the domain is ignored",
			email:     "player@mailinator.com.",
			lookup:    &fakeLookup{records: mxRecords("mx.mailinator.com.")},
			wantErr:   disposable.ErrRefused,
			wantCalls: 0,
		},
		{
			name:      "Edge: NXDOMAIN answer refuses the domain",
			email:     "player@nonexistent.example",
			lookup:    &fakeLookup{err: notFound},
			wantErr:   disposable.ErrRefused,
			wantCalls: 1,
		},
		{
			name:      "Edge: empty MX answer refuses the domain",
			email:     "player@nomx.example",
			lookup:    &fakeLookup{records: nil},
			wantErr:   disposable.ErrRefused,
			wantCalls: 1,
		},
		{
			name:      "Edge: null MX (RFC 7505) refuses the domain",
			email:     "player@nullmx.example",
			lookup:    &fakeLookup{records: mxRecords(".")},
			wantErr:   disposable.ErrRefused,
			wantCalls: 1,
		},
		{
			name:      "Edge: a nil MX record is skipped rather than trusted",
			email:     "player@nilrecord.example",
			lookup:    &fakeLookup{records: []*net.MX{nil}},
			wantErr:   disposable.ErrRefused,
			wantCalls: 1,
		},
		{
			name:      "Edge: DNS timeout fails open and accepts the address",
			email:     "player@slow.example",
			lookup:    &fakeLookup{err: timeout},
			wantCalls: 1,
		},
		{
			name:      "Edge: non-NotFound resolver error fails open",
			email:     "player@broken.example",
			lookup:    &fakeLookup{err: errors.New("resolver unavailable")},
			wantCalls: 1,
		},
		{
			name:      "Edge: address without an @ is not checked",
			email:     "not-an-address",
			lookup:    &fakeLookup{},
			wantCalls: 0,
		},
		{
			name:      "Edge: address with an empty domain is not checked",
			email:     "player@",
			lookup:    &fakeLookup{},
			wantCalls: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			checker := disposable.NewChecker(tc.lookup.lookup)

			err := checker.Check(ctx, tc.email)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantCalls, tc.lookup.calls)
		})
	}
}

func TestChecker_CheckCanceledContextFailsOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	lookup := &fakeLookup{err: context.Canceled}

	err := disposable.NewChecker(lookup.lookup).Check(ctx, "player@example.com")

	require.NoError(t, err, "a canceled lookup is an infrastructure outcome, not a refusal")
}

func TestChecker_CheckLookupRespectsTimeout(t *testing.T) {
	var deadline time.Time
	lookup := func(ctx context.Context, _ string) ([]*net.MX, error) {
		var ok bool
		deadline, ok = ctx.Deadline()
		require.True(t, ok, "the MX lookup must carry a deadline")
		return mxRecords("mx.example.com."), nil
	}

	before := time.Now()
	require.NoError(t, disposable.NewChecker(lookup).Check(context.Background(), "player@example.com"))

	remaining := deadline.Sub(before)
	assert.Greater(t, remaining, time.Second, "the lookup budget is about 2s")
	assert.LessOrEqual(t, remaining, 2*time.Second+100*time.Millisecond)
}

func TestNewChecker_NilLookupUsesDefaultResolver(t *testing.T) {
	t.Run("Edge: a nil lookup is accepted and does not panic on a blocklisted domain", func(t *testing.T) {
		checker := disposable.NewChecker(nil)

		err := checker.Check(context.Background(), "player@yopmail.com")

		require.ErrorIs(t, err, disposable.ErrRefused)
	})
}
