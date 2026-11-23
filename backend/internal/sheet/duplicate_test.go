package sheet_test

import (
	"context"
	"errors"
	"testing"

	"legnaris/backend/internal/events"
	eventsmocks "legnaris/backend/internal/events/mocks"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/sheeterr"
	"legnaris/backend/internal/sheet/sheettest"
	"legnaris/backend/internal/sheet/template"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const duplicateCampaignID = "01900000-0000-7000-8000-0000000000cc"

func duplicateSource(t *testing.T) sheet.Sheet {
	t.Helper()
	campaignID := duplicateCampaignID
	sectionExpr := "2d6"
	return sheet.Sheet{
		ID:         updateSheetID,
		TemplateID: "fantasy_v1",
		OwnerID:    ownerUserID,
		Kind:       plan.KindNPC,
		Name:       "Brom",
		Data: map[string]any{
			"stats":     map[string]any{"dexterity": 14.0},
			"modifiers": map[string]any{"dexterityMod": 2.0},
		},
		Layout: &sheet.Layout{
			SectionOrder: []string{"stats", "modifiers"},
			Pinned:       []string{"stats.dexterity"},
		},
		Dice: &sheet.Dice{
			Section: map[string]*string{"stats": &sectionExpr},
			Row:     map[string]*string{"stats.dexterity": nil},
		},
		Version:    4,
		CampaignID: &campaignID,
	}
}

func newDuplicateService(t *testing.T, repo sheet.Repository, plans sheet.PlanSource, tx sheet.TransactionRunner, pub events.Publisher) sheet.Service {
	t.Helper()
	tmplSvc := templatemocks.NewMockService(t)
	tmplSvc.EXPECT().
		FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "fantasy_v1"}).
		Return(&template.Template{ID: "fantasy_v1", State: template.StateActive}, nil).
		Maybe()
	return sheet.NewService(repo, tmplSvc, sheettest.DAGMap{}, testLog, pub,
		sheet.WithPlanLimits(plans, tx),
	)
}

func TestService_Duplicate(t *testing.T) {
	source := duplicateSource(t)

	tests := []struct {
		name       string
		source     sheet.Sheet
		setup      func(repo *mocks.MockRepository)
		plans      func(t *testing.T) sheet.PlanSource
		wantCode   apperr.Code
		wantSheet  bool
		wantStatus int
	}{
		{
			name:   "Happy path: an NPC is copied with its values, layout and dice",
			source: source,
			setup: func(repo *mocks.MockRepository) {
				repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindNPC, 2).Return(nil).Once()
				repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
			},
			wantSheet:  true,
			wantStatus: 201,
		},
		{
			name: "Happy path: an Enemy is copied with its own per-kind limit",
			source: func() sheet.Sheet {
				cp := duplicateSource(t)
				cp.Kind = plan.KindEnemy
				cp.Dice = nil
				return cp
			}(),
			setup: func(repo *mocks.MockRepository) {
				repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindEnemy, 2).Return(nil).Once()
				repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
			},
			wantSheet:  true,
			wantStatus: 201,
		},
		{
			name: "Edge: a sheet without layout or dice copies them as absent",
			source: func() sheet.Sheet {
				cp := duplicateSource(t)
				cp.Layout = nil
				cp.Dice = nil
				return cp
			}(),
			setup: func(repo *mocks.MockRepository) {
				repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindNPC, 2).Return(nil).Once()
				repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
			},
			wantSheet:  true,
			wantStatus: 201,
		},
		{
			name: "Error: a character sheet cannot be duplicated",
			source: func() sheet.Sheet {
				cp := duplicateSource(t)
				cp.Kind = plan.KindCharacter
				return cp
			}(),
			wantCode:   apperr.CodeSheetKindMismatch,
			wantStatus: 422,
		},
		{
			name:   "Error: duplicating over the per-kind limit is a plan limit refusal",
			source: source,
			setup: func(repo *mocks.MockRepository) {
				repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindNPC, 2).Return(sheeterr.ErrSheetLimitReached).Once()
			},
			wantCode:   apperr.CodePlanLimitReached,
			wantStatus: 403,
		},
		{
			name:   "Error: a create failure is internal",
			source: source,
			setup: func(repo *mocks.MockRepository) {
				repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindNPC, 2).Return(nil).Once()
				repo.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("disk on fire")).Once()
			},
			wantCode:   apperr.CodeInternal,
			wantStatus: 500,
		},
		{
			name:       "Error: a missing sheet is not found",
			source:     source,
			wantCode:   apperr.CodeNotFound,
			wantStatus: 404,
		},
		{
			name: "Error: a foreign sheet is not found",
			source: func() sheet.Sheet {
				cp := duplicateSource(t)
				cp.OwnerID = "01900000-0000-7000-8000-0000000000bb"
				return cp
			}(),
			wantCode:   apperr.CodeNotFound,
			wantStatus: 404,
		},
		{
			name:   "Error: a plan source failure is internal",
			source: source,
			plans: func(t *testing.T) sheet.PlanSource {
				plans := mocks.NewMockPlanSource(t)
				plans.EXPECT().PlanOf(mock.Anything, ownerUserID).Return(plan.Plan{}, errors.New("plan down")).Once()
				return plans
			},
			wantCode:   apperr.CodeInternal,
			wantStatus: 500,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			if tc.name == "Error: a missing sheet is not found" {
				repo.EXPECT().FetchByID(mock.Anything, updateSheetID).Return(nil, sheeterr.ErrSheetNotFound).Once()
			} else {
				repo.EXPECT().FetchByID(mock.Anything, updateSheetID).RunAndReturn(
					func(context.Context, string) (*sheet.Sheet, error) {
						cp := tc.source
						return &cp, nil
					},
				).Once()
			}
			if tc.setup != nil {
				tc.setup(repo)
			}
			plans := sheet.PlanSource(planFor(t, ownerUserID, plan.Free))
			if tc.plans != nil {
				plans = tc.plans(t)
			}
			svc := newDuplicateService(t, repo, plans, runInline(t), events.NoOp{})

			copied, err := svc.Duplicate(context.Background(), sheet.DuplicateSheetCommand{SheetID: updateSheetID, OwnerID: ownerUserID})
			if tc.wantCode != "" {
				require.Error(t, err)
				assert.Nil(t, copied)
				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, tc.wantCode, appErr.Code)
				assert.Equal(t, tc.wantStatus, appErr.HttpStatus())
				if tc.wantCode == apperr.CodePlanLimitReached {
					assert.Equal(t, map[string]any{"kind": "npc", "limit": 2}, appErr.Details)
				}
				repo.AssertNotCalled(t, "Create")
				return
			}
			require.NoError(t, err)
			require.NotNil(t, copied)
			assert.NotEqual(t, tc.source.ID, copied.ID)
			require.NoError(t, idgen.Validate(copied.ID))
			assert.Equal(t, tc.source.OwnerID, copied.OwnerID)
			assert.Equal(t, tc.source.TemplateID, copied.TemplateID)
			assert.Equal(t, tc.source.Kind, copied.Kind)
			assert.Equal(t, tc.source.Name+" (copy)", copied.Name)
			assert.Equal(t, tc.source.Data, copied.Data)
			assert.Equal(t, tc.source.Layout, copied.Layout)
			assert.Equal(t, tc.source.Dice, copied.Dice)
			assert.Nil(t, copied.CampaignID, "a copy never inherits a campaign link")
		})
	}
}

func TestService_Duplicate_RejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name     string
		cmd      sheet.DuplicateSheetCommand
		wantCode apperr.Code
	}{
		{name: "Error: a missing owner is unauthorized", cmd: sheet.DuplicateSheetCommand{SheetID: updateSheetID}, wantCode: apperr.CodeUnauthorized},
		{name: "Error: a malformed sheet id is invalid", cmd: sheet.DuplicateSheetCommand{SheetID: "not-a-uuid", OwnerID: ownerUserID}, wantCode: apperr.CodeInvalidRequest},
		{name: "Error: a missing sheet id is invalid", cmd: sheet.DuplicateSheetCommand{OwnerID: ownerUserID}, wantCode: apperr.CodeInvalidRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			svc := newDuplicateService(t, repo, mocks.NewMockPlanSource(t), mocks.NewMockTransactionRunner(t), events.NoOp{})
			_, err := svc.Duplicate(context.Background(), tc.cmd)
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, tc.wantCode, appErr.Code)
			repo.AssertNotCalled(t, "FetchByID")
		})
	}
}

func TestService_Duplicate_WithoutPlanLimitsRefuses(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FetchByID(mock.Anything, updateSheetID).Return(&sheet.Sheet{
		ID: updateSheetID, TemplateID: "fantasy_v1", OwnerID: ownerUserID, Kind: plan.KindNPC,
	}, nil).Once()
	tmplSvc := templatemocks.NewMockService(t)
	tmplSvc.EXPECT().
		FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "fantasy_v1"}).
		Return(&template.Template{ID: "fantasy_v1", State: template.StateActive}, nil).
		Once()
	svc := sheet.NewService(repo, tmplSvc, sheettest.DAGMap{}, testLog, events.NoOp{})

	_, err := svc.Duplicate(context.Background(), sheet.DuplicateSheetCommand{SheetID: updateSheetID, OwnerID: ownerUserID})
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

func TestService_Duplicate_EmitsSheetCreatedWithoutValues(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	pub := eventsmocks.NewMockPublisher(t)
	got := captureEvents(pub)
	repo.EXPECT().FetchByID(mock.Anything, updateSheetID).RunAndReturn(
		func(context.Context, string) (*sheet.Sheet, error) {
			cp := duplicateSource(t)
			cp.Name = "Secret"
			return &cp, nil
		},
	).Once()
	repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindNPC, 2).Return(nil).Once()
	repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newDuplicateService(t, repo, planFor(t, ownerUserID, plan.Free), runInline(t), pub)
	copied, err := svc.Duplicate(context.Background(), sheet.DuplicateSheetCommand{SheetID: updateSheetID, OwnerID: ownerUserID})
	require.NoError(t, err)

	require.Len(t, *got, 1)
	ev := (*got)[0]
	assert.Equal(t, "sheet.created", ev.Type)
	assert.Equal(t, copied.ID, ev.AggregateID)
	assert.Equal(t, map[string]any{"template_id": "fantasy_v1"}, ev.Data)
}

func TestService_Delete_RemovesLinksAndSlotInOneTransaction(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	cleaner := mocks.NewMockLinkCleaner(t)
	removed := []sheet.RemovedLink{{CampaignID: "01900000-0000-7000-8000-0000000000aa", SheetID: updateSheetID, OwnerID: ownerUserID}}
	repo.EXPECT().FetchByID(mock.Anything, updateSheetID).
		Return(&sheet.Sheet{ID: updateSheetID, OwnerID: ownerUserID, Kind: plan.KindNPC}, nil).Once()
	repo.EXPECT().DeleteByID(mock.Anything, updateSheetID).Return(nil).Once()
	cleaner.EXPECT().DeleteForSheet(mock.Anything, updateSheetID).Return(removed, nil).Once()
	repo.EXPECT().ReleaseSlot(mock.Anything, ownerUserID, plan.KindNPC).Return(nil).Once()
	cleaner.EXPECT().PublishSheetUnlinked(plan.KindNPC, removed).Once()
	svc := sheet.NewService(repo, templatemocks.NewMockService(t), sheettest.DAGMap{}, testLog, events.NoOp{},
		sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t)),
		sheet.WithLinkCleaner(cleaner),
	)

	require.NoError(t, svc.Delete(context.Background(), sheet.DeleteSheetCommand{SheetID: updateSheetID, OwnerID: ownerUserID}))
}

func TestService_Delete_LinkCleanupFailureAbortsTheDelete(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	cleaner := mocks.NewMockLinkCleaner(t)
	repo.EXPECT().FetchByID(mock.Anything, updateSheetID).
		Return(&sheet.Sheet{ID: updateSheetID, OwnerID: ownerUserID, Kind: plan.KindNPC}, nil).Once()
	repo.EXPECT().DeleteByID(mock.Anything, updateSheetID).Return(nil).Once()
	cleaner.EXPECT().DeleteForSheet(mock.Anything, updateSheetID).Return(nil, errors.New("link store down")).Once()
	svc := sheet.NewService(repo, templatemocks.NewMockService(t), sheettest.DAGMap{}, testLog, events.NoOp{},
		sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t)),
		sheet.WithLinkCleaner(cleaner),
	)

	err := svc.Delete(context.Background(), sheet.DeleteSheetCommand{SheetID: updateSheetID, OwnerID: ownerUserID})
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
	repo.AssertNotCalled(t, "ReleaseSlot")
	cleaner.AssertNotCalled(t, "PublishSheetUnlinked", mock.Anything, mock.Anything)
}
