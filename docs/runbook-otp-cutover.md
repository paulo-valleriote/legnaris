# Runbook — OTP registration cutover (LGN-158, D13)

Rolls out email-verified registration to production (k3s + Argo CD, see
`deploy/k8s/production/RUNBOOK.md`). There is no feature flag: the contract of
`POST /auth/register` changes in one release, so the order below is what keeps
users from seeing a broken state.

> **Status:** written from the repository only; nothing here has been executed
> against production. Rehearse steps 1–3 on k3d (`make k8s-up`) first.

Context: epic LGN-138 (decisions D1–D15); D13 is this cutover. "HITL card"
is LGN-157, the human-only Resend/DNS/API-key setup.

## What changes for clients

| | Old API | New API |
|---|---|---|
| `POST /auth/register` | 201 with a session | 201 **without** a session: `challenge_id` (normal) or `{delivery: "deferred"}` (daily allowance spent, or the provider breaker open for more than 5 min) |
| Login, unverified account | session | new `verify_email` challenge, or 503 + `Retry-After` when the allowance is spent |
| Existing accounts and sessions | — | no backfill; `Refresh` revokes the family and verification happens at the next login |

An old web bundle expects a session after register, so it must not be served
against the new API for longer than the update window (step 3).

## Roles and tools

- Operator with `kubectl` on the VPS, access to `ops.legnaris.com.br` (Grafana,
  GitHub login) and the repository.
- Production images are pinned by `newTag` in
  `deploy/k8s/overlays/production/kustomization.yaml`; the `gitops-bump`
  workflow opens the PR that rewrites them and Argo CD (`selfHeal`) applies it.
- Prometheus alerts: `https://ops.legnaris.com.br/grafana` (**Email pipeline**
  dashboard) and `kubectl -n monitoring port-forward` to Prometheus
  (`/alerts`).

## Step 0 — Preconditions (abort the cutover if any fails)

- [ ] LGN-157 done: `mail.legnaris.com.br` verified in Resend (SPF, DKIM,
      DMARC `p=none`), `RESEND_API_KEY` sealed in `legnaris-secrets`.
- [ ] LGN-153 merged: no `AUTH_REQUIRE_EMAIL_VERIFICATION` left in env, compose,
      k8s or `.env.example`.
- [ ] All implementation cards of LGN-138 merged (LGN-145 … LGN-156, LGN-159,
      LGN-160) and CI green on the commit being released.
- [ ] A Mongo backup from the last 24 h exists (`mongodb-backup` CronJob) and a
      restore was verified (`deploy/k8s/backup/restore-verify.yaml`).
- [ ] Alerts are loaded in the cluster (`make k8s-rules` for the Kubernetes
      rule set; `deploy/prometheus/rules.yml` for compose): the pipeline
      alerts, the captcha-trigger alerts `EmailRegisterBurstTooLarge` and
      `EmailCapReachedEarly` (LGN-159) and an alert on
      `legnaris_email_breaker_open == 1` (job `legnaris-worker`).
      **Open blocker:** `rules.yml` has no breaker alert today (only the
      metric exists). The cutover must not start until that rule is merged;
      the 48 h window depends on it.
- [ ] The previous `newTag` values for `legnaris-app` and `legnaris-web` are
      written down (rollback targets): `git log -p -- deploy/k8s/overlays/production/kustomization.yaml`.
- [ ] Release window chosen outside peak hours; the Resend daily allowance
      (free tier: 100/day, resets 00:00 UTC) is not already spent.

## Step 1 — Email infrastructure with the **old** API

Broker, relay, worker and real Resend are brought up while the old API still
serves production, so the pipeline is proven before anything depends on it.

1. Sync the infra (relay, worker, RabbitMQ) with the **old** `legnaris-app`
   tag. The old API does not send registration codes, so the delivery proof
   comes from the LGN-157 test, not from a real signup.
2. Health checks:
   ```bash
   kubectl get pods                                  # relay, worker, rabbitmq-0 Running
   kubectl rollout status deployment/relay deployment/worker --timeout=300s
   kubectl logs deploy/worker | tail                 # EMAIL_PROVIDER=resend, no startup error
   ```
   Prometheus targets `legnaris-relay`, `legnaris-worker`, `legnaris-rabbitmq`
   are `up`; `rabbitmq_queue_consumers{queue="email.otp"} >= 1`;
   `EmailPipelineSeriesMissing` is not firing.
3. Delivery test: repeat the LGN-157 test delivery through the **deployed**
   worker (not from a laptop) to an inbox the operator controls and confirm it
   arrives from
   `Legnaris <no-reply@mail.legnaris.com.br>` and passes SPF/DKIM
   (message headers).

**Abort criteria:** worker pod in `CreateContainerConfigError`
(missing `RESEND_API_KEY`), test message not delivered within 5 min, SPF/DKIM
failing, or any pipeline alert firing. Nothing user-facing changed yet, so
abort = fix and repeat.

## Step 2 — New API

1. Merge the `gitops-bump` PR that moves `legnaris-app` to the release tag.
   Argo CD syncs; the rollout is `maxUnavailable: 0`, gated by `/ready`.
   ```bash
   kubectl rollout status deployment/app deployment/relay deployment/worker --timeout=300s
   curl -fsS https://api.legnaris.com.br/ready
   ```
2. Contract smoke test against production with a throwaway address you own:
   - `POST /auth/register` → 201, **no** `Set-Cookie`, body has `challenge_id`
     (or `delivery: "deferred"` if the allowance is spent).
   - The code arrives; `POST /auth/otp/verify` establishes the session.
   - Login with an unverified account → new `verify_email` challenge.
3. Metrics: `legnaris_auth_otp_issued_total` increments;
   `legnaris_auth_otp_cap_blocked_total` stays flat.

From this point the **old web** (still served) breaks on register: it expects a
session. Keep the gap to step 3 as short as possible.

**Abort criteria:** `/ready` failing, register answering 5xx, a session cookie on
a register response, no code delivered for the smoke test, or
`EmailWorkQueueHasNoConsumer` firing. Roll back per "Rollback" below.

## Step 3 — New web and the service-worker update

`web/vite.config.ts` uses `registerType: 'prompt'`: a returning PWA keeps the
old bundle until the user accepts the "A new version is available" notice
(`web/src/pwa/update_prompt.tsx`). The worker answers `SKIP_WAITING`
(`web/src/pwa/sw.ts`) and the page reloads after `applyUpdate()`. So the update
is **offered**, not forced, and old bundles can live on in open tabs and in
installed PWAs.

1. Merge the `gitops-bump` PR that moves `legnaris-web`.
   ```bash
   kubectl rollout status deployment/web --timeout=300s
   curl -fsS https://app.legnaris.com.br/ | head
   ```
2. Verify the update path in a browser **that has the old version cached**
   (open the old app first, then deploy):
   - DevTools → Application → Service Workers: a new worker is `waiting`.
   - The update notice appears; **Reload** activates it and the page reloads on
     the new bundle (no stuck "waiting" worker, no blank page).
   - Register in the new web: deferred notice or code screen, never a silent
     failure.
3. Verify the old bundle's failure mode, still in its tab: registering against
   the new API must end in a visible error, not a hang or a half-signed-in
   state. Reloading must recover.
4. `sw.js` and `index.html` must be served `Cache-Control: no-cache`
   (`web/nginx.conf` does this) so browsers can see the new worker; confirm with
   `curl -sI https://app.legnaris.com.br/sw.js`.
5. If the old-bundle failure is not recoverable by a reload, abort and roll the
   web back (the API stays; the old web is then the broken half, so roll the
   API back too if the old web cannot register at all).

### Known deviation: the update is not forced

D13 and LGN-158 call for a forced service-worker update, but the shipped PWA
uses `registerType: 'prompt'`, so nothing forces activation. Until a user
accepts the notice or closes every tab, they stay on the old bundle, whose
register screen expects a session. Decide before the cutover, and record the
choice on LGN-158:

- **Accept the gap** (no code change): the old bundle shows an error on
  register until the user reloads; existing signed-in users are unaffected.
  Prefer a low-traffic window.
- **Force it**: ship a web change that activates the new worker immediately
  (`registerType: 'autoUpdate'` or calling `applyUpdate()` on `needRefresh`),
  as its own card and release, before step 2.

## Step 4 — 48 h observation

Start the clock after step 3. Watch the **Email pipeline** dashboard; each
alert's meaning and response are in `deploy/README.md`, "Pipeline alerts and
runbook". What to watch:

| Signal | Source | Concern |
|---|---|---|
| Codes not delivered | `EmailWorkQueueHasNoConsumer`, `EmailOldestMessageTooOld`, `EmailDeadLetterQueueHoldsMessages` | critical/high: nobody can finish registering |
| Provider refusing | `EmailProviderRefusingSends`, `legnaris_email_breaker_open` (worker) | breaker open > 5 min makes the API answer `deferred` |
| Allowance | `EmailDailyAllowanceReached`, `legnaris_auth_otp_cap_blocked_total` | resends use at most 30% of the cap; new signups keep the rest |
| Verification failures | `EmailVerificationFailuresClimbing` | late delivery or code guessing |
| Captcha trigger (D14) | `EmailRegisterBurstTooLarge` (> 30 `register` 429s in 24 h), `EmailCapReachedEarly` (cap reached before 12:00 UTC on 2 days of the week) | turning captcha on is a manual decision |

Check at +1 h, +6 h, +24 h and +48 h:

- Registrations vs. verifications: most `otp_issued_total{purpose="verify_email"}`
  should be followed by a successful `otp_verify_total`.
- No `failed` rows piling up in `email_outbox`; `email.dead` is empty.
- 5xx rate and `/ready` on the API unchanged from the pre-cutover baseline.
- DMARC: after two clean weeks move `p=none` to `quarantine` (LGN-142).

**Close the cutover** when 48 h pass with no critical/high alert left unresolved.
Record the outcome on LGN-138.

## Rollback

There is no flag: roll back by reverting images, in reverse order, through Git
(Argo CD `selfHeal` reverts manual `kubectl` edits, so do not patch the
cluster by hand).

1. **Web first** (if step 3 was reached): revert the `legnaris-web` `newTag` to
   the previous `sha-` value. Browsers already on the new bundle keep running
   it until they reload; the next load fetches the reverted `sw.js`, which
   installs as an update the same way. Until then the new web talks to the
   old API, so roll the API back promptly.
2. **API**: revert the `legnaris-app` `newTag` to the previous `sha-` value.
   Relay and worker stay up (they are harmless with the old API).
3. Verify `/ready`, a login and an existing session; confirm the old register
   issues a session again.

Data notes: no data migration is needed, but accounts created while the new API
ran and not yet verified keep `EmailVerifiedAt` empty. The old API does not
demand a code, so those accounts can sign in without verifying while it runs;
they are asked for a code again once the new API is redeployed. Pending codes
in the outbox expire with `EMAIL_OUTBOX_RETENTION`.

**Roll back when:** register or login is failing for most users, no code is
delivered and the cause is not understood within 30 minutes, or the old-bundle
recovery in step 3 fails.

**Do not roll back for:** `deferred` responses while the allowance is spent or
the breaker is open (designed behavior), or a single provider outage (the
outbox accumulates and drains).
