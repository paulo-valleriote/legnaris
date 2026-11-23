package sheet_test

import (
	"context"
	"log/slog"
	"testing"

	"legnaris/backend/internal/events"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/template"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func lifecycleTemplate(state template.State) *template.Template {
	return &template.Template{
		ID:     limitTemplateID,
		Name:   "Lifecycle",
		State:  state,
		Fields: limitFields,
		Kinds:  []plan.SheetKind{plan.KindCharacter, plan.KindNPC, plan.KindEnemy},
	}
}

func lifecycleTemplateService(t *testing.T, state template.State) *templatemocks.MockService {
	t.Helper()
	svc := templatemocks.NewMockService(t)
	svc.EXPECT().
		FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: limitTemplateID}).
		Return(lifecycleTemplate(state), nil).
		Maybe()
	return svc
}

func newLifecycleService(t *testing.T, repo sheet.Repository, tmplSvc *templatemocks.MockService, draftsAllowed bool) sheet.Service {
	t.Helper()
	return sheet.NewService(repo, tmplSvc, limitDAG(t), logger.NewLogger(slog.LevelError), events.NoOp{},
		sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t)),
		sheet.WithDraftTemplates(draftsAllowed),
	)
}

func TestService_CreateTemplateLifecycle(t *testing.T) {
	tests := []struct {
		name          string
		state         template.State
		draftsAllowed bool
		wantCreated   bool
		wantMessage   string
	}{
		{name: "Edge: a draft outside development is refused as unavailable", state: template.StateDraft, draftsAllowed: false, wantMessage: sheet.MsgTemplateStateRefused},
		{name: "Happy path: a draft is creatable in development", state: template.StateDraft, draftsAllowed: true, wantCreated: true},
		{name: "Happy path: an active template is creatable outside development", state: template.StateActive, draftsAllowed: false, wantCreated: true},
		{name: "Happy path: an active template is creatable in development", state: template.StateActive, draftsAllowed: true, wantCreated: true},
		{name: "Error: a retired template is refused as retired outside development", state: template.StateRetired, draftsAllowed: false, wantMessage: sheet.MsgTemplateRetiredRefused},
		{name: "Error: a retired template is refused as retired in development too", state: template.StateRetired, draftsAllowed: true, wantMessage: sheet.MsgTemplateRetiredRefused},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			if tc.wantCreated {
				repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindCharacter, 1).Return(nil).Once()
				repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
			}
			svc := newLifecycleService(t, repo, lifecycleTemplateService(t, tc.state), tc.draftsAllowed)

			created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
				TemplateID: limitTemplateID, Kind: string(plan.KindCharacter), SheetName: "Hero", OwnerID: ownerUserID,
			})

			if tc.wantCreated {
				require.NoError(t, err)
				require.NotNil(t, created)
				assert.Equal(t, limitTemplateID, created.TemplateID)
				return
			}

			require.Error(t, err)
			assert.Nil(t, created)
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
			assert.Equal(t, 400, appErr.HttpStatus())
			assert.Equal(t, tc.wantMessage, appErr.Message)
			repo.AssertNotCalled(t, "ReserveSlot")
			repo.AssertNotCalled(t, "Create")
		})
	}
}

func TestService_CreateWithValuesTemplateLifecycle(t *testing.T) {
	tests := []struct {
		name          string
		state         template.State
		draftsAllowed bool
		wantCreated   bool
	}{
		{name: "Edge: accepting a draft generation outside development is refused", state: template.StateDraft, draftsAllowed: false},
		{name: "Happy path: accepting a draft generation in development creates the Sheet", state: template.StateDraft, draftsAllowed: true, wantCreated: true},
		{name: "Happy path: accepting a retired generation outside development creates the Sheet", state: template.StateRetired, draftsAllowed: false, wantCreated: true},
		{name: "Happy path: accepting a retired generation in development creates the Sheet", state: template.StateRetired, draftsAllowed: true, wantCreated: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			if tc.wantCreated {
				repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindCharacter, 1).Return(nil).Once()
				repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
			}
			svc := newLifecycleService(t, repo, lifecycleTemplateService(t, tc.state), tc.draftsAllowed)

			created, err := svc.CreateWithValues(context.Background(), sheet.CreateWithValuesCommand{
				TemplateID: limitTemplateID, Kind: string(plan.KindCharacter), SheetName: "Hero", OwnerID: ownerUserID,
				Values: map[string]any{"stats": map[string]any{"dexterity": 14}},
			}, nil)

			if tc.wantCreated {
				require.NoError(t, err)
				require.NotNil(t, created)
				return
			}

			require.Error(t, err)
			assert.Nil(t, created)
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
			assert.Equal(t, sheet.MsgTemplateStateRefused, appErr.Message)
			repo.AssertNotCalled(t, "ReserveSlot")
			repo.AssertNotCalled(t, "Create")
		})
	}
}

func TestService_DuplicateTemplateLifecycle(t *testing.T) {
	tests := []struct {
		name          string
		state         template.State
		draftsAllowed bool
		wantCopied    bool
	}{
		{name: "Edge: duplicating a draft outside development is refused", state: template.StateDraft, draftsAllowed: false},
		{name: "Happy path: duplicating a draft in development creates a copy", state: template.StateDraft, draftsAllowed: true, wantCopied: true},
		{name: "Happy path: duplicating an active sheet creates a copy", state: template.StateActive, draftsAllowed: false, wantCopied: true},
		{name: "Happy path: duplicating a retired template keeps working", state: template.StateRetired, draftsAllowed: false, wantCopied: true},
		{name: "Happy path: duplicating a retired template in development keeps working", state: template.StateRetired, draftsAllowed: true, wantCopied: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			repo.EXPECT().FetchByID(mock.Anything, updateSheetID).Return(&sheet.Sheet{
				ID: updateSheetID, TemplateID: limitTemplateID, OwnerID: ownerUserID, Kind: plan.KindNPC, Name: "Brom",
			}, nil).Once()
			if tc.wantCopied {
				repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindNPC, 2).Return(nil).Once()
				repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
			}
			svc := newLifecycleService(t, repo, lifecycleTemplateService(t, tc.state), tc.draftsAllowed)

			copied, err := svc.Duplicate(context.Background(), sheet.DuplicateSheetCommand{SheetID: updateSheetID, OwnerID: ownerUserID})

			if tc.wantCopied {
				require.NoError(t, err)
				require.NotNil(t, copied)
				assert.Equal(t, limitTemplateID, copied.TemplateID)
				return
			}

			require.Error(t, err)
			assert.Nil(t, copied)
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
			assert.Equal(t, 400, appErr.HttpStatus())
			assert.Equal(t, sheet.MsgTemplateStateRefused, appErr.Message)
			repo.AssertNotCalled(t, "ReserveSlot")
			repo.AssertNotCalled(t, "Create")
		})
	}
}

func TestService_PreviewTemplateRefusesHiddenDraft(t *testing.T) {
	tests := []struct {
		name       string
		state      template.State
		catalogErr error
		wantCode   apperr.Code
		wantStatus int
	}{
		{
			name:       "Edge: a draft outside development is not found",
			state:      template.StateDraft,
			catalogErr: template.ErrTemplateNotFound,
			wantCode:   apperr.CodeNotFound,
			wantStatus: 404,
		},
		{
			name:  "Happy path: an active template previews",
			state: template.StateActive,
		},
		{
			name:  "Happy path: a retired template still previews for an existing Sheet",
			state: template.StateRetired,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmplSvc := templatemocks.NewMockService(t)
			call := tmplSvc.EXPECT().
				FetchCatalogTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: limitTemplateID})
			if tc.catalogErr != nil {
				call.Return(nil, tc.catalogErr).Once()
			} else {
				call.Return(lifecycleTemplate(tc.state), nil).Once()
			}
			svc := sheet.NewService(mocks.NewMockRepository(t), tmplSvc, limitDAG(t),
				logger.NewLogger(slog.LevelError), events.NoOp{})

			cascades, err := svc.PreviewTemplate(context.Background(), sheet.PreviewTemplateCommand{
				TemplateID: limitTemplateID,
				Data:       map[string]any{"stats": map[string]any{"dexterity": 14}},
			})

			if tc.wantCode == "" {
				require.NoError(t, err)
				assert.NotNil(t, cascades)
				return
			}
			require.Error(t, err)
			assert.Nil(t, cascades)
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, tc.wantCode, appErr.Code)
			assert.Equal(t, tc.wantStatus, appErr.HttpStatus())
		})
	}
}

func TestService_ExistingDraftSheetKeepsWorkingOutsideDevelopment(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FetchByID(mock.Anything, updateSheetID).Return(&sheet.Sheet{
		ID: updateSheetID, TemplateID: limitTemplateID, OwnerID: ownerUserID, Kind: plan.KindCharacter, Name: "Hero",
	}, nil).Once()
	repo.EXPECT().UpdateLayout(mock.Anything, updateSheetID, mock.Anything).Return(nil).Once()

	tmplSvc := lifecycleTemplateService(t, template.StateDraft)
	svc := sheet.NewService(repo, tmplSvc, limitDAG(t), logger.NewLogger(slog.LevelError), events.NoOp{},
		sheet.WithDraftTemplates(false))

	layout, err := svc.SetLayout(context.Background(), sheet.SetLayoutCommand{
		SheetID: updateSheetID,
		OwnerID: ownerUserID,
		Layout: &sheet.Layout{
			SectionOrder: []string{"stats"},
			Pinned:       []string{"stats.dexterity"},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, layout)
	assert.Equal(t, []string{"stats"}, layout.SectionOrder)
	assert.Equal(t, []string{"stats.dexterity"}, layout.Pinned)
}
