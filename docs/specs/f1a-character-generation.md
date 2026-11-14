# F1a: Character generation on the Free trial

Epic: LGN-221. Roadmap: LGN-181. ADRs: 0007, 0008, 0009, 0010, 0013.

First public release. Every user is Free and gets a lifetime trial of 3 generations. The paid daily quota ships but is first reachable in F1b. Cost is bounded by the trial, a global daily job cap and a kill-switch.

Each section below states the contract the card must meet. The Jira card holds the full scope and test list; this file records the decisions taken while aligning the epic.

## Decisions

- OpenRouter is the only LLM gateway, with default routing and fallbacks (ADR 0008). LGN-227 confirmed that an endpoint serves the pinned model.
- Trial eligibility is its own card (LGN-229). LGN-182 only owns the counter and the quota.
- LGN-186 is the generation API; LGN-230 is the worker.
- The account-deletion cascade in F1a covers user, Sheets, generations, consent and trial. Campaigns, NPCs/Monsters and analytics are follow-ups (LGN-216, LGN-217, LGN-206).
- LGN-210 (billing Provider port) belongs to F1b.
- Go-live gate: LGN-190, LGN-195, LGN-196, LGN-226 done and deployed, LGN-227 concluded.

## Definition of done (all cards)

- Tests cover edge cases first, then the happy path.
- `make fmt && make lint && make vet && make test-race` are clean.
- The ADR is updated when behavior diverges from it.
- Mocks are regenerated after interface changes.

## Wave 0

### LGN-227: ZDR spike

Confirm that at least one OpenRouter endpoint for `deepseek/deepseek-v4.1-flash` satisfies `zdr: true`, `data_collection: "deny"` and `ignore: ["DeepSeek"]`. Time-box: 1 day. Append the result to ADR 0008. On failure, reopen the model and gateway decision.

### LGN-228: privacy policy and consent text v1

English and pt-BR text. Lists OpenRouter and the serving provider as processors, the transfer abroad, 24 h retention in Legnaris and how to revoke. The consent version constant (LGN-194) must match it.

## Wave 1

### LGN-197: publisher and topology

Contract in the card. Errors: the broker being down never blocks the caller; drops are counted in `legnaris_events_dropped_total{type,reason}`.

### LGN-183: Sheet kind and limits

Errors: `SHEET_LIMIT_REACHED` 409, `INVALID_REQUEST` 400 for a missing or invalid kind.

### LGN-182: trial counter and quota

Errors: `PLAN_UPGRADE_REQUIRED` 403 when the trial is exhausted or the account has 0 allowance; `QUOTA_EXCEEDED` 429 with `Retry-After`.

### LGN-229: trial gating

Grants the allowance (3) only to an OTP-verified, non-disposable email with at most 3 accounts per IP per 24 h. Accounts over the cap are created with 0 allowance.

### LGN-184: FieldSchema generation metadata

Contract in the card. Boot validation rejects inconsistent metadata.

### LGN-185: llm.Client and OpenRouter adapter

Depends on LGN-227. Errors separate transient from permanent failures; "no eligible provider" is transient and maps to `provider_unavailable`.

### LGN-194: LLM consent

Depends on LGN-228. Errors: `LLM_CONSENT_REQUIRED` 403, `VALIDATION` 400 on a version mismatch.

## Wave 2 and 3

### LGN-208: event_outbox and relay

Contract in the card.

### LGN-198: sheet.* and user.registered events

Optional side lane, outside the go-live gate.

### LGN-209: generic sweeper

Contract in the card. Exposes the per-queue metrics that LGN-226 alerts on.

## Wave 4

### LGN-186: generation API

Gate order on `POST /generations`: kill-switch, consent, Sheet limit, generation in progress, global daily cap, quota. Errors: `LLM_CONSENT_REQUIRED` 403, `SHEET_LIMIT_REACHED` 409, `GENERATION_IN_PROGRESS` 409, 503 when the kill-switch is on or the cap is reached, `QUOTA_EXCEEDED` 429, `PLAN_UPGRADE_REQUIRED` 403. The cap is an atomic counter per UTC day and exposes a metric for the 80% alert.

### LGN-230: generation worker

Claims jobs with a lease, validates the output as untrusted data and refunds the quota on every terminal failure in the same transaction as the terminal state.

### LGN-226: alerts and dashboards

Adds the 80% global cap alert and outbox metrics to the alert set in the card.

## Wave 5

### LGN-190: account deletion MVP slice

Two-step request: `POST /auth/me/deletion` sends a one-time code to the account's address, and `DELETE /auth/me {challenge_id, code}` marks the user `deleting` in a transaction. A code of another purpose or account is the uniform code failure and is not consumed. The `users deleting` sweeper queue removes Sheets, generations, consent, trial, the account's challenges and the e-mail rows keyed by its address, then writes a tombstone and inserts `user.deleted` into the outbox in the same transaction.

## Track B

### LGN-195: consent revocation screen

Starts after LGN-194 and LGN-228.

### LGN-196: generation flow

Starts after LGN-182, LGN-186, LGN-230, LGN-194 and LGN-228. No checkout in F1a.

## Deploy checklist

- Spending limit set in the OpenRouter dashboard.
- `GENERATION_ENABLED` and the daily cap configured.
- OpenRouter prompt logging disabled.
