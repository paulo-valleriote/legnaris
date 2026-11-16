package auth_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"
	"legnaris/backend/shared/idgen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func newChallengeStore(t *testing.T) (auth.ChallengeRepository, *mongo.Collection) {
	t.Helper()

	mdb := dbtest.Database(t)
	collection := mdb.Collection("auth_challenges")

	require.NoError(t, db.EnsureIndexes(context.Background(), mdb))
	return auth.NewChallengeRepository(mdb), collection
}

func mustCanonicalID(t *testing.T) string {
	t.Helper()

	id, err := idgen.New()
	require.NoError(t, err)
	return id
}

func mustChallenge(t *testing.T, userID string, purpose auth.Purpose, now time.Time) auth.Challenge {
	t.Helper()

	return auth.Challenge{
		ID:           mustCanonicalID(t),
		UserID:       userID,
		Purpose:      purpose,
		CodeHash:     "digest-" + userID,
		ExpiresAt:    now.Add(time.Hour),
		CeilingAt:    now.Add(2 * time.Hour),
		Attempts:     0,
		CreatedAt:    now,
		LastSentAt:   now,
		LastIntentID: mustCanonicalID(t),
	}
}

func TestChallengeRepository_Insert(t *testing.T) {
	repo, _ := newChallengeStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	userID := mustCanonicalID(t)

	t.Run("Happy path: one active row is admitted per account and purpose", func(t *testing.T) {
		first := mustChallenge(t, userID, otherPurpose, now)
		require.NoError(t, repo.Insert(ctx, first))

		second := mustChallenge(t, userID, otherPurpose, now)
		err := repo.Insert(ctx, second)
		require.Error(t, err)
		assert.ErrorIs(t, err, auth.ErrChallengeActive, "a concurrent active row must be rejected")
	})

	t.Run("Happy path: a different purpose is admitted", func(t *testing.T) {
		other := mustChallenge(t, userID, auth.PurposeVerifyEmail, now)
		require.NoError(t, repo.Insert(ctx, other))
	})

	t.Run("Happy path: a consumed row does not block a new active one", func(t *testing.T) {
		require.NoError(t, repo.CloseActive(ctx, userID, otherPurpose, now))
		next := mustChallenge(t, userID, otherPurpose, now)
		require.NoError(t, repo.Insert(ctx, next))
	})
}

func TestChallengeRepository_CloseActive(t *testing.T) {
	repo, _ := newChallengeStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	userID := mustCanonicalID(t)

	t.Run("Happy path: the active row is consumed", func(t *testing.T) {
		active := mustChallenge(t, userID, otherPurpose, now)
		require.NoError(t, repo.Insert(ctx, active))

		require.NoError(t, repo.CloseActive(ctx, userID, otherPurpose, now.Add(time.Minute)))

		stored, err := repo.Fetch(ctx, active.ID)
		require.NoError(t, err)
		require.NotNil(t, stored.ConsumedAt)
		assert.Equal(t, now.Add(time.Minute).Unix(), stored.ConsumedAt.Unix())

		reclaimed, err := repo.ClaimAttempt(ctx, active.ID, now, 5)
		assert.ErrorIs(t, err, auth.ErrChallengeUnusable)
		assert.Nil(t, reclaimed, "a closed challenge must never be claimable")
	})
}

func TestChallengeRepository_ClaimAttempt(t *testing.T) {
	repo, collection := newChallengeStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("Happy path: the attempt count increments and the post-image is returned", func(t *testing.T) {
		challenge := mustChallenge(t, mustCanonicalID(t), otherPurpose, now)
		require.NoError(t, repo.Insert(ctx, challenge))

		claimed, err := repo.ClaimAttempt(ctx, challenge.ID, now, 5)
		require.NoError(t, err)
		assert.Equal(t, 1, claimed.Attempts, "the post-image must show the incremented count")

		again, err := repo.ClaimAttempt(ctx, challenge.ID, now, 5)
		require.NoError(t, err)
		assert.Equal(t, 2, again.Attempts, "the count must keep increasing durably")
	})

	t.Run("Edge: one below the cap is claimable, at the cap it is not", func(t *testing.T) {
		challenge := mustChallenge(t, mustCanonicalID(t), otherPurpose, now)
		challenge.Attempts = 4
		require.NoError(t, repo.Insert(ctx, challenge))

		claimed, err := repo.ClaimAttempt(ctx, challenge.ID, now, 5)
		require.NoError(t, err, "the fifth attempt is still claimable")
		assert.Equal(t, 5, claimed.Attempts)

		_, err = repo.ClaimAttempt(ctx, challenge.ID, now, 5)
		assert.ErrorIs(t, err, auth.ErrChallengeUnusable, "the sixth attempt is refused")

		var stored auth.Challenge
		require.NoError(t, collection.FindOne(ctx, bson.D{{Key: "_id", Value: challenge.ID}}).Decode(&stored))
		assert.Equal(t, 5, stored.Attempts, "an unusable row must not be mutated")
	})

	t.Run("Edge: an expired row is unusable and untouched", func(t *testing.T) {
		challenge := mustChallenge(t, mustCanonicalID(t), otherPurpose, now)
		challenge.ExpiresAt = now
		require.NoError(t, repo.Insert(ctx, challenge))

		_, err := repo.ClaimAttempt(ctx, challenge.ID, now, 5)
		assert.ErrorIs(t, err, auth.ErrChallengeUnusable)

		var stored auth.Challenge
		require.NoError(t, collection.FindOne(ctx, bson.D{{Key: "_id", Value: challenge.ID}}).Decode(&stored))
		assert.Zero(t, stored.Attempts)
	})

	t.Run("Edge: a consumed row is unusable", func(t *testing.T) {
		challenge := mustChallenge(t, mustCanonicalID(t), otherPurpose, now)
		require.NoError(t, repo.Insert(ctx, challenge))
		_, err := repo.Consume(ctx, challenge.ID, challenge.CodeHash, now)
		require.NoError(t, err)

		_, err = repo.ClaimAttempt(ctx, challenge.ID, now, 5)
		assert.ErrorIs(t, err, auth.ErrChallengeUnusable)
	})

	t.Run("Edge: an unknown row is unusable", func(t *testing.T) {
		_, err := repo.ClaimAttempt(ctx, mustCanonicalID(t), now, 5)
		assert.ErrorIs(t, err, auth.ErrChallengeUnusable)
	})
}

func TestChallengeRepository_ClaimAttempt_Concurrency(t *testing.T) {
	repo, _ := newChallengeStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	challenge := mustChallenge(t, mustCanonicalID(t), otherPurpose, now)
	require.NoError(t, repo.Insert(ctx, challenge))

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.ClaimAttempt(ctx, challenge.ID, now, 5)
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	for err := range results {
		assert.NoError(t, err, "the attempt claim is not the exclusive one")
	}
}

func TestChallengeRepository_Consume(t *testing.T) {
	repo, _ := newChallengeStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("Happy path: the first consume wins and the second reports false", func(t *testing.T) {
		challenge := mustChallenge(t, mustCanonicalID(t), otherPurpose, now)
		require.NoError(t, repo.Insert(ctx, challenge))

		won, err := repo.Consume(ctx, challenge.ID, challenge.CodeHash, now)
		require.NoError(t, err)
		assert.True(t, won)

		again, err := repo.Consume(ctx, challenge.ID, challenge.CodeHash, now)
		require.NoError(t, err)
		assert.False(t, again, "a consumed challenge must never be consumed twice")
	})

	t.Run("Edge: a mismatching digest never consumes", func(t *testing.T) {
		challenge := mustChallenge(t, mustCanonicalID(t), otherPurpose, now)
		require.NoError(t, repo.Insert(ctx, challenge))

		won, err := repo.Consume(ctx, challenge.ID, "other-digest", now)
		require.NoError(t, err)
		assert.False(t, won)
	})

	t.Run("Edge: two concurrent consumes admit exactly one winner", func(t *testing.T) {
		challenge := mustChallenge(t, mustCanonicalID(t), otherPurpose, now)
		require.NoError(t, repo.Insert(ctx, challenge))

		var wg sync.WaitGroup
		wins := make(chan bool, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				won, err := repo.Consume(ctx, challenge.ID, challenge.CodeHash, now)
				if err != nil {
					wins <- false
					return
				}
				wins <- won
			}()
		}
		wg.Wait()
		close(wins)

		winners := 0
		for won := range wins {
			if won {
				winners++
			}
		}
		assert.Equal(t, 1, winners, "exactly one concurrent consume must win")
	})
}

func TestChallengeRepository_Extend(t *testing.T) {
	repo, _ := newChallengeStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	cooldown := 60 * time.Second

	t.Run("Happy path: an elapsed cool-down extends the deadline and the last-sent stamp", func(t *testing.T) {
		challenge := mustChallenge(t, mustCanonicalID(t), otherPurpose, now)
		require.NoError(t, repo.Insert(ctx, challenge))

		later := now.Add(2 * time.Minute)
		extended, err := repo.Extend(ctx, challenge.ID, later, cooldown, later.Add(10*time.Minute))
		require.NoError(t, err)
		assert.True(t, extended)

		stored, err := repo.Fetch(ctx, challenge.ID)
		require.NoError(t, err)
		assert.Equal(t, later.Add(10*time.Minute).Unix(), stored.ExpiresAt.Unix())
		assert.Equal(t, later.Unix(), stored.LastSentAt.Unix())
	})

	t.Run("Edge: a resend inside the cool-down is suppressed and mutates nothing", func(t *testing.T) {
		challenge := mustChallenge(t, mustCanonicalID(t), otherPurpose, now)
		require.NoError(t, repo.Insert(ctx, challenge))

		inside := now.Add(cooldown - time.Second)
		extended, err := repo.Extend(ctx, challenge.ID, inside, cooldown, inside.Add(10*time.Minute))
		require.NoError(t, err)
		assert.False(t, extended, "a resend inside the cool-down must be suppressed")

		stored, err := repo.Fetch(ctx, challenge.ID)
		require.NoError(t, err)
		assert.Equal(t, challenge.ExpiresAt.Unix(), stored.ExpiresAt.Unix())
		assert.Equal(t, challenge.LastSentAt.Unix(), stored.LastSentAt.Unix())
	})

	t.Run("Edge: exactly at the cool-down boundary the resend is allowed", func(t *testing.T) {
		challenge := mustChallenge(t, mustCanonicalID(t), otherPurpose, now)
		require.NoError(t, repo.Insert(ctx, challenge))

		atBoundary := now.Add(cooldown)
		extended, err := repo.Extend(ctx, challenge.ID, atBoundary, cooldown, atBoundary.Add(10*time.Minute))
		require.NoError(t, err)
		assert.True(t, extended)
	})

	t.Run("Edge: a consumed or expired row is never extended", func(t *testing.T) {
		challenge := mustChallenge(t, mustCanonicalID(t), otherPurpose, now)
		challenge.ExpiresAt = now
		require.NoError(t, repo.Insert(ctx, challenge))

		extended, err := repo.Extend(ctx, challenge.ID, now.Add(time.Hour), cooldown, now.Add(time.Hour))
		require.NoError(t, err)
		assert.False(t, extended)
	})
}

func TestChallengeRepository_Fetch(t *testing.T) {
	repo, _ := newChallengeStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("Error: an unknown identity is a typed not-found", func(t *testing.T) {
		_, err := repo.Fetch(ctx, mustCanonicalID(t))
		assert.ErrorIs(t, err, auth.ErrChallengeNotFound)
	})

	t.Run("Happy path: a stored row round-trips and classifies as usable", func(t *testing.T) {
		challenge := mustChallenge(t, mustCanonicalID(t), auth.PurposeVerifyEmail, now)
		require.NoError(t, repo.Insert(ctx, challenge))

		stored, err := repo.Fetch(ctx, challenge.ID)
		require.NoError(t, err)
		assert.Equal(t, challenge.UserID, stored.UserID)
		assert.Equal(t, auth.PurposeVerifyEmail, stored.Purpose)
		assert.Equal(t, challenge.CodeHash, stored.CodeHash)
		assert.Equal(t, challenge.LastIntentID, stored.LastIntentID)
		assert.True(t, stored.Usable(now, 5))
	})

	t.Run("Edge: an expired row classifies as expired for the observer", func(t *testing.T) {
		challenge := mustChallenge(t, mustCanonicalID(t), otherPurpose, now)
		challenge.ExpiresAt = now
		require.NoError(t, repo.Insert(ctx, challenge))

		stored, err := repo.Fetch(ctx, challenge.ID)
		require.NoError(t, err)
		assert.False(t, stored.Usable(now, 5))
		assert.Equal(t, auth.ResultExpired, stored.FailureResult(now, 5))
	})
}

func TestChallengeRepository_CountForUserSince(t *testing.T) {
	repo, _ := newChallengeStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	userID := mustCanonicalID(t)
	other := mustCanonicalID(t)

	for i := 0; i < 3; i++ {
		challenge := mustChallenge(t, userID, otherPurpose, now.Add(time.Duration(i)*time.Minute))
		require.NoError(t, repo.Insert(ctx, challenge))
		require.NoError(t, repo.CloseActive(ctx, userID, otherPurpose, now.Add(time.Duration(i)*time.Minute)))
	}
	require.NoError(t, repo.Insert(ctx, mustChallenge(t, other, otherPurpose, now)))

	t.Run("Happy path: every challenge at or after the window start is counted", func(t *testing.T) {
		count, err := repo.CountForUserSince(ctx, userID, now.Add(-time.Second))
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("Edge: the window start excludes earlier rows", func(t *testing.T) {
		count, err := repo.CountForUserSince(ctx, userID, now.Add(time.Minute))
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("Edge: another account's challenges are not counted", func(t *testing.T) {
		count, err := repo.CountForUserSince(ctx, other, now.Add(-time.Second))
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}
