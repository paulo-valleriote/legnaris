package billing

import (
	"context"
	"errors"
	"time"

	"legnaris/backend/internal/plan"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
)

const maxCheckoutAttempts = 3

var ErrPlanSyncNotConfigured = errors.New("billing checkout plan sync is not configured")

const (
	MsgInvalidUser         = "invalid user"
	MsgAlreadySubscribed   = "an active subscription already exists"
	MsgBillingUnavailable  = "billing is temporarily unavailable"
	MsgCheckoutUnavailable = "checkout is unavailable"
	MsgCheckoutBusy        = "checkout is busy, retry"
)

type PlanSyncer interface {
	SyncPlan(ctx context.Context, userID string, now time.Time) error
}

type Config struct {
	PriceID    string
	SuccessURL string
	CancelURL  string
	Now        func() time.Time
	Tx         Transactor
	PlanSync   PlanSyncer
}

type Checkout struct {
	URL       string
	ExpiresAt time.Time
}

type Service struct {
	store    Store
	provider Provider
	cfg      Config
}

func NewService(store Store, provider Provider, cfg Config) *Service {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Service{store: store, provider: provider, cfg: cfg}
}

func (s *Service) Checkout(ctx context.Context, userID string) (Checkout, error) {
	if err := idgen.Validate(userID); err != nil {
		return Checkout{}, apperr.NewError(apperr.CodeInvalidRequest, MsgInvalidUser, err)
	}
	for attempt := 0; attempt < maxCheckoutAttempts; attempt++ {
		out, done, err := s.checkoutOnce(ctx, userID)
		if err != nil {
			return Checkout{}, err
		}
		if done {
			return out, nil
		}
	}
	return Checkout{}, apperr.NewError(apperr.CodeConflict, MsgCheckoutBusy, nil)
}

func (s *Service) checkoutOnce(ctx context.Context, userID string) (Checkout, bool, error) {
	now := s.cfg.Now()
	live, found, err := s.store.FindLive(ctx, userID)
	if err != nil {
		return Checkout{}, false, apperr.NewError(apperr.CodeInternal, MsgCheckoutUnavailable, err)
	}
	if found && IsExpiredComp(live, now) {
		canceled, err := s.cancelExpiredComp(ctx, live.ID, userID, now)
		if err != nil {
			return Checkout{}, false, apperr.NewError(apperr.CodeInternal, MsgCheckoutUnavailable, err)
		}
		if !canceled {
			return Checkout{}, false, nil
		}
		found = false
	}

	sub, out, done, retry, err := s.nextSubscription(ctx, userID, live, found, now)
	if err != nil {
		return Checkout{}, false, err
	}
	if retry {
		return Checkout{}, false, nil
	}
	if done {
		return out, true, nil
	}
	return s.startSession(ctx, sub)
}

func (s *Service) nextSubscription(ctx context.Context, userID string, live Subscription, found bool, now time.Time) (Subscription, Checkout, bool, bool, error) {
	sub := live
	switch {
	case found && (live.Status == StatusActive || live.Status == StatusPastDue):
		return Subscription{}, Checkout{}, false, false, apperr.NewError(apperr.CodeAlreadySubscribed, MsgAlreadySubscribed, nil)
	case found && live.CheckoutURL != "" && live.CheckoutExpiresAt.After(now):
		return Subscription{}, Checkout{URL: live.CheckoutURL, ExpiresAt: live.CheckoutExpiresAt}, true, false, nil
	case found && live.CheckoutURL != "":
		if err := s.store.Cancel(ctx, live.ID); err != nil {
			return Subscription{}, Checkout{}, false, false, apperr.NewError(apperr.CodeInternal, MsgCheckoutUnavailable, err)
		}
		return Subscription{}, Checkout{}, false, true, nil
	case !found:
		fresh, err := newPending(userID, now)
		if err != nil {
			return Subscription{}, Checkout{}, false, false, apperr.NewError(apperr.CodeInternal, MsgCheckoutUnavailable, err)
		}
		if err := s.store.InsertPending(ctx, fresh); err != nil {
			if errors.Is(err, ErrLiveExists) {
				return Subscription{}, Checkout{}, false, true, nil
			}
			return Subscription{}, Checkout{}, false, false, apperr.NewError(apperr.CodeInternal, MsgCheckoutUnavailable, err)
		}
		sub = fresh
	}

	return sub, Checkout{}, false, false, nil
}

func (s *Service) cancelExpiredComp(ctx context.Context, subID, userID string, now time.Time) (bool, error) {
	if s.cfg.Tx == nil || s.cfg.PlanSync == nil {
		return false, ErrPlanSyncNotConfigured
	}
	var canceled bool
	err := s.cfg.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		applied, err := s.store.CancelComp(txCtx, subID, now)
		if err != nil {
			return err
		}
		if !applied {
			return nil
		}
		canceled = true
		return s.cfg.PlanSync.SyncPlan(txCtx, userID, now)
	})
	if err != nil {
		return false, err
	}
	return canceled, nil
}

func newPending(userID string, now time.Time) (Subscription, error) {
	id, err := idgen.New()
	if err != nil {
		return Subscription{}, err
	}
	return Subscription{
		ID:        id,
		UserID:    userID,
		PlanID:    plan.IDPaid,
		Provider:  ProviderStripe,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *Service) startSession(ctx context.Context, sub Subscription) (Checkout, bool, error) {
	customer, err := s.provider.CreateCustomer(ctx, CustomerRequest{
		IdempotencyKey: "customer:" + sub.UserID,
		UserID:         sub.UserID,
	})
	if err != nil {
		return Checkout{}, false, mapProviderError(err)
	}

	session, err := s.provider.CreateCheckoutSession(ctx, CheckoutRequest{
		IdempotencyKey: "checkout:" + sub.ID,
		UserID:         sub.UserID,
		SubscriptionID: sub.ID,
		CustomerID:     customer.ID,
		PriceID:        s.cfg.PriceID,
		SuccessURL:     s.cfg.SuccessURL,
		CancelURL:      s.cfg.CancelURL,
	})
	if err != nil {
		return Checkout{}, false, mapProviderError(err)
	}

	attached, err := s.store.AttachCheckout(ctx, sub.ID, customer.ID, session.URL, session.ExpiresAt)
	if err != nil {
		return Checkout{}, false, apperr.NewError(apperr.CodeInternal, MsgCheckoutUnavailable, err)
	}
	return Checkout{URL: session.URL, ExpiresAt: session.ExpiresAt}, attached, nil
}

func mapProviderError(err error) error {
	if errors.Is(err, ErrProviderUnavailable) {
		return apperr.NewError(apperr.CodeBillingUnavailable, MsgBillingUnavailable, err)
	}
	return apperr.NewError(apperr.CodeInternal, MsgCheckoutUnavailable, err)
}
