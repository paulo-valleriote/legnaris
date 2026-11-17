package sheet

import (
	"context"
	"errors"
	"time"

	"legnaris/backend/internal/plan"
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
	copied := Sheet{
		ID:         sheetID,
		TemplateID: stored.TemplateID,
		OwnerID:    stored.OwnerID,
		Kind:       stored.Kind,
		Name:       stored.Name + duplicateNameSuffix,
		Data:       maputils.DeepCopy(stored.Data),
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
		if errors.Is(err, ErrSheetLimitReached) {
			return nil, apperr.NewError(apperr.CodePlanLimitReached, MsgPlanLimitReached, err).
				WithDetails(map[string]any{detailKindKey: string(stored.Kind), "limit": limit})
		}
		return nil, translateRepoErr("duplicate", err)
	}
	s.publishSheetEvent(eventSheetCreated, cmd.OwnerID, copied.ID, copied.Version, map[string]any{detailTemplateIDKey: copied.TemplateID})
	return &copied, nil
}
