# Contract: CSRF token (stateless, session-bound)

**Requirements**: AUTH-09, AUTH-10, AUTH-11, AUTH-12 | **Backing document**:
[data-model.md `CSRFToken` / `Cookie descriptors`](../data-model.md) and
[research.md `R2` / `R3` / `R4`](../research.md)

**Modules**: `internal/auth/csrf.go` (the manager), `internal/auth/service.go` (issue on every session
establishment/rotation and verify on refresh/logout), `internal/app/auth.go` (the CSRF cookie and the
`X-CSRF-Token` header), `internal/platform/cors/cors.go` (advertise the header on preflight).

## Interface

```go
// internal/auth
type CSRFTokenManager interface {
    Issue(familyID string) (string, error)
    Verify(token, familyID string) error // nil when valid, a typed/ sentinel error otherwise
}

func NewCSRFTokenManager(secret []byte) CSRFTokenManager
```

Token wire format: `base64url( nonce ‖ HMAC-SHA256(key, "csrf/v1" ‖ 0x00 ‖ familyID ‖ 0x00 ‖ nonce) )`,
`nonce` = 16 random bytes, `key = HMAC-SHA256(secret, "legnaris/csrf/v1")` where `secret` is the JWT
secret.

## Rules

1. **Issued with the session** (AUTH-09, AUTH-12). Every session establishment or rotation — login,
   registration and every successful refresh — issues a fresh token (a new nonce) and sets it in the
   script-readable `__Host-legnaris_csrf` cookie; the client echoes it in `X-CSRF-Token`.
2. **Script-readable, not HttpOnly** (AUTH-09). The CSRF cookie carries `Secure`, `SameSite=Strict`,
   `Path=/`, no `Domain`, and is **not** `HttpOnly`, so the page can read and echo it; the refresh
   cookie stays `HttpOnly`.
3. **Server-bound, not equality** (AUTH-11). The guard accepts the request only when the header verifies
   against the **resolved session's family id** with the server secret; equality between the header and
   the CSRF cookie, a missing header, a malformed value, a tampered MAC or a token minted for another
   session are each refused (AUTH-10). The MAC comparison is constant-time (`hmac.Equal`).
4. **Refused ⇒ untouched** (AUTH-10, R3). A refused call answers 403 and leaves the session unmodified;
   the CSRF check runs after the cookie-presence check (so an absent cookie is 401, not 403) and before
   the reuse/expiry branch (so an invalid CSRF call never revokes a family).
5. **Rotated and cleared with the session** (AUTH-12). A rotation issues a new CSRF token together with
   the new refresh cookie; logout clears the CSRF cookie alongside the refresh cookie.
6. **Not a credential in transit** (AUTH-08, AUTH-25). The CSRF token is not a password and is not
   logged; the signing key (the "CSRF secret") never appears in a body, a log or a URL.

## Obligations

1. **AUTH-09** — login/registration/refresh set the CSRF cookie; the client echoes `X-CSRF-Token`.
2. **AUTH-10** — refresh/logout refuse a missing, malformed or unbound token with 403, session
   untouched.
3. **AUTH-11** — a token issued for one session is refused for another; equality alone never passes.
4. **AUTH-12** — rotation rotates the CSRF token; logout clears it.

## Test obligations

- **Happy path**: `Issue(family)` then `Verify(token, family)` succeeds; a session established and
  rotated carries two different CSRF values.
- **Edge**: an empty/`null` token, a non-base64 value, a truncated value and a value with a flipped byte
  all fail `Verify`; a token issued for `familyA` fails `Verify` against `familyB`; the same value
  presented twice verifies twice (stateless, not single-use).
- **Error**: at the HTTP boundary, refresh and logout with a missing, malformed or foreign
  `X-CSRF-Token` answer 403 with `FORBIDDEN`; a refused call leaves the session and its cookies
  unchanged; refresh with the cookie present but CSRF absent answers 403, while refresh with no cookie
  at all answers 401.
- **Security**: the token and the signing key appear in no response body and no log record.

## Traceability

- Issuance and rotation: AUTH-09, AUTH-12.
- Verification and refusal: AUTH-10, AUTH-11.
- Secret handling: AUTH-08, AUTH-25.
