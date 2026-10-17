# Phase 1 — Data Model: Web Auth Session

The feature holds one in-memory session, decorates the F2 transport, and extends the F5 local store
with one clear-all operation; it re-declares none of the wire models. The payload it reads and writes
is the F6 session body (`{ data: { access_token, token_type, expires_in } }`) and the F2 `Sheet`
model, and the store it clears is the F5 `SheetStore`. Every field traces to a functional requirement
in [spec.md](./spec.md).

Naming: TypeScript types use `PascalCase`, values and functions `camelCase`; the backend's
`snake_case` keys never appear outside the F2 decoders. No in-memory or local value holds a
credential that outlives the session (`WAS-01`, `WAS-32`).

---

## AuthSession

The single session the gate, the screens and the transport read; held for the app's lifetime and
never persisted (`WAS-01`…`WAS-24`).

| Field | Type | Rules |
|---|---|---|
| `accessToken` | `string \| null` | the server's short-lived token; **memory only**, never written to any store (`WAS-01`, `WAS-32`) |
| `version` | `number` | the session generation; incremented at login, registration, logout and forced logout, and **never** when a refresh applies a rotated token (`WAS-17`, `AD-010`) |
| `state` | `'restoring' \| 'unauthenticated' \| 'authenticating' \| 'authenticated'` | what the gate renders (`WAS-03`, `WAS-08`) |
| `logoutInFlight` | `Promise<void> \| undefined` | the shared teardown so a second activation issues no second `/auth/logout` (`WAS-23`) |
| `refreshInFlight` | `Promise<string \| null> \| undefined` | the single in-flight refresh every concurrent `401` shares (`WAS-13`); lives on the session so the decorator and the restore path cannot each start one |

**Derived invariants**

| Derivation | Rule |
|---|---|
| `isAuthenticated` | `state === 'authenticated'`; the gate renders the route tree only then (`WAS-08`) |
| `isBusy` | the screen disables the primary action while `state === 'authenticating'` (`WAS-09`) |
| `authHeader` | `accessToken === null ? undefined : 'Bearer ' + accessToken`; attached only to a protected path (`WAS-11`, `WAS-16`) |

**Transitions**

| From | Trigger | To | Effect |
|---|---|---|---|
| initial | mount | `restoring` | one silent refresh (`WAS-03`) |
| `restoring` | refresh ok | `authenticated` | hold the token, **no** version bump (`WAS-03`) |
| `restoring` | refresh `401` | `unauthenticated` | no session (`WAS-04`) |
| `restoring` | refresh transient | `unauthenticated` | keep the session-local data (`WAS-05`) |
| `unauthenticated` | submit | `authenticating` | the auth call in flight (`WAS-09`) |
| `authenticating` | ok | `authenticated` | version bump **before** applying (`WAS-01`, `WAS-02`) |
| `authenticating` | failure | `unauthenticated` | rethrow the mapped error, keep nothing (`WAS-07`) |
| `authenticated` | logout | `unauthenticated` | version bump, drop token, sweep (`WAS-19`) |
| `authenticated` | refresh rejected | `unauthenticated` | forced logout: version bump, drop token, sweep (`WAS-14`, `WAS-20`) |

---

## SessionBody

The decoded `POST /auth/{register,login,refresh}` payload — the only credential the client ever sees,
and the only one it holds in memory (`WAS-01`, `WAS-06`).

| Field | Type | Rules |
|---|---|---|
| `accessToken` | `string` | decoded from `access_token`; the raw token never reaches a log or a URL (`WAS-06`, `WAS-32`) |
| `tokenType` | `string` | decoded from `token_type` (informational; the client always sends `Bearer`) |
| `expiresIn` | `number` | decoded from `expires_in` (informational; the client does not schedule an expiry — it refreshes on `401`) |

**Invariants**: there is no `refresh_token` field (F6 keeps it out of the body); a body missing or
mistyping a required field is a `DefensiveParseFailure`, not a partial session (`WAS-02`).

---

## AuthenticatedTransport

The decorator around the F2 `Transport` that owns the Bearer posture and the silent-refresh lifecycle
(`WAS-11`…`WAS-17`).

| Step | Rule |
|---|---|
| classify | a path is public when it is `/health` or starts with `/auth/` (`WAS-16`) |
| attach | a protected request gets `Authorization: Bearer <access>` when a token is held (`WAS-11`) |
| send | capture `version`, then send through the inner transport |
| pass through | a public request or a non-`401` response returns unchanged (`WAS-11`, `WAS-16`) |
| guard | a `401` whose session version changed in the interim returns unchanged — no refresh, no replay (`WAS-17`) |
| refresh | exactly one shared refresh across concurrent `401`s (`WAS-13`) |
| replay | a token and an unchanged version ⇒ send the request once more with the new Bearer (`WAS-12`) |
| forced logout | a rejected refresh (`null`) and an unchanged version ⇒ `onUnauthorized()`, return the original `401` (`WAS-14`) |
| transient | a refresh that throws propagates; the session and its data survive (`WAS-15`) |

**Invariants**: the decorator never attaches a Bearer to a public path and never starts two refreshes
(`WAS-13`, `WAS-16`); it replays at most once per refused request (`WAS-12`); it never ends a session
that has been replaced (`WAS-17`).

---

## CSRFToken

The value read from the script-readable cookie and echoed on the two state-changing auth routes
(`WAS-18`).

| Field | Type | Rules |
|---|---|---|
| name | `'__Host-legnaris_csrf'` | the contract-fixed cookie name (`csrf.contract.md`) |
| reader | `(cookieString: string) => string \| undefined` | total: an absent, empty or malformed cookie yields `undefined`, never a throw |
| echo | `X-CSRF-Token` header | set on `refresh` and `logout` only, resolved at request time; absent ⇒ no header (`WAS-18`) |

**Invariants**: the `HttpOnly` refresh cookie is never read; the CSRF value is never stored, logged or
placed in a URL (`WAS-18`, `WAS-32`).

---

## SessionBoundary and SessionLocalData

The definitive end of a session and the local records it owns (`WAS-19`…`WAS-24`).

| Concept | Definition |
|---|---|
| definitive end | login, registration, logout and forced logout bump the session version; a refresh applying a token does not (`WAS-17`) |
| session-local data | every record in the F5 `SheetStore` — the cached sheets and the drafts of the current session (`WAS-21`) |
| sweep | `clearSessionData(store)` runs `bestEffort(() => store.clearAll())`, so a failing store never blocks the end (`WAS-21`) |
| transients | a refresh that fails transiently is **not** a boundary; the records survive (`WAS-05`, `WAS-15`, `SC-006`) |

**Transitions**

| Event | Version | Token | Local data | Gate |
|---|---|---|---|---|
| login / register | `+1` | set | unchanged (nothing cached yet) | `authenticated` (`WAS-01`, `WAS-02`) |
| refresh applied | unchanged | replaced | unchanged | unchanged (`WAS-17`) |
| logout | `+1` | cleared | swept | `unauthenticated` (`WAS-19`) |
| forced logout | `+1` | cleared | swept | `unauthenticated` (`WAS-14`, `WAS-20`) |
| transient refresh | unchanged | unchanged | **kept** | `unauthenticated` (`WAS-05`) |

**Invariants**: after a boundary, the next session (`+1` again) can render no cached sheet and no
draft of the finished session — every record was removed, not merely hidden (`WAS-22`, `SC-005`).

---

## YourSheet

The resolve situation of the caller's single owner-scoped sheet, reached from the session-only
navigation entry (`WAS-25`…`WAS-29`).

| Kind | Fields | Rules |
|---|---|---|
| loading | — | shown until `GET /sheets` settles (`WAS-25`) |
| empty | — | the server answered `404`/`NOT_FOUND`; the designed empty line, distinct from loading and failure (`WAS-26`) |
| sheet | `sheet: Sheet` | the caller's sheet; an Open action navigates to `/sheets/:id`; a Delete control opens the confirmation (`WAS-25`, `WAS-28`) |
| failure | `message: string` | the mapped copy for any non-`NOT_FOUND` outcome, with a Retry that re-requests exactly once (`WAS-27`); an undecodable payload takes this same path |

**Delete transitions**

| Step | Rule |
|---|---|
| request | activating Delete opens the confirmation dialog stating the exact consequence (`WAS-28`) |
| cancel | closes the dialog and issues **zero** requests; the sheet stays (`WAS-28`, `SC-007`) |
| confirm | issues exactly **one** `DELETE /sheets/:id` (`WAS-28`, `SC-007`) |
| success | evicts the sheet and its draft from the local store, then shows `empty` (`WAS-29`) |
| failure | keeps the sheet and shows the mapped copy (`WAS-29`) |

**Invariants**: the server, never the local cache, decides the resolve kind (`WAS-25`); the delete is
the only request the surface issues on confirm, and none on cancel (`WAS-28`).

---

## SheetStore (port) — extension

The F5 persistence boundary gains exactly one operation for this feature (`WAS-21`).

| Operation | Input | Output | Rules |
|---|---|---|---|
| `clearAll()` | — | `Promise<void>` | deletes every `sheets` and `drafts` record; best-effort at the call site, never throwing into the boundary (`WAS-21`) |

**Invariants**: the extension removes records only; it stores no credential and adds no new object
store (`WAS-31`, `WAS-32`). The in-memory double empties both maps; the `idb` adapter clears both
object stores.

---

## MessageKey additions

The feature's copy resolves through the single catalog (`web/src/l10n/strings.ts`); no key is removed
(`WAS-31`). Adding these keys keeps every string out of the components (`SC-008`).

| Key | Used by |
|---|---|
| `auth.title` | the auth surface's identity line (`WAS-09`) |
| `auth.modeLogin` / `auth.modeRegister` | the mode switch (`WAS-09`) |
| `auth.emailLabel` / `auth.passwordLabel` / `auth.usernameLabel` | the form fields (`WAS-09`) |
| `auth.submitLogin` / `auth.submitRegister` | the single primary action (`WAS-09`) |
| `auth.restoring` | the restoring state before the first restore settles (`WAS-03`) |
| `nav.yourSheet` / `nav.logout` | the session-only navigation entry and sign-out (`WAS-19`, `WAS-25`) |
| `yourSheet.title` / `yourSheet.loading` | the destination's header and loading state (`WAS-25`) |
| `yourSheet.empty` | the designed empty state (`WAS-26`) |
| `yourSheet.open` / `yourSheet.retry` | the Open and Retry actions (`WAS-25`, `WAS-27`) |
| `yourSheet.delete` | the delete control (`WAS-28`) |
| `yourSheet.deleteTitle` / `yourSheet.deleteMessage` / `yourSheet.deleteConfirm` | the confirmation dialog stating the exact consequence (`WAS-28`) |
| `yourSheet.cancel` | the dialog's safe action (`WAS-28`) |
| `api.error.unauthorized` / `api.error.forbidden` / `api.error.rateLimited` | the three classified session codes (`WAS-30`) |

**Invariants**: no user-facing literal exists in `web/src/auth/` or in the new `web/src/sheet/`
modules; every branch resolves a key through `message()`, which is what `WAS-31` and `SC-008` assert.
