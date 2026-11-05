package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/auth"
	"legnaris/backend/shared/apperr"
)

// TestChallengePolicy_ResendDayCap pins the resend share of the daily cap: the
// floor of the cap times the ratio, so resends never take the first sends'
// reserved part (D5).
func TestChallengePolicy_ResendDayCap(t *testing.T) {
	tests := []struct {
		name  string
		cap   int
		ratio float64
		want  int
	}{
		{name: "Happy path: the documented 80 at 30% leaves 24 resends", cap: 80, ratio: 0.30, want: 24},
		{name: "Edge: the floor drops the fraction", cap: 3, ratio: 0.5, want: 1},
		{name: "Edge: a share below one whole send allows no resend", cap: 1, ratio: 0.30, want: 0},
		{name: "Edge: a zero cap allows no resend", cap: 0, ratio: 0.30, want: 0},
		{name: "Edge: a full ratio gives resends the whole cap", cap: 80, ratio: 1, want: 80},
		{name: "Edge: the exact boundary 24 of 80 is kept, not floored to 23", cap: 80, ratio: 0.3, want: 24},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy := auth.ChallengePolicy{DailySendCap: tc.cap, ResendCapRatio: tc.ratio}
			assert.Equal(t, tc.want, policy.ResendDayCap())
		})
	}
}

// TestResendChallenge_ResendShare pins the resend share on the resend path:
// a resend is refused once the day's resends reach the share even while the
// total cap has room, with its own observer result, and a count failure is
// internal (D5, OTP-13, OTP-17).
func TestResendChallenge_ResendShare(t *testing.T) {
	ctx := context.Background()
	storeErr := errors.New("store down")

	tests := []struct {
		name   string
		wire   func(f *challengeFixture)
		assert func(t *testing.T, f *challengeFixture, err error)
	}{
		{
			name: "Edge: resends at the share are refused as reserved",
			wire: func(f *challengeFixture) {
				f.expectSentCount(challengeTestPolicy.DailySendCap - 1)
				f.expectResendCount(challengeTestPolicy.ResendDayCap())
				f.codeObs.EXPECT().ResendRequested(auth.ResendReserved).Once()
			},
			assert: func(t *testing.T, f *challengeFixture, err error) {
				assertAppErr(t, err, apperr.CodeEmailUnavailable)
				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, nextDayRetry(challengeTestNow), appErr.RetryAfter,
					"the refusal retries at the next UTC day")
			},
		},
		{
			name: "Edge: a total cap refusal is still reported as the total cap",
			wire: func(f *challengeFixture) {
				f.expectSentCount(challengeTestPolicy.DailySendCap)
				f.codeObs.EXPECT().SendCapBlocked().Once()
				f.codeObs.EXPECT().ResendRequested(auth.ResendCapped).Once()
			},
			assert: func(t *testing.T, f *challengeFixture, err error) {
				assertAppErr(t, err, apperr.CodeEmailUnavailable)
			},
		},
		{
			name: "Error: the resend count fails",
			wire: func(f *challengeFixture) {
				f.expectSentCount(0)
				f.mailer.EXPECT().ResendsSince(mock.Anything, mock.Anything).Return(0, storeErr).Once()
			},
			assert: func(t *testing.T, f *challengeFixture, err error) {
				assertAppErr(t, err, apperr.CodeInternal)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newChallengeFixture(t)
			row := f.validRow(validUserID, otherPurpose)
			f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(row, nil).Once()
			f.expectAccountCount(0)
			tc.wire(f)

			result, err := f.service.ResendChallenge(ctx, auth.ResendChallengeCommand{ChallengeID: validUserID})

			assert.Nil(t, result)
			tc.assert(t, f, err)
		})
	}
}

// TestResendChallenge_ResendShareBelowLimitProceeds pins the boundary one below
// the share: a resend that leaves the share at its last free slot is accepted
// and recorded (D5).
func TestResendChallenge_ResendShareBelowLimitProceeds(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)

	row := f.validRow(validUserID, otherPurpose)
	row.LastSentAt = challengeTestNow.Add(-2 * time.Minute)
	f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(row, nil).Once()
	f.expectAccountCount(0)
	f.expectSentCount(challengeTestPolicy.DailySendCap - 2)
	f.mailer.EXPECT().ResendsSince(mock.Anything, mock.Anything).Return(challengeTestPolicy.ResendDayCap()-1, nil).Once()
	f.repo.EXPECT().
		Extend(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.ResendCooldown, mock.Anything).
		Return(true, nil).
		Once()
	f.mailer.EXPECT().Requeue(mock.Anything, validFamily, challengeTestNow).Return(freshIdentityID, nil).Once()
	f.codeObs.EXPECT().ResendRequested(auth.ResendSent).Once()

	result, err := f.service.ResendChallenge(ctx, auth.ResendChallengeCommand{ChallengeID: validUserID})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 60, result.ResendAfter)
}

// TestIssueChallenge_IgnoresResendShare pins that first sends never consult the
// resend share: with every resend slot spent, issuance still records its send
// because the reserved part of the cap is theirs (D5).
func TestIssueChallenge_IgnoresResendShare(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)

	f.expectAccountCount(0)
	f.expectSentCount(0)
	f.repo.EXPECT().CloseActive(mock.Anything, validUserID, otherPurpose, challengeTestNow).Return(nil).Once()
	f.repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()
	f.mailer.EXPECT().Record(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	f.codeObs.EXPECT().ChallengeIssued(otherPurpose).Once()

	issued, err := f.service.IssueChallenge(ctx, auth.IssueChallengeCommand{UserID: validUserID, Email: testEmail, Purpose: otherPurpose})

	require.NoError(t, err)
	require.NotNil(t, issued)
}

// nextDayRetry is the retry hint a refusal at now carries: the time left until
// the next UTC midnight.
func nextDayRetry(now time.Time) time.Duration {
	utc := now.UTC()
	nextMidnight := time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC)
	return nextMidnight.Sub(now)
}
