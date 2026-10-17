# Contract: Auth API (the four session operations)

**Requirements**: WAS-01, WAS-02, WAS-06, WAS-07, WAS-18, WAS-30, WAS-32 | **Backing document**:
[data-model.md `SessionBody` / `CSRFToken`](../data-model.md) and
[research.md `R1` / `R2` / `R5` / `R7`](../research.md); the consumed backend contract is
`006-backend-browser-auth-cookies/contracts/auth-session.contract.md` and `csrf.contract.md`.

**Modules**: `web/src/api/request.ts` (the shared caller both clients build on), `web/src/api/auth_api.ts`
(the four operations), `web/src/api/csrf.ts` (the cookie reader), `web/src/api/transport.ts` (the widened
`credentials` posture and the `DELETE` method).

## Interface

| Operation | Request | Success | Failure |
|---|---|---|---|
| `register(input)` | `POST /auth/register`, `credentials: 'include'`, JSON `{email, username, password}` | `201` + `SessionBody` | `400` validation, `409` duplicate — mapped, not surfaced raw (WAS-07) |
| `login(input)` | `POST /auth/login`, `credentials: 'include'`, JSON `{email, password}` | `200` + `SessionBody` | `401` invalid, `429` rate limited (WAS-07, WAS-30) |
| `refresh()` | `POST /auth/refresh`, `credentials: 'include'`, `X-CSRF-Token: <read>` | `200` + `SessionBody` | `401` rejected, `403` bad CSRF, `429` (WAS-18, WAS-30) |
| `logout()` | `POST /auth/logout`, `credentials: 'include'`, `X-CSRF-Token: <read>` | `204`, empty body | `403` bad CSRF (best-effort at the caller) |

```ts
export interface SessionBody {
  readonly accessToken: string;
  readonly tokenType: string;
  readonly expiresIn: number;
}

export interface AuthApi {
  register(input: { email: string; username: string; password: string }): Promise<SessionBody>;
  login(input: { email: string; password: string }): Promise<SessionBody>;
  refresh(): Promise<SessionBody>;
  logout(): Promise<void>;
}
```

## Rules

1. **Cookie transport only** (WAS-06). All four calls send `credentials: 'include'` so the browser
   attaches the `HttpOnly` refresh cookie on the auth routes; no call reads the cookie itself and no
   credential appears in a URL or a body beyond the submitted email/username/password (WAS-32).
2. **CSRF on the state-changing routes** (WAS-18). `refresh` and `logout` echo the value of the
   script-readable `__Host-legnaris_csrf` cookie in `X-CSRF-Token`, resolved at request time; an
   absent cookie sends no header and the server's `403` is mapped (`R7`). `register` and `login`
   carry no CSRF header (the server issues the token with the session).
3. **Registration signs in** (WAS-02). `register` returns a `SessionBody` and the caller establishes
   the session from it; the client issues **no** follow-up `login`.
4. **Body decode is defensive** (WAS-30). A `SessionBody` is decoded from `{data:{access_token,
   token_type, expires_in}}`; a missing or mistyped field is a `DefensiveParseFailure`. A `204`
   logout decodes through the no-content path and returns `void`.
5. **Failures are classified, never raw** (WAS-07, WAS-30). A non-2xx becomes a `DomainFailure` with
   its interface-facing kind through the F2 mapper; the server message is never rendered (`WAS-10`).
6. **One helper, one transport** (R1). Every call goes through the shared caller over the injected
   `Transport`; no operation builds its own request. The `AuthApi` is built over the **raw** transport
   so `/auth/*` can never carry a Bearer or trigger a refresh (WAS-16).

## Obligations

1. **WAS-01** — `login`/`refresh` yield an access token the caller holds in memory only.
2. **WAS-02** — `register` establishes the session with one request.
3. **WAS-06** — every auth call attaches the cookie; no credential leaks to a URL or a log.
4. **WAS-18** — `refresh`/`logout` echo the CSRF cookie; the `HttpOnly` cookie is never read.
5. **WAS-30** — `401`/`403`/`429` map to their kinds; an unclassified code stays generic.

## Test obligations

- **Happy path**: `login` sends one `credentials: 'include'` request and decodes the session body;
  `register` sends one request and no follow-up; `refresh`/`logout` echo the CSRF header; `logout`
  accepts a `204` with no body.
- **Edge**: an absent CSRF cookie sends no header and maps `403`; a malformed session body is a
  `DefensiveParseFailure`; an empty logout body is not a parse failure.
- **Error**: `401` maps to `unauthorized`, `403` to `forbidden`, `429` to `rateLimited`, an unmapped
  code to `unknown`; a `409` on registration renders the generic copy, never the server message.
- **Security**: no request carries a Bearer header; no credential appears in a URL or a log record.

## Traceability

- Operations and posture: WAS-01, WAS-02, WAS-06.
- CSRF echo: WAS-18.
- Classification: WAS-07, WAS-30, WAS-32.
