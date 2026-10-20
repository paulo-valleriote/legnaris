# Quickstart: Sign-in Hardening — The Code Step Replaces the Immediate Session, and Unverified Accounts Cannot Renew

**Feature**: `.specs/features/018-auth-2fa-session-gate/` | **Date**: 2026-10-20

How to prove the feature works, end to end and requirement by requirement. Nothing here is
implementation guidance — the runnable steps come from `tasks.md` and the code.

## Prerequisites

| Need | How |
|---|---|
| Go 1.26.4 | `go version` |
| A local MongoDB 7.0 | `make dev-db` (the repository's dev-only compose service) |
| The existing auth + challenge stack | features `015`, `016`, `017` are already on `main` |
| No broker or provider | the API writes the send intention to MongoDB; the relay/worker are not needed to validate this feature |

Unit and race suites need **no service**: every seam this feature touches is behind an interface or a
mockery mock. Only the end-to-end scenario below wants MongoDB, and its integration suite self-skips
without it.

## 1. Automated gates (the primary evidence)

```bash
make fmt && make lint && make vet && make lint-specs   # required before a PR
make test-race                                          # the backend gate, race detector on
make mocks                                              # after the Service signature change
```

Expected: all green; `check_requirement_ids` resolves every `TFA-*` and every amended superseded ID.

Targeted, fastest loop while implementing:

```bash
go test -race -run 'Gate|Start|Refresh|Profile|Bool' ./internal/auth/... ./internal/app/... ./shared/env/...
go test -race ./internal/app/... -run Integration      # self-skips without MongoDB
```

## 2. Legacy parity — the switch off (`TFA-12`, `SC-004`)

With `AUTH_REQUIRE_EMAIL_VERIFICATION` unset (the default):

```bash
make dev-db          # terminal 1
make run             # terminal 2, no AUTH_REQUIRE_EMAIL_VERIFICATION
```

1. `POST /auth/register` with a fresh address → **`201`** with `access_token`, and both `__Host-`
   cookies set. *(the pre-existing contract)*
2. `POST /auth/login` with those credentials → **`200`** with an access token and cookies.
3. `POST /auth/refresh` with the cookies and the CSRF header → **`200`**, rotated cookies.
4. `POST /auth/me` with the access token → **`200`** with `email_verified: false`.

Any deviation here is a regression: the switch must change nothing while it is off.

## 3. The gated flow — the switch on (`SC-001`, `TFA-01`…`TFA-05`)

```bash
AUTH_REQUIRE_EMAIL_VERIFICATION=true make run
```

1. **Register** → **`201`** with `{challenge_id, expires_in, resend_after}` and **no `Set-Cookie`
   header**. Assert the absence explicitly (`curl -i`).
2. **Read the code** (dev only, since no worker runs): the API recorded a send intention —
   `mongosh legnaris --eval 'db.email_outbox.find(...).sort({created_at:-1}).limit(1)'` and read
   `payload.code`. *The code appearing here is expected and is the only place it is stored in the
   clear; it must never appear in a log line or any HTTP response.*
3. **Verify** → `POST /auth/challenge/verify {challenge_id, code}` → **`200`** with
   `access_token` and both cookies. This is the first session the account ever gets.
4. **Sign in again** → `POST /auth/login` → **`202`** with a challenge and no cookie; verify it → `200`
   and a session. Both purposes land on the same endpoint, chosen server-side.
5. **Wrong password** and **unknown address** → both **`401`** with the same code and message; neither
   starts a challenge.
6. **Identity read** → `GET /auth/me` → `200` with `email_verified: true` after step 3, `false`
   before it, and never an `email_verified_at` field.

## 4. The renewal gate (`TFA-06`, `SC-003`)

1. With the switch **off**, sign in once and keep the `__Host-` refresh cookie (this session has no
   verification timestamp only if the account never verified; use a fresh account and skip
   verification for this scenario).
2. Restart with `AUTH_REQUIRE_EMAIL_VERIFICATION=true`.
3. `POST /auth/refresh` with that cookie + CSRF header → **`401`**; then `mongosh` confirms the
   credential's family is revoked and no new row was minted.
4. Sign in and complete the code step (step 3 above), then refresh again → **`200`**.

## 5. The rollout switch (`TFA-07`, `TFA-16`)

```bash
AUTH_REQUIRE_EMAIL_VERIFICATION=maybe make run      # expect: boots, warns, gate stays off
env | grep AUTH_REQUIRE_EMAIL_VERIFICATION          # absent => default off
grep -c AUTH_REQUIRE_EMAIL_VERIFICATION deploy/.env.example deploy/README.md
make -n run | grep AUTH_REQUIRE_EMAIL_VERIFICATION  # the Makefile exports it
```

`deploy/compose_test.go` must still pass with the required set unchanged.

## 6. Traceability sweep

| Requirement | Evidence |
|---|---|
| `TFA-01`, `TFA-02`, `TFA-03` | §3.1–3.5 + `internal/app/auth_test.go` (both gate states) |
| `TFA-04` | §3.3–3.4 + the `017` integration suite, unchanged |
| `TFA-05` | §3.5 + the hasher-call-count assertion in `internal/auth/service_test.go` |
| `TFA-06` | §4 + `contracts/refresh-gate.contract.md`'s test list |
| `TFA-07`, `TFA-12` | §2, §5 |
| `TFA-08`, `TFA-09` | §3.6 + `internal/user/user_test.go`, `internal/app/auth_test.go` |
| `TFA-10` | every failure table in `contracts/*` asserted at the boundary |
| `TFA-11` | the `017` suites pass unchanged (`make test-race`) |
| `TFA-13` | the allowance-spent and store-failure cases in §3, plus the sign-in recovery path |
| `TFA-14` | `make lint-specs` clean after the three spec amendments |
| `TFA-15` | §1 |
| `TFA-16` | §5 + the English/secret-logging assertions |

## Expected end state

`make test-race` green with the gate exercised on **both** settings, `make lint-specs` clean, and the
three amended specs carrying a pointer to the superseding requirement. Nothing is pushed; the commits
stay local until the user authorizes otherwise.
