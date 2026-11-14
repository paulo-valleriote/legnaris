package trial_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/internal/trial"
	"legnaris/backend/internal/trial/mocks"
	"legnaris/backend/shared/apperr"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) read() time.Time { return c.now }

func newLimiter(c *fakeClock) *ratelimit.Limiter {
	return ratelimit.New(trial.MaxAccountsPerIP, trial.IPWindow, c.read)
}

func verified(userID, ip string) trial.Candidate {
	return trial.Candidate{UserID: userID, Email: "ana@example.com", EmailVerified: true, ClientIP: ip}
}

func TestGateEvaluate(t *testing.T) {
	t0 := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		candidate       trial.Candidate
		disposable      bool
		checkErr        error
		grantErr        error
		wantDecision    trial.Decision
		wantErr         bool
		wantGrant       int
		wantDomainCheck int
	}{
		{
			name:            "Happy path: verified, non-disposable email under the IP cap gets the allowance",
			candidate:       verified("user-1", "203.0.113.7"),
			wantDecision:    trial.DecisionGranted,
			wantGrant:       1,
			wantDomainCheck: 1,
		},
		{
			name:         "Edge: unverified email gets no allowance and no side effects",
			candidate:    trial.Candidate{UserID: "user-1", Email: "ana@example.com", ClientIP: "203.0.113.7"},
			wantDecision: trial.DecisionNotVerified,
		},
		{
			name:            "Edge: disposable domain gets no allowance",
			candidate:       verified("user-1", "203.0.113.7"),
			disposable:      true,
			wantDecision:    trial.DecisionDisposable,
			wantDomainCheck: 1,
		},
		{
			name:         "Edge: empty IP is refused before any shared counter is touched",
			candidate:    verified("user-1", ""),
			wantDecision: trial.DecisionInvalidIP,
		},
		{
			name:         "Edge: the clientip unknown sentinel is not a usable IP",
			candidate:    verified("user-1", "unknown"),
			wantDecision: trial.DecisionInvalidIP,
		},
		{
			name:         "Edge: malformed IP is refused",
			candidate:    verified("user-1", "300.1.1.1"),
			wantDecision: trial.DecisionInvalidIP,
		},
		{
			name:            "Edge: IPv6 address is accepted as a key",
			candidate:       verified("user-1", "2001:db8::1"),
			wantDecision:    trial.DecisionGranted,
			wantGrant:       1,
			wantDomainCheck: 1,
		},
		{
			name:            "Error: domain checker failure fails closed without a grant",
			candidate:       verified("user-1", "203.0.113.7"),
			checkErr:        errors.New("resolver down"),
			wantErr:         true,
			wantDomainCheck: 1,
		},
		{
			name:            "Error: grant port failure is returned to the caller",
			candidate:       verified("user-1", "203.0.113.7"),
			grantErr:        errors.New("mongo down"),
			wantErr:         true,
			wantGrant:       1,
			wantDomainCheck: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clock := &fakeClock{now: t0}
			granter := mocks.NewMockGranter(t)
			domains := mocks.NewMockDomainChecker(t)

			if tc.wantDomainCheck > 0 {
				domains.EXPECT().IsDisposable(mock.Anything, tc.candidate.Email).
					Return(tc.disposable, tc.checkErr).Times(tc.wantDomainCheck)
			}
			if tc.wantGrant > 0 {
				granter.EXPECT().Grant(mock.Anything, tc.candidate.UserID).
					Return(tc.grantErr).Times(tc.wantGrant)
			}

			gate := trial.NewGate(granter, domains, newLimiter(clock))
			got, err := gate.Evaluate(context.Background(), tc.candidate)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantDecision, got)
		})
	}
}

func TestGateEvaluateEmptyUserID(t *testing.T) {
	t0 := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		userID   string
		wantCode apperr.Code
	}{
		{
			name:     "Error: empty user id is an invalid request",
			userID:   "",
			wantCode: apperr.CodeInvalidRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gate := trial.NewGate(mocks.NewMockGranter(t), mocks.NewMockDomainChecker(t), newLimiter(&fakeClock{now: t0}))
			_, err := gate.Evaluate(context.Background(), verified(tc.userID, "203.0.113.7"))

			var typed *apperr.Error
			require.ErrorAs(t, err, &typed)
			assert.Equal(t, tc.wantCode, typed.Code)
		})
	}
}

func TestGateIPCap(t *testing.T) {
	t0 := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

	t.Run("Edge: the 4th account from one IP in 24h gets no allowance but is not an error", func(t *testing.T) {
		clock := &fakeClock{now: t0}
		granter := mocks.NewMockGranter(t)
		domains := mocks.NewMockDomainChecker(t)
		domains.EXPECT().IsDisposable(mock.Anything, "ana@example.com").Return(false, nil).Times(4)
		granter.EXPECT().Grant(mock.Anything, mock.Anything).Return(nil).Times(3)

		gate := trial.NewGate(granter, domains, newLimiter(clock))
		for _, id := range []string{"user-a", "user-b", "user-c"} {
			decision, err := gate.Evaluate(context.Background(), verified(id, "203.0.113.7"))
			require.NoError(t, err)
			require.Equal(t, trial.DecisionGranted, decision)
		}

		decision, err := gate.Evaluate(context.Background(), verified("user-d", "203.0.113.7"))
		require.NoError(t, err)
		assert.Equal(t, trial.DecisionIPLimitReached, decision)
	})

	t.Run("Edge: a refused disposable or unverified candidate does not consume an IP slot", func(t *testing.T) {
		clock := &fakeClock{now: t0}
		granter := mocks.NewMockGranter(t)
		domains := mocks.NewMockDomainChecker(t)
		domains.EXPECT().IsDisposable(mock.Anything, "spam@mailinator.test").Return(true, nil).Once()
		domains.EXPECT().IsDisposable(mock.Anything, "ana@example.com").Return(false, nil).Times(3)
		granter.EXPECT().Grant(mock.Anything, mock.Anything).Return(nil).Times(3)

		gate := trial.NewGate(granter, domains, newLimiter(clock))

		disposable := verified("user-x", "203.0.113.7")
		disposable.Email = "spam@mailinator.test"
		decision, err := gate.Evaluate(context.Background(), disposable)
		require.NoError(t, err)
		require.Equal(t, trial.DecisionDisposable, decision)

		unverified := trial.Candidate{UserID: "user-y", Email: "ana@example.com", ClientIP: "203.0.113.7"}
		decision, err = gate.Evaluate(context.Background(), unverified)
		require.NoError(t, err)
		require.Equal(t, trial.DecisionNotVerified, decision)

		for _, id := range []string{"user-a", "user-b", "user-c"} {
			decision, err := gate.Evaluate(context.Background(), verified(id, "203.0.113.7"))
			require.NoError(t, err)
			assert.Equal(t, trial.DecisionGranted, decision)
		}
	})

	t.Run("Edge: the cap is per IP, not global", func(t *testing.T) {
		clock := &fakeClock{now: t0}
		granter := mocks.NewMockGranter(t)
		domains := mocks.NewMockDomainChecker(t)
		domains.EXPECT().IsDisposable(mock.Anything, mock.Anything).Return(false, nil).Times(4)
		granter.EXPECT().Grant(mock.Anything, mock.Anything).Return(nil).Times(4)

		gate := trial.NewGate(granter, domains, newLimiter(clock))
		for _, id := range []string{"user-a", "user-b", "user-c"} {
			_, err := gate.Evaluate(context.Background(), verified(id, "203.0.113.7"))
			require.NoError(t, err)
		}
		decision, err := gate.Evaluate(context.Background(), verified("user-d", "198.51.100.9"))
		require.NoError(t, err)
		assert.Equal(t, trial.DecisionGranted, decision)
	})

	t.Run("Edge: window boundary is exactly 24h after the oldest grant", func(t *testing.T) {
		clock := &fakeClock{now: t0}
		granter := mocks.NewMockGranter(t)
		domains := mocks.NewMockDomainChecker(t)
		domains.EXPECT().IsDisposable(mock.Anything, mock.Anything).Return(false, nil).Times(5)
		granter.EXPECT().Grant(mock.Anything, mock.Anything).Return(nil).Times(4)

		gate := trial.NewGate(granter, domains, newLimiter(clock))
		for _, id := range []string{"user-a", "user-b", "user-c"} {
			_, err := gate.Evaluate(context.Background(), verified(id, "203.0.113.7"))
			require.NoError(t, err)
		}

		clock.now = t0.Add(trial.IPWindow - time.Nanosecond)
		decision, err := gate.Evaluate(context.Background(), verified("user-d", "203.0.113.7"))
		require.NoError(t, err)
		assert.Equal(t, trial.DecisionIPLimitReached, decision, "one nanosecond before 24h the cap still holds")

		clock.now = t0.Add(trial.IPWindow)
		decision, err = gate.Evaluate(context.Background(), verified("user-e", "203.0.113.7"))
		require.NoError(t, err)
		assert.Equal(t, trial.DecisionGranted, decision, "at exactly 24h the oldest grant has aged out")
	})

	t.Run("Edge: concurrent registrations from one IP grant exactly the cap", func(t *testing.T) {
		clock := &fakeClock{now: t0}
		const attempts = 20
		granter := mocks.NewMockGranter(t)
		domains := mocks.NewMockDomainChecker(t)
		domains.EXPECT().IsDisposable(mock.Anything, mock.Anything).Return(false, nil).Times(attempts)
		granter.EXPECT().Grant(mock.Anything, mock.Anything).Return(nil).Times(trial.MaxAccountsPerIP)

		gate := trial.NewGate(granter, domains, newLimiter(clock))

		var (
			wg      sync.WaitGroup
			mu      sync.Mutex
			granted int
		)
		for i := 0; i < attempts; i++ {
			wg.Add(1)
			go func(userID string) {
				defer wg.Done()
				decision, err := gate.Evaluate(context.Background(), verified(userID, "203.0.113.7"))
				if err != nil || decision != trial.DecisionGranted {
					return
				}
				mu.Lock()
				granted++
				mu.Unlock()
			}("user-" + string(rune('a'+i)))
		}
		wg.Wait()
		assert.Equal(t, trial.MaxAccountsPerIP, granted)
	})
}
