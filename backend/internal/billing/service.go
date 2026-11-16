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

const (
	MsgInvalidUser         = "invalid user"
	MsgAlreadySubscribed   = "an active subscription already exists"
	MsgBillingUnavailable  = "billing is temporarily unavailable"
	MsgCheckoutUnavailable = "checkout is unavailable"
	MsgCheckoutBusy        = "checkout is busy, retry"
)

type Config struct {
	PriceID    string
	SuccessURL string
	CancelURL  string
	Now        func() time.Time
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

	sub := live
	switch {
	case found && live.Status == StatusActive:
		return Checkout{}, false, apperr.NewError(apperr.CodeAlreadySubscribed, MsgAlreadySubscribed, nil)
	case found && live.CheckoutURL != "" && live.CheckoutExpiresAt.After(now):
		return Checkout{URL: live.CheckoutURL, ExpiresAt: live.CheckoutExpiresAt}, true, nil
	case found && live.CheckoutURL != "":
		if err := s.store.Cancel(ctx, live.ID); err != nil {
			return Checkout{}, false, apperr.NewError(apperr.CodeInternal, MsgCheckoutUnavailable, err)
		}
		return Checkout{}, false, nil
	case !found:
		fresh, err := newPending(userID, now)
		if err != nil {
			return Checkout{}, false, apperr.NewError(apperr.CodeInternal, MsgCheckoutUnavailable, err)
		}
		if err := s.store.InsertPending(ctx, fresh); err != nil {
			if errors.Is(err, ErrLiveExists) {
				return Checkout{}, false, nil
			}
			return Checkout{}, false, apperr.NewError(apperr.CodeInternal, MsgCheckoutUnavailable, err)
		}
		sub = fresh
	}

	return s.startSession(ctx, sub)
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
