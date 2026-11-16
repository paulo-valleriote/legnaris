package app_test

import (
	"context"
	"errors"
	"testing"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/user"
	"legnaris/backend/internal/user/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPlanResolverFromSource(t *testing.T) {
	t.Run("Happy path: a user whose stored plan is paid resolves as paid", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(mock.Anything, planUserID).Return(&user.User{ID: planUserID, Plan: "paid"}, nil)

		paid, err := app.PlanResolverFromSource(user.NewPlanSource(repo))(context.Background(), planUserID)
		require.NoError(t, err)
		assert.True(t, paid)
	})

	t.Run("Edge: a user with no stored plan resolves as free", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(mock.Anything, planUserID).Return(&user.User{ID: planUserID}, nil)

		paid, err := app.PlanResolverFromSource(user.NewPlanSource(repo))(context.Background(), planUserID)
		require.NoError(t, err)
		assert.False(t, paid)
	})

	t.Run("Edge: an unknown stored plan falls back to free", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(mock.Anything, planUserID).Return(&user.User{ID: planUserID, Plan: "platinum"}, nil)

		paid, err := app.PlanResolverFromSource(user.NewPlanSource(repo))(context.Background(), planUserID)
		require.NoError(t, err)
		assert.False(t, paid)
	})

	t.Run("Error: a failed user lookup is returned to the caller", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(mock.Anything, planUserID).Return(nil, errors.New("db down"))

		_, err := app.PlanResolverFromSource(user.NewPlanSource(repo))(context.Background(), planUserID)
		assert.Error(t, err)
	})
}
