package generation_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func gatesPass(f fixture) {
	f.allowTemplate(plan.KindCharacter)
	f.consent.EXPECT().Gate(mock.Anything, userID).Return(nil)
	f.sheets.EXPECT().CheckSheetLimit(mock.Anything, userID, plan.KindCharacter).
		Return(sheet.LimitStatus{Kind: plan.KindCharacter, Used: 0, Limit: 1}, nil)
	f.store.EXPECT().ActiveByUser(mock.Anything, userID).Return("", false, nil)
}

func TestCreate_Happy(t *testing.T) {
	tests := []struct {
		name       string
		concept    string
		plan       plan.Plan
		setupQuota func(f fixture)
		wantKind   string
		wantDay    string
	}{
		{
			name:    "Edge: a concept of exactly 10 characters is accepted",
			concept: "0123456789",
			plan:    plan.Free,
			setupQuota: func(f fixture) {
				f.quota.EXPECT().ReserveTrial(mock.Anything, userID).Return(nil)
			},
			wantKind: generation.ReservationTrial,
		},
		{
			name:    "Edge: a concept of exactly 1000 characters is accepted",
			concept: stringOf('x', 1000),
			plan:    plan.Free,
			setupQuota: func(f fixture) {
				f.quota.EXPECT().ReserveTrial(mock.Anything, userID).Return(nil)
			},
			wantKind: generation.ReservationTrial,
		},
		{
			name:    "Happy path: a paid player draws from the daily quota and records the day",
			concept: validText,
			plan:    plan.Paid,
			setupQuota: func(f fixture) {
				f.quota.EXPECT().ReserveDaily(mock.Anything, userID).Return("2026-11-14", nil)
			},
			wantKind: generation.ReservationDaily,
			wantDay:  "2026-11-14",
		},
		{
			name:    "Happy path: a Free player draws from the lifetime trial",
			concept: validText,
			plan:    plan.Free,
			setupQuota: func(f fixture) {
				f.quota.EXPECT().ReserveTrial(mock.Anything, userID).Return(nil)
			},
			wantKind: generation.ReservationTrial,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, defaultCfg())
			gatesPass(f)
			f.runInline()
			f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(1, true, nil)
			f.observer.EXPECT().GenerationCapUsage(1, 10)
			f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(tc.plan, nil)
			tc.setupQuota(f)

			var inserted generation.Document
			f.store.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(
				func(_ context.Context, doc generation.Document) error {
					inserted = doc
					return nil
				})
			f.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

			id, err := f.svc.Create(context.Background(), generation.CreateCommand{
				UserID: userID, TemplateID: templateID, Kind: "character", Concept: tc.concept,
			})
			require.NoError(t, err)
			assert.NotEmpty(t, id)
			assert.Equal(t, id, inserted.ID)
			assert.Equal(t, generation.StatusQueued, inserted.Status)
			assert.Equal(t, userID, inserted.ActiveUserID, "the in-progress slot names the player")
			assert.Equal(t, userID, inserted.UserID)
			assert.Equal(t, "character", inserted.Kind)
			assert.Equal(t, tc.wantKind, inserted.Quota.Kind)
			assert.Equal(t, tc.wantDay, inserted.Quota.DayUTC)
			assert.Equal(t, inserted.CreatedAt.Add(24*time.Hour), inserted.ExpiresAt, "retention is 24 h")
		})
	}
}

func TestCreate_PublishFailureStillAccepts(t *testing.T) {
	tests := []struct {
		name       string
		publishErr error
	}{
		{name: "Edge: a publish error still answers the id", publishErr: errors.New("broker down")},
		{name: "Edge: a refused publish still answers the id", publishErr: fmt.Errorf("nacked")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, defaultCfg())
			gatesPass(f)
			f.runInline()
			f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(1, true, nil)
			f.observer.EXPECT().GenerationCapUsage(1, 10)
			f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Free, nil)
			f.quota.EXPECT().ReserveTrial(mock.Anything, userID).Return(nil)
			f.store.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil)
			f.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(tc.publishErr)

			id, err := f.svc.Create(context.Background(), generation.CreateCommand{
				UserID: userID, TemplateID: templateID, Kind: "character", Concept: validText,
			})
			require.NoError(t, err)
			assert.NotEmpty(t, id)
		})
	}
}

func TestCreate_CapBoundary(t *testing.T) {
	tests := []struct {
		name     string
		used     int
		ok       bool
		wantCode apperr.Code
	}{
		{name: "Edge: the job that takes the last unit is accepted", used: 10, ok: true},
		{name: "Edge: the job past the limit is refused", used: 10, ok: false, wantCode: apperr.CodeGenerationCapReached},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, defaultCfg())
			gatesPass(f)
			f.runInline()
			f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(tc.used, tc.ok, nil)
			f.observer.EXPECT().GenerationCapUsage(10, 10)
			f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Free, nil)
			if tc.ok {
				f.quota.EXPECT().ReserveTrial(mock.Anything, userID).Return(nil)
				f.store.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil)
				f.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil)
			}

			_, err := f.svc.Create(context.Background(), generation.CreateCommand{
				UserID: userID, TemplateID: templateID, Kind: "character", Concept: validText,
			})
			if tc.wantCode == "" {
				require.NoError(t, err)
				return
			}
			requireCode(t, err, tc.wantCode, 503)
		})
	}
}

func TestFetch(t *testing.T) {
	const jobID = "0190a1b2-0000-7000-8000-0000000000aa"
	values := map[string]any{"stats": map[string]any{"dexterity": 12}}

	tests := []struct {
		name       string
		id         string
		stored     func() (generation.Document, error)
		wantCode   apperr.Code
		wantStatus int
		wantValues bool
		wantError  string
	}{
		{name: "Error: a malformed id is refused before any read", id: "not-a-uuid", wantCode: apperr.CodeInvalidRequest, wantStatus: 400},
		{name: "Error: a missing document is 404", id: jobID, stored: func() (generation.Document, error) {
			return generation.Document{}, generation.ErrNotFound
		}, wantCode: apperr.CodeNotFound, wantStatus: 404},
		{name: "Error: a foreign document is 404 too", id: jobID, stored: func() (generation.Document, error) {
			return generation.Document{}, generation.ErrNotFound
		}, wantCode: apperr.CodeNotFound, wantStatus: 404},
		{name: "Error: a storage failure is an internal error", id: jobID, stored: func() (generation.Document, error) {
			return generation.Document{}, errors.New("connection reset")
		}, wantCode: apperr.CodeInternal, wantStatus: 500},
		{name: "Happy path: a queued job reads without values", id: jobID, stored: func() (generation.Document, error) {
			return generation.Document{ID: jobID, Status: generation.StatusQueued, Values: values, Kind: "character", CreatedAt: time.Unix(0, 0).UTC()}, nil
		}, wantValues: false},
		{name: "Happy path: a succeeded job exposes its values", id: jobID, stored: func() (generation.Document, error) {
			return generation.Document{ID: jobID, Status: generation.StatusSucceeded, Values: values, Kind: "character"}, nil
		}, wantValues: true},
		{name: "Happy path: a failed job exposes its error code and no values", id: jobID, stored: func() (generation.Document, error) {
			return generation.Document{ID: jobID, Status: generation.StatusFailed, ErrorCode: "provider_unavailable", Values: values}, nil
		}, wantValues: false, wantError: "provider_unavailable"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, defaultCfg())
			if tc.stored != nil {
				doc, err := tc.stored()
				f.store.EXPECT().FindOwned(mock.Anything, userID, tc.id).Return(doc, err)
				if err == nil && doc.Status == generation.StatusSucceeded {
					f.templates.EXPECT().FetchTemplate(mock.Anything, mock.Anything).
						Return(&template.Template{ID: doc.TemplateID, NameLeaf: "identity.characterName"}, nil)
				}
			}

			view, err := f.svc.Fetch(context.Background(), userID, tc.id)
			if tc.wantCode != "" {
				requireCode(t, err, tc.wantCode, tc.wantStatus)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantValues, view.Values != nil)
			assert.Equal(t, tc.wantError, view.ErrorCode)
		})
	}
}

func TestAccept(t *testing.T) {
	const jobID = "0190a1b2-0000-7000-8000-0000000000aa"
	const sheetID = "0190a1b2-0000-7000-8000-0000000000bb"
	const otherSheetID = "0190a1b2-0000-7000-8000-0000000000cc"
	heroValues := map[string]any{"name": "Hero"}

	succeeded := generation.Document{ID: jobID, UserID: userID, TemplateID: templateID, Status: generation.StatusSucceeded}
	accepted := generation.Document{ID: jobID, UserID: userID, Status: generation.StatusAccepted, SheetID: sheetID}

	tests := []struct {
		name       string
		cmd        generation.AcceptCommand
		stored     *generation.Document
		storedErr  error
		sheetErr   error
		race       bool
		wantCreate bool
		wantSheet  string
		wantCode   apperr.Code
		wantStatus int
	}{
		{name: "Error: a malformed id is refused", cmd: generation.AcceptCommand{UserID: userID, ID: "x", SheetName: "Hero", Values: heroValues}, wantCode: apperr.CodeInvalidRequest, wantStatus: 400},
		{name: "Error: a blank sheet name is refused", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "   "}, wantCode: apperr.CodeInvalidRequest, wantStatus: 400},
		{name: "Error: missing values are refused before the generation is read", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "Hero"}, wantCode: apperr.CodeInvalidRequest, wantStatus: 400},
		{name: "Error: empty values are refused before the generation is read", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "Hero", Values: map[string]any{}}, wantCode: apperr.CodeInvalidRequest, wantStatus: 400},
		{name: "Error: a foreign or expired generation is 404", cmd: generation.AcceptCommand{UserID: otherUser, ID: jobID, SheetName: "Hero", Values: heroValues}, storedErr: generation.ErrNotFound, wantCode: apperr.CodeNotFound, wantStatus: 404},
		{name: "Error: a queued generation cannot be accepted", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "Hero", Values: heroValues}, stored: &generation.Document{ID: jobID, Status: generation.StatusQueued}, wantCode: apperr.CodeConflict, wantStatus: 409},
		{name: "Error: a running generation cannot be accepted", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "Hero", Values: heroValues}, stored: &generation.Document{ID: jobID, Status: generation.StatusRunning}, wantCode: apperr.CodeConflict, wantStatus: 409},
		{name: "Error: a failed generation cannot be accepted", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "Hero", Values: heroValues}, stored: &generation.Document{ID: jobID, Status: generation.StatusFailed}, wantCode: apperr.CodeConflict, wantStatus: 409},
		{name: "Error: the Sheet limit refuses the accept with 409", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "Hero", Values: heroValues}, stored: &succeeded,
			sheetErr: apperr.NewError(apperr.CodeSheetLimitReached, sheet.MsgSheetLimitReached, nil), wantCode: apperr.CodeSheetLimitReached, wantStatus: 409},
		{name: "Error: a value the DAG refuses is a 400", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "Hero", Values: heroValues}, stored: &succeeded,
			sheetErr: apperr.NewError(apperr.CodeUnknownField, "field does not exist", nil), wantCode: apperr.CodeUnknownField, wantStatus: 400},
		{name: "Happy path: a succeeded generation becomes a Sheet", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "Hero", Values: heroValues}, stored: &succeeded, wantCreate: true, wantSheet: sheetID},
		{name: "Edge: a repeat accept answers the same Sheet with no new creation", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "Hero", Values: heroValues}, stored: &accepted, wantSheet: sheetID},
		{name: "Edge: a lost accept race answers the winner's Sheet", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "Hero", Values: heroValues}, stored: &succeeded, race: true, wantSheet: otherSheetID},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, defaultCfg())
			if tc.stored != nil || tc.storedErr != nil {
				doc := generation.Document{}
				if tc.stored != nil {
					doc = *tc.stored
				}
				f.store.EXPECT().FindOwned(mock.Anything, tc.cmd.UserID, tc.cmd.ID).Return(doc, tc.storedErr).Once()
			}
			if tc.race {
				f.store.EXPECT().MarkAccepted(mock.Anything, userID, jobID, sheetID).Return(false, nil).Once()
				f.store.EXPECT().FindOwned(mock.Anything, userID, jobID).
					Return(generation.Document{ID: jobID, Status: generation.StatusAccepted, SheetID: otherSheetID}, nil).Once()
			}
			if tc.wantCreate || tc.race || tc.sheetErr != nil {
				f.sheets.EXPECT().CreateWithValues(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
					func(ctx context.Context, cmd sheet.CreateWithValuesCommand, hook sheet.AfterInsertHook) (*sheet.Sheet, error) {
						assert.Equal(t, templateID, cmd.TemplateID)
						assert.Equal(t, "character", cmd.Kind)
						assert.Equal(t, "Hero", cmd.SheetName)
						if tc.sheetErr != nil {
							return nil, tc.sheetErr
						}
						created := &sheet.Sheet{ID: sheetID}
						if err := hook(ctx, created); err != nil {
							return nil, fmt.Errorf("aborted: %w", err)
						}
						return created, nil
					}).Once()
				if tc.wantCreate {
					f.store.EXPECT().MarkAccepted(mock.Anything, userID, jobID, sheetID).Return(true, nil).Once()
				}
			}

			result, err := f.svc.Accept(context.Background(), tc.cmd)
			if tc.wantCode != "" {
				requireCode(t, err, tc.wantCode, tc.wantStatus)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantCreate, result.Created)
			assert.Equal(t, tc.wantSheet, result.SheetID)
		})
	}
}

func TestActiveGenerationID(t *testing.T) {
	const jobID = "0190a1b2-0000-7000-8000-0000000000aa"
	tests := []struct {
		name    string
		id      string
		active  bool
		err     error
		wantErr bool
	}{
		{name: "Edge: no job in progress reports none", active: false},
		{name: "Happy path: a queued job is reported", id: jobID, active: true},
		{name: "Error: a lookup failure is an internal error", err: errors.New("down"), wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, defaultCfg())
			f.store.EXPECT().ActiveByUser(mock.Anything, userID).Return(tc.id, tc.active, tc.err)

			id, ok, err := f.svc.ActiveGenerationID(context.Background(), userID)
			if tc.wantErr {
				requireCode(t, err, apperr.CodeInternal, 500)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.active, ok)
			assert.Equal(t, tc.id, id)
		})
	}
}

func stringOf(r rune, n int) string {
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return string(out)
}
