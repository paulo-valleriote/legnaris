package admin

import (
	"context"
	"errors"
	"strings"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/idgen"
)

var errRefundTaken = errors.New("refund already used")

var errCompStateChanged = errors.New("comp state changed")

type Service struct {
	deps Deps
	now  func() time.Time
}

func NewService(deps Deps) *Service {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &Service{deps: deps, now: deps.Now}
}

func (s *Service) User(ctx context.Context, id string) (View, error) {
	account, err := s.fetchByID(ctx, id)
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, account)
}

func (s *Service) UserByEmail(ctx context.Context, rawEmail string) (View, error) {
	account, err := s.fetchByEmail(ctx, rawEmail)
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, account)
}

func (s *Service) GrantComp(ctx context.Context, actorID, targetID string, cmd CompGrant) (billing.Subscription, error) {
	if _, err := s.fetchByID(ctx, targetID); err != nil {
		return billing.Subscription{}, err
	}
	reason, err := normalizeReason(cmd.Reason)
	if err != nil {
		return billing.Subscription{}, err
	}
	if !validCompPlan(cmd.PlanID) {
		return billing.Subscription{}, apperr.NewError(apperr.CodeInvalidRequest, MsgInvalidPlan, nil)
	}

	now := s.now().UTC()
	if cmd.ExpiresAt != nil && !cmd.ExpiresAt.After(now) {
		return billing.Subscription{}, apperr.NewError(apperr.CodeInvalidRequest, MsgExpiryNotFuture, nil)
	}
	if err := s.ensureNoLiveSubscription(ctx, targetID); err != nil {
		return billing.Subscription{}, err
	}

	id, err := idgen.New()
	if err != nil {
		return billing.Subscription{}, apperr.NewError(apperr.CodeInternal, MsgAdminUnavailable, err)
	}
	sub := billing.Subscription{
		ID:               id,
		UserID:           targetID,
		PlanID:           cmd.PlanID,
		Provider:         billing.ProviderComp,
		Status:           billing.StatusActive,
		CurrentPeriodEnd: cmd.ExpiresAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	payload := map[string]any{"plan_id": cmd.PlanID}
	if cmd.ExpiresAt != nil {
		payload["expires_at"] = *cmd.ExpiresAt
	}
	err = s.deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.deps.Subscriptions.InsertSubscription(txCtx, sub); err != nil {
			return err
		}
		if err := s.deps.Tx.Step(txCtx, db.StepAfterEntityWrite); err != nil {
			return err
		}
		return s.audit(txCtx, actorID, ActionCompGrant, targetID, reason, payload)
	})
	switch {
	case errors.Is(err, billing.ErrLiveExists):
		return billing.Subscription{}, apperr.NewError(apperr.CodeConflict, MsgLiveSubscriptionExist, err)
	case err != nil:
		return billing.Subscription{}, apperr.NewError(apperr.CodeInternal, MsgAdminUnavailable, err)
	}
	return sub, nil
}

func (s *Service) RevokeComp(ctx context.Context, actorID, targetID, rawReason string) error {
	if err := validateID(targetID); err != nil {
		return err
	}
	reason, err := normalizeReason(rawReason)
	if err != nil {
		return err
	}

	live, found, err := s.deps.Subscriptions.FindLive(ctx, targetID)
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgAdminUnavailable, err)
	}
	if !found {
		return apperr.NewError(apperr.CodeNotFound, MsgCompNotFound, nil)
	}
	if live.Provider != billing.ProviderComp {
		return apperr.NewError(apperr.CodeConflict, MsgNotComp, nil)
	}

	err = s.deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		canceled, err := s.deps.Subscriptions.CancelComp(txCtx, live.ID, s.now().UTC())
		if err != nil {
			return err
		}
		if !canceled {
			return errCompStateChanged
		}
		if err := s.deps.Tx.Step(txCtx, db.StepAfterEntityWrite); err != nil {
			return err
		}
		return s.audit(txCtx, actorID, ActionCompRevoke, targetID, reason, map[string]any{"subscription_id": live.ID})
	})
	switch {
	case errors.Is(err, errCompStateChanged):
		return apperr.NewError(apperr.CodeConflict, MsgCompStateChanged, err)
	case err != nil:
		return apperr.NewError(apperr.CodeInternal, MsgAdminUnavailable, err)
	}
	return nil
}

func (s *Service) ResetQuota(ctx context.Context, actorID, targetID, rawReason string) error {
	if _, err := s.fetchByID(ctx, targetID); err != nil {
		return err
	}
	reason, err := normalizeReason(rawReason)
	if err != nil {
		return err
	}

	err = s.deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.deps.Quotas.ResetDaily(txCtx, targetID); err != nil {
			return err
		}
		if err := s.deps.Tx.Step(txCtx, db.StepAfterEntityWrite); err != nil {
			return err
		}
		return s.audit(txCtx, actorID, ActionQuotaReset, targetID, reason, map[string]any{"scope": "daily"})
	})
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgAdminUnavailable, err)
	}
	return nil
}

func (s *Service) Refund(ctx context.Context, actorID, targetID string, override bool, rawReason string) error {
	if err := validateID(targetID); err != nil {
		return err
	}
	reason, err := normalizeReason(rawReason)
	if err != nil {
		return err
	}

	now := s.now().UTC()
	live, found, err := s.deps.Subscriptions.FindLive(ctx, targetID)
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgAdminUnavailable, err)
	}
	if !found {
		return refundDenied(billing.ReasonNotSubscribed)
	}
	if denial := billing.RefundDenial(live, now, override); denial != "" {
		return refundDenied(denial)
	}

	err = s.deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		requested, err := s.deps.Subscriptions.RequestRefund(txCtx, live.ID, now)
		if err != nil {
			return err
		}
		if !requested {
			return errRefundTaken
		}
		if err := s.deps.Tx.Step(txCtx, db.StepAfterEntityWrite); err != nil {
			return err
		}
		return s.audit(txCtx, actorID, ActionRefundRequest, targetID, reason, map[string]any{
			"override":        override,
			"subscription_id": live.ID,
		})
	})
	switch {
	case errors.Is(err, billing.ErrRefundAlreadyUsed), errors.Is(err, errRefundTaken):
		return refundDenied(billing.ReasonAlreadyUsed)
	case err != nil:
		return apperr.NewError(apperr.CodeInternal, MsgAdminUnavailable, err)
	}
	return nil
}

func (s *Service) view(ctx context.Context, account *user.User) (View, error) {
	effective, err := s.deps.Plans.PlanOf(ctx, account.ID)
	if err != nil {
		return View{}, apperr.NewError(apperr.CodeInternal, MsgAdminUnavailable, err)
	}
	daily, err := s.deps.Quotas.DailyStatus(ctx, account.ID)
	if err != nil {
		return View{}, err
	}
	trial, err := s.deps.Quotas.TrialStatus(ctx, account.ID)
	if err != nil {
		return View{}, err
	}
	live, found, err := s.deps.Subscriptions.FindLive(ctx, account.ID)
	if err != nil {
		return View{}, apperr.NewError(apperr.CodeInternal, MsgAdminUnavailable, err)
	}

	out := View{
		User: UserView{ID: account.ID, Email: account.Email},
		Plan: effective.ID,
		DailyQuota: DailyQuotaView{
			Limit:     daily.Limit,
			Used:      daily.Used,
			Remaining: daily.Remaining,
			ResetsAt:  daily.ResetsAt,
		},
		Trial: TrialView{
			Allowance: trial.Allowance,
			Used:      trial.Used,
			Remaining: trial.Remaining,
		},
	}
	if found {
		subscription := SubscriptionViewOf(live)
		out.Subscription = &subscription
	}
	return out, nil
}

func (s *Service) fetchByID(ctx context.Context, id string) (*user.User, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	account, err := s.deps.Users.FetchByID(ctx, id)
	if errors.Is(err, user.ErrUserNotFound) {
		return nil, apperr.NewError(apperr.CodeNotFound, MsgUserNotFound, err)
	}
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgAdminUnavailable, err)
	}
	return account, nil
}

func (s *Service) fetchByEmail(ctx context.Context, rawEmail string) (*user.User, error) {
	trimmed := strings.TrimSpace(rawEmail)
	if trimmed == "" {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, MsgEmailRequired, nil)
	}
	canonical, err := user.CanonicalEmail(trimmed)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, MsgInvalidEmail, err)
	}

	account, err := s.deps.Users.FetchByEmail(ctx, canonical)
	if err == nil {
		return account, nil
	}
	if !errors.Is(err, user.ErrUserNotFound) {
		return nil, apperr.NewError(apperr.CodeInternal, MsgAdminUnavailable, err)
	}
	if trimmed == canonical {
		return nil, apperr.NewError(apperr.CodeNotFound, MsgUserNotFound, err)
	}
	account, err = s.deps.Users.FetchByEmail(ctx, trimmed)
	if errors.Is(err, user.ErrUserNotFound) {
		return nil, apperr.NewError(apperr.CodeNotFound, MsgUserNotFound, err)
	}
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgAdminUnavailable, err)
	}
	return account, nil
}

func (s *Service) ensureNoLiveSubscription(ctx context.Context, userID string) error {
	live, found, err := s.deps.Subscriptions.FindLive(ctx, userID)
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgAdminUnavailable, err)
	}
	if found && live.Status == billing.StatusActive {
		return apperr.NewError(apperr.CodeConflict, MsgLiveSubscriptionExist, nil)
	}
	return nil
}

func (s *Service) audit(ctx context.Context, actorID, action, targetID, reason string, payload map[string]any) error {
	id, err := idgen.New()
	if err != nil {
		return err
	}
	return s.deps.Audits.Insert(ctx, Entry{
		ID:           id,
		ActorUserID:  actorID,
		Action:       action,
		TargetUserID: targetID,
		Payload:      payload,
		Reason:       reason,
		CreatedAt:    s.now().UTC(),
	})
}

func normalizeReason(raw string) (string, error) {
	reason := strings.TrimSpace(raw)
	if reason == "" {
		return "", apperr.NewError(apperr.CodeInvalidRequest, MsgReasonRequired, nil)
	}
	if len(reason) > MaxReasonLength {
		return "", apperr.NewError(apperr.CodeInvalidRequest, MsgReasonTooLong, nil)
	}
	return reason, nil
}

func validCompPlan(id string) bool {
	return id != plan.IDFree && plan.Lookup(id).ID == id
}

func validateID(id string) error {
	if err := idgen.Validate(id); err != nil {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgInvalidUser, err)
	}
	return nil
}

func refundDenied(reason string) error {
	return apperr.NewError(apperr.CodeRefundNotEligible, billing.MsgRefundNotEligible, nil).
		WithDetails(map[string]any{"reason": reason})
}
