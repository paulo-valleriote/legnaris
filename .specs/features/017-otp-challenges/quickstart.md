# Quickstart: Email Code Challenges

**Feature**: `.specs/features/017-otp-challenges/` | **Date**: 2026-10-20

A runnable validation guide for the code step: it proves the challenge record, the code, the two
endpoints and their bounds behave as `spec.md` requires. It is a validation guide, not an
implementation sketch — the schema and the interfaces live in
[data-model.md](./data-model.md) and [contracts/](./contracts/).

## Prerequisites

- Go 1.26.4 (`go version`).
- A local MongoDB for the self-skipping integration suites: `make dev-db` (the repository's dev
  stack), or any standalone `mongo:7.0` reachable through `DATABASE_URI`.
- The environment the app already needs, plus the one new required variable. A root `.env` with at
  least:

  ```dotenv
  DATABASE_URI=mongodb://localhost:27017
  JWT_SECRET=<any development value>
  EMAIL_CODE_SECRET=<a development value of at least 32 bytes, distinct from JWT_SECRET>
  ```

  `make run` reads the root `.env`; `make dev-db` starts the database if none is running.

## 1. Boot and the required secret

| Command | Expected |
|---|---|
| `make run` with `EMAIL_CODE_SECRET` set | the API starts; no route error; `GET /health` answers 200 |
| `make run` with `EMAIL_CODE_SECRET` unset, blank, or 31 bytes | the process exits with a typed error before serving; no listener is bound (`OTP-18`, `SC-008`) |
| `make lint-specs` | clean: every `OTP-*` citation resolves (`OTP-20`) |

## 2. The unit and race suites (no services)

```bash
go test -race ./internal/auth/... ./internal/app/... ./internal/email/... ./shared/...
```

Expected: green, with the challenge suites exercising, edge case first — a replayed code, two
concurrent consumptions, the fifth wrong guess, expiry at the exact instant, the cool-down boundary,
the ceiling clamp, an unknown/consumed/expired identity, a malformed identity, digest binding across
accounts and purposes, a weak secret, the cap boundary, and a store failure mid-flow — then the happy
paths. `internal/auth/code_test.go` generates 10,000 codes and asserts the six-digit shape with a
leading zero. The whole run needs no MongoDB (those suites self-skip) and no broker (`OTP-20`,
`SC-001`, `SC-007`).

## 3. The challenge flow end to end (MongoDB running)

```bash
go test -run TestChallenge -v ./internal/app
```

The integration suite drives the interim trigger of this stage, because no public route creates a
challenge until registration and sign-in are wired (stage 4):

1. **Issue** — call `auth.Service.IssueChallenge` for a seeded account with purpose `verify_email`;
   assert a `challenge_id`, the configured `expires_in` and `resend_after`, a stored digest (never
   the code), and exactly one pending row in `email_outbox` whose payload carries the code
   (`OTP-01`, `OTP-02`, `OTP-12`).
2. **Verify** — `POST /auth/challenge/verify` with that identity and the issued code; assert 200, the
   session body, the refresh and CSRF cookies, and that the account's verification timestamp is now
   set (`OTP-07`).
3. **Replay** — the same call again; assert 401 `OTP_INVALID` and that nothing was issued
   (`OTP-04`, `OTP-08`).
4. **Resend** — issue again, `POST /auth/challenge/resend` twice in a row; assert both answer 202
   with the same body, exactly one new intention was recorded, and the code still verifies
   (`OTP-10`, `OTP-11`).
5. **Expire** — issue with the injected clock, step past the deadline, verify; assert the uniform 401
   and that `auth_otp_verify_total{result="expired"}` moved (`OTP-06`, `OTP-16`).
6. **Exhaust** — issue, submit five wrong codes, then the right one; assert every attempt answered the
   same 401 and the challenge never consumed (`OTP-05`, `OTP-08`).
7. **Cap** — seed the day's intentions up to `AUTH_OTP_DAILY_SEND_CAP`, then issue; assert 503
   `EMAIL_UNAVAILABLE` with `Retry-After` and no new intention (`OTP-13`).

## 4. The disclosure sweep

```bash
go test -run 'Challenge|Otp' -v ./internal/app ./internal/auth
```

Expected: green, including the assertions that no 6-digit code and no unmasked address appear in any
response body, header, log line or error message produced by the suites, and that the five
verification failures answer byte-identical 401 bodies (`OTP-08`, `OTP-15`, `SC-002`, `SC-003`).

## 5. The store invariants (MongoDB running)

```bash
go test -v ./internal/auth -run ChallengeRepository
go test -v ./shared/db
```

Expected: green — the unique partial index rejects a second active challenge for one account and
purpose and admits one after consumption; two concurrent claims admit exactly one winner; a second
`Consume` reports `false`; `Extend` suppresses a resend inside the cool-down and clamps to the
ceiling; the TTL index and the `email_outbox.created_at` index exist with the documented options
(`OTP-01`, `OTP-04`, `OTP-06`, `OTP-10`, `OTP-13`).

## 6. Full gate

```bash
make fmt && make lint && make vet && make lint-specs && make test-race
```

Expected: clean, no sleeps in the boundary suites, no service required beyond the self-skipping
MongoDB suites.

## What this feature does not validate

- Registration or sign-in issuing a challenge and ceasing to establish a session; renewal refusing an
  unverified account; the verification state on any response — all stage 4.
- The client's handling of `OTP_INVALID`/`EMAIL_UNAVAILABLE` (copy, state machine) — stage 5.
- The worker that renders and sends the email, and the broker that carries it — the preceding
  feature's pipeline owns delivery; this feature only records the intention.
