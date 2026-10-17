# Contract: Auth gate and the auth surface

**Requirements**: WAS-03, WAS-04, WAS-05, WAS-07, WAS-08, WAS-09, WAS-10, WAS-24 | **Backing
document**: [data-model.md `AuthSession`](../data-model.md) and
[research.md `R3` / `R8` / `R9`](../research.md).

**Modules**: `web/src/auth/auth_gate.tsx` (the gate and its restoring state),
`web/src/auth/auth_screen.tsx` (the sign-in / registration form), `web/src/app/routes.ts` (the route
audiences), `web/src/app/router.tsx` (the gate wraps the route table).

## Interface

| State | Rendered |
|---|---|
| first restore in flight (`restoring`) | the minimal restoring state, never the sign-in form (WAS-03) |
| `unauthenticated` | `AuthScreen` **only** — no `AppShell`, no catalog, no sheet (WAS-08) |
| `authenticating` | `AuthScreen` with the single primary action disabled and progress shown (WAS-09) |
| `authenticated` | `AppShell` with the route tree; the current location, reset to `/` on a boundary (WAS-24) |

```ts
// web/src/app/routes.ts — every data route is session-only.
export type RouteAudience = 'public' | 'session';
// catalog '/', sheet '/sheets/:id', your-sheet '/your-sheet', not-found '*' → audience: 'session'
```

## Rules

1. **Restore once, before deciding** (WAS-03). The gate runs one restore on mount and shows the
   restoring state until it settles, so a returning user never sees the sign-in form flash.
2. **A rejected cookie is no session** (WAS-04). A `401` from the silent refresh settles at
   `unauthenticated` and shows the sign-in form.
3. **A transient failure keeps the data** (WAS-05). An offline/`5xx`/unreadable restore settles at
   `unauthenticated` but does **not** sweep the session-local data (see
   [session-boundary.contract.md](./session-boundary.contract.md)); the next load retries.
4. **No session ⇒ no surface** (WAS-08). While no session is held only the auth surface renders; the
   catalog, the sheet and the "Your sheet" destination are never mounted.
5. **One form, one action** (WAS-09). The auth surface offers sign-in and registration through a mode
   switch, submits through `AuthSession`, disables the single primary action and shows progress while
   a submit is in flight, and renders the password as a hidden-content control.
6. **Failure keeps values, hides existence** (WAS-07, WAS-10). A failed submit keeps the typed values
   and renders the mapped copy resolved from the catalog (never the server message), and the copy
   never reveals whether an email exists.
7. **Deep links are not resumed** (WAS-24). A route visited without a session renders the sign-in form
   and is not resumed after sign-in; the next session starts on the catalog. A session end resets the
   location to `/` so no route from the finished session survives.

## Obligations

1. **WAS-03** — one restoring refresh before the session state is decided.
2. **WAS-04** — a rejected silent refresh shows the sign-in form.
3. **WAS-05** — a transient refresh failure shows the sign-in form and keeps local data.
4. **WAS-08** — only the auth surface renders without a session.
5. **WAS-09** — one place, one disabled primary action, one hidden password control.
6. **WAS-10** — catalog copy, no email-existence signal, server text rendered as text.
7. **WAS-24** — deep link not resumed; next session starts on the catalog.

## Test obligations

- **Happy path**: mount with a valid cookie ⇒ restoring then the authenticated shell; submit a valid
  login ⇒ the shell renders.
- **Edge**: mount with no cookie ⇒ the sign-in form with no request beyond one silent refresh; a
  transient restore failure ⇒ the sign-in form and the session-local data intact; a deep link to
  `/sheets/abc` while signed out ⇒ the sign-in form, and after sign-in the catalog.
- **Error**: a `401` from submit keeps the values and renders the mapped copy; a server message
  carrying `<script>` renders as text; the copy is identical for a duplicate email and any other
  registration failure.
- **Interface**: the primary action is disabled while a submit is in flight and progress is shown; the
  password input is a hidden-content control with an accessible name.

## Traceability

- Gate states: WAS-03, WAS-04, WAS-05, WAS-08, WAS-24.
- Surface: WAS-07, WAS-09, WAS-10.
