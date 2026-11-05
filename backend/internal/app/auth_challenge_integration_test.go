package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"testing"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// codePattern matches the only shape a code may take.
var codePattern = regexp.MustCompile(`^\d{6}$`)

// userIDByEmail resolves the seeded account's canonical identity.
func userIDByEmail(t *testing.T, mdb *mongo.Database, email string) string {
	t.Helper()

	var stored struct {
		ID string `bson:"_id"`
	}
	require.NoError(t, mdb.Collection("users").FindOne(context.Background(), bson.M{"email": email}).Decode(&stored))
	require.NotEmpty(t, stored.ID)
	return stored.ID
}

// pendingCode reads the code from the store's recorded send intention: it is
// the only durable home of the clear value, so this is how a test learns it
// (OTP-12).
func pendingCode(t *testing.T, mdb *mongo.Database) string {
	t.Helper()

	var intent struct {
		Payload struct {
			Code    string `bson:"code"`
			Purpose string `bson:"purpose"`
		} `bson:"payload"`
	}
	require.NoError(t, mdb.Collection("email_outbox").
		FindOne(context.Background(), bson.M{"status": "pending"}).Decode(&intent))
	require.Regexp(t, codePattern, intent.Payload.Code)
	return intent.Payload.Code
}

func outboxCount(t *testing.T, mdb *mongo.Database) int64 {
	t.Helper()

	count, err := mdb.Collection("email_outbox").CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	return count
}

func challengeExpiry(t *testing.T, mdb *mongo.Database, challengeID string) time.Time {
	t.Helper()

	var stored struct {
		ExpiresAt time.Time `bson:"expires_at"`
	}
	require.NoError(t, mdb.Collection("auth_challenges").
		FindOne(context.Background(), bson.M{"_id": challengeID}).Decode(&stored))
	return stored.ExpiresAt
}

// TestIntegration_ChallengeIssueVerify drives the first P1 journey end to end:
// issue, verify with the code the intention carries, observe the session and the
// account's verification timestamp, then replay and see the uniform failure
// (OTP-02, OTP-04, OTP-07, OTP-08, OTP-12).
func TestIntegration_ChallengeIssueVerify(t *testing.T) {
	h, mdb, modules := newIntegrationStack(t, nil)
	ctx := context.Background()

	const email = "challenge-verify@example.com"
	registerAuthUser(t, h, mdb, email, "challenge-verify")
	userID := userIDByEmail(t, mdb, email)

	issued, err := modules.Auth.Service.IssueChallenge(ctx, auth.IssueChallengeCommand{
		UserID:  userID,
		Email:   email,
		Purpose: auth.PurposeVerifyEmail,
	})
	require.NoError(t, err)
	require.NotEmpty(t, issued.ChallengeID)
	assert.Equal(t, 600, issued.ExpiresIn)

	code := pendingCode(t, mdb)

	t.Run("Happy path: the challenge stores a digest, never the code", func(t *testing.T) {
		var stored struct {
			CodeHash string `bson:"code_hash"`
		}
		require.NoError(t, mdb.Collection("auth_challenges").
			FindOne(ctx, bson.M{"_id": issued.ChallengeID}).Decode(&stored))
		assert.NotEqual(t, code, stored.CodeHash, "the code must never be stored in the clear")
		assert.Equal(t, int64(1), outboxCount(t, mdb), "exactly one intention records the code")
	})

	t.Run("Happy path: verifying establishes the session and the verification timestamp", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, verifyRoute,
			`{"challenge_id":"`+issued.ChallengeID+`","code":"`+code+`"}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		session := decodeSession(t, rec)
		require.NotEmpty(t, session.AccessToken)
		cookies := cookiesFrom(t, rec)
		require.NotEmpty(t, cookies.refresh, "verification must set the refresh cookie")
		require.NotEmpty(t, cookies.csrf, "verification must set the CSRF cookie")

		var user struct {
			VerifiedAt *time.Time `bson:"email_verified_at"`
		}
		require.NoError(t, mdb.Collection("users").FindOne(ctx, bson.M{"_id": userID}).Decode(&user))
		require.NotNil(t, user.VerifiedAt, "the account's verification timestamp must be recorded")
	})

	t.Run("Error: replaying the consumed code answers the uniform 401", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, verifyRoute,
			`{"challenge_id":"`+issued.ChallengeID+`","code":"`+code+`"}`)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Empty(t, rec.Header().Get("Set-Cookie"))

		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, string(apperr.CodeOTPInvalid), body.Error.Code)
		assert.Equal(t, auth.MsgOTPInvalid, body.Error.Message)
	})
}

// TestIntegration_ChallengeResend drives the second P1 journey: a resend keeps
// the code valid, records exactly one extra intention, and a resend on the
// consumed challenge changes nothing (OTP-10, OTP-11).
func TestIntegration_ChallengeResend(t *testing.T) {
	h, mdb, modules := newIntegrationStack(t, nil)
	ctx := context.Background()

	const email = "challenge-resend@example.com"
	registerAuthUser(t, h, mdb, email, "challenge-resend")
	userID := userIDByEmail(t, mdb, email)

	issued, err := modules.Auth.Service.IssueChallenge(ctx, auth.IssueChallengeCommand{
		UserID:  userID,
		Email:   email,
		Purpose: otherPurpose,
	})
	require.NoError(t, err)
	code := pendingCode(t, mdb)
	require.Equal(t, int64(1), outboxCount(t, mdb))

	t.Run("Happy path: a suppressed and an accepted resend answer identically", func(t *testing.T) {
		// The first resend is inside the just-started cool-down: it is answered
		// but records nothing (OTP-10).
		suppressed := doRequest(t, h, http.MethodPost, resendRoute, `{"challenge_id":"`+issued.ChallengeID+`"}`)
		require.Equal(t, http.StatusAccepted, suppressed.Code, suppressed.Body.String())
		assert.Equal(t, int64(1), outboxCount(t, mdb), "a suppressed resend records no intention")

		// Backdate the last-sent instant to simulate the cool-down having
		// elapsed, so the next resend records a new intention.
		_, err := mdb.Collection("auth_challenges").UpdateOne(ctx,
			bson.M{"_id": issued.ChallengeID},
			bson.M{"$set": bson.M{"last_sent_at": time.Now().UTC().Add(-2 * time.Minute)}},
		)
		require.NoError(t, err)

		accepted := doRequest(t, h, http.MethodPost, resendRoute, `{"challenge_id":"`+issued.ChallengeID+`"}`)
		require.Equal(t, http.StatusAccepted, accepted.Code, accepted.Body.String())

		assert.Equal(t, suppressed.Body.String(), accepted.Body.String(), "the answers must be indistinguishable")
		assert.Equal(t, int64(2), outboxCount(t, mdb), "exactly one extra intention is recorded")

		var body response.Success[auth.ResendResult]
		require.NoError(t, json.Unmarshal(accepted.Body.Bytes(), &body))
		assert.Equal(t, 60, body.Data.ResendAfter)
	})

	t.Run("Happy path: the code still verifies after the resend", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, verifyRoute,
			`{"challenge_id":"`+issued.ChallengeID+`","code":"`+code+`"}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("Edge: a resend on the consumed challenge changes nothing", func(t *testing.T) {
		before := outboxCount(t, mdb)
		rec := doRequest(t, h, http.MethodPost, resendRoute, `{"challenge_id":"`+issued.ChallengeID+`"}`)

		require.Equal(t, http.StatusAccepted, rec.Code, "the answer is indistinguishable")
		assert.Equal(t, before, outboxCount(t, mdb), "a consumed challenge is never resurrected")
	})
}

// TestIntegration_ChallengeDailyCap drives the cap end to end: reaching it
// refuses issuance and resend with the typed unavailable outcome, writes no
// intention and never moves a deadline (OTP-13, SC-004).
func TestIntegration_ChallengeDailyCap(t *testing.T) {
	h, mdb, modules := newIntegrationStack(t, map[string]string{"AUTH_OTP_DAILY_SEND_CAP": "2"})
	ctx := context.Background()

	const email = "challenge-cap@example.com"
	registerAuthUser(t, h, mdb, email, "challenge-cap")
	userID := userIDByEmail(t, mdb, email)

	issued, err := modules.Auth.Service.IssueChallenge(ctx, auth.IssueChallengeCommand{
		UserID:  userID,
		Email:   email,
		Purpose: otherPurpose,
	})
	require.NoError(t, err, "one below the cap must be admitted")
	require.Equal(t, int64(1), outboxCount(t, mdb))

	// Seed one more intention to reach the configured cap of two.
	now := time.Now().UTC()
	_, err = mdb.Collection("email_outbox").InsertOne(ctx, bson.M{
		"_id":             "seed-intention-at-cap",
		"status":          "pending",
		"created_at":      now,
		"next_attempt_at": now,
		"payload":         bson.M{},
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), outboxCount(t, mdb))

	deadlineBefore := challengeExpiry(t, mdb, issued.ChallengeID)

	t.Run("Edge: issuing at the cap is refused and writes no intention", func(t *testing.T) {
		_, err := modules.Auth.Service.IssueChallenge(ctx, auth.IssueChallengeCommand{
			UserID:  userID,
			Email:   email,
			Purpose: auth.PurposeVerifyEmail,
		})

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeEmailUnavailable, appErr.Code)
		assert.Equal(t, 503, appErr.HttpStatus())
		assert.Positive(t, appErr.RetryAfter, "the refusal carries a retry hint")
		assert.Equal(t, int64(2), outboxCount(t, mdb))
	})

	t.Run("Edge: resending at the cap leaves the deadline and the outbox unmoved", func(t *testing.T) {
		_, err := modules.Auth.Service.ResendChallenge(ctx, auth.ResendChallengeCommand{ChallengeID: issued.ChallengeID})

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeEmailUnavailable, appErr.Code)
		assert.Equal(t, deadlineBefore, challengeExpiry(t, mdb, issued.ChallengeID))
		assert.Equal(t, int64(2), outboxCount(t, mdb))
	})

	t.Run("Happy path: one below the cap writes the intention", func(t *testing.T) {
		_, err := mdb.Collection("email_outbox").DeleteOne(ctx, bson.M{"_id": "seed-intention-at-cap"})
		require.NoError(t, err)

		_, err = modules.Auth.Service.IssueChallenge(ctx, auth.IssueChallengeCommand{
			UserID:  userID,
			Email:   email,
			Purpose: otherPurpose,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(2), outboxCount(t, mdb))
	})
}
