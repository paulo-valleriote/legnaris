package stripe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"legnaris/backend/internal/billing"

	stripesdk "github.com/stripe/stripe-go/v87"
)

const (
	defaultTimeout = 15 * time.Second
	metadataUserID = "user_id"
	cardPayment    = "card"

	checkoutIntegrationIdentifier = "hosted_web_0001"
)

var ErrMissingConfiguration = errors.New("stripe secret key is not configured")

type Config struct {
	SecretKey     string
	WebhookSecret string
	BaseURL       string
	Timeout       time.Duration
}

type Provider struct {
	client        *stripesdk.Client
	webhookSecret string
}

func New(cfg Config) (*Provider, error) {
	if cfg.SecretKey == "" {
		return nil, ErrMissingConfiguration
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	backendCfg := &stripesdk.BackendConfig{HTTPClient: &http.Client{Timeout: timeout}}
	if cfg.BaseURL != "" {
		backendCfg.URL = stripesdk.String(cfg.BaseURL)
	}
	client := stripesdk.NewClient(cfg.SecretKey, stripesdk.WithBackends(stripesdk.NewBackendsWithConfig(backendCfg)))
	return &Provider{client: client, webhookSecret: cfg.WebhookSecret}, nil
}

func (p *Provider) CreateCustomer(ctx context.Context, req billing.CustomerRequest) (billing.Customer, error) {
	params := &stripesdk.CustomerCreateParams{
		Metadata: map[string]string{metadataUserID: req.UserID},
	}
	params.SetIdempotencyKey(req.IdempotencyKey)

	customer, err := p.client.V1Customers.Create(ctx, params)
	if err != nil {
		return billing.Customer{}, mapError(err)
	}
	return billing.Customer{ID: customer.ID}, nil
}

func (p *Provider) CreateCheckoutSession(ctx context.Context, req billing.CheckoutRequest) (billing.CheckoutSession, error) {
	params := &stripesdk.CheckoutSessionCreateParams{
		Mode:                      stripesdk.String(string(stripesdk.CheckoutSessionModeSubscription)),
		Customer:                  stripesdk.String(req.CustomerID),
		ClientReferenceID:         stripesdk.String(req.SubscriptionID),
		AllowedPaymentMethodTypes: []*string{stripesdk.String(cardPayment)},
		LineItems: []*stripesdk.CheckoutSessionCreateLineItemParams{{
			Price:    stripesdk.String(req.PriceID),
			Quantity: stripesdk.Int64(1),
		}},
		SuccessURL: stripesdk.String(req.SuccessURL),
		CancelURL:  stripesdk.String(req.CancelURL),
		Metadata: map[string]string{
			metadataUserID:    req.UserID,
			"subscription_id": req.SubscriptionID,
		},
		UIMode:                   stripesdk.String(string(stripesdk.CheckoutSessionUIModeHostedPage)),
		BillingAddressCollection: stripesdk.String(string(stripesdk.CheckoutSessionBillingAddressCollectionAuto)),
		PhoneNumberCollection: &stripesdk.CheckoutSessionCreatePhoneNumberCollectionParams{
			Enabled: stripesdk.Bool(false),
		},
		AutomaticTax: &stripesdk.CheckoutSessionCreateAutomaticTaxParams{
			Enabled: stripesdk.Bool(false),
		},
		AllowPromotionCodes:     stripesdk.Bool(false),
		PaymentMethodCollection: stripesdk.String(string(stripesdk.CheckoutSessionPaymentMethodCollectionAlways)),
		SubmitType:              stripesdk.String(string(stripesdk.CheckoutSessionSubmitTypeAuto)),
		SavedPaymentMethodOptions: &stripesdk.CheckoutSessionCreateSavedPaymentMethodOptionsParams{
			PaymentMethodSave: stripesdk.String(string(stripesdk.CheckoutSessionSavedPaymentMethodOptionsPaymentMethodSaveEnabled)),
		},
		IntegrationIdentifier: stripesdk.String(checkoutIntegrationIdentifier),
		OriginContext:         stripesdk.String(string(stripesdk.CheckoutSessionOriginContextWeb)),
	}
	params.SetIdempotencyKey(req.IdempotencyKey)

	session, err := p.client.V1CheckoutSessions.Create(ctx, params)
	if err != nil {
		return billing.CheckoutSession{}, mapError(err)
	}
	return billing.CheckoutSession{
		ID:        session.ID,
		URL:       session.URL,
		ExpiresAt: time.Unix(session.ExpiresAt, 0).UTC(),
	}, nil
}

func mapError(err error) error {
	var stripeErr *stripesdk.Error
	if errors.As(err, &stripeErr) && isPermanentStatus(stripeErr.HTTPStatusCode) {
		return fmt.Errorf("%w: %w", billing.ErrProviderRejected, err)
	}
	return fmt.Errorf("%w: %w", billing.ErrProviderUnavailable, err)
}

func isPermanentStatus(status int) bool {
	return status >= 400 && status < 500 && status != http.StatusConflict && status != http.StatusTooManyRequests
}
