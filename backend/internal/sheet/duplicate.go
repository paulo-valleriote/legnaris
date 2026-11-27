package sheet

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/sheeterr"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/maputils"
)

type DuplicateSheetCommand struct {
	SheetID string `json:"-"`
	OwnerID string `json:"-"`
}

const (
	MsgDuplicateKindMismatch = "only npc and enemy sheets can be duplicated"
	MsgPlanLimitReached      = "plan limit reached for this sheet kind"
	duplicateNameSuffix      = " (copy)"
)

func (s *ServiceImpl) duplicateTemplate(ctx context.Context, stored *Sheet) (*template.Template, error) {
	tmpl, err := s.templateService.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: stored.TemplateID})
	if err != nil {
		return nil, translateTemplateErr(err)
	}
	if !tmpl.Available(s.draftsAllowed) {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, MsgTemplateStateRefused, nil)
	}
	return tmpl, nil
}

func duplicateName(tmpl *template.Template, data map[string]any, stored string) (string, error) {
	base := tmpl.SuggestName(data)
	if base == "" {
		base = strings.TrimSpace(stored)
	}
	name := fitDuplicateName(tmpl, base)
	if tmpl.NameLeaf == "" {
		return name, nil
	}
	if err := template.Path(strings.Split(tmpl.NameLeaf, ".")).Set(data, name); err != nil {
		return "", apperr.NewError(apperr.CodeInternal, "failed to copy sheet name", err)
	}
	return name, nil
}

func fitDuplicateName(tmpl *template.Template, base string) string {
	if base == "" {
		return "(copy)"
	}
	maxLength := tmpl.NameMaxLength()
	if maxLength > 0 {
		allowed := maxLength - utf8.RuneCountInString(duplicateNameSuffix)
		if allowed < 0 {
			allowed = 0
		}
		runes := []rune(base)
		if len(runes) > allowed {
			base = string(runes[:allowed])
		}
	}
	if base == "" {
		return "(copy)"
	}
	return base + duplicateNameSuffix
}

func (s *ServiceImpl) Duplicate(ctx context.Context, cmd DuplicateSheetCommand) (*Sheet, error) {
	if err := requireOwner(cmd.OwnerID); err != nil {
		return nil, err
	}
	if err := requireSheetID(cmd.SheetID); err != nil {
		return nil, err
	}
	stored, err := s.repository.FetchByID(ctx, cmd.SheetID)
	if err != nil {
		return nil, translateRepoErr("duplicate", err)
	}
	if err := checkOwnership(stored, cmd.OwnerID); err != nil {
		return nil, err
	}
	if stored.Kind != plan.KindNPC && stored.Kind != plan.KindEnemy {
		return nil, apperr.NewError(apperr.CodeSheetKindMismatch, MsgDuplicateKindMismatch, nil)
	}
	tmpl, err := s.duplicateTemplate(ctx, stored)
	if err != nil {
		return nil, err
	}
	if !s.planLimitsEnabled() {
		return nil, apperr.NewError(apperr.CodeInternal, MsgLimitsNotConfigured, nil)
	}
	current, err := s.plans.PlanOf(ctx, cmd.OwnerID)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, "failed to resolve plan", err)
	}
	limit := current.SheetLimit(stored.Kind)
	sheetID, err := idgen.New()
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, "failed to generate sheet id", err)
	}
	now := time.Now()
	data := maputils.DeepCopy(stored.Data)
	name, err := duplicateName(tmpl, data, stored.Name)
	if err != nil {
		return nil, err
	}
	copied := Sheet{
		ID:         sheetID,
		TemplateID: stored.TemplateID,
		OwnerID:    stored.OwnerID,
		Kind:       stored.Kind,
		Name:       name,
		Data:       data,
		Layout:     stored.Layout.Clone(),
		Dice:       stored.Dice.Clone(),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	err = s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repository.ReserveSlot(txCtx, cmd.OwnerID, stored.Kind, limit); err != nil {
			return err
		}
		return s.repository.Create(txCtx, &copied)
	})
	if err != nil {
		if errors.Is(err, sheeterr.ErrSheetLimitReached) {
			return nil, apperr.NewError(apperr.CodePlanLimitReached, MsgPlanLimitReached, err).
				WithDetails(map[string]any{detailKindKey: string(stored.Kind), "limit": limit})
		}
		return nil, translateRepoErr("duplicate", err)
	}
	s.publishSheetEvent(eventSheetCreated, cmd.OwnerID, copied.ID, copied.Version, map[string]any{detailTemplateIDKey: copied.TemplateID})
	return &copied, nil
}
