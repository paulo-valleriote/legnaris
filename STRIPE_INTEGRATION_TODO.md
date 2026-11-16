# Stripe integration: remaining setup

The Checkout Session is created in one place:
[backend/internal/billing/stripe/provider.go](backend/internal/billing/stripe/provider.go), `Provider.CreateCheckoutSession`.
It is reached through `POST /billing/checkout` (LGN-210). The rest of billing is described in ADR 0009 and ADR 0013.

## Values to replace

No placeholders in code. The values the session needs come from the environment, and they must be set before go-live.

**Files that read these values:**
- [backend/shared/env/env.go](backend/shared/env/env.go)
- [deploy/docker-compose.yml](deploy/docker-compose.yml) (passes them to `app` and `relay`)

| Field | Source | What to set |
|-------|--------|-------------|
| mode | code: `subscription` | Already correct. The paid Plan is recurring. |
| success_url | `BILLING_SUCCESS_URL` | The web page the user returns to after paying, e.g. `https://<app-domain>/plans?checkout=success`. |
| cancel_url | `BILLING_CANCEL_URL` | The web page the user returns to after leaving Checkout, e.g. `https://<app-domain>/plans`. |
| line_items[].price | `STRIPE_PRICE_ID_USD` | The id of the recurring USD 6.99/month Price (`price_...`) from https://dashboard.stripe.com/prices. |
| API key | `STRIPE_SECRET_KEY` | `sk_test_...` in test mode. In production, use a restricted key kept in the secret manager (sealed secrets, ADR 0004). |
| Webhook secret | `STRIPE_WEBHOOK_SECRET` | The signing secret of the endpoint that points to `POST /billing/webhook`. |

If any of these is missing, billing answers `503 BILLING_UNAVAILABLE`.

## Configured parameters

These were chosen in Checkout Studio and are already set in code.

**Files containing these parameters:**
- [backend/internal/billing/stripe/provider.go](backend/internal/billing/stripe/provider.go)
- [backend/internal/billing/stripe/provider_test.go](backend/internal/billing/stripe/provider_test.go) (asserts every value)

| Parameter | Value |
|-----------|-------|
| ui_mode | hosted_page (stripe-go v87) |
| billing_address_collection | auto |
| phone_number_collection.enabled | false |
| automatic_tax.enabled | false |
| allow_promotion_codes | false |
| payment_method_collection | always |
| submit_type | auto |
| saved_payment_method_options.payment_method_save | enabled |
| integration_identifier | hosted_web_0001 |
| origin_context | web |

The following parameters are not part of Checkout Studio, but they are kept on purpose:

- `customer` and `client_reference_id` link the session to the user and to the `pending` subscription.
- `allowed_payment_method_types=[card]` enforces the card-only rule from ADR 0009.
- `metadata.user_id` and `metadata.subscription_id` let the webhook find the user.
- The `checkout:<subscription_id>` idempotency key keeps checkout idempotent.

Removing any of these would break checkout idempotency and webhook processing.

## Setup

1. In the Dashboard, create the Product and its recurring monthly Price of USD 6.99. Put the Price id in `STRIPE_PRICE_ID_USD`. The Product name and description are what the hosted page shows.
2. Create a webhook endpoint for `https://<api-domain>/billing/webhook`. Subscribe it to `checkout.session.completed` and `customer.subscription.*`, then copy its signing secret.
3. In the Customer portal settings, enable cancellation. `POST /billing/portal` relies on it.
4. Run locally with `make dev`, export the variables above, and forward webhooks with `stripe listen --forward-to localhost:<PORT>/billing/webhook`.
5. Go-live is blocked until LGN-192 (business entity and taxes) is resolved. `automatic_tax` stays disabled until then.

## How it works

1. The web calls `POST /billing/checkout`. The API creates a `pending` subscription, the Stripe customer and the Checkout Session, and returns the session URL.
2. The web redirects the browser to the Stripe-hosted page. No Stripe.js is loaded.
3. Stripe sends webhooks to `/billing/webhook`. They are stored in the `billing_events` inbox and processed per user by the sweeper.
4. The consumer re-reads the subscription from Stripe, updates `subscriptions` and `users.plan`, and emits `plan.changed`, all in one transaction.
5. A daily reconciliation pass corrects any drift.

## Testing

- `4242 4242 4242 4242`: payment succeeds.
- `4000 0000 0000 9995`: card declined (insufficient funds).
- `4000 0025 0000 3155`: requires 3D Secure authentication.

Use any future expiry date, any CVC and any postal code. Use test-mode keys only.

## Next steps

- Run the Mongo integration tests against a replica set (`make dev-db`). They self-skip without `DATABASE_URI`.
- Add Stripe to the privacy policy (ADR 0009).
- BRL pricing is post-MVP (LGN-193).

## Resources

- https://support.stripe.com
- https://docs.stripe.com/mcp
