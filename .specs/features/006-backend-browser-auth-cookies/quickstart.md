# Quickstart: validate the browser auth cookie contract

**Feature**: `.specs/features/006-backend-browser-auth-cookies/` | **Date**: 2026-10-16

Runnable scenarios that prove the feature works end to end. Each lists what to run and the outcome that
means it passed. Prerequisites: the Go toolchain (`go version` → 1.26.x). The unit scenarios need **no**
MongoDB; the integration scenarios self-skip unless MongoDB is reachable, so start it once with `make
dev-db` (then stop it with `docker stop legnaris-mongo`). `JWT_SECRET` must be set for anything that
boots the app (`export JWT_SECRET=$(openssl rand -base64 32)`); `CORS_ALLOWED_ORIGINS` defaults to
`http://localhost:3000,http://localhost:5173`, so `http://localhost:5173` is the allowlisted origin the
scenarios use.

Paths referenced here are defined in [plan.md](./plan.md) (source layout),
[data-model.md](./data-model.md) (shapes) and the four documents under [contracts/](./contracts/).

---

## 1. The pure rules pass with no database

```bash
go test ./internal/auth/... ./internal/platform/ratelimit/... ./shared/apperr/... ./shared/logger/... ./shared/env/...
```

**Outcome**: the CSRF token round-trips and refuses tampered/foreign/malformed values; the limiter
admits under the cap, refuses past it, frees the window and drops a key; the new `RATE_LIMITED` code maps
to 429; the redaction covers the new cookie names; the new defaults fall back when unset. No MongoDB is
required — this is the state CI runs in.

---

## 2. Login and registration set the cookie and the session body

```bash
go test ./internal/app/ -run 'TestIntegration_Auth(Register|Login)$' -v
```

**Outcome**: `POST /auth/login` (and `POST /auth/register`, which now signs the browser in) answers 2xx
with a body carrying `access_token`/`token_type`/`expires_in` and **no** `refresh_token`, and a
`Set-Cookie` for `__Host-legnaris_refresh` with `HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/`, no
`Domain`, `Max-Age` = the configured refresh lifetime, plus `__Host-legnaris_csrf` (not `HttpOnly`)
(AUTH-01, AUTH-02, AUTH-09, AUTH-24, SC-001).

---

## 3. Refresh rotates the credential and the CSRF token

```bash
go test ./internal/app/ -run TestIntegration_AuthRefreshRotation -v
```

**Outcome**: `POST /auth/refresh` presenting the cookie and the matching `X-CSRF-Token` consumes the
credential, sets a **new** refresh cookie value **and** a new CSRF cookie value, and returns a new access
token in a body with no refresh token (AUTH-03, AUTH-12); presenting the already-rotated credential
again answers 401 and revokes the whole family, so its successor is dead too (AUTH-07).

---

## 4. Logout clears the cookies and revokes the family

```bash
go test ./internal/app/ -run TestIntegration_AuthLogout -v
```

**Outcome**: `POST /auth/logout` with the cookie and CSRF answers 204, clears both cookies (same names,
empty value, `Max-Age=0`) and revokes the family, so a later refresh with the same credential is 401
(AUTH-04, AUTH-06); logout with no cookie is also 204 and changes nothing.

---

## 5. CSRF is required and a refusal leaves the session untouched

```bash
go test ./internal/app/ -run 'TestAuthController_Refresh|TestAuthController_Logout|TestOrigin' -v
```

**Outcome**: refresh or logout with a missing, malformed or foreign `X-CSRF-Token` answers 403
(`FORBIDDEN`) and neither rotates nor revokes; refresh with the cookie present but CSRF absent is 403,
while refresh with no cookie at all is 401 (AUTH-06, AUTH-10, AUTH-11, SC-002).

---

## 6. A cookie-authenticated call with no allowed origin is refused

```bash
go test ./internal/app/ -run TestOrigin -v
```

**Outcome**: a refresh or logout whose `Origin` is absent, or present but outside the allowlist (or whose
`Referer` is), answers 403 **before** any credential work; a valid `Origin` lets the call reach the CSRF
and credential checks (AUTH-13, SC-002).

---

## 7. CORS is credentialed and exact-origin

```bash
go test ./internal/app/ ./internal/platform/cors/ -run 'CORS|Preflight' -v
```

**Outcome**: a preflight from the allowlisted origin answers with that exact
`Access-Control-Allow-Origin`, `Access-Control-Allow-Credentials: true`, `Vary: Origin` and
`X-CSRF-Token` advertised, and **not** `X-Refresh-Token`; a preflight from an unlisted origin advertises
no allow-origin and no allow-headers, and no response ever carries `*` with credentials (AUTH-14,
AUTH-15, AUTH-16, AUTH-21, SC-003).

---

## 8. Repeated auth attempts are limited

```bash
go test ./internal/app/ -run TestRateLimit -v
```

**Outcome**: after the configured number of attempts from one key, the next answers **429** with a
`Retry-After` header, the service mock records **zero** calls (no credential verified, no token issued),
a second address or account is unaffected, and once the window frees the key is admitted again; the block
increments `legnaris_auth_rate_limited_total` (AUTH-17…AUTH-20, SC-004).

---

## 9. The native transport is gone

```bash
! grep -rn "X-Refresh-Token" internal/ && \
go test ./internal/app/ -run 'TestAuthController|TestPreflight' -v
```

**Outcome**: no Go source reads the retired header and no allowlist advertises it; a request that sets
`X-Refresh-Token` has its outcome determined solely by the cookie, exactly as if the header were absent
(AUTH-21, AUTH-22, SC-005).

---

## 10. A manual walkthrough against a running API

```bash
make dev-db
export JWT_SECRET=$(openssl rand -base64 32)
export CORS_ALLOWED_ORIGINS=http://localhost:5173
make run &                                   # API on :8080
jar=$(mktemp)
origin=http://localhost:5173

# Register signs the browser in; the jar captures both cookies.
curl -sS -c "$jar" -H "Origin: $origin" -H 'Content-Type: application/json' \
  -d '{"email":"player@example.com","username":"player","password":"sup3r-secret"}' \
  http://localhost:8080/auth/register
# → 201 {"data":{"access_token":"…","token_type":"Bearer","expires_in":900}} and no refresh_token

csrf=$(awk '$6=="__Host-legnaris_csrf"{print $7}' "$jar")

# Refresh sends the cookie + the CSRF header; the jar is rewritten with rotated values.
curl -sS -b "$jar" -c "$jar" -H "Origin: $origin" -H "X-CSRF-Token: $csrf" \
  -X POST http://localhost:8080/auth/refresh
# → 200 with a new access_token; both cookie values changed

# Without the CSRF header the same call is refused and the session is untouched.
curl -sS -o /dev/null -w '%{http_code}\n' -b "$jar" -H "Origin: $origin" \
  -X POST http://localhost:8080/auth/refresh        # → 403

# Logout clears the cookies.
csrf=$(awk '$6=="__Host-legnaris_csrf"{print $7}' "$jar")
curl -sS -b "$jar" -c "$jar" -H "Origin: $origin" -H "X-CSRF-Token: $csrf" \
  -o /dev/null -w '%{http_code}\n' -X POST http://localhost:8080/auth/logout   # → 204
```

**Outcome**: the response bodies carry an access token and never a refresh token, the jar holds an
`HttpOnly` `__Host-legnaris_refresh` and a readable `__Host-legnaris_csrf`, a missing CSRF header is 403,
and logout is 204 with the cookies removed. (Kill the API and `docker stop legnaris-mongo` afterwards.)

---

## 11. The gates stay green

```bash
make fmt && make lint && make vet && make lint-specs && make test-race
```

**Outcome**: formatting, lint, vet, the spec lint (every `AUTH-*` citation resolves to `spec.md`, no
vacuous assertion) and the race-detector suite all pass. No new dependency is declared, so the security
gates see nothing new to vet.

---

## Traceability

| Scenario | Requirements |
|---|---|
| 1 | AUTH-11, AUTH-19, AUTH-20, AUTH-23, AUTH-25 |
| 2 | AUTH-01, AUTH-02, AUTH-09, AUTH-24, SC-001 |
| 3 | AUTH-03, AUTH-07, AUTH-12 |
| 4 | AUTH-04, AUTH-06 |
| 5 | AUTH-06, AUTH-10, AUTH-11, SC-002 |
| 6 | AUTH-13, SC-002 |
| 7 | AUTH-14, AUTH-15, AUTH-16, AUTH-21, SC-003 |
| 8 | AUTH-17, AUTH-18, AUTH-19, AUTH-20, SC-004 |
| 9 | AUTH-21, AUTH-22, SC-005 |
| 10 | AUTH-01…AUTH-04, AUTH-08, AUTH-10 |
| 11 | AUTH-23, AUTH-27, AUTH-28 |

## Validation outcome

Not yet run — this feature is planned, not implemented. The outcome table is filled during
implementation, as F5's `quickstart.md` shows.
