package sheet_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"legnaris/backend/internal/events"
	eventsmocks "legnaris/backend/internal/events/mocks"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/sheeterr"
	"legnaris/backend/internal/sheet/template"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	campaignCreateID      = "01900000-0000-7000-8000-0000000000cc"
	campaignCreateOtherID = "01900000-0000-7000-8000-0000000000cd"
)

func newCampaignCreateService(
	t *testing.T,
	repo sheet.Repository,
	state template.State,
	kinds []plan.SheetKind,
	scope sheet.CampaignScope,
	publisher events.Publisher,
) sheet.Service {
	t.Helper()
	templates := templatemocks.NewMockService(t)
	templates.EXPECT().
		FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: limitTemplateID}).
		Return(&template.Template{
			ID:     limitTemplateID,
			Name:   "Campaign",
			State:  state,
			Fields: limitFields,
			Kinds:  kinds,
		}, nil).
		Maybe()
	return sheet.NewService(repo, templates, limitDAG(t), logger.NewLogger(slog.LevelError), publisher,
		sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t)),
		sheet.WithCampaignScope(scope),
	)
}

func allSheetKinds() []plan.SheetKind {
	return []plan.SheetKind{plan.KindCharacter, plan.KindNPC, plan.KindEnemy}
}

func TestService_CreateRetiredTemplateFromCampaign(t *testing.T) {
	portErr := errors.New("campaign lookup failed")
	tests := []struct {
		name         string
		campaignID   string
		scopeAllowed bool
		scopeErr     error
		scopeCalled  bool
		reserveErr   error
		wantCreated  bool
		wantCode     apperr.Code
		wantMessage  string
	}{
		{
			name:        "Edge: a malformed campaign id is refused with the campaign message",
			campaignID:  "not-a-uuid",
			wantCode:    apperr.CodeInvalidRequest,
			wantMessage: sheet.MsgCampaignScopeRefused,
		},
		{
			name:        "Edge: a missing campaign id keeps the template state refusal",
			wantCode:    apperr.CodeInvalidRequest,
			wantMessage: sheet.MsgTemplateStateRefused,
		},
		{
			name:        "Error: a foreign campaign or a non-member is refused",
			campaignID:  campaignCreateOtherID,
			scopeCalled: true,
			wantCode:    apperr.CodeInvalidRequest,
			wantMessage: sheet.MsgCampaignScopeRefused,
		},
		{
			name:        "Error: a template outside the campaign scope is refused",
			campaignID:  campaignCreateID,
			scopeCalled: true,
			wantCode:    apperr.CodeInvalidRequest,
			wantMessage: sheet.MsgCampaignScopeRefused,
		},
		{
			name:        "Error: a campaign lookup failure is refused with the same message",
			campaignID:  campaignCreateID,
			scopeErr:    portErr,
			scopeCalled: true,
			wantCode:    apperr.CodeInvalidRequest,
			wantMessage: sheet.MsgCampaignScopeRefused,
		},
		{
			name:         "Error: a reached plan limit still reports the sheet limit",
			campaignID:   campaignCreateID,
			scopeAllowed: true,
			scopeCalled:  true,
			reserveErr:   sheeterr.ErrSheetLimitReached,
			wantCode:     apperr.CodeSheetLimitReached,
			wantMessage:  sheet.MsgSheetLimitReached,
		},
		{
			name:         "Happy path: a retired template is creatable inside an allowing campaign",
			campaignID:   campaignCreateID,
			scopeAllowed: true,
			scopeCalled:  true,
			wantCreated:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			scope := mocks.NewMockCampaignScope(t)
			if tc.scopeCalled {
				scope.EXPECT().
					AllowsTemplate(mock.Anything, ownerUserID, tc.campaignID, limitTemplateID).
					Return(tc.scopeAllowed, tc.scopeErr).
					Once()
			}
			if tc.wantCreated || tc.reserveErr != nil {
				repo.EXPECT().
					ReserveSlot(mock.Anything, ownerUserID, plan.KindCharacter, plan.Free.SheetLimit(plan.KindCharacter)).
					Return(tc.reserveErr).
					Once()
			}
			if tc.wantCreated {
				repo.EXPECT().
					Create(mock.Anything, mock.MatchedBy(func(created *sheet.Sheet) bool {
						return created.TemplateID == limitTemplateID &&
							created.Kind == plan.KindCharacter &&
							created.CampaignID == nil
					})).
					Return(nil).
					Once()
			}
			publisher := eventsmocks.NewMockPublisher(t)
			published := make([]events.Event, 0)
			publisher.EXPECT().Publish(mock.Anything).Run(func(ev events.Event) {
				published = append(published, ev)
			}).Maybe()

			svc := newCampaignCreateService(t, repo, template.StateRetired, allSheetKinds(),
				scope, publisher)

			created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
				TemplateID: limitTemplateID,
				Kind:       string(plan.KindCharacter),
				SheetName:  "Hero",
				CampaignID: tc.campaignID,
				OwnerID:    ownerUserID,
			})

			if tc.wantCreated {
				require.NoError(t, err)
				require.NotNil(t, created)
				assert.Equal(t, limitTemplateID, created.TemplateID)
				assert.Nil(t, created.CampaignID)
				repo.AssertNotCalled(t, "LinkCampaign", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				require.Len(t, published, 1)
				assert.Equal(t, "sheet.created", published[0].Type)
				return
			}

			require.Error(t, err)
			assert.Nil(t, created)
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, tc.wantCode, appErr.Code)
			assert.Equal(t, tc.wantMessage, appErr.Message)
			repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
			repo.AssertNotCalled(t, "LinkCampaign", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			assert.Empty(t, published)
		})
	}
}

func TestService_CreateRetiredTemplateFromCampaignForEachKind(t *testing.T) {
	kinds := []struct {
		name string
		kind plan.SheetKind
	}{
		{name: "Happy path: character", kind: plan.KindCharacter},
		{name: "Happy path: npc", kind: plan.KindNPC},
		{name: "Happy path: enemy", kind: plan.KindEnemy},
	}
	for _, tc := range kinds {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			scope := mocks.NewMockCampaignScope(t)
			scope.EXPECT().
				AllowsTemplate(mock.Anything, ownerUserID, campaignCreateID, limitTemplateID).
				Return(true, nil).
				Once()
			repo.EXPECT().
				ReserveSlot(mock.Anything, ownerUserID, tc.kind, plan.Free.SheetLimit(tc.kind)).
				Return(nil).
				Once()
			repo.EXPECT().
				Create(mock.Anything, mock.MatchedBy(func(created *sheet.Sheet) bool {
					return created.Kind == tc.kind && created.CampaignID == nil
				})).
				Return(nil).
				Once()

			svc := newCampaignCreateService(t, repo, template.StateRetired, allSheetKinds(),
				scope, events.NoOp{})

			created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
				TemplateID: limitTemplateID,
				Kind:       string(tc.kind),
				SheetName:  "Retired",
				CampaignID: campaignCreateID,
				OwnerID:    ownerUserID,
			})

			require.NoError(t, err)
			require.NotNil(t, created)
			assert.Equal(t, tc.kind, created.Kind)
			assert.Nil(t, created.CampaignID)
			repo.AssertNotCalled(t, "LinkCampaign", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestService_CreateRetiredTemplateFromCampaignKindAndDraftGates(t *testing.T) {
	tests := []struct {
		name          string
		state         template.State
		kinds         []plan.SheetKind
		kind          plan.SheetKind
		draftsAllowed bool
		wantMessage   string
		scopeCalled   bool
	}{
		{
			name:        "Error: a wrong kind is refused even with an allowing campaign",
			state:       template.StateRetired,
			kinds:       []plan.SheetKind{plan.KindCharacter},
			kind:        plan.KindNPC,
			scopeCalled: true,
			wantMessage: sheet.MsgTemplateKindRefused,
		},
		{
			name:        "Error: a draft with a valid campaign is still refused outside development",
			state:       template.StateDraft,
			kinds:       allSheetKinds(),
			kind:        plan.KindCharacter,
			wantMessage: sheet.MsgTemplateStateRefused,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			scope := mocks.NewMockCampaignScope(t)
			if tc.scopeCalled {
				scope.EXPECT().
					AllowsTemplate(mock.Anything, ownerUserID, campaignCreateID, limitTemplateID).
					Return(true, nil).
					Once()
			}
			templates := templatemocks.NewMockService(t)
			templates.EXPECT().
				FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: limitTemplateID}).
				Return(&template.Template{
					ID:     limitTemplateID,
					Name:   "Campaign",
					State:  tc.state,
					Fields: limitFields,
					Kinds:  tc.kinds,
				}, nil).
				Maybe()
			svc := sheet.NewService(repo, templates, limitDAG(t), logger.NewLogger(slog.LevelError), events.NoOp{},
				sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t)),
				sheet.WithCampaignScope(scope),
				sheet.WithDraftTemplates(tc.draftsAllowed),
			)

			created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
				TemplateID: limitTemplateID,
				Kind:       string(tc.kind),
				SheetName:  "Draft",
				CampaignID: campaignCreateID,
				OwnerID:    ownerUserID,
			})

			require.Error(t, err)
			assert.Nil(t, created)
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
			assert.Equal(t, tc.wantMessage, appErr.Message)
			repo.AssertNotCalled(t, "ReserveSlot", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		})
	}
}

func TestService_CreateActiveTemplateDoesNotCheckCampaign(t *testing.T) {
	tests := []struct {
		name       string
		campaignID string
	}{
		{name: "Edge: an active template ignores a malformed campaign id", campaignID: "not-a-uuid"},
		{name: "Edge: an active template ignores a foreign campaign id", campaignID: campaignCreateOtherID},
		{name: "Happy path: an active template without a campaign id still creates", campaignID: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			scope := mocks.NewMockCampaignScope(t)
			repo.EXPECT().
				ReserveSlot(mock.Anything, ownerUserID, plan.KindCharacter, plan.Free.SheetLimit(plan.KindCharacter)).
				Return(nil).
				Once()
			repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

			svc := newCampaignCreateService(t, repo, template.StateActive, allSheetKinds(),
				scope, events.NoOp{})

			created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
				TemplateID: limitTemplateID,
				Kind:       string(plan.KindCharacter),
				SheetName:  "Active",
				CampaignID: tc.campaignID,
				OwnerID:    ownerUserID,
			})

			require.NoError(t, err)
			require.NotNil(t, created)
			assert.Nil(t, created.CampaignID)
			scope.AssertNotCalled(t, "AllowsTemplate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestService_CreateRetiredTemplateWithoutCampaignPort(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	templates := templatemocks.NewMockService(t)
	templates.EXPECT().
		FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: limitTemplateID}).
		Return(&template.Template{
			ID:     limitTemplateID,
			Name:   "Campaign",
			State:  template.StateRetired,
			Fields: limitFields,
			Kinds:  allSheetKinds(),
		}, nil).
		Once()
	svc := sheet.NewService(repo, templates, limitDAG(t), logger.NewLogger(slog.LevelError), events.NoOp{},
		sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t)),
	)

	created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
		TemplateID: limitTemplateID,
		Kind:       string(plan.KindCharacter),
		SheetName:  "Hero",
		CampaignID: campaignCreateID,
		OwnerID:    ownerUserID,
	})

	require.Error(t, err)
	assert.Nil(t, created)
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
	assert.Equal(t, sheet.MsgCampaignScopeRefused, appErr.Message)
	repo.AssertNotCalled(t, "ReserveSlot", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
