package stripe

import (
	"context"
	"fmt"

	"legnaris/backend/internal/billing"

	stripesdk "github.com/stripe/stripe-go/v87"
)

const (
	invoicePaid = "paid"
	paymentPaid = "paid"
	latestOnly  = 1
)

func (p *Provider) RefundLatest(ctx context.Context, req billing.RefundRequest) error {
	paymentIntent, err := p.latestPaymentIntent(ctx, req.ProviderSubscriptionID)
	if err != nil {
		return err
	}
	params := &stripesdk.RefundCreateParams{PaymentIntent: stripesdk.String(paymentIntent)}
	if req.IdempotencyKey != "" {
		params.SetIdempotencyKey(req.IdempotencyKey)
	}
	if _, err := p.client.V1Refunds.Create(ctx, params); err != nil {
		return mapError(err)
	}
	return nil
}

func (p *Provider) latestPaymentIntent(ctx context.Context, providerSubscriptionID string) (string, error) {
	invoices := p.client.V1Invoices.List(ctx, &stripesdk.InvoiceListParams{
		Subscription: stripesdk.String(providerSubscriptionID),
		Status:       stripesdk.String(invoicePaid),
		ListParams:   stripesdk.ListParams{Limit: stripesdk.Int64(latestOnly)},
	})
	invoiceList := invoices.Data()
	if err := invoices.Err(); err != nil {
		return "", mapError(err)
	}
	if len(invoiceList) == 0 {
		return "", fmt.Errorf("subscription %s has no paid invoice: %w", providerSubscriptionID, billing.ErrProviderRejected)
	}

	payments := p.client.V1InvoicePayments.List(ctx, &stripesdk.InvoicePaymentListParams{
		Invoice:    stripesdk.String(invoiceList[0].ID),
		Status:     stripesdk.String(paymentPaid),
		ListParams: stripesdk.ListParams{Limit: stripesdk.Int64(latestOnly)},
	})
	paymentList := payments.Data()
	if err := payments.Err(); err != nil {
		return "", mapError(err)
	}
	if len(paymentList) == 0 || paymentList[0].Payment == nil || paymentList[0].Payment.PaymentIntent == nil {
		return "", fmt.Errorf("invoice %s has no paid payment intent: %w", invoiceList[0].ID, billing.ErrProviderRejected)
	}
	return paymentList[0].Payment.PaymentIntent.ID, nil
}
