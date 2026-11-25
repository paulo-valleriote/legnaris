package generation_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/engine"
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

func newRegistryTemplateFixture(t *testing.T, draftsAllowed bool) fixture {
	t.Helper()
	registry, err := template.BuildRegistry(engine.NewFormulaEngine(), template.WithDraftsAllowed(draftsAllowed))
	require.NoError(t, err)
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
	f.svc = generation.NewService(generation.Config{
		Enabled: true, DailyCap: 10, DraftTemplates: draftsAllowed,
	}, generation.Deps{
		Store:     f.store,
		Publisher: f.publisher,
		Consent:   f.consent,
		Sheets:    f.sheets,
		Plans:     f.plans,
		Templates: template.NewService(registry),
		Quota:     f.quota,
		Tx:        f.tx,
		Observer:  f.observer,
		Logger:    logger.NewLogger(slog.LevelError),
	}, func() time.Time { return now })
	return f
}

func TestCreate_CatalogGenerationGate(t *testing.T) {
	tests := []struct {
		name          string
		templateID    string
		kind          string
		draftsAllowed bool
		wantCreated   bool
		wantMessage   string
	}{
		{name: "Happy path: the 5E Character is queued outside development", templateID: "fifth_character_v1", kind: "character", wantCreated: true},
		{name: "Happy path: the 5E Stat Block is queued as an NPC outside development", templateID: "fifth_stat_block_v1", kind: "npc", wantCreated: true},
		{name: "Happy path: the 5E Stat Block is queued as an enemy in development", templateID: "fifth_stat_block_v1", kind: "enemy", draftsAllowed: true, wantCreated: true},
		{name: "Error: the retired fantasy Template is refused outside development", templateID: "fantasy_v1", kind: "character", wantMessage: generation.MsgTemplateRetired},
		{name: "Error: the retired fantasy Template is refused in development too", templateID: "fantasy_v1", kind: "character", draftsAllowed: true, wantMessage: generation.MsgTemplateRetired},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newRegistryTemplateFixture(t, tc.draftsAllowed)
			sheetKind := plan.SheetKind(tc.kind)

			if tc.wantCreated {
				f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Paid, nil)
				f.consent.EXPECT().Gate(mock.Anything, userID).Return(nil)
				f.sheets.EXPECT().CheckSheetLimit(mock.Anything, userID, sheetKind).
					Return(sheet.LimitStatus{Kind: sheetKind, Used: 0, Limit: 1}, nil)
				f.store.EXPECT().ActiveByUser(mock.Anything, userID).Return("", false, nil)
				f.runInline()
				f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(1, true, nil)
				f.observer.EXPECT().GenerationCapUsage(1, 10)
				f.quota.EXPECT().ReserveDaily(mock.Anything, userID).Return("2026-11-14", nil)
				f.store.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil)
				f.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()
			}

			id, err := f.svc.Create(context.Background(), generation.CreateCommand{
				UserID: userID, TemplateID: tc.templateID, Kind: tc.kind, Concept: validText,
			})

			if tc.wantCreated {
				require.NoError(t, err)
				assert.NotEmpty(t, id)
				return
			}

			assert.Empty(t, id)
			requireCode(t, err, apperr.CodeInvalidRequest, 400)
			assert.Equal(t, tc.wantMessage, appErrorOf(t, err).Message)
			f.plans.AssertNotCalled(t, "PlanOf")
			f.consent.AssertNotCalled(t, "Gate")
			f.sheets.AssertNotCalled(t, "CheckSheetLimit")
			f.store.AssertNotCalled(t, "ReserveCap")
			f.store.AssertNotCalled(t, "Insert")
			f.quota.AssertNotCalled(t, "ReserveDaily", mock.Anything, mock.Anything)
		})
	}
}
