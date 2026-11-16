package llmconsent_test

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/internal/llmconsent"
	"legnaris/backend/internal/llmconsent/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_Status(t *testing.T) {
	tests := []struct {
		name     string
		record   llmconsent.Record
		fetchErr error
		wantCode apperr.Code
		wantErr  bool
	}{
		{name: "Edge: user without consent reports an empty record", record: llmconsent.Record{}},
		{name: "Edge: withdrawn consent reports no date", record: llmconsent.Record{Version: llmconsent.Version}},
		{name: "Edge: outdated consent is returned as stored", record: consentedAt("0")},
		{name: "Happy path: current consent is returned as stored", record: consentedAt(llmconsent.Version)},
		{name: "Error: missing user is not found", fetchErr: llmconsent.ErrUserNotFound, wantErr: true, wantCode: apperr.CodeNotFound},
		{name: "Error: store failure is not an apperr", fetchErr: errStore, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			consents := mocks.NewMockConsentStore(t)
			consents.EXPECT().Fetch(mock.Anything, "user-1").Return(tc.record, tc.fetchErr).Once()
			svc := llmconsent.NewService(consents, mocks.NewMockGenerationStore(t), mocks.NewMockQuotaRefunder(t), mocks.NewMockTransactor(t), time.Now)

			record, err := svc.Status(context.Background(), "user-1")

			if !tc.wantErr {
				require.NoError(t, err)
				assert.Equal(t, tc.record, record)
				return
			}
			require.Error(t, err)
			assert.Equal(t, llmconsent.Record{}, record)
			if tc.wantCode != "" {
				_ = requireAppCode(t, err, tc.wantCode)
				return
			}
			require.ErrorIs(t, err, tc.fetchErr)
		})
	}
}
