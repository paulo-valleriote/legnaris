# Resend in production (LGN-142)

Research for "Resend em produção: limites, domínio e remetente". Sources are
Resend's official docs and this repository. Items that could not be confirmed
from a primary source are marked **UNVERIFIED**. No secrets are recorded here.

## Summary

- Free plan: **100 emails/day** (UTC day, resets at midnight UTC), **3,000 emails/month**, **10 requests/second** (no burst; the 11th request in a second gets HTTP 429).
- Free plan allows up to **3 verified domains** and **3 inboxes**. Bounce rate must stay under 4%, spam rate under 0.08%.
- Multiple recipients (To/CC/BCC) each count as a separate email against the daily and monthly quotas.
- Resend requires at least one verified domain before sending. Its docs recommend a sending subdomain (e.g. `updates.example.com`), added as its own domain entry.
- DMARC is **recommended, not required** per Resend's domain page. SPF and DKIM records come from the dashboard's Records tab and must match exactly.
- `from` takes `Name <email@domain>` or a bare address; the address must be at a verified domain.
- `reply_to` is supported by the Send Email API (string or array). **The repo's Resend adapter does not send it.**
- Event types include `email.delivered`, `email.delivery_delayed`, `email.bounced`, `email.complained`, `email.failed`, `email.suppressed`, plus `suppression.added` / `suppression.removed`. Webhooks are verified with `svix-id`, `svix-timestamp`, `svix-signature` headers.
- **Repo mismatch:** `AUTH_OTP_DAILY_SEND_CAP` defaults to **80**, which is below the 100/day free cap (fits, with 20 emails of headroom for any other mail sent from the same account). The code comment says it is deliberately below 100. The cap counts only OTP sends, so any other transactional mail sharing the account draws from the same 100.
- **Repo gap:** `EMAIL_PROVIDER` defaults to `log` (no e-mail is sent). Production needs `resend` plus `RESEND_API_KEY` and `EMAIL_FROM`, which are not yet set in the production overlay.
- **Rate-limit gap:** the Resend adapter has no 429 / retry handling (grep of `backend/internal/email/resend/` finds none). Worker prefetch defaults to 10, so bursts could exceed 10 req/s. **UNVERIFIED** whether this is a real problem at current volume.

## 1. Free plan limits

Source: https://resend.com/docs/knowledge-base/account-quotas-and-limits

| Limit | Free plan value | Notes |
|---|---|---|
| Daily transactional | 100 emails/day | UTC calendar day, not a rolling 24h window |
| Monthly transactional | 3,000 emails/month | |
| Per-second rate | 10 requests/second | No burst allowance; 11th request in same second gets 429 |
| Recipients per email | No per-email cap stated | Each To/CC/BCC address counts as a separate email toward quotas |
| Marketing | Up to 1,000 contacts/month | Not used by OTP mail |
| Verified domains | 3 | |
| Inboxes | 3 | |
| Received emails | Count toward the same daily/monthly quotas | |
| Bounce rate | Must stay under 4% | |
| Spam rate | Must stay under 0.08% | |
| Data retention | 30 days | |

Batch Email API: up to 100 emails per call, each batch counts as one request against the rate limit. The page does not say whether this applies only to free accounts. (Same source.)

Send API `to` field: "Max 50" recipients per request (https://resend.com/docs/api-reference/emails/send-email).

## 2. Verified domain requirements

- Must add and verify at least one domain before sending. Quote: "You must add and verify at least one domain to send emails with Resend." (https://resend.com/docs/dashboard/domains/introduction)
- Sending from a subdomain is recommended (`updates.example.com`), to isolate reputation. Each subdomain is added individually and shows as its own Domains entry. (https://resend.com/docs/dashboard/domains/introduction)
- Lookalike domains (e.g. `example-alerts.com` instead of `alerts.example.com`) may be flagged as spoofing. (same)
- DNS records: Resend requires "TXT and MX or CNAME records" for DKIM and SPF, copied exactly from the dashboard's **Records** tab. Do not proxy CNAMEs (e.g. Cloudflare orange cloud). (https://resend.com/docs/add-a-domain)
- Return-Path defaults to `send.<domain>` and can be customized. (https://resend.com/docs/add-a-domain)
- **DMARC:** presented as a step after verification, "important for email deliverability", not a requirement. Resend's domain page says "Implement DMARC and BIMI to build trust and improve inbox placement" (recommended). (https://resend.com/docs/dashboard/domains/introduction, https://resend.com/docs/add-a-domain)
- Exact host names and values for SPF/DKIM/DMARC were **not** present in the fetched pages. They are shown only in the dashboard Records tab. **UNVERIFIED:** the typical pattern (DKIM TXT at `resend._domainkey.<sub>`, SPF TXT `v=spf1 include:amazonses.com ~all` at `send.<sub>`, MX `feedback-smtp.<region>.amazonses.com` at `send.<sub>`, DMARC TXT `_dmarc.<domain>` with `v=DMARC1; p=none;`). Copy the actual values from the dashboard; do not type them from this list.
- The `from` address rules: once a domain is verified, any address at that domain can be used with no extra configuration. Shared or public domains (gmail.com etc.) cannot be used. (https://resend.com/docs/dashboard/domains/introduction)

## 3. Sender and reply-to

- `from` (string, required). Display name format: `Name <email@example.com>`; example `Acme <onboarding@example.dev>`. (https://resend.com/docs/api-reference/emails/send-email)
- `reply_to`: "Reply-to email address. For multiple addresses, send as an array of strings." Accepts string or string array. The parameter heading on the page was garbled in extraction, so the exact field name should be re-checked in the reference page. (https://resend.com/docs/api-reference/emails/send-email)
- `to` (string | string[], required, max 50). `cc`, `bcc` (string | string[], optional). `headers` (object, optional). (same)
- `Idempotency-Key` request header: unique per request, expires after 24 hours, max 256 characters. Useful for OTP retries. (same)
- **Repo:** `backend/internal/email/resend/sender.go` does not reference `reply_to` (grep for "reply" returned no matches). Not needed for OTP mail (no-reply), but it is a gap if support replies are wanted. Adding it requires a test per AGENTS.md.

## 4. OTP deliverability: bounces, complaints, suppression, webhooks

Event types (https://resend.com/docs/webhooks/event-types):

- `email.sent`: API request succeeded; delivery attempted.
- `email.delivered`: accepted by recipient's mail server.
- `email.delivery_delayed`: temporary failure (full inbox, transient server issue).
- `email.bounced`: recipient's server permanently rejected the email (hard bounce).
- `email.complained`: delivered, but recipient marked it as spam.
- `email.failed`: could not send (invalid recipient, API key problem, domain verification issue, quota limit).
- `email.suppressed`: Resend suppressed the send.
- `email.opened`, `email.clicked`, `email.scheduled`, `email.received`.
- `suppression.added` / `suppression.removed`: an address is added automatically after a hard bounce or spam complaint, or manually via dashboard/API, and removed when it leaves the list. (https://resend.com/docs/webhooks/event-types)

Signature verification (https://resend.com/docs/webhooks/verify-webhooks-requests):

- Headers: `svix-id`, `svix-timestamp`, `svix-signature`.
- Verify the **raw** request body as a string; re-serializing parsed JSON breaks the signature.
- Options: the Resend SDK `resend.webhooks.verify()` (throws on invalid input), or the `svix` library's `Webhook` class with `verify(payload, headers)`.
- Signing secret: shown on the webhook's details page, also returned by the create/retrieve/list webhook API.
- Secret format not stated (**UNVERIFIED**).

Deliverability limits to watch (Section 1): bounce < 4%, spam complaints < 0.08%. Exceeding these risks account restriction.

Resend's bounce-handling article (`/docs/knowledge-base/what-is-a-bounce-and-how-to-handle-it`) returned 404 at fetch time; the suppression behavior above comes from the event-types page. **UNVERIFIED:** Resend's own recommendation on retrying soft bounces.

The repo does not currently consume Resend webhooks (no webhook route found in the grep of `backend/` for `resend`; only the sender adapter, the outbox, and metrics reference it). **UNVERIFIED** as a complete search; treat as "not found" rather than "confirmed absent".

## 5. Operator steps

Resend API key (dashboard): create a key with a name, a permission level, and an optional domain restriction. Permission can be changed later only in the Dashboard. The key value is shown once and cannot be viewed later. Exact permission names (e.g. "Sending access") were not present in the fetched doc (`https://resend.com/docs/dashboard/api-keys/introduction`); **UNVERIFIED**. Prefer a sending-only key restricted to the sending domain, if the dashboard offers it.

Sealed secret in k3s: the repo uses **Bitnami Sealed Secrets** (not an invented tool).

- `Makefile` target `k8s-platform` installs `sealed-secrets` via Helm (`Makefile:196-201`).
- `RUNBOOK.md:21-42`: create the gitignored plain env file `deploy/k8s/overlays/production/secrets.env` (keys listed in `deploy/k8s/overlays/local/secrets.env.example`), then run `scripts/k8s-seal-secrets.sh`, which writes `deploy/k8s/overlays/production/sealed-secrets.yaml`, and commit that sealed file.
- `scripts/k8s-seal-secrets.sh:2-31`: seals `secrets.env` into secret `legnaris-secrets` in namespace `default`.
- Back up the Sealed Secrets key off the VPS (RUNBOOK.md:27-28).

Steps:
1. Sign up / sign in at Resend; add a sending subdomain (e.g. `auth.<domain>`) in the Domains page.
2. Copy every DNS record shown in the Records tab (DKIM TXT, SPF TXT/MX-or-CNAME) exactly into the DNS provider. Keep Cloudflare proxy off for CNAMEs.
3. Wait for verification in the dashboard.
4. Optionally add DMARC after verification (start with `p=none`, monitor, then tighten). Recommended, not required.
5. Create a sending-scoped API key (`re_...`). Copy it once.
6. Add `RESEND_API_KEY=<key>` and `EMAIL_FROM="Legnaris <no-reply@auth.<domain>>"` to `deploy/k8s/overlays/production/secrets.env` (`EMAIL_FROM` belongs in the ConfigMap per RUNBOOK.md:40-42 if preferred; the runbook says "in the sealed secret and the ConfigMap").
7. Set `EMAIL_PROVIDER=resend` in the production ConfigMap (base default is `log` in `deploy/k8s/base/app.yaml:13` and the worker reads it at `deploy/k8s/base/worker.yaml:26`).
8. Run `scripts/k8s-seal-secrets.sh`, review the generated `sealed-secrets.yaml`, commit only the sealed file. Confirm `secrets.env` is not tracked.
9. Keep `AUTH_OTP_DAILY_SEND_CAP` at 80 (or lower) for the 100/day free cap; decide whether other transactional mail shares the account.
10. Decide whether to add a webhook endpoint for `email.bounced` / `email.complained` (not in repo today). Store the signing secret the same way as `RESEND_API_KEY`.

## 6. Repo comparison

| Variable | Default in code | Source | Notes |
|---|---|---|---|
| `EMAIL_PROVIDER` | `log` | `backend/shared/env/env.go:123`, `:259` | `log` sends nothing. Production must set `resend`. Base manifests set `log` (`deploy/k8s/base/app.yaml:13`). |
| `RESEND_API_KEY` | `""` | `backend/shared/env/env.go:56`, `:260` | Consumed by the worker (`backend/cmd/worker/main.go:124`). Not in base manifests; must come from the sealed secret. |
| `EMAIL_FROM` | `""` | `backend/shared/env/env.go:261` | No default; an empty value is not validated in the grep (**UNVERIFIED** whether the adapter rejects it). |
| `AUTH_OTP_DAILY_SEND_CAP` | `80` | `backend/shared/env/env.go:143`, `:282` | Comment at `env.go:137-138`: "deliberately below the provider's free-tier 100". Enforced by the UTC-day window (`backend/internal/auth/challenge_policy.go:60-65`) and `ErrSendBudgetExhausted` (`backend/internal/auth/challenge_mailer.go:27-31`). Wired in `backend/internal/app/bootstrap.go:139`. |
| `AUTH_OTP_USER_LIMIT` / `AUTH_OTP_USER_WINDOW` | `10` / `1h` | `env.go:144-145` | Per-account bound, below the 3,000/month cap in aggregate. |
| `AUTH_OTP_RESEND_COOLDOWN` | `60s` | `env.go:140` | |
| `AUTH_OTP_MAX_ATTEMPTS` | `5` | `env.go:142` | |
| `AUTH_OTP_TTL` / `AUTH_OTP_CEILING` | `10m` / `30m` | `env.go:138-139` | |
| `EMAIL_WORKER_PREFETCH` | `10` | `env.go:~133` | Concurrency relevant to the 10 req/s limit. |
| `EMAIL_WORKER_MAX_ATTEMPTS` | `5` | `env.go:134` | |
| `EMAIL_CODE_SECRET` | none (required) | `deploy/k8s/base/app.yaml:53-54` | From `legnaris-secrets`. |

**Fit of `AUTH_OTP_DAILY_SEND_CAP=80` against the free plan:**

- Fits the 100/day daily cap, with 20 emails of headroom for any other send from the same account (the cap counts only OTP sends; other mail shares the same 100 quota). Confirm no other e-mail uses this account.
- Monthly: 80 x 31 = 2,480 at the maximum day, under 3,000. Fits.
- Per second: the cap does not bound burst rate. With prefetch 10 and no 429 handling in `backend/internal/email/resend/`, a burst can exceed 10 req/s. **Mismatch / risk.** Either lower prefetch or add 429 backoff with a test.
- Production secrets and `EMAIL_PROVIDER=resend` are not yet configured in the repo. **Mismatch:** the production overlay currently produces no OTP email.

Other deploy references: `deploy/k8s/README.md:15-16` (secrets never committed; `secrets.env.example`), `deploy/k8s/overlays/production/kustomization.yaml:4-5` (secrets listed only after sealing), `deploy/k8s/base/relay.yaml`, `deploy/k8s/base/worker.yaml:23-28` (worker env wiring).

## Operator checklist (HITL task)

1. In Resend, go to Domains and add a sending subdomain (e.g. `auth.<your-domain>`).
2. Open that domain's Records tab and copy each record exactly. Add them in the DNS provider. Do not proxy CNAMEs.
3. Click Verify in Resend. Wait until the domain shows verified.
4. Optional but recommended: add a DMARC TXT record (`_dmarc.<your-domain>`), start with `p=none`.
5. Create an API key with sending-only permission, restricted to the subdomain if offered. Copy it once.
6. Edit `deploy/k8s/overlays/production/secrets.env` (gitignored): set `RESEND_API_KEY` and `EMAIL_FROM` (format `Name <address@verified-subdomain>`).
7. Set `EMAIL_PROVIDER=resend` in the production ConfigMap.
8. Confirm `AUTH_OTP_DAILY_SEND_CAP` is at most 80 and that no other mail uses this Resend account.
9. Run `scripts/k8s-seal-secrets.sh`, review `deploy/k8s/overlays/production/sealed-secrets.yaml`, and commit only the sealed file.
10. Back up the Sealed Secrets controller key off the VPS (RUNBOOK.md:27-28).
11. Send one test OTP to a real inbox and confirm delivery in the Resend dashboard.
12. Decide on the 429/backoff and webhook follow-ups (tracked separately).

## Primary sources

- https://resend.com/docs/knowledge-base/account-quotas-and-limits
- https://resend.com/docs/dashboard/domains/introduction
- https://resend.com/docs/add-a-domain
- https://resend.com/docs/api-reference/emails/send-email
- https://resend.com/docs/webhooks/event-types
- https://resend.com/docs/webhooks/verify-webhooks-requests
- https://resend.com/docs/dashboard/api-keys/introduction (fetched; permission details not present)
- Not reachable at fetch time (404): https://resend.com/docs/knowledge-base/what-is-a-bounce-and-how-to-handle-it
