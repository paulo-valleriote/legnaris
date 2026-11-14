---
status: accepted
---

# LLM generation runs only with versioned consent and a paid, pinned model

Character generation (ADR 0007) sends a player's concept text to a third-party gateway (OpenRouter) that routes it to a DeepSeek model. The serving provider is chosen per request by OpenRouter and may be the DeepSeek first-party API or another company, which may process outside Brazil, including in China. Before any concept leaves Legnaris, the player must give explicit, versioned consent, and that consent is the first gate on `POST /generations`. Every request pins the paid DeepSeek V4.1 Flash model and uses OpenRouter's default routing with fallbacks. Concept and output stay inside the 24 h `generation` document and never reach logs. Generated content lives only on the player's own Sheet. Decided in LGN-176. Strict zero-data-retention routing was dropped on 2026-11-14 (see Decisions, Provider).

## Considered Options

- **Implicit consent through the terms of service:** LGPD art. 33 needs a specific basis to transfer data abroad. Consent buried in the terms is not specific, and it cannot be revoked separately.
- **Free model variants (`:free` IDs on OpenRouter):** free endpoints may log or train on prompts. That is training-adjacent use we cannot accept.
- **OpenCode as the primary gateway (briefly chosen in LGN-174):** it does not name the upstream DeepSeek host or give a retention period, and offers no per-request routing control. It is not used, not even as a fallback.
- **Output denylist filter for copyrighted names:** it produces false positives, someone has to maintain the list, and the output only ever lands on a private Sheet.
- **A dedicated moderation API call on the concept:** it adds a vendor, latency and cost. Quota (Free = 0), rate limits and model refusals already bound abuse.
- **Strict zero-data-retention routing (`zdr: true`, `data_collection: "deny"`, `ignore: ["DeepSeek"]`):** the original decision. It was dropped on 2026-11-14 (see Decisions, Provider).

## Decisions

- **Consent:**
  - The user record stores `llm_consent_at` and `llm_consent_version`.
  - The current version is a constant in code. Bump it whenever the disclosure changes, for example a new provider, model, region or retention period.
  - `POST /generations` runs these gates in order: consent → Sheet limit → generation in progress → quota. A missing or outdated consent returns `LLM_CONSENT_REQUIRED` 403.
  - `PUT /me/llm-consent {version}` records consent idempotently. A `version` other than the current one returns `VALIDATION` 400.
  - The disclosure names OpenRouter as the gateway and DeepSeek V4.1 Flash as the model. It states that the concept is sent to OpenRouter and to the provider it routes to, which may be the DeepSeek API or another company and may process outside Brazil, including in China. It states that provider retention and training follow each provider's own terms, the 24 h retention in Legnaris, and how to revoke.
  - Revocation (LGPD art. 8 §5): `DELETE /me/llm-consent` clears both fields and deletes the user's `generation` documents. It uses the same cascade as account deletion.
- **Client:**
  - A modal shows the disclosure the first time the player taps Generate, and again after a version bump.
  - Free users see the Plan upsell instead and are never asked for consent.
- **Provider:**
  - `LLM_MODEL` is pinned to `deepseek/deepseek-v4.1-flash`, the paid model.
  - Config validation fails at boot when the model ID ends in `:free`. This rule stays.
  - Every request sends the OpenRouter `provider` object with `require_parameters: true` and `allow_fallbacks: true`. It sends no `zdr`, `data_collection` or `ignore` field.
  - OpenRouter prompt logging stays disabled on the account.
  - **Routing decision (2026-11-14, product):** strict ZDR routing was dropped. The concept is free text that the player chooses to send. It is not sensitive personal data, and the consent discloses that it may reach the DeepSeek first-party API or another provider, including one that processes in China. Retention and training are governed by each provider's own terms. Because of this, a `DeepSeek` ignore rule and a ZDR filter are no longer part of the design.
  - When no endpoint can serve a request, the request still fails instead of silently falling back to anything else. The job ends `failed` with `provider_unavailable` and the quota is refunded.
- **Copyright:**
  - The fixed system prompt forbids names, characters and settings from published works.
  - There is no output filter.
  - Generated values exist only on the player's Sheet and never enter the code-defined Template catalog.
- **Abuse:**
  - There is no moderation call.
  - A model refusal is handled as a validation failure: the normal retries run, and the job ends `failed` with the quota refunded (ADR 0007).
- **Logs:**
  - On top of ADR 0007's rule (never log concept or output), the `llm` adapter never logs or propagates the body of a gateway error response, because it may echo the prompt.
  - Only the HTTP status and a classified error code are kept.
- **Deletion races:**
  - The worker's terminal write is a conditional `findOneAndUpdate` on `_id` and `status=running`.
  - If the document is gone because the account was deleted or consent was revoked, the worker acks and discards: nothing is written and no refund is issued.

## Consequences

- LGPD:
  - Account deletion also removes `generation` documents.
  - Data export does not include them, because they last at most 24 h.
  - The privacy policy must list OpenRouter and the model providers it may route to as processors, and the transfer of the concept abroad.
  - Account deletion also erases the personal data kept outside the account's own rows: the account's challenge rows, the `email_outbox` messages addressed to it and the notice claims keyed by its address, in the same transaction as the record (decided 2026-11-14, LGN-190 review). The request is confirmed by a one-time code sent to the account's address before anything is marked for deletion. Operational note: `docs/runbook-sweeper-dead-items.md`, `users_deleting`.
- Changing the gateway, model or routing is a privacy change, not just a configuration change: bump the consent version and update the disclosure text.
- Tests:
  - Consent gate ordering and the 403 code.
  - Version mismatch on `PUT` returns 400.
  - Revocation cascade.
  - Boot rejects `:free` models.
  - Every adapter request carries the default `provider` object (`httptest`), with no `zdr`, `data_collection` or `ignore` field.
  - The adapter does not leak error bodies (`httptest`).
  - The conditional terminal write is a no-op on a missing document.

## Spike note: endpoint check for `deepseek/deepseek-v4.1-flash` (LGN-227, resolved)

- **Checked on:** 2026-11-14, first with public OpenRouter listings (`GET /api/v1/models/deepseek/deepseek-v4.1-flash/endpoints` and `GET /api/v1/endpoints/zdr`), then with one live completion.
- **Slug:** `deepseek/deepseek-v4.1-flash` exists. A dated variant, `deepseek/deepseek-v4.1-flash-1017`, is also listed. It is not the pinned slug, and this note does not cover it.
- **Public listing:** the ZDR listing returned 32 healthy endpoints for the exact slug, served by 23 providers. No first-party DeepSeek endpoint was in that list.
- **Live completion (resolved):** one completion with default routing (no `zdr`, no `data_collection`, no `ignore`) succeeded. OpenRouter served it through the DeepSeek provider on model `deepseek/deepseek-v4.1-flash`.
- **Outcome:** the endpoint exists and serves the pinned model. Strict ZDR routing was not kept; see Decisions, Provider (2026-11-14). The `:free` ban and the model pin stand.
