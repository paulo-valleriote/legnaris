# Contract: Session boundary (sign-out and the local-data sweep)

**Requirements**: WAS-19, WAS-20, WAS-21, WAS-22, WAS-23, WAS-32 | **Backing document**:
[data-model.md `SessionBoundary` / `SessionLocalData` / `SheetStore` extension](../data-model.md) and
[research.md `R3` / `R6`](../research.md). The desktop rule source is `AD-010`.

**Modules**: `web/src/auth/auth_session.ts` (`logout`, `onUnauthorized`), `web/src/library/session_data.ts`
(the sweep), `web/src/library/sheet_store.ts` + `web/src/library/idb_sheet_store.ts` (`clearAll`),
`web/src/shell/left_nav.tsx` (the sign-out activation).

## Interface

| Operation | Trigger | Effect |
|---|---|---|
| `logout()` | the sign-out activation | single-flight teardown (§Rules 4); best-effort `/auth/logout`; drop the token; sweep; show the sign-in form (WAS-19) |
| `onUnauthorized()` | a rejected refresh in the transport | forced logout: drop the token; sweep; show the sign-in form (WAS-14, WAS-20) |
| `clearSessionData(store)` | either boundary | `bestEffort(() => store.clearAll())` (WAS-21) |
| `store.clearAll()` | the sweep | deletes every cached sheet and every draft (WAS-21) |

## Rules

1. **A definitive end sweeps** (WAS-19, WAS-20). Sign-out and forced logout each drop the in-memory
   token, flip the session to `unauthenticated`, and clear the session-local data — the same teardown
   on both paths.
2. **Everything local goes** (WAS-21). The sweep is every cached sheet and every draft, and it is
   best-effort: a failing store never blocks the session end.
3. **Nothing survives to the next session** (WAS-22). After a boundary, the next session can render no
   cached sheet and no draft of the finished session.
4. **Single-flight sign-out** (WAS-23). The in-flight guard is set synchronously before the first
   `await`, so an activation while one is in flight returns the same future and issues no second
   `/auth/logout`; the guard clears on completion so a later sign-out runs a fresh teardown.
5. **Transients keep the data** (WAS-05, WAS-15). A refresh that fails transiently is not a boundary
   and never calls the sweep.
6. **No credential anywhere** (WAS-32). No boundary writes or reads a token from a store, a log or a
   URL; `clearAll` removes records only.

## Obligations

1. **WAS-19** — sign-out: best-effort logout, drop the token, show the form, sweep.
2. **WAS-20** — forced logout sweeps exactly as sign-out does.
3. **WAS-21** — the sweep removes every cached sheet and draft; a failing store never blocks the end.
4. **WAS-22** — the next session renders none of the finished session's records.
5. **WAS-23** — at most one `/auth/logout` per sign-out.
6. **WAS-32** — the access and CSRF tokens never appear in a local store, a log or a URL.

## Test obligations

- **Happy path**: sign in, cache a sheet and leave a draft, sign out ⇒ the store is empty and the
  sign-in form is shown; a forced logout clears the store the same way.
- **Edge**: activate sign-out twice in one frame ⇒ one `/auth/logout`; a failing `clearAll` still ends
  the session and shows the form; a transient refresh failure leaves the cached sheet and the draft in
  place.
- **Error**: a failed `/auth/logout` never blocks the local teardown.
- **Security**: a scan of the store records after a boundary finds no credential-shaped value, and no
  token appears in a log record or a URL.

## Traceability

- Sign-out: WAS-19, WAS-23.
- Forced logout: WAS-14, WAS-20.
- Sweep: WAS-21, WAS-22.
- Credential hygiene: WAS-32.
