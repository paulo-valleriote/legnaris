# Contract: Browser auth session (cookie transport and session body)

**Requirements**: AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, AUTH-06, AUTH-07, AUTH-08, AUTH-21,
AUTH-22, AUTH-23, AUTH-24, AUTH-26, AUTH-27 | **Backing document**:
[data-model.md `Session` / `SessionResponse` / `Cookie descriptors` / `RefreshSession`](../data-model.md)
and [research.md `R1` / `R3` / `R4` / `R8` / `R9`](../research.md)

**Modules**: `internal/auth/service.go` (session material and the verification order),
`internal/auth/cookie.go` (name constants and attribute builders), `internal/app/auth.go` (the
controller: read the cookie, set/clear the cookies, serialise the session body),
`internal/app/router.go` (route mounting).

## Interface

| Endpoint | Reads | Succeeds with | Failure |
|---|---|---|---|
| `POST /auth/register` | JSON body | 201 + session body + refresh & CSRF cookies (`AUTH-02`) | 400 invalid, 409 duplicate (existing) |
| `POST /auth/login` | JSON body | 200 + session body + refresh & CSRF cookies (`AUTH-01`) | 401 invalid credentials (existing) |
| `POST /auth/refresh` | `__Host-legnaris_refresh` cookie + `X-CSRF-Token` header | 200 + session body + rotated refresh & CSRF cookies (`AUTH-03`) | 401 no/unknown/expired/rotated credential (`AUTH-06`, `AUTH-07`); 403 bad CSRF or origin |
| `POST /auth/logout` | `__Host-legnaris_refresh` cookie + `X-CSRF-Token` header | 204, family revoked, both cookies cleared (`AUTH-04`) | 204 no-op when no credential; 403 bad CSRF or origin |

```go
// internal/auth
type Session struct {
    AccessToken   string
    ExpiresIn     int
    RefreshToken  string // raw; cookie only
    RefreshMaxAge int    // seconds
    CSRFToken     string
    UserID        string
}

type SessionResponse struct {
    AccessToken string `json:"access_token"`
    TokenType   string `json:"token_type"`
    ExpiresIn   int    `json:"expires_in"`
}

type RefreshCommand struct{ RefreshToken, CSRFToken string }
type LogoutCommand  struct{ RefreshToken, CSRFToken string }

// internal/auth/cookie.go
const (
    RefreshCookieName = "__Host-legnaris_refresh"
    CSRFCookieName    = "__Host-legnaris_csrf"
)
func RefreshCookie(value string, maxAge int) *http.Cookie
func CSRFCookie(value string, maxAge int) *http.Cookie
func ClearRefreshCookie() *http.Cookie
func ClearCSRFCookie() *http.Cookie
```

## Rules

1. **Cookie transport only** (AUTH-05). The refresh credential is read from the cookie on `/auth/refresh`
   and `/auth/logout`; a value supplied in a request header or body is **ignored**. The retired
   `X-Refresh-Token` header is read on no route (AUTH-21) and its presence changes no outcome (AUTH-22).
2. **Attributes are contract-fixed** (AUTH-01, AUTH-09, AUTH-24). The refresh cookie is `HttpOnly`,
   `Secure`, `SameSite=Strict`, `Path=/`, no `Domain`, `Max-Age = AuthRefreshTTL` seconds; the CSRF
   cookie has the same attributes but is **not** `HttpOnly`. Either prefix is `__Host-`.
3. **No credential in the body** (AUTH-01, AUTH-02, AUTH-03, AUTH-08). Login, registration and refresh
   return the session body, which has no refresh field; the raw credential and the CSRF token never
   appear in a body, a log record, or a URL.
4. **Registration signs in** (AUTH-02). The account is created first (a duplicate email still answers
   409) and success then behaves exactly like login: session body plus both cookies.
5. **Rotation** (AUTH-03, AUTH-07, AUTH-12). A successful refresh consumes the presented credential,
   mints a successor in the same family, sets a new refresh **and** CSRF cookie, and returns a new
   access token. A credential that was already rotated is 401 and additionally revokes the family.
6. **Logout** (AUTH-04, AUTH-06). With a session, logout revokes the family, clears both cookies and
   answers 204. With no credential, or one that resolves to nothing, logout is a 204 no-op (nothing is
   changed, so no CSRF is owed).
7. **Verification order** (R3). Origin (403) → cookie presence (401 refresh / 204 logout) → credential
   resolution (401 unknown) → CSRF (403, session untouched) → revoked/expired (401) → effect. The
   origin and CSRF checks precede every mutation.
8. **Bearers untouched** (AUTH-26). `/sheets` and `/templates` keep `Authorization: Bearer` and never
   require a cookie; the access-token contract is unchanged.

## Obligations

1. **AUTH-01** — login sets the refresh cookie with the exact attribute set and returns the access token
   in the body with no refresh token.
2. **AUTH-02** — registration creates the account and establishes the session (body + cookie).
3. **AUTH-03** — refresh consumes, rotates, sets the new cookie and returns a new access token.
4. **AUTH-04** — logout revokes the family, clears the cookie and answers 204.
5. **AUTH-05** — the credential comes from the cookie only.
6. **AUTH-06** — absent cookie ⇒ refresh 401, logout 204.
7. **AUTH-07** — unknown/expired/rotated ⇒ 401; reuse revokes the family.
8. **AUTH-08** — the credential appears in no body, log or URL.
9. **AUTH-21/AUTH-22** — the native header is unread and inert.
10. **AUTH-24** — names/attributes fixed; lifetime from `AuthRefreshTTL`.

## Test obligations

- **Happy path**: login, registration and refresh each set a refresh cookie with the full attribute set,
  set the CSRF cookie, and return a body carrying an access token and no refresh token; logout clears
  both cookies and revokes the family; refresh changes both cookie values.
- **Edge**: the refresh cookie absent ⇒ refresh 401, logout 204; a credential supplied in a header or
  body is ignored; a request setting `X-Refresh-Token` is resolved solely by the cookie; a dropped
  `HttpOnly`/`Secure`/`SameSite=Strict`/`Path=/`/`__Host-` attribute fails the suite; a cleared cookie
  carries the same name and a `Max-Age` of 0.
- **Error**: unknown, expired and already-rotated credentials ⇒ 401; a rejected request leaves the
  session untouched; the body of a failure never contains a credential.

## Traceability

- Transport and body: AUTH-01, AUTH-02, AUTH-03, AUTH-05, AUTH-08, AUTH-24, AUTH-26.
- Session lifecycle: AUTH-04, AUTH-06, AUTH-07.
- Retirement: AUTH-21, AUTH-22.
- Cross-cutting: AUTH-23, AUTH-27.
