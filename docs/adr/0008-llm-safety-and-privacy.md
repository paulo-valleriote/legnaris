---
status: accepted
---

# LLM generation runs only with versioned consent and a paid, pinned model

Character generation (ADR 0007) sends a player's concept text to a third-party gateway (OpenCode) that serves a DeepSeek model hosted in the US/EU. Before any concept leaves Legnaris, the player must give explicit, versioned consent, and that consent is the first gate on `POST /generations`. The gateway accepts only the paid `deepseek-v4.1-flash` model. Concept and output stay inside the 24 h `generation` document and never reach logs. Generated content lives only on the player's own Sheet. Decided in LGN-176.

## Considered Options

- **Implicit consent through the terms of service:** LGPD art. 33 needs a specific basis to transfer data abroad. Consent buried in the terms is not specific, and it cannot be revoked separately.
- **Free model tier (`-free` IDs):** the OpenCode terms let it use Content from "unpaid" accounts to improve its services. That is training-adjacent use we cannot accept.
- **Output denylist filter for copyrighted names:** it produces false positives, someone has to maintain the list, and the output only ever lands on a private Sheet.
- **A dedicated moderation API call on the concept:** it adds a vendor, latency and cost. Quota (Free = 0), rate limits and model refusals already bound abuse.

## Decisions

- **Consent:**
  - The user record stores `llm_consent_at` and `llm_consent_version`.
  - The current version is a constant in code. Bump it whenever the disclosure changes, for example a new provider, model, region or retention period.
  - `POST /generations` runs these gates in order: consent → Sheet limit → generation in progress → quota. A missing or outdated consent returns `LLM_CONSENT_REQUIRED` 403.
  - `PUT /me/llm-consent {version}` records consent idempotently. A `version` other than the current one returns `VALIDATION` 400.
  - The disclosure names OpenCode as the gateway and DeepSeek as the model, US/EU hosting, 24 h retention in Legnaris, and that the concept is sent to the provider.
  - Revocation (LGPD art. 8 §5): `DELETE /me/llm-consent` clears both fields and deletes the user's `generation` documents. It uses the same cascade as account deletion.
- **Client:**
  - A modal shows the disclosure the first time the player taps Generate, and again after a version bump.
  - Free users see the Plan upsell instead and are never asked for consent.
- **Provider:**
  - `LLM_MODEL` is pinned to `deepseek-v4.1-flash`, the paid model.
  - Config validation fails at boot when the model ID ends in `-free`.
  - Known gap: OpenCode does not name the upstream DeepSeek host or give a retention period for it. Our only guarantee is OpenCode's statement that "providers follow a zero-retention policy and do not use your data for model training". Re-check this whenever the gateway or model changes.
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
  - The privacy policy must list OpenCode as a processor and the US/EU transfer of the concept.
- Changing the gateway or model is a privacy change, not just a configuration change: bump the consent version and update the disclosure text.
- Tests:
  - Consent gate ordering and the 403 code.
  - Version mismatch on `PUT` returns 400.
  - Revocation cascade.
  - Boot rejects `-free` models.
  - The adapter does not leak error bodies (`httptest`).
  - The conditional terminal write is a no-op on a missing document.
