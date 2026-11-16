package generation_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/generation/mocks"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	userID     = "0190a1b2-0000-7000-8000-000000000001"
	otherUser  = "0190a1b2-0000-7000-8000-000000000002"
	templateID = "fantasy_v1"
	validText  = "A grizzled dwarf smith who forgot his name"
	fixedNow   = "2026-11-14T10:00:00Z"
)

type fixture struct {
	store     *mocks.MockStore
	publisher *mocks.MockPublisher
	consent   *mocks.MockConsentGate
	sheets    *mocks.MockSheetService
	plans     *mocks.MockPlanSource
	templates *mocks.MockTemplateSource
	quota     *mocks.MockQuota
	tx        *mocks.MockTransactor
	observer  *mocks.MockCapObserver
	svc       *generation.Service
}

func newFixture(t *testing.T, cfg generation.Config) fixture {
	t.Helper()
	now, err := time.Parse(time.RFC3339, fixedNow)
	require.NoError(t, err)
	f := fixture{
		store:     mocks.NewMockStore(t),
		publisher: mocks.NewMockPublisher(t),
		consent:   mocks.NewMockConsentGate(t),
		sheets:    mocks.NewMockSheetService(t),
		plans:     mocks.NewMockPlanSource(t),
		templates: mocks.NewMockTemplateSource(t),
		quota:     mocks.NewMockQuota(t),
		tx:        mocks.NewMockTransactor(t),
		observer:  mocks.NewMockCapObserver(t),
	}
	f.svc = generation.NewService(cfg, generation.Deps{
		Store:     f.store,
		Publisher: f.publisher,
		Consent:   f.consent,
		Sheets:    f.sheets,
		Plans:     f.plans,
		Templates: f.templates,
		Quota:     f.quota,
		Tx:        f.tx,
		Observer:  f.observer,
		Logger:    logger.NewLogger(slog.LevelError),
	}, func() time.Time { return now })
	return f
}

func (f fixture) runInline() {
	f.tx.EXPECT().WithTransaction(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Maybe()
}

func (f fixture) allowTemplate(kinds ...plan.SheetKind) {
	f.templates.EXPECT().FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: templateID}).
		Return(&template.Template{ID: templateID, Name: "Fantasy", Kinds: kinds}, nil).Maybe()
}

func defaultCfg() generation.Config {
	return generation.Config{Enabled: true, DailyCap: 10}
}

func appErrorOf(t *testing.T, err error) *apperr.Error {
	t.Helper()
	require.Error(t, err)
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	return appErr
}

func requireCode(t *testing.T, err error, code apperr.Code, status int) {
	t.Helper()
	appErr := appErrorOf(t, err)
	assert.Equal(t, code, appErr.Code)
	assert.Equal(t, status, appErr.HttpStatus())
}

func TestCreate_Gates(t *testing.T) {
	tests := []struct {
		name       string
		cfg        generation.Config
		cmd        generation.CreateCommand
		setup      func(f fixture)
		wantCode   apperr.Code
		wantStatus int
	}{
		{
			name:       "Error: the kill-switch refuses before anything is read or reserved",
			cfg:        generation.Config{Enabled: false, DailyCap: 10},
			cmd:        generation.CreateCommand{UserID: userID, TemplateID: templateID, Kind: "character", Concept: validText},
			wantCode:   apperr.CodeProviderUnavailable,
			wantStatus: 503,
		},
		{
			name:       "Error: an empty user is unauthorized",
			cmd:        generation.CreateCommand{UserID: "", TemplateID: templateID, Kind: "character", Concept: validText},
			wantCode:   apperr.CodeUnauthorized,
			wantStatus: 401,
		},
		{
			name:       "Error: a kind other than character is rejected",
			cmd:        generation.CreateCommand{UserID: userID, TemplateID: templateID, Kind: "npc", Concept: validText},
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
		},
		{
			name:       "Edge: a concept of 9 characters is too short",
			cmd:        generation.CreateCommand{UserID: userID, TemplateID: templateID, Kind: "character", Concept: strings.Repeat("a", 9)},
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
		},
		{
			name:       "Edge: a concept of 1001 characters is too long",
			cmd:        generation.CreateCommand{UserID: userID, TemplateID: templateID, Kind: "character", Concept: strings.Repeat("a", 1001)},
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
		},
		{
			name:     "Edge: whitespace does not count toward the minimum",
			cmd:      generation.CreateCommand{UserID: userID, TemplateID: templateID, Kind: "character", Concept: "   " + strings.Repeat("a", 9) + "   "},
			wantCode: apperr.CodeInvalidRequest, wantStatus: 400,
		},
		{
			name: "Error: a template that is missing is refused before the gates",
			cmd:  generation.CreateCommand{UserID: userID, TemplateID: "missing", Kind: "character", Concept: validText},
			setup: func(f fixture) {
				f.templates.EXPECT().FetchTemplate(mock.Anything, mock.Anything).
					Return(nil, apperr.NewError(apperr.CodeNotFound, "template not found", nil))
			},
			wantCode: apperr.CodeNotFound, wantStatus: 404,
		},
		{
			name: "Error: a template that refuses characters is rejected",
			cmd:  generation.CreateCommand{UserID: userID, TemplateID: templateID, Kind: "character", Concept: validText},
			setup: func(f fixture) {
				f.allowTemplate(plan.KindNPC)
			},
			wantCode: apperr.CodeInvalidRequest, wantStatus: 400,
		},
		{
			name: "Error: missing consent refuses before the Sheet limit is read",
			cmd:  generation.CreateCommand{UserID: userID, TemplateID: templateID, Kind: "character", Concept: validText},
			setup: func(f fixture) {
				f.allowTemplate(plan.KindCharacter)
				f.consent.EXPECT().Gate(mock.Anything, userID).
					Return(apperr.NewError(apperr.CodeLLMConsentRequired, "llm consent is required", nil))
			},
			wantCode: apperr.CodeLLMConsentRequired, wantStatus: 403,
		},
		{
			name: "Error: a full character allowance refuses before a job is checked",
			cmd:  generation.CreateCommand{UserID: userID, TemplateID: templateID, Kind: "character", Concept: validText},
			setup: func(f fixture) {
				f.allowTemplate(plan.KindCharacter)
				f.consent.EXPECT().Gate(mock.Anything, userID).Return(nil)
				f.sheets.EXPECT().CheckSheetLimit(mock.Anything, userID, plan.KindCharacter).
					Return(sheet.LimitStatus{Kind: plan.KindCharacter, Used: 1, Limit: 1}, nil)
			},
			wantCode: apperr.CodeSheetLimitReached, wantStatus: 409,
		},
		{
			name: "Error: a job already queued or running refuses before the cap",
			cmd:  generation.CreateCommand{UserID: userID, TemplateID: templateID, Kind: "character", Concept: validText},
			setup: func(f fixture) {
				f.allowTemplate(plan.KindCharacter)
				f.consent.EXPECT().Gate(mock.Anything, userID).Return(nil)
				f.sheets.EXPECT().CheckSheetLimit(mock.Anything, userID, plan.KindCharacter).
					Return(sheet.LimitStatus{Kind: plan.KindCharacter, Used: 0, Limit: 1}, nil)
				f.store.EXPECT().ActiveByUser(mock.Anything, userID).Return("0190a1b2-0000-7000-8000-0000000000aa", true, nil)
			},
			wantCode: apperr.CodeGenerationInProgress, wantStatus: 409,
		},
		{
			name: "Error: the global daily cap refuses with 503 and a Retry-After",
			cmd:  generation.CreateCommand{UserID: userID, TemplateID: templateID, Kind: "character", Concept: validText},
			setup: func(f fixture) {
				f.allowTemplate(plan.KindCharacter)
				f.consent.EXPECT().Gate(mock.Anything, userID).Return(nil)
				f.sheets.EXPECT().CheckSheetLimit(mock.Anything, userID, plan.KindCharacter).
					Return(sheet.LimitStatus{Kind: plan.KindCharacter, Used: 0, Limit: 1}, nil)
				f.store.EXPECT().ActiveByUser(mock.Anything, userID).Return("", false, nil)
				f.runInline()
				f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Free, nil)
				f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(10, false, nil)
				f.observer.EXPECT().GenerationCapUsage(10, 10)
			},
			wantCode: apperr.CodeGenerationCapReached, wantStatus: 503,
		},
		{
			name: "Error: an exhausted trial refuses with PLAN_UPGRADE_REQUIRED and inserts nothing",
			cmd:  generation.CreateCommand{UserID: userID, TemplateID: templateID, Kind: "character", Concept: validText},
			setup: func(f fixture) {
				f.allowTemplate(plan.KindCharacter)
				f.consent.EXPECT().Gate(mock.Anything, userID).Return(nil)
				f.sheets.EXPECT().CheckSheetLimit(mock.Anything, userID, plan.KindCharacter).
					Return(sheet.LimitStatus{Kind: plan.KindCharacter, Used: 0, Limit: 1}, nil)
				f.store.EXPECT().ActiveByUser(mock.Anything, userID).Return("", false, nil)
				f.runInline()
				f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(1, true, nil)
				f.observer.EXPECT().GenerationCapUsage(1, 10)
				f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Free, nil)
				f.quota.EXPECT().ReserveTrial(mock.Anything, userID).
					Return(apperr.NewError(apperr.CodePlanUpgradeRequired, "upgrade", nil))
			},
			wantCode: apperr.CodePlanUpgradeRequired, wantStatus: 403,
		},
		{
			name: "Error: a second insert for the same player maps to GENERATION_IN_PROGRESS",
			cmd:  generation.CreateCommand{UserID: userID, TemplateID: templateID, Kind: "character", Concept: validText},
			setup: func(f fixture) {
				f.allowTemplate(plan.KindCharacter)
				f.consent.EXPECT().Gate(mock.Anything, userID).Return(nil)
				f.sheets.EXPECT().CheckSheetLimit(mock.Anything, userID, plan.KindCharacter).
					Return(sheet.LimitStatus{Kind: plan.KindCharacter, Used: 0, Limit: 1}, nil)
				f.store.EXPECT().ActiveByUser(mock.Anything, userID).Return("", false, nil)
				f.runInline()
				f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(1, true, nil)
				f.observer.EXPECT().GenerationCapUsage(1, 10)
				f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Free, nil)
				f.quota.EXPECT().ReserveTrial(mock.Anything, userID).Return(nil)
				f.store.EXPECT().Insert(mock.Anything, mock.Anything).Return(generation.ErrActiveExists)
			},
			wantCode: apperr.CodeGenerationInProgress, wantStatus: 409,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg
			if cfg == (generation.Config{}) {
				cfg = defaultCfg()
			}
			f := newFixture(t, cfg)
			if tc.setup != nil {
				tc.setup(f)
			}

			id, err := f.svc.Create(context.Background(), tc.cmd)
			assert.Empty(t, id)
			requireCode(t, err, tc.wantCode, tc.wantStatus)
			if tc.wantCode == apperr.CodeGenerationCapReached {
				appErr := appErrorOf(t, err)
				assert.Greater(t, appErr.RetryAfter, time.Duration(0))
				assert.LessOrEqual(t, appErr.RetryAfter, 24*time.Hour)
			}
		})
	}
}
