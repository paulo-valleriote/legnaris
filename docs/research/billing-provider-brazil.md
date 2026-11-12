# Billing provider for Legnaris subscription Plans (Brazil-first)

Ticket: LGN-171 (parent LGN-169)
Research date: 2026-11-12. Prices and product facts are as seen on that date. Anything marked **unverified** could not be confirmed from a primary source during this research and must be checked before a decision.

## 1. Question

Which payment provider fits Legnaris subscription Plans (per-user monthly plans with usage limits, mostly Brazilian users, small solo-dev team)?

## 2. Method and limits

- Primary sources only: provider pricing/docs pages, plus regulator material (Banco Central) where cited. Third-party blogs and calculators were used only to locate primary pages or to flag conflicts, and are labeled as such.
- Several official pages could not be fetched during this research: Asaas `/precos` (redirects to a login page), Efí (`efi.com.br` did not resolve), Stripe `docs.stripe.com/billing/subscriptions/pix` variants (404 on one URL), and Mercado Pago PIX Automático docs (404). Those gaps are flagged below.
- No account was opened and no sandbox was exercised. Nothing here was tested against the Legnaris codebase.

## 3. Providers compared

### 3.1 Stripe (Brazil)

| Topic | Finding | Source |
|---|---|---|
| Card fees | Domestic cards 3.99% + R$ 0.39 per transaction; international cards +2% | https://stripe.com/br/pricing (fetched 2026-11-12) |
| PIX | 1.19% per PIX paid, labeled "somente por convite" (invitation only) | https://stripe.com/br/pricing |
| Boleto | R$ 3.45 per boleto paid | https://stripe.com/br/pricing |
| Billing (recurring) fee | 0.7% of Billing volume (applies to Billing volume processed inside and outside Stripe; excludes one-time invoices) | https://stripe.com/br/pricing |
| Payouts | Flexible schedule: continuous, weekly, or monthly; Dashboard shows expected deposit dates. No Brazil-specific D+N detail on the page | https://stripe.com/br/pricing |
| PIX Automático | Supported for recurring payments via mandates: customer authorizes charges for subscriptions; subscription setup via Checkout or Setup Intents/Subscriptions API. Mandate options include `amount` (max per cycle, default BRL 400), `amount_type` (`maximum` or `fixed`), `payment_schedule` (monthly default; daily is not allowed), `start_date` (at least 3 days ahead), `end_date` | https://docs.stripe.com/payments/pix/pix-automatico (fetched) |
| PIX Automático timing | Pre-debit notification to the customer 3 days before the charge; charge at billing cycle + 3 days; PaymentIntent stays `processing` until terminal state | same |
| PIX Automático retries | Payment retried once a day for 3 days after a retryable failure (enabled by default). Stripe Billing can retry once after 1 day for unscheduled payments, only after contacting support | same |
| IOF | Applies to each recurring transaction; can be absorbed via `amount_includes_iof` | same |
| Availability caveat | A search snippet of Stripe's general PIX page reportedly says PIX Automático is not available in Brazil, but the same page reportedly says Checkout subscriptions use it. **Unverified and contradictory**; confirm in the Dashboard or with Stripe support. Source of the snippet: https://docs.stripe.com/payments/pix.md (not fetched directly) | WebSearch result |
| Boleto recurring | Not verified in this research | - |
| NFS-e | Not covered by Stripe; Brazilian invoicing needs a separate integration | - |
| Go SDK | Stripe publishes an official Go SDK (`stripe-go`) and the API is well documented. Not re-verified on a page in this research | general knowledge, **unverified here** |
| Sandbox | Test mode is standard | general knowledge, **unverified here** |
| Merchant of record | No; Stripe is a payment processor, so Legnaris is responsible for tax and invoicing | general knowledge |

### 3.2 Asaas

| Topic | Finding | Source |
|---|---|---|
| Monthly fee | None (no mensalidade, no adesão) | https://www.asaas.com/precos-e-taxas (page partially readable), WebSearch summary |
| PIX | Price is R$ 0.99 per received PIX on the fetched page, with R$ 1.99 shown next to it with no explanation. A search summary says R$ 0.99 for the first 3 months then R$ 1.99; **conflicting, treat as unverified** | https://www.asaas.com/precos-e-taxas; https://blog.asaas.com/taxas-asaas/ |
| Boleto | Fetched page shows R$ 0.99 per paid boleto alongside R$ 1.99; a partner page shows R$ 1.79 (promotional). **Conflicting** | same |
| Card debit | R$ 0.35 + 1.89% | https://www.asaas.com/precos-e-taxas |
| Card credit | Search summary: R$ 0.49 + 2.99% à vista; R$ 0.49 + 3.49% for 7-12 installments; a temporary promotion lists R$ 0.49 + 1.99% for 2-6 installments. **Not confirmed on the primary page** | https://www.asaas.com/precos-e-taxas (credit rows were truncated) |
| Receivables | Card receivables without advance: 32 days per search summary; advance costs 1.25% per month. **Unverified** | WebSearch summary |
| NFS-e | Automatic NFS-e issuance per subscription supported; search summary cites R$ 0.49 per note issued, no monthly fee. Requires company registration (pessoa jurídica), municipal registration and municipal webservice homologation (MEI exempt from homologation). Cancellation via `POST /v3/invoices/{id}/cancel`; not every municipality permits automatic cancellation | https://docs.asaas.com/reference/atualizar-configuracao-para-emissao-de-notas-fiscais; https://asaas.com/nota-fiscal (via WebSearch) |
| Subscriptions | Assinaturas (recurring billing) exist; subscription charges use a stored payment method (card) | https://docs.asaas.com/docs/assinaturas-recorrência (via WebSearch) |
| PIX Automático | Supported. Authorization is created with `paymentCreationMode`: `MANUAL` (default; the app creates each charge through the API) or `SUBSCRIPTION` (a subscription generates charges, needs `value`). Authorization is only active after the payer's bank confirms it to Asaas; if no confirmation, the authorization is refused even if the first payment was received. Flow "Jornada 3": the first PIX QR Code carries both the initial charge and the authorization | https://docs.asaas.com/docs/pix-automatico (fetched) |
| PIX Automático limits and cancellation | Page does not state limits; cancellation and refusal reasons are on separate pages not fetched | same |
| Webhooks | Payment events include PAYMENT_CREATED, PAYMENT_OVERDUE, PAYMENT_CONFIRMED, PAYMENT_RECEIVED. Delivery is at-least-once, so handlers must be idempotent; payloads carry an event id (per changelog). Third-party guide says the `asaas-access-token` header carries a token you configure and that webhooks are not HMAC-signed; **unverified against Asaas's own page** | https://docs.asaas.com/docs/webhook-para-cobrancas; https://docs.asaas.com/docs/how-to-implement-idempotence-in-webhooks; https://hookdeck.com/webhooks/platforms/guide-to-asaas-webhooks-features-and-best-practices |
| Dunning and trials | Not verified on a primary page in this research | - |
| Plan changes (upgrade/downgrade, proration) | Not verified in this research | - |
| Go SDK | No official Go SDK found. An unofficial package (`github.com/mobilemindtec/go-payments/asaas`) exists and does not clearly cover webhooks. Plan on REST with a thin client | https://pkg.go.dev/github.com/mobilemindtec/go-payments/asaas (via WebSearch) |
| Sandbox | Not verified in this research | - |
| International cards | Not applicable; domestic-focused | - |
| Merchant of record | No | general |

### 3.3 Mercado Pago

| Topic | Finding | Source |
|---|---|---|
| Payment methods for subscriptions | Pix, card (credit and debit), boleto, saved methods, line of credit | https://www.mercadopago.com.br/developers/pt/docs/subscriptions/landing (fetched) |
| Subscriptions | Created via Subscriptions API; frequency weekly, monthly, or yearly; first payment through Mercado Pago, later charges automatic | same |
| Trials | Trial period supported (no config detail on the fetched page) | same |
| Retries | Automatic retries on declined charges; no schedule or count published on the fetched page | same |
| Plan changes / proration | Proration mentioned ("Valor proporcional (pro rata)"); mechanics not on fetched page | same |
| Webhooks | Topics not listed on the fetched page; docs link to notifications/IPN pages. Names `subscription_preapproval` and `subscription_authorized_payment` were not seen on the fetched page, so they are **unverified** | same |
| PIX fees | Payment link page: PIX 0.99%. Point Mini (Brazilian point-of-sale terminal, a different product) cites about 0.49% PIX and 4.98% credit à vista for small volumes. Third-party calculators conflict on whether PIX is free. **No single authoritative 2026 table found** | https://www.mercadopago.com.br/ferramentas-para-vender/link-de-pagamento (search snippet); https://www.mercadopago.com.br/blog/taxas-point-mini-upgrade-maquininha (via WebSearch) |
| Card fees | Payment link page: credit 4.98% at once, 4.48% at 14 days, 3.99% at 30 days; installments 2x 2.53% up to 11x 16.12% | same |
| PIX Automático | **Unverified.** Primary PIX Automático page returned 404 during this research; PIX refund page does not mention it | https://www.mercadopago.com.br/developers/pt/docs/checkout-api-payments/payment-methods/pix (fetched) |
| Go SDK | Not verified in this research | - |
| Merchant of record | No | general |

### 3.4 Pagar.me (Stone)

| Topic | Finding | Source |
|---|---|---|
| Fees | Fetched pricing page: PIX 0.99%; credit à vista 4.19%; receivables next day. Boleto, installments, monthly fees and subscription fees are not on the fetched page | https://www.pagar.me/precos (fetched) |
| Subscriptions | Recorrência exists: Plans (reusable templates), Subscriptions (from a plan or standalone), cycle renewal | https://docs.pagar.me/ (fetched) |
| PIX Automático | **Not found.** Search returned no Pagar.me page; cannot confirm support | WebSearch (no Pagar.me result) |
| Webhooks | Documented; multiple endpoints; event filtering | https://docs.pagar.me/ (fetched) |
| Sandbox | Summary did not confirm a general sandbox; only a PIX simulator and a KYC test environment were mentioned | https://docs.pagar.me/ (fetched) |
| Go SDK | No official SDK listed in the fetched summary | https://docs.pagar.me/ (fetched) |
| Merchant of record | No | general |

### 3.5 Efí (formerly Gerencianet)

- Could not fetch `efi.com.br` (DNS failure). Search results show Efí has recurring PIX and a PIX Automático product ("Bolix Automático" launch reported by finsidersbrasil). Fee schedule **not verified**. Source for product existence: https://finsidersbrasil.com.br/giro-noticias/apos-sucesso-do-pix-automatico-efi-bank-lanca-bolix-automatico/ (WebSearch snippet, not primary).
- Action: obtain fees and API docs from Efí directly before deciding.

### 3.6 Lemon Squeezy (merchant of record)

| Topic | Finding | Source |
|---|---|---|
| Fee | 5% + US$ 0.50 per transaction; volume pricing via sales; additional fees may apply | https://www.lemonsqueezy.com/pricing (fetched) |
| Merchant of record | Yes; handles sales tax and VAT calculation and remittance | same |
| Brazil | Brazil is **not mentioned** as a supported seller/payout country; payouts go by bank wire or PayPal, twice a month, to "more than 200 countries" (list not on page). Brazilian PIX and boleto are not offered as checkout methods on the fetched page | same |
| Verdict | Not a fit for BRL PIX/boleto checkout. Would only fit an international, card-only product | derived |

### 3.7 Other options noted, not fully evaluated

- **Iugu** (Brazilian billing platform): search surfaced Iugu PIX Automático guide, not fetched. **Unverified.**
- **Paddle**: changelog entry "pix automatico" (2026), suggesting PIX Automático support for subscriptions in Paddle's merchant-of-record product, with constraints (no daily interval). Paddle's Brazil seller availability, fees and tax handling were **not verified**. Source: https://developer.paddle.com/changelog/2026/pix-automatico (WebSearch snippet).
- **dLocal, y.uno, FastSpring, Adyen, Solvimon, EBANX, PagBrasil**: appear in search as PIX Automático providers; not evaluated.

## 4. Comparison summary

| Criterion | Stripe | Asaas | Mercado Pago | Pagar.me | Efí | Lemon Squeezy |
|---|---|---|---|---|---|---|
| Recurring subscriptions | Yes (Billing) | Yes (assinaturas) | Yes | Yes (Recorrência) | Yes (recurring PIX) | Yes (MoR) |
| PIX one-off | Yes (1.19%, invite) | Yes (R$ 0.99 or 1.99, conflicting) | Yes (0.99% link page) | Yes (0.99%) | Yes (fee unverified) | No |
| PIX Automático | Yes (mandates, 3-day pre-debit) | Yes (MANUAL or SUBSCRIPTION) | Unverified | Not found | Product exists, unverified | Via Paddle-like only, unverified |
| Boleto | Yes (R$ 3.45) | Yes (R$ 0.99-1.99) | Yes | Yes (fee not fetched) | Yes | No |
| Card (domestic) | 3.99% + R$ 0.39 | Debit 1.89% + R$ 0.35; credit ~2.99%+R$0.49 (partly unverified) | 4.98% at once (link page) | 4.19% credit à vista | Unverified | 5% + US$ 0.50 |
| Monthly fee | None listed; Billing 0.7% of volume | None | None on pages checked | None on pages checked | Unverified | None |
| NFS-e | Not offered | Yes (automatic per subscription, R$ 0.49/note reported) | Not checked | Not checked | Not checked | No (MoR) |
| Go SDK | Official (stripe-go) | None official | Not checked | None found | Not checked | No |
| Hosted checkout | Yes | Yes | Yes | Yes | Yes | Yes |
| Merchant of record | No | No | No | No | No | Yes |

Fee values are as of 2026-11-12 from the cited pages and are subject to change.

## 5. Recommendation

**Primary: Asaas**, subject to verification. Reasons:

1. Brazilian-first fit: PIX, boleto, card, and recurring PIX (PIX Automático) in one account, no monthly fee, and a published REST API.
2. Built-in NFS-e per subscription, which removes a separate Brazilian invoicing integration. This matters for a solo developer.
3. PIX Automático "SUBSCRIPTION" mode lets Asaas drive the recurring cycle, reducing custom dunning logic. "MANUAL" mode keeps cycle control in Legnaris, which suits per-user limits and plan changes.
4. Low per-transaction fees for PIX and boleto.

**Runner-up: Stripe Brazil** if the team wants the most mature Go SDK, a documented sandbox, and first-class webhooks and Billing, and accepts 0.7% Billing fee plus higher PIX/boleto fees and a separate NFS-e integration. Confirm PIX Automático availability for the Brazilian account first, since the docs are contradictory in the search snippet.

**Not recommended:** Lemon Squeezy (no BRL PIX/boleto, 5% + US$ 0.50). Paddle is worth a follow-up only if the product later sells internationally and wants MoR tax handling.

Before committing to Asaas, verify:

- Current PIX and card rates on `asaas.com/precos-e-taxas` (the fetched page was truncated and the PIX rate was ambiguous).
- The full webhook event list and signature/auth scheme against Asaas's own docs (the auth detail came from a third-party guide).
- Dunning, retry schedule, trial support, and plan change/proration behavior for subscriptions. These were not verified.
- Sandbox availability and Go client options.

## 6. Open risks

1. **Unverified primary sources.** Asaas pricing (truncated), Efí (unreachable), Mercado Pago PIX Automático (404), Stripe PIX Automático in Brazil (contradictory snippet). Each affects the recommendation.
2. **PIX Automático flow dependency.** Authorization requires the payer's bank to confirm. Some banks and payers may not complete it, so a Legnaris subscription may not activate even after the first payment. Need a fallback (card or boleto) in the Plans flow.
3. **Pre-debit delay.** Stripe charges at cycle + 3 days and PIX Automático has a 3-day notification window. Plan start, renewal and limit-reset semantics must account for this lag.
4. **Amount caps.** Stripe PIX mandates have a default BRL 400 max per cycle and `fixed` vs `maximum` types. Plan price changes (upgrades) may require a new mandate. Asaas caps and rules are not verified.
5. **Webhook security and idempotency.** At-least-once delivery is confirmed for Asaas (per Asaas docs); signature scheme is unverified. The backend must dedupe by event id and verify the source before changing entitlements.
6. **Proration and plan changes.** Not verified on any provider. Per-user plan limits will be affected by mid-cycle upgrades and downgrades; Legnaris may need to implement proration itself.
7. **Tax and NFS-e.** Asaas NFS-e requires municipal webservice homologation and a company registration (pessoa jurídica); not every municipality allows automatic cancellation. Confirm with the target municipality. Stripe and Pagar.me NFS-e: not checked.
8. **Fee drift.** Some rates are promotional (Asaas, Mercado Pago). Re-check before signing.
9. **Vendor lock-in.** Subscription state is provider-held; design an internal `subscription` table keyed by provider IDs so migration is possible.
10. **Go integration.** No official Go SDK was confirmed for Asaas or Pagar.me; expect a thin REST client and contract tests with recorded fixtures.
11. **Legal and consumer rules.** Not covered here: Brazilian consumer rules for recurring charges, cancellation rights, and LGPD data handling for payer data.

## 7. Sources

- https://stripe.com/br/pricing
- https://docs.stripe.com/payments/pix/pix-automatico
- https://docs.stripe.com/payments/pix.md (search snippet only)
- https://www.asaas.com/precos-e-taxas
- https://blog.asaas.com/taxas-asaas/
- https://docs.asaas.com/docs/pix-automatico
- https://docs.asaas.com/reference/atualizar-configuracao-para-emissao-de-notas-fiscais
- https://docs.asaas.com/docs/webhook-para-cobrancas
- https://docs.asaas.com/docs/how-to-implement-idempotence-in-webhooks
- https://asaas.com/nota-fiscal (search snippet)
- https://hookdeck.com/webhooks/platforms/guide-to-asaas-webhooks-features-and-best-practices (third-party)
- https://pkg.go.dev/github.com/mobilemindtec/go-payments/asaas (unofficial)
- https://www.mercadopago.com.br/developers/pt/docs/subscriptions/landing
- https://www.mercadopago.com.br/developers/pt/docs/checkout-api-payments/payment-methods/pix
- https://www.mercadopago.com.br/ferramentas-para-vender/link-de-pagamento (search snippet)
- https://www.pagar.me/precos
- https://docs.pagar.me/
- https://www.lemonsqueezy.com/pricing
- https://developer.paddle.com/changelog/2026/pix-automatico (search snippet)
- https://finsidersbrasil.com.br/giro-noticias/apos-sucesso-do-pix-automatico-efi-bank-lanca-bolix-automatico/ (search snippet)
- Banco Central PIX Automático guide: https://www.bcb.gov.br/content/estabilidadefinanceira/pix/guia_pix_automatico.pdf (search result, not fetched)
- Regulatory background (search result, not fetched): Resolução CMN nº 5.251 (25 Sep 2025) makes PIX Automático mandatory for interbank debit authorization, effective 13 Oct 2025 for PJ recipients. https://www.cnnbrasil.com.br/economia/macroeconomia/pix-automatico-sera-obrigatorio-em-debitos-entre-bancos-a-partir-de-outubro/

Not fetched and not verified: Efí fees, Iugu, Paddle Brazil availability, Stripe Go SDK and sandbox specifics, Asaas dunning and proration, Mercado Pago webhook topic names and Go SDK.
