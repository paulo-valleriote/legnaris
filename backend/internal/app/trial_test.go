package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/auth"
	authmocks "legnaris/backend/internal/auth/mocks"
	"legnaris/backend/internal/email/disposable"
	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/internal/trial"
	trialmocks "legnaris/backend/internal/trial/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	trialUserID    = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e0f"
	trialEmail     = "ana@example.com"
	trialClientIP  = "203.0.113.7"
	trialRefusedIP = "203.0.113.8"
)

var errCheckerDown = errors.New("resolver unavailable")

func TestTrialDomainCheckerIsDisposable(t *testing.T) {
	tests := []struct {
		name         string
		checkErr     error
		wantDisposed bool
		wantErr      bool
	}{
		{name: "Happy path: an accepted address is not disposable", checkErr: nil, wantDisposed: false},
		{name: "Happy path: the typed disposable refusal marks the domain disposable", checkErr: apperr.NewError(apperr.CodeDisposableEmail, "refused", nil), wantDisposed: true},
		{name: "Happy path: the disposable checker's refusal marks the domain disposable", checkErr: disposable.ErrRefused, wantDisposed: true},
		{name: "Edge: a refusal wrapped by another layer is still disposable", checkErr: fmt.Errorf("register: %w", disposable.ErrRefused), wantDisposed: true},
		{name: "Edge: a typed refusal wrapped by another layer is still disposable", checkErr: fmt.Errorf("check: %w", apperr.NewError(apperr.CodeDisposableEmail, "refused", nil)), wantDisposed: true},
		{name: "Error: a failed check is an error and grants nothing", checkErr: errCheckerDown, wantDisposed: false, wantErr: true},
		{name: "Error: an unrelated typed error is an error, not a disposable verdict", checkErr: apperr.NewError(apperr.CodeInternal, "boom", nil), wantDisposed: false, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			checker := authmocks.NewMockEmailChecker(t)
			checker.EXPECT().Check(mock.Anything, trialEmail).Return(tc.checkErr).Once()

			disposed, err := trialDomainChecker{checker: checker}.IsDisposable(context.Background(), trialEmail)

			assert.Equal(t, tc.wantDisposed, disposed)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func newTestTrialVerifier(t *testing.T, granter trial.Granter, checker auth.EmailChecker) auth.EmailVerifier {
	t.Helper()
	ips := ratelimit.New(trial.MaxAccountsPerIP, trial.IPWindow, time.Now)
	return newTrialVerifier(granter, checker, ips, logger.NewLogger(slog.LevelError))
}

func TestTrialVerifierEmailVerified(t *testing.T) {
	t.Run("Happy path: a verified, accepted account is granted the allowance", func(t *testing.T) {
		granter := trialmocks.NewMockGranter(t)
		checker := authmocks.NewMockEmailChecker(t)
		checker.EXPECT().Check(mock.Anything, trialEmail).Return(nil).Once()
		granter.EXPECT().Grant(mock.Anything, trialUserID).Return(nil).Once()

		newTestTrialVerifier(t, granter, checker).EmailVerified(context.Background(), trialUserID, trialEmail, trialClientIP)
	})

	t.Run("Edge: a disposable domain is refused without consuming a grant", func(t *testing.T) {
		granter := trialmocks.NewMockGranter(t)
		checker := authmocks.NewMockEmailChecker(t)
		checker.EXPECT().Check(mock.Anything, trialEmail).Return(disposable.ErrRefused).Once()

		newTestTrialVerifier(t, granter, checker).EmailVerified(context.Background(), trialUserID, trialEmail, trialClientIP)
	})

	t.Run("Edge: an unresolvable client address is refused before any grant", func(t *testing.T) {
		granter := trialmocks.NewMockGranter(t)
		checker := authmocks.NewMockEmailChecker(t)

		newTestTrialVerifier(t, granter, checker).EmailVerified(context.Background(), trialUserID, trialEmail, "unknown")
	})

	t.Run("Edge: the fourth account from one address gets no allowance and is not an error", func(t *testing.T) {
		granter := trialmocks.NewMockGranter(t)
		checker := authmocks.NewMockEmailChecker(t)
		checker.EXPECT().Check(mock.Anything, mock.Anything).Return(nil)
		granter.EXPECT().Grant(mock.Anything, mock.Anything).Return(nil).Times(trial.MaxAccountsPerIP)

		verifier := newTestTrialVerifier(t, granter, checker)
		for i := 0; i < trial.MaxAccountsPerIP+1; i++ {
			verifier.EmailVerified(context.Background(), trialUserID, trialEmail, trialRefusedIP)
		}
	})

	t.Run("Error: a failed domain check grants nothing and does not panic", func(t *testing.T) {
		granter := trialmocks.NewMockGranter(t)
		checker := authmocks.NewMockEmailChecker(t)
		checker.EXPECT().Check(mock.Anything, trialEmail).Return(errCheckerDown).Once()

		newTestTrialVerifier(t, granter, checker).EmailVerified(context.Background(), trialUserID, trialEmail, trialClientIP)
	})

	t.Run("Error: a failed grant is logged and never reported to the verification", func(t *testing.T) {
		granter := trialmocks.NewMockGranter(t)
		checker := authmocks.NewMockEmailChecker(t)
		checker.EXPECT().Check(mock.Anything, trialEmail).Return(nil).Once()
		granter.EXPECT().Grant(mock.Anything, trialUserID).Return(errCheckerDown).Once()

		newTestTrialVerifier(t, granter, checker).EmailVerified(context.Background(), trialUserID, trialEmail, trialClientIP)
	})

	t.Run("Edge: an empty user id is logged and grants nothing", func(t *testing.T) {
		granter := trialmocks.NewMockGranter(t)
		checker := authmocks.NewMockEmailChecker(t)

		newTestTrialVerifier(t, granter, checker).EmailVerified(context.Background(), "", trialEmail, trialClientIP)
	})
}
