package generation_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/generation/amqp"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func gatesPass(f fixture, kind plan.SheetKind) {
	f.allowTemplate(plan.KindCharacter, plan.KindNPC, plan.KindEnemy)
	f.consent.EXPECT().Gate(mock.Anything, userID).Return(nil)
	f.sheets.EXPECT().CheckSheetLimit(mock.Anything, userID, kind).
		Return(sheet.LimitStatus{Kind: kind, Used: 0, Limit: 1}, nil)
	f.store.EXPECT().ActiveByUser(mock.Anything, userID).Return("", false, nil)
}

func TestCreate_Happy(t *testing.T) {
	tests := []struct {
		name    string
		concept string
		kind    plan.SheetKind
		wantDay string
	}{
		{
			name:    "Edge: a concept of exactly 10 characters is accepted",
			concept: "0123456789",
			wantDay: "2026-11-14",
		},
		{
			name:    "Edge: a concept of exactly 1000 characters is accepted",
			concept: stringOf('x', 1000),
			wantDay: "2026-11-14",
		},
		{
			name:    "Happy path: a paid player draws from the daily quota and records the day",
			concept: validText,
			wantDay: "2026-11-14",
		},
		{
			name:    "Happy path: an NPC generation stores its kind",
			concept: validText,
			kind:    plan.KindNPC,
			wantDay: "2026-11-14",
		},
		{
			name:    "Happy path: an enemy generation stores its kind",
			concept: validText,
			kind:    plan.KindEnemy,
			wantDay: "2026-11-14",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kind := tc.kind
			if kind == "" {
				kind = plan.KindCharacter
			}
			f := newFixture(t, defaultCfg())
			gatesPass(f, kind)
			f.runInline()
			f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(1, true, nil)
			f.observer.EXPECT().GenerationCapUsage(1, 10)
			f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Paid, nil)
			f.quota.EXPECT().ReserveDaily(mock.Anything, userID).Return(tc.wantDay, nil)

			var inserted generation.Document
			f.store.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(
				func(_ context.Context, doc generation.Document) error {
					inserted = doc
					return nil
				})
			f.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

			id, err := f.svc.Create(context.Background(), generation.CreateCommand{
				UserID: userID, TemplateID: templateID, Kind: string(kind), Concept: tc.concept,
			})
			require.NoError(t, err)
			assert.NotEmpty(t, id)
			assert.Equal(t, id, inserted.ID)
			assert.Equal(t, generation.StatusQueued, inserted.Status)
			assert.Equal(t, userID, inserted.ActiveUserID, "the in-progress slot names the player")
			assert.Equal(t, userID, inserted.UserID)
			assert.Equal(t, string(kind), inserted.Kind)
			assert.Equal(t, generation.ReservationDaily, inserted.Quota.Kind)
			assert.Equal(t, tc.wantDay, inserted.Quota.DayUTC)
			assert.Equal(t, inserted.CreatedAt.Add(24*time.Hour), inserted.ExpiresAt, "retention is 24 h")
		})
	}
}

func TestCreate_SanitizesConcept(t *testing.T) {
	tests := []struct {
		name    string
		concept string
		want    string
	}{
		{
			name:    "Happy path: a ten rune concept padded with zero-width spaces passes and is stored clean",
			concept: strings.Repeat("a", 10) + strings.Repeat("\u200b", 50),
			want:    strings.Repeat("a", 10),
		},
		{
			name:    "Edge: format, bidi and private-use characters are stripped before storage",
			concept: "\ufeff" + validText + "\u200d\u202e\ue000",
			want:    validText,
		},
		{
			name:    "Edge: tabs and line endings are normalized before storage",
			concept: "um\tmago\r\nda\rterra",
			want:    "um mago\nda\nterra",
		},
		{
			name:    "Happy path: fullwidth text is NFKC-folded before storage",
			concept: "ｕｍ ｍａｇｏ ｄｏ ｎｏｒｔｅ",
			want:    "um mago do norte",
		},
		{
			name:    "Happy path: accented Portuguese and emoji are stored unchanged",
			concept: "coração, ação 🧙",
			want:    "coração, ação 🧙",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, defaultCfg())
			gatesPass(f, plan.KindCharacter)
			f.runInline()
			f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(1, true, nil)
			f.observer.EXPECT().GenerationCapUsage(1, 10)
			f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Paid, nil)
			f.quota.EXPECT().ReserveDaily(mock.Anything, userID).Return("2026-11-14", nil)

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
			assert.Equal(t, tc.want, inserted.Concept)
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
			gatesPass(f, plan.KindCharacter)
			f.runInline()
			f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(1, true, nil)
			f.observer.EXPECT().GenerationCapUsage(1, 10)
			f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Paid, nil)
			f.quota.EXPECT().ReserveDaily(mock.Anything, userID).Return("2026-11-14", nil)
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
			gatesPass(f, plan.KindCharacter)
			f.runInline()
			f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(tc.used, tc.ok, nil)
			f.observer.EXPECT().GenerationCapUsage(10, 10)
			f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Paid, nil)
			if tc.ok {
				f.quota.EXPECT().ReserveDaily(mock.Anything, userID).Return("2026-11-14", nil)
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
		wantKind   string
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
		}, wantValues: false, wantKind: "character"},
		{name: "Happy path: a succeeded job exposes its values", id: jobID, stored: func() (generation.Document, error) {
			return generation.Document{ID: jobID, Status: generation.StatusSucceeded, Values: values, Kind: "npc"}, nil
		}, wantValues: true, wantKind: "npc"},
		{name: "Edge: a legacy document without a kind reads as a character", id: jobID, stored: func() (generation.Document, error) {
			return generation.Document{ID: jobID, Status: generation.StatusQueued, Kind: ""}, nil
		}, wantKind: "character"},
		{name: "Happy path: a failed job exposes its error code and no values", id: jobID, stored: func() (generation.Document, error) {
			return generation.Document{ID: jobID, Status: generation.StatusFailed, ErrorCode: "provider_unavailable", Values: values}, nil
		}, wantValues: false, wantError: "provider_unavailable", wantKind: "character"},
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
			assert.Equal(t, tc.wantKind, view.Kind)
		})
	}
}

func TestAccept(t *testing.T) {
	const jobID = "0190a1b2-0000-7000-8000-0000000000aa"
	const sheetID = "0190a1b2-0000-7000-8000-0000000000bb"
	const otherSheetID = "0190a1b2-0000-7000-8000-0000000000cc"
	heroValues := map[string]any{"name": "Hero"}

	succeeded := generation.Document{ID: jobID, UserID: userID, TemplateID: templateID, Status: generation.StatusSucceeded}
	succeededNPC := generation.Document{ID: jobID, UserID: userID, TemplateID: templateID, Kind: "npc", Status: generation.StatusSucceeded}
	succeededEnemy := generation.Document{ID: jobID, UserID: userID, TemplateID: templateID, Kind: "enemy", Status: generation.StatusSucceeded}
	accepted := generation.Document{ID: jobID, UserID: userID, Status: generation.StatusAccepted, SheetID: sheetID}
	acceptedNPC := generation.Document{ID: jobID, UserID: userID, Kind: "npc", Status: generation.StatusAccepted, SheetID: sheetID}

	tests := []struct {
		name       string
		cmd        generation.AcceptCommand
		stored     *generation.Document
		storedErr  error
		sheetErr   error
		race       bool
		sheetKind  string
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
		{name: "Happy path: an NPC generation becomes an NPC Sheet", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "Villager", Values: heroValues}, stored: &succeededNPC, sheetKind: "npc", wantCreate: true, wantSheet: sheetID},
		{name: "Happy path: an enemy generation becomes an enemy Sheet", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "Wolf", Values: heroValues}, stored: &succeededEnemy, sheetKind: "enemy", wantCreate: true, wantSheet: sheetID},
		{name: "Edge: a repeat accept answers the same Sheet with no new creation", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "Hero", Values: heroValues}, stored: &accepted, wantSheet: sheetID},
		{name: "Edge: a repeat accept keeps the NPC kind and creates nothing", cmd: generation.AcceptCommand{UserID: userID, ID: jobID, SheetName: "Villager", Values: heroValues}, stored: &acceptedNPC, sheetKind: "npc", wantSheet: sheetID},
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
				kind := tc.sheetKind
				if kind == "" {
					kind = "character"
				}
				f.sheets.EXPECT().CreateWithValues(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
					func(ctx context.Context, cmd sheet.CreateWithValuesCommand, hook sheet.AfterInsertHook) (*sheet.Sheet, error) {
						assert.Equal(t, templateID, cmd.TemplateID)
						assert.Equal(t, kind, cmd.Kind)
						assert.Equal(t, tc.cmd.SheetName, cmd.SheetName)
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

func TestCreate_SharedQuotaAcrossKinds(t *testing.T) {
	t.Run("Edge: a paid account that drops to Free refuses the next generation with no reservation", func(t *testing.T) {
		f := newFixture(t, defaultCfg())
		f.runInline()

		gatesPass(f, plan.KindCharacter)
		f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(1, true, nil).Once()
		f.observer.EXPECT().GenerationCapUsage(1, 10).Once()
		f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Paid, nil).Once()
		f.quota.EXPECT().ReserveDaily(mock.Anything, userID).Return("2026-11-14", nil).Once()
		f.store.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()
		f.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

		_, err := f.svc.Create(context.Background(), generation.CreateCommand{
			UserID: userID, TemplateID: templateID, Kind: "character", Concept: validText,
		})
		require.NoError(t, err)

		f.allowTemplate(plan.KindCharacter, plan.KindNPC)
		f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Free, nil).Once()

		_, err = f.svc.Create(context.Background(), generation.CreateCommand{
			UserID: userID, TemplateID: templateID, Kind: "npc", Concept: validText,
		})
		requireCode(t, err, apperr.CodePlanUpgradeRequired, 403)
	})

	t.Run("Edge: a character daily reservation blocks a later NPC with QUOTA_EXCEEDED", func(t *testing.T) {
		f := newFixture(t, defaultCfg())
		f.runInline()

		gatesPass(f, plan.KindCharacter)
		f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(1, true, nil).Once()
		f.observer.EXPECT().GenerationCapUsage(1, 10).Once()
		f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Paid, nil).Once()
		f.quota.EXPECT().ReserveDaily(mock.Anything, userID).Return("2026-11-14", nil).Once()
		f.store.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()
		f.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

		_, err := f.svc.Create(context.Background(), generation.CreateCommand{
			UserID: userID, TemplateID: templateID, Kind: "character", Concept: validText,
		})
		require.NoError(t, err)

		gatesPass(f, plan.KindNPC)
		f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(2, true, nil).Once()
		f.observer.EXPECT().GenerationCapUsage(2, 10).Once()
		f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Paid, nil).Once()
		f.quota.EXPECT().ReserveDaily(mock.Anything, userID).
			Return("", apperr.NewError(apperr.CodeQuotaExceeded, "quota exceeded", nil)).Once()

		_, err = f.svc.Create(context.Background(), generation.CreateCommand{
			UserID: userID, TemplateID: templateID, Kind: "npc", Concept: validText,
		})
		requireCode(t, err, apperr.CodeQuotaExceeded, 429)
	})
}

func TestCreateWorkerAcceptFlowsForEveryKind(t *testing.T) {
	tests := []struct {
		name string
		kind plan.SheetKind
	}{
		{name: "Happy path: an NPC generation runs through create, worker and accept", kind: plan.KindNPC},
		{name: "Happy path: an enemy generation runs through create, worker and accept", kind: plan.KindEnemy},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, defaultCfg())
			gatesPass(f, tc.kind)
			f.runInline()
			f.store.EXPECT().ReserveCap(mock.Anything, mock.Anything, 10, mock.Anything).Return(1, true, nil)
			f.observer.EXPECT().GenerationCapUsage(1, 10)
			f.plans.EXPECT().PlanOf(mock.Anything, userID).Return(plan.Paid, nil)
			f.quota.EXPECT().ReserveDaily(mock.Anything, userID).Return("2026-11-14", nil)

			var inserted generation.Document
			f.store.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(
				func(_ context.Context, doc generation.Document) error {
					inserted = doc
					return nil
				})
			var published generation.Message
			f.publisher.EXPECT().Publish(mock.Anything, mock.Anything).RunAndReturn(
				func(_ context.Context, msg generation.Message) error {
					published = msg
					return nil
				}).Once()

			id, err := f.svc.Create(context.Background(), generation.CreateCommand{
				UserID: userID, TemplateID: templateID, Kind: string(tc.kind), Concept: validText,
			})
			require.NoError(t, err)
			assert.Equal(t, string(tc.kind), inserted.Kind)
			assert.Equal(t, id, published.ID)
			assert.Equal(t, string(tc.kind), published.Kind)
			assert.Equal(t, "generation."+string(tc.kind)+".requested", amqp.RoutingKey(published.Kind))

			sheetID := "0190a1b2-0000-7000-8000-0000000000bb"
			succeeded := inserted
			succeeded.Status = generation.StatusSucceeded
			f.store.EXPECT().FindOwned(mock.Anything, userID, id).Return(succeeded, nil).Once()
			var sheetKind string
			f.sheets.EXPECT().CreateWithValues(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
				func(ctx context.Context, cmd sheet.CreateWithValuesCommand, hook sheet.AfterInsertHook) (*sheet.Sheet, error) {
					sheetKind = cmd.Kind
					created := &sheet.Sheet{ID: sheetID}
					if err := hook(ctx, created); err != nil {
						return nil, err
					}
					return created, nil
				}).Once()
			f.store.EXPECT().MarkAccepted(mock.Anything, userID, id, sheetID).Return(true, nil).Once()

			result, err := f.svc.Accept(context.Background(), generation.AcceptCommand{
				UserID: userID, ID: id, SheetName: "Generated", Values: map[string]any{"identity": map[string]any{"name": "Brom"}},
			})
			require.NoError(t, err)
			assert.True(t, result.Created)
			assert.Equal(t, string(tc.kind), sheetKind)
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
