package billing

import (
	"context"

	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
)

const MsgNoSubscription = "no subscription to manage"

func (s *Service) Portal(ctx context.Context, userID string) (string, error) {
	live, err := s.managedSubscription(ctx, userID, false)
	if err != nil {
		return "", err
	}
	session, err := s.provider.CreatePortalSession(ctx, PortalRequest{CustomerID: live.ProviderCustomerID})
	if err != nil {
		return "", mapProviderError(err)
	}
	return session.URL, nil
}

func (s *Service) CancelAtPeriodEnd(ctx context.Context, userID string) error {
	live, err := s.managedSubscription(ctx, userID, true)
	if err != nil {
		return err
	}
	if err := s.provider.CancelAtPeriodEnd(ctx, CancelRequest{ProviderSubscriptionID: live.ProviderSubID}); err != nil {
		return mapProviderError(err)
	}
	return nil
}

func (s *Service) managedSubscription(ctx context.Context, userID string, needsProviderSubscription bool) (Subscription, error) {
	if err := idgen.Validate(userID); err != nil {
		return Subscription{}, apperr.NewError(apperr.CodeInvalidRequest, MsgInvalidUser, err)
	}
	live, found, err := s.store.FindLive(ctx, userID)
	if err != nil {
		return Subscription{}, apperr.NewError(apperr.CodeInternal, MsgBillingUnavailable, err)
	}
	if !found || live.ProviderCustomerID == "" {
		return Subscription{}, apperr.NewError(apperr.CodeNoSubscription, MsgNoSubscription, nil)
	}
	if needsProviderSubscription && (live.Status != StatusActive || live.ProviderSubID == "") {
		return Subscription{}, apperr.NewError(apperr.CodeNoSubscription, MsgNoSubscription, nil)
	}
	return live, nil
}
