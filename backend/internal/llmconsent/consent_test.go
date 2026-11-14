package llmconsent_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"legnaris/backend/internal/llmconsent"
	"legnaris/backend/internal/llmconsent/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var errStore = errors.New("store unavailable")

var fixedNow = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

func consentedAt(version string) llmconsent.Record {
	at := fixedNow.Add(-time.Hour)
	return llmconsent.Record{At: &at, Version: version}
}

func requireAppCode(t *testing.T, err error, code apperr.Code) *apperr.Error {
	t.Helper()
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, code, appErr.Code)
	return appErr
}

func TestCheckConsent(t *testing.T) {
	tests := []struct {
		name     string
		record   llmconsent.Record
		wantCode apperr.Code
		wantErr  bool
	}{
		{name: "Edge: never consented is refused", record: llmconsent.Record{}, wantErr: true, wantCode: apperr.CodeLLMConsentRequired},
		{name: "Edge: timestamp without version is refused", record: llmconsent.Record{At: &fixedNow}, wantErr: true, wantCode: apperr.CodeLLMConsentRequired},
		{name: "Edge: version without timestamp is refused", record: llmconsent.Record{Version: llmconsent.Version}, wantErr: true, wantCode: apperr.CodeLLMConsentRequired},
		{name: "Edge: outdated version is refused", record: consentedAt("0"), wantErr: true, wantCode: apperr.CodeLLMConsentRequired},
		{name: "Happy path: current version passes", record: consentedAt(llmconsent.Version)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := llmconsent.CheckConsent(tc.record)
			if !tc.wantErr {
				require.NoError(t, err)
				return
			}
			appErr := requireAppCode(t, err, tc.wantCode)
			assert.Equal(t, http.StatusForbidden, appErr.HttpStatus())
		})
	}
}

func TestService_Grant(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		storeErr  error
		wantStore bool
		wantCode  apperr.Code
	}{
		{name: "Edge: empty version is a validation error", version: "", wantCode: apperr.CodeValidation},
		{name: "Edge: outdated version is a validation error", version: "0", wantCode: apperr.CodeValidation},
		{name: "Edge: future version is a validation error", version: "3", wantCode: apperr.CodeValidation},
		{name: "Edge: retired version 1 is a validation error", version: "1", wantCode: apperr.CodeValidation},
		{name: "Error: missing user is not found", version: llmconsent.Version, storeErr: llmconsent.ErrUserNotFound, wantStore: true, wantCode: apperr.CodeNotFound},
		{name: "Error: store failure is not an apperr", version: llmconsent.Version, storeErr: errStore, wantStore: true},
		{name: "Happy path: current version is recorded", version: llmconsent.Version, wantStore: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			consents := mocks.NewMockConsentStore(t)
			if tc.wantStore {
				consents.EXPECT().Grant(mock.Anything, "user-1", tc.version, fixedNow).Return(tc.storeErr).Once()
			}
			svc := llmconsent.NewService(consents, mocks.NewMockGenerationStore(t), mocks.NewMockQuotaRefunder(t), mocks.NewMockTransactor(t), func() time.Time { return fixedNow })

			err := svc.Grant(context.Background(), "user-1", tc.version)

			switch {
			case tc.wantCode != "":
				_ = requireAppCode(t, err, tc.wantCode)
			case tc.storeErr != nil:
				require.ErrorIs(t, err, tc.storeErr)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestService_Revoke(t *testing.T) {
	queued := llmconsent.OpenGeneration{ID: "gen-1", UserID: "user-1", ReservationKind: llmconsent.ReservationTrial}
	running := llmconsent.OpenGeneration{ID: "gen-2", UserID: "user-1", ReservationKind: llmconsent.ReservationDaily, DayUTC: "2026-11-14"}

	tests := []struct {
		name        string
		clearErr    error
		listErr     error
		open        []llmconsent.OpenGeneration
		refundErr   error
		deleteErr   error
		deleted     int64
		wantRefunds []llmconsent.OpenGeneration
		wantDelete  bool
		wantErr     bool
		wantErrIs   error
	}{
		{name: "Edge: revoke without a consent or generations is a no-op", deleted: 0, wantDelete: true},
		{name: "Edge: terminal generations are not refunded again", deleted: 2, wantDelete: true},
		{name: "Happy path: clears consent and deletes generations in one transaction", deleted: 3, wantDelete: true},
		{
			name: "Happy path: refunds every open generation before deleting it",
			open: []llmconsent.OpenGeneration{queued, running}, deleted: 2,
			wantRefunds: []llmconsent.OpenGeneration{queued, running}, wantDelete: true,
		},
		{name: "Error: clear failure rolls back and skips the refund and delete", clearErr: errStore, wantErr: true, wantErrIs: errStore},
		{name: "Error: list failure rolls back and skips the delete", listErr: errStore, wantErr: true, wantErrIs: errStore},
		{
			name: "Error: refund failure rolls back and skips the delete", open: []llmconsent.OpenGeneration{queued},
			refundErr: errStore, wantRefunds: []llmconsent.OpenGeneration{queued}, wantErr: true, wantErrIs: errStore,
		},
		{
			name: "Error: delete failure is returned", open: []llmconsent.OpenGeneration{queued},
			deleteErr: errStore, wantRefunds: []llmconsent.OpenGeneration{queued}, wantDelete: true, wantErr: true, wantErrIs: errStore,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			consents := mocks.NewMockConsentStore(t)
			generations := mocks.NewMockGenerationStore(t)
			refunder := mocks.NewMockQuotaRefunder(t)
			tx := mocks.NewMockTransactor(t)

			tx.EXPECT().WithTransaction(mock.Anything, mock.Anything).RunAndReturn(
				func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
			).Once()
			consents.EXPECT().Clear(mock.Anything, "user-1").Return(tc.clearErr).Once()
			if tc.clearErr == nil {
				generations.EXPECT().ListOpenByUserID(mock.Anything, "user-1").Return(tc.open, tc.listErr).Once()
			}
			for _, open := range tc.wantRefunds {
				refunder.EXPECT().RefundOpen(mock.Anything, open).Return(tc.refundErr).Once()
			}
			if tc.wantDelete {
				generations.EXPECT().DeleteByUserID(mock.Anything, "user-1").Return(tc.deleted, tc.deleteErr).Once()
			}

			svc := llmconsent.NewService(consents, generations, refunder, tx, time.Now)
			err := svc.Revoke(context.Background(), "user-1")

			if !tc.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorIs(t, err, tc.wantErrIs)
		})
	}
}

func TestService_Gate(t *testing.T) {
	tests := []struct {
		name     string
		record   llmconsent.Record
		fetchErr error
		wantCode apperr.Code
		wantErr  bool
	}{
		{name: "Edge: user without consent is refused", record: llmconsent.Record{}, wantErr: true, wantCode: apperr.CodeLLMConsentRequired},
		{name: "Edge: outdated consent is refused", record: consentedAt("0"), wantErr: true, wantCode: apperr.CodeLLMConsentRequired},
		{name: "Error: missing user is not found", fetchErr: llmconsent.ErrUserNotFound, wantErr: true, wantCode: apperr.CodeNotFound},
		{name: "Error: store failure is not an apperr", fetchErr: errStore, wantErr: true},
		{name: "Happy path: current consent passes", record: consentedAt(llmconsent.Version)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			consents := mocks.NewMockConsentStore(t)
			consents.EXPECT().Fetch(mock.Anything, "user-1").Return(tc.record, tc.fetchErr).Once()
			svc := llmconsent.NewService(consents, mocks.NewMockGenerationStore(t), mocks.NewMockQuotaRefunder(t), mocks.NewMockTransactor(t), time.Now)

			err := svc.Gate(context.Background(), "user-1")

			if !tc.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			if tc.wantCode != "" {
				_ = requireAppCode(t, err, tc.wantCode)
				return
			}
			require.ErrorIs(t, err, tc.fetchErr)
		})
	}
}
