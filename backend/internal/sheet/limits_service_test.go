package sheet_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/sheettest"
	"legnaris/backend/internal/sheet/template"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// limitTemplateID is the template the limit tests create Sheets from. It accepts
// only characters, so kind refusals are exercised too.
const limitTemplateID = "limit_test"

// limitFields is a minimal Template tree: one editable number, enough for a
// creation to run the DAG.
var limitFields = template.FieldGroup{
	"stats": {Children: template.FieldGroup{
		"dexterity": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
	}},
}

// limitDAG is the compiled DAG the limit tests serve from the in-memory registry.
func limitDAG(t *testing.T) sheettest.DAGMap {
	t.Helper()
	dag, err := template.BuildDAG(limitTemplateID, limitFields, engine.NewFormulaEngine())
	require.NoError(t, err)
	return sheettest.DAGMap{limitTemplateID: dag}
}

// limitTemplateService serves the limit template with the given kinds declared.
func limitTemplateService(t *testing.T, kinds ...plan.SheetKind) *templatemocks.MockService {
	t.Helper()
	svc := templatemocks.NewMockService(t)
	svc.EXPECT().
		FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: limitTemplateID}).
		Return(&template.Template{ID: limitTemplateID, Name: "Limit", Fields: limitFields, Kinds: kinds}, nil).
		Maybe()
	return svc
}

// runInline makes the transaction mock run its callback in place, so the
// repository expectations see the same calls production makes inside a transaction.
func runInline(t *testing.T) *mocks.MockTransactionRunner {
	t.Helper()
	tx := mocks.NewMockTransactionRunner(t)
	tx.EXPECT().
		WithTransaction(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		Maybe()
	return tx
}

// planFor serves the owner's plan from the plan source mock.
func planFor(t *testing.T, owner string, p plan.Plan) *mocks.MockPlanSource {
	t.Helper()
	plans := mocks.NewMockPlanSource(t)
	plans.EXPECT().PlanOf(mock.Anything, owner).Return(p, nil).Maybe()
	return plans
}

// newLimitedService builds the sheet service with plan limits wired, the way
// production does.
func newLimitedService(repo sheet.Repository, tmpl *templatemocks.MockService, dags sheettest.DAGMap, plans sheet.PlanSource, tx sheet.TransactionRunner) sheet.Service {
	return sheet.NewService(repo, tmpl, dags, logger.NewLogger(slog.LevelError), events.NoOp{}, sheet.WithPlanLimits(plans, tx))
}

func TestService_CreateKindValidation(t *testing.T) {
	tests := []struct {
		name string
		kind string
	}{
		{name: "Error: a missing kind is refused", kind: ""},
		{name: "Error: an unknown kind is refused", kind: "vehicle"},
		{name: "Error: a kind in the wrong case is refused", kind: "Character"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t) // no expectations: nothing is read or written
			svc := newLimitedService(repo, templatemocks.NewMockService(t), limitDAG(t), mocks.NewMockPlanSource(t), mocks.NewMockTransactionRunner(t))

			created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
				TemplateID: limitTemplateID, Kind: tc.kind, SheetName: "x", OwnerID: ownerUserID,
			})
			require.Error(t, err)
			assert.Nil(t, created)

			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
			assert.Equal(t, 400, appErr.HttpStatus())
			assert.Equal(t, sheet.MsgKindInvalid, appErr.Message)
		})
	}
}

func TestService_CreateTemplateRefusesKind(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	svc := newLimitedService(repo, limitTemplateService(t, plan.KindCharacter), limitDAG(t),
		planFor(t, ownerUserID, plan.Free), mocks.NewMockTransactionRunner(t))

	created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
		TemplateID: limitTemplateID, Kind: string(plan.KindNPC), SheetName: "npc", OwnerID: ownerUserID,
	})
	require.Error(t, err)
	assert.Nil(t, created)

	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
	assert.Equal(t, sheet.MsgTemplateKindRefused, appErr.Message)
	repo.AssertNotCalled(t, "ReserveSlot")
	repo.AssertNotCalled(t, "Create")
}

func TestService_CreateEnforcesPlanLimit(t *testing.T) {
	tests := []struct {
		name        string
		plan        plan.Plan
		kind        plan.SheetKind
		wantLimit   int
		reserveErr  error
		wantCode    apperr.Code
		wantCreated bool
	}{
		{name: "Happy path: a Free character under its allowance is created", plan: plan.Free, kind: plan.KindCharacter, wantLimit: 1, wantCreated: true},
		{name: "Happy path: a Paid character reserves against the Paid allowance", plan: plan.Paid, kind: plan.KindCharacter, wantLimit: 10, wantCreated: true},
		{name: "Error: a Free character at its allowance is a 409 with no insert", plan: plan.Free, kind: plan.KindCharacter, wantLimit: 1, reserveErr: sheet.ErrSheetLimitReached, wantCode: apperr.CodeSheetLimitReached},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, tc.kind, tc.wantLimit).Return(tc.reserveErr).Once()
			if tc.wantCreated {
				repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(s *sheet.Sheet) bool {
					return s.OwnerID == ownerUserID && s.Kind == tc.kind
				})).Return(nil).Once()
			}
			svc := newLimitedService(repo, limitTemplateService(t, tc.kind), limitDAG(t),
				planFor(t, ownerUserID, tc.plan), runInline(t))

			created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
				TemplateID: limitTemplateID, Kind: string(tc.kind), SheetName: "Mine", OwnerID: ownerUserID,
			})
			if !tc.wantCreated {
				require.Error(t, err)
				assert.Nil(t, created)
				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, tc.wantCode, appErr.Code)
				assert.Equal(t, 409, appErr.HttpStatus())
				assert.Equal(t, map[string]any{"kind": string(tc.kind), "limit": tc.wantLimit}, appErr.Details)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.kind, created.Kind)
			assert.Equal(t, ownerUserID, created.OwnerID)
		})
	}
}

func TestService_CreateInsertFailureIsNotALimit(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindCharacter, 1).Return(nil).Once()
	repo.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("disk on fire")).Once()
	svc := newLimitedService(repo, limitTemplateService(t, plan.KindCharacter), limitDAG(t),
		planFor(t, ownerUserID, plan.Free), runInline(t))

	created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
		TemplateID: limitTemplateID, Kind: string(plan.KindCharacter), SheetName: "Race", OwnerID: ownerUserID,
	})
	require.Error(t, err)
	assert.Nil(t, created)

	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
	assert.Equal(t, 500, appErr.HttpStatus())
}

func TestService_CreateWithoutPlanLimitsRefuses(t *testing.T) {
	repo := mocks.NewMockRepository(t) // no expectations: must never be called
	svc := sheet.NewService(repo, limitTemplateService(t, plan.KindCharacter), limitDAG(t), logger.NewLogger(slog.LevelError), events.NoOp{})

	created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
		TemplateID: limitTemplateID, Kind: string(plan.KindCharacter), SheetName: "x", OwnerID: ownerUserID,
	})
	require.Error(t, err)
	assert.Nil(t, created)

	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

func TestService_CreateMissingOwnerIsUnauthorized(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	svc := newLimitedService(repo, templatemocks.NewMockService(t), limitDAG(t),
		mocks.NewMockPlanSource(t), mocks.NewMockTransactionRunner(t))

	created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
		TemplateID: limitTemplateID, Kind: string(plan.KindCharacter), SheetName: "Ownerless",
	})
	require.Error(t, err)
	assert.Nil(t, created)

	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeUnauthorized, appErr.Code)
	assert.Equal(t, sheet.MsgOwnerNotProvided, appErr.Message)
}

func TestService_DeleteReleasesSlotInTransaction(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FetchByID(mock.Anything, updateSheetID).
		Return(&sheet.Sheet{ID: updateSheetID, OwnerID: ownerUserID, Kind: plan.KindNPC}, nil).Once()
	repo.EXPECT().DeleteByID(mock.Anything, updateSheetID).Return(nil).Once()
	repo.EXPECT().ReleaseSlot(mock.Anything, ownerUserID, plan.KindNPC).Return(nil).Once()
	svc := newLimitedService(repo, templatemocks.NewMockService(t), limitDAG(t), planFor(t, ownerUserID, plan.Free), runInline(t))

	require.NoError(t, svc.Delete(context.Background(), sheet.DeleteSheetCommand{SheetID: updateSheetID, OwnerID: ownerUserID}))
}

func TestService_DeleteFailureKeepsSlot(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FetchByID(mock.Anything, updateSheetID).
		Return(&sheet.Sheet{ID: updateSheetID, OwnerID: ownerUserID, Kind: plan.KindCharacter}, nil).Once()
	repo.EXPECT().DeleteByID(mock.Anything, updateSheetID).Return(errors.New("write refused")).Once()
	svc := newLimitedService(repo, templatemocks.NewMockService(t), limitDAG(t), planFor(t, ownerUserID, plan.Free), runInline(t))

	err := svc.Delete(context.Background(), sheet.DeleteSheetCommand{SheetID: updateSheetID, OwnerID: ownerUserID})
	require.Error(t, err)
	repo.AssertNotCalled(t, "ReleaseSlot")
}

func TestService_CheckSheetLimit(t *testing.T) {
	tests := []struct {
		name      string
		owner     string
		kind      plan.SheetKind
		plan      plan.Plan
		used      int
		wantUsed  int
		wantLimit int
		wantErr   apperr.Code
	}{
		{name: "Happy path: an owner under the allowance reads its usage", owner: ownerUserID, kind: plan.KindNPC, plan: plan.Free, used: 1, wantUsed: 1, wantLimit: 2},
		{name: "Happy path: a Paid owner reads the Paid allowance", owner: ownerUserID, kind: plan.KindCharacter, plan: plan.Paid, used: 4, wantUsed: 4, wantLimit: 10},
		{name: "Edge: an owner with no counter reads zero used", owner: ownerUserID, kind: plan.KindMonster, plan: plan.Free, used: 0, wantUsed: 0, wantLimit: 2},
		{name: "Error: a missing owner is refused before any read", owner: "", kind: plan.KindNPC, wantErr: apperr.CodeUnauthorized},
		{name: "Error: an unknown kind is refused before any read", owner: ownerUserID, kind: "vehicle", wantErr: apperr.CodeInvalidRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			if tc.wantErr == "" {
				repo.EXPECT().CountOf(mock.Anything, tc.owner, tc.kind).Return(tc.used, nil).Once()
			}
			svc := newLimitedService(repo, templatemocks.NewMockService(t), limitDAG(t), planFor(t, tc.owner, tc.plan), runInline(t))

			status, err := svc.CheckSheetLimit(context.Background(), tc.owner, tc.kind)
			if tc.wantErr != "" {
				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, tc.wantErr, appErr.Code)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantUsed, status.Used)
			assert.Equal(t, tc.wantLimit, status.Limit)
			assert.Equal(t, tc.wantUsed >= tc.wantLimit, status.Reached())
		})
	}
}

func TestService_CheckSheetLimitReachedAtAllowance(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().CountOf(mock.Anything, ownerUserID, plan.KindCharacter).Return(1, nil).Once()
	svc := newLimitedService(repo, templatemocks.NewMockService(t), limitDAG(t), planFor(t, ownerUserID, plan.Free), runInline(t))

	status, err := svc.CheckSheetLimit(context.Background(), ownerUserID, plan.KindCharacter)
	require.NoError(t, err)
	assert.True(t, status.Reached(), "one character on Free is the whole allowance")
}

func TestService_PlanUsage(t *testing.T) {
	tests := []struct {
		name   string
		plan   plan.Plan
		counts map[plan.SheetKind]int
		want   []sheet.LimitStatus
	}{
		{
			name:   "Happy path: Free reports every kind with its allowance",
			plan:   plan.Free,
			counts: map[plan.SheetKind]int{plan.KindCharacter: 1},
			want: []sheet.LimitStatus{
				{Kind: plan.KindCharacter, Used: 1, Limit: 1},
				{Kind: plan.KindNPC, Used: 0, Limit: 2},
				{Kind: plan.KindMonster, Used: 0, Limit: 2},
			},
		},
		{
			name:   "Happy path: Paid reports the Paid allowances",
			plan:   plan.Paid,
			counts: map[plan.SheetKind]int{plan.KindNPC: 3},
			want: []sheet.LimitStatus{
				{Kind: plan.KindCharacter, Used: 0, Limit: 10},
				{Kind: plan.KindNPC, Used: 3, Limit: 50},
				{Kind: plan.KindMonster, Used: 0, Limit: 50},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			for _, kind := range plan.SheetKinds() {
				repo.EXPECT().CountOf(mock.Anything, ownerUserID, kind).Return(tc.counts[kind], nil).Once()
			}
			svc := newLimitedService(repo, templatemocks.NewMockService(t), limitDAG(t), planFor(t, ownerUserID, tc.plan), runInline(t))

			usage, err := svc.PlanUsage(context.Background(), ownerUserID)
			require.NoError(t, err)
			assert.Equal(t, tc.plan.ID, usage.PlanID)
			assert.Equal(t, tc.want, usage.Kinds)
		})
	}
}

// limitOpts wires a Free owner's plan and an inline transaction, the defaults
// the creation tests share.
func limitOpts(t *testing.T) sheet.ServiceOption {
	t.Helper()
	return sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t))
}

func TestService_PlanUsageRefusesMissingOwner(t *testing.T) {
	svc := newLimitedService(mocks.NewMockRepository(t), templatemocks.NewMockService(t), limitDAG(t),
		mocks.NewMockPlanSource(t), runInline(t))

	_, err := svc.PlanUsage(context.Background(), "")
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeUnauthorized, appErr.Code)
}
