# Contract: Credentialed CORS and the origin guard

**Requirements**: AUTH-13, AUTH-14, AUTH-15, AUTH-16, AUTH-21 | **Backing document**:
[data-model.md `AllowedOriginSet`](../data-model.md) and [research.md `R5` / `R6` / `R9`](../research.md)

**Modules**: `internal/platform/cors/cors.go` (the response headers), `internal/app/origin.go` (the
server-side guard), `internal/app/router.go` (route mounting).

## Interface

```go
// internal/platform/cors — extended config
middleware.CORSConfig{
    AllowOrigins:     slices.Clone(origins), // exact origins, never "*"
    AllowCredentials: true,                  // echoed only for an allowlisted origin
    AllowHeaders:     []string{ContentType, Authorization, XRequestID, csrfHeader}, // X-Refresh-Token gone
    ExposeHeaders:    []string{XRequestID},
}

// internal/app/origin.go — the guard on /auth/refresh and /auth/logout
func requireAllowedOrigin(origins []string) echo.MiddlewareFunc // 403 when Origin/Referer is absent or unmatched
```

## Rules

1. **Exact credentialed origin** (AUTH-14). A cross-origin request from an allowlisted origin is answered
   with that exact origin in `Access-Control-Allow-Origin`, `Access-Control-Allow-Credentials: true`,
   and `Vary: Origin`. Echo's default origin function matches case-insensitively and echoes the
   allowlist entry; it never emits `*`.
2. **Unlisted origin gets nothing** (AUTH-15). A request from an origin outside the allowlist receives
   no `Access-Control-Allow-Origin`, and `*` is never combined with credentials (Echo refuses to build
   that config at construction).
3. **Preflight advertises the CSRF header** (AUTH-16). The preflight for the auth routes advertises
   `X-CSRF-Token` and **does not** advertise the retired `X-Refresh-Token` (AUTH-21); from an unlisted
   origin it advertises no allow-origin and no allow-headers.
4. **Server-side origin guard, fail closed** (AUTH-13). `/auth/refresh` and `/auth/logout` reject with
   403 (`FORBIDDEN`, `app.MsgOriginNotAllowed`) when `Origin` — falling back to the origin of
   `Referer` — is absent or not in `CORS_ALLOWED_ORIGINS`, before any credential work. Matching is
   exact; one allowlist governs both the CORS headers and the guard.
5. **Scope** (AUTH-26). The guard protects only the two cookie-authenticated state-changing auth routes;
   `/sheets` and `/templates` (Bearer, no cookie) are not guarded and may legitimately omit `Origin`.

## Obligations

1. **AUTH-13** — a missing or disallowed `Origin`/`Referer` on refresh/logout is 403 before any
   credential work.
2. **AUTH-14** — an allowlisted origin is echoed exactly with credentials and `Vary: Origin`.
3. **AUTH-15** — an unlisted origin gets no allow-origin header and never a wildcard.
4. **AUTH-16** — the preflight advertises `X-CSRF-Token` and no longer the native refresh header.

## Test obligations

- **Happy path**: a preflight from the allowlisted origin answers 204 with the exact origin, credentials
  and `X-CSRF-Token` advertised; a real cross-origin request likewise carries the exact origin.
- **Edge**: a preflight from an unlisted origin advertises no allow-origin and no allow-headers; a
  request with no `Origin` to a bearer route is not rejected by the guard.
- **Error**: refresh/logout with `Origin` absent ⇒ 403; with `Origin` unlisted ⇒ 403 and no
  allow-origin header; with `Referer` disallowed ⇒ 403; with a valid `Origin` ⇒ the guard lets it
  through to the CSRF/credential checks.
- **Security**: no response to an allowlisted origin carries `Access-Control-Allow-Origin: *`; the
  retired `X-Refresh-Token` name appears in no allowlist.

## Traceability

- Response headers: AUTH-14, AUTH-15, AUTH-16.
- Server-side guard: AUTH-13.
- Retirement: AUTH-21.
