package user_test

import (
	"context"
	"errors"
	"testing"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/user"
	"legnaris/backend/internal/user/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPlanSource_PlanOf(t *testing.T) {
	dbDown := errors.New("connection refused")

	tests := []struct {
		name     string
		stored   *user.User
		fetchErr error
		wantID   string
		wantErr  error
	}{
		{name: "Happy path: a paid user resolves to Paid", stored: &user.User{ID: "u1", Plan: plan.IDPaid}, wantID: plan.IDPaid},
		{name: "Happy path: a free user resolves to Free", stored: &user.User{ID: "u1", Plan: plan.IDFree}, wantID: plan.IDFree},
		{name: "Edge: a missing stored plan reads as Free", stored: &user.User{ID: "u1"}, wantID: plan.IDFree},
		{name: "Edge: an unknown stored plan reads as Free", stored: &user.User{ID: "u1", Plan: "enterprise"}, wantID: plan.IDFree},
		{name: "Error: a missing user is reported", fetchErr: user.ErrUserNotFound, wantErr: user.ErrUserNotFound},
		{name: "Error: a database failure is reported", fetchErr: dbDown, wantErr: dbDown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			repo.EXPECT().FetchByID(mock.Anything, "u1").Return(tc.stored, tc.fetchErr).Once()

			got, err := user.NewPlanSource(repo).PlanOf(context.Background(), "u1")
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantID, got.ID)
		})
	}
}
