# Desktop Session Boundary (logout/login hygiene) Specification

Source: manual QA of the desktop app after `front-end-phase-6-auth` (epic [LGN-88](https://paulovalleriotedev.atlassian.net/browse/LGN-88), PR #61), which reported two bugs: (1) after logging in with another account, the previous user's sheet is still displayed; (2) logging out on the sheet page and logging into another account reopens the old sheet page and traps the user — no way back to the catalog. Root causes were located by code review: the auth gate swaps only its home subtree (routes pushed on the app's root navigator survive session changes), `AuthenticatedHome` mounts Screen 2 with no route beneath it (back is a no-op and the left-nav entry has no `onTap`), and session-local data (local sheet library index + draft backups) is never reset at session end. A fourth, rarer defect sits in the transport: an in-flight refresh or `401` from a finished session can force-logout the session that replaced it. Contract source: unchanged backend; `docs/TDD-desktop-mvp.md` and `front-end-phase-6-auth` (FE6-13 … FE6-20).

## Problem Statement

The desktop app treats a session change as a provider-state flip, but three things outlive it: routes pushed on the root navigator (the previous account's sheet page stays on screen), Screen 2 mounted as root content with nothing to pop (the user is trapped with no path to the catalog), and the persisted local sheet library and draft backups (the next account sees the previous account's sheet listed). A fourth, rarer defect sits in the transport: an in-flight refresh or `401` from a finished session can force-logout the session that replaced it. This feature makes the session the boundary for navigation, local data, and in-flight auth work.

## Goals

- [ ] Ending a session unmounts every screen opened during it; the next session starts with an empty navigation stack.
- [ ] Screen 2 always has a working back action to the catalog — including the post-login auto-open path that currently traps the user.
- [ ] No local data of a finished session (sheet library index, draft backups) is visible to the next session.
- [ ] A logout that would discard pending local edits asks for explicit confirmation first.
- [ ] Auth responses belonging to a finished session never disturb the current session.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| ------- | ------ |
| Removing the local sheet-library code entirely | `front-end-phase-6-auth` design records removal as a follow-up card; this feature only resets its contents at session end |
| Making the left-nav "Catalog" entry navigable | The Screen 2 back action (NAV-06) restores the path; nav-entry semantics stay unchanged in this fix |
| Multi-account switching / account switcher | Sessions still end only via logout or forced logout; no new auth surface |
| Backend auth/ownership changes | Contract is unchanged; the backend already scopes sheets by owner |
| Confirm dialogs for other destructive actions | Existing confirmations (leave, refresh, library removal) are unchanged |
| Draft restore semantics within a session | Unchanged (DRB-06 … DRB-11) |
| Offline behavior, retry policy, rate limits | Unchanged from the features that defined them |

---

## Assumptions & Open Questions

Every ambiguity is resolved or recorded here - nothing is left silently unclear.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | --------------- | --------- | ---------- |
| Navigation ownership | The authenticated branch of `AuthGate` mounts its own `Navigator`; Screen 2 and the creation dialog push into it, so unmounting the branch unmounts them | User decision over a `popUntil` listener on transitions: structural (navigation dies with the session), no timing race with route animations | y |
| Cleanup trigger | Clear the local library and drafts on explicit logout **and** on forced logout (rejected refresh); **not** when a transient restore failure falls back to the auth screen | A definitive session end invalidates the data; a transient failure keeps the stored token (FE6-06, FE6-09), so the session is not over | n |
| Cleanup scope | In-memory + persisted library index and every persisted draft file | User decision: "lib + drafts no logout" | y |
| Dirty-logout confirmation | Read pending edits at tap time on the active sheet; confirm only when edits exist; barrier/Esc cancel | User decision; matches the desktop/AGENTS.md destructive-confirmation rule and avoids dialog overuse | y |
| Forced logout while dirty | No confirmation; teardown proceeds | The refresh token is revoked server-side, so the session cannot be kept alive locally | n |
| Cleanup failures | Best-effort: a library or draft write failure never blocks logout | Existing house pattern (eviction and library-removal deletes are best-effort); the session must always end | n |
| Base beneath the pushed Screen 2 | The catalog renders beneath the post-login Screen 2 push | One navigation model for Screen 2 — FE6-14 "open that sheet" is satisfied by a push; back then lands on a real surface, and the template detail it needs loads anyway | n |
| Double logout activation | At most one `/auth/logout` request per logout, even if the action fires twice in the same frame | The logout button unmounts on the state flip, but two queued taps can otherwise race the token read | n |
| Late-response hardening | Session-version bump on login/register **and** a transport guard so `onUnauthorized()` only fires while the rejected token still belongs to the current session | The version bump alone does not stop the transport's forced logout: `send()` calls `onUnauthorized()` whenever its refresh returns null | n |

**Open questions:** none - all resolved or logged above (required before the spec is confirmed).

---

## User Stories

### P1: Navigation dies with the session ⭐ MVP

**User Story**: As a user who shares this machine, I want ending a session to unmount every screen opened during it, so that the next account never sees the previous account's sheet page.

**Why P1**: Reported bug 2 (the worse one) and the full-screen vector of bug 1 — logout on the sheet page leaves the previous account's sheet mounted, above the login form or above the next session's home.

**Acceptance Criteria**:

1. WHEN logout completes THEN the system SHALL render only the auth screen, with no route pushed during that session still mounted. (NAV-01) <!-- event-driven -->
2. WHEN a session becomes authenticated THEN the system SHALL start from an empty navigation stack, with no route from a previous session mounted. (NAV-02) <!-- event-driven -->
3. WHEN a forced logout (rejected refresh) ends the session THEN the system SHALL unmount every route pushed during that session. (NAV-03) <!-- event-driven -->
4. WHILE no session is authenticated the system SHALL render no catalog, sheet, or creation-dialog surface (FE6-13 unchanged). (NAV-04) <!-- state-driven -->

**Independent Test**: Log in as A, open a sheet from the catalog (pushed route), log out from that page → assert only `AuthScreen` is mounted; log in as B → assert zero routes from A's session (NavigatorObserver) and no `SheetScreen` from A.

---

### P1: The catalog is always reachable from Screen 2 ⭐ MVP

**User Story**: As a player, I want the back action on my sheet to always return to the catalog, so that I am never trapped on the sheet after logging in.

**Why P1**: Reported bug 2's "não permite fazer mais nada, não dá pra voltar ao catálogo" — `AuthenticatedHome` mounts Screen 2 as root content, where `maybePop()` has no route to pop and the left-nav entry has no `onTap`.

**Acceptance Criteria**:

1. WHEN `GET /sheets` responds `200` after authentication THEN the system SHALL open Screen 2 as a route pushed above the catalog. (NAV-05) <!-- event-driven -->
2. WHEN the user activates the back action on Screen 2 opened by the post-login sheet resolution THEN the system SHALL return to the catalog. (NAV-06) <!-- event-driven -->
3. IF the post-login sheet resolution settles more than once for one authenticated transition (a Retry after a failure, or a rebuild while in flight) THEN the system SHALL NOT push a second Screen 2 route. (NAV-07) <!-- unwanted-behavior -->
4. The system SHALL keep F5 and Ctrl+S dispatching to whichever Screen 2 is mounted, wherever it was opened from. (NAV-08) <!-- ubiquitous -->

**Independent Test**: Log in as an account that owns a sheet → Screen 2 opens over the catalog; activate back → catalog visible; trigger a Retry plus a rebuild → exactly one Screen 2 route in the observer log.

---

### P1: Session-local data does not survive the session ⭐ MVP

**User Story**: As a player logging into my own account after someone else used the app, I want the local "My sheets" list and draft backups to reflect only a live session, so that I never see another account's sheet.

**Why P1**: Reported bug 1's catalog vector — with one sheet per user, an account that owns no sheet sees only foreign entries in the local library, which are persisted across logins.

**Acceptance Criteria**:

1. WHEN logout completes THEN the system SHALL clear the local sheet library index (in memory and on disk) and delete every persisted draft backup. (DATA-01) <!-- event-driven -->
2. WHEN a forced logout ends the session THEN the system SHALL clear the local sheet library index and every persisted draft backup. (DATA-02) <!-- event-driven -->
3. IF clearing the local library or a draft backup fails THEN the system SHALL still complete the logout. (DATA-03) <!-- unwanted-behavior -->
4. IF the app falls back to the auth screen after a transient restore failure THEN the system SHALL keep the stored refresh token, the local library, and the draft backups. (DATA-04) <!-- unwanted-behavior -->
5. WHEN logout is activated while the active sheet has pending local edits THEN the system SHALL show a confirmation dialog that states the unsaved edits will be discarded. (DATA-05) <!-- event-driven -->
6. IF the user cancels that confirmation (action, Esc, or barrier) THEN the system SHALL keep the session, the pending edits, and the persisted library and drafts, and SHALL NOT send `/auth/logout`. (DATA-06) <!-- unwanted-behavior -->
7. WHEN logout is activated without pending edits THEN the system SHALL end the session without showing a confirmation dialog. (DATA-07) <!-- event-driven -->
8. IF logout is activated again while the first logout is still in flight THEN the system SHALL NOT issue a second `/auth/logout` request. (DATA-08) <!-- unwanted-behavior -->

**Independent Test**: As A, create a sheet and leave pending edits; log out → confirmation appears, cancel keeps session and files; confirm → auth screen, `sheet_library.json` empty, `drafts/` empty. As B (no sheet) log in → "Minhas fichas" renders no entry.

---

### P2: A finished session cannot disturb the current one

**User Story**: As a user switching accounts, I want late responses from the previous session's in-flight requests to be discarded, so that my new session is never silently logged out or reset.

**Why P2**: Rare race (an access token or refresh overlapping the account switch); correctness hardening for the same boundary this feature draws.

**Acceptance Criteria**:

1. WHEN login or register completes THEN the system SHALL invalidate any refresh started before that login, so its result cannot be applied to the new session. (SES-01) <!-- event-driven -->
2. IF a request issued under a previous session receives a `401` after a new session has authenticated THEN the system SHALL NOT clear the new session's access token or its stored refresh token. (SES-02) <!-- unwanted-behavior -->

**Independent Test**: Hold a refresh future open across logout + login as B; complete it with the old session's token → assert B stays authenticated and the stored refresh token survives.

---

## Edge Cases

- IF logout is activated twice in the same frame THEN the system SHALL send at most one `/auth/logout` request (DATA-08).
- IF a pending-edit confirmation is open when the session otherwise ends (forced logout) THEN the system SHALL proceed without the dialog (assumption: forced logout skips confirmation).
- WHEN the app is killed without logout THEN the system SHALL keep the persisted library and drafts, because the stored refresh token still restores that same session on the next launch.
- IF the cleanup of one draft file fails THEN the system SHALL still clear the library index and complete the logout (no partial-cleanup deadlock).

---

## Requirement Traceability

Each requirement gets a unique ID for tracking across design, tasks, and validation.

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------- |
| NAV-01 | P1: Navigation dies with the session | Design | In Tasks |
| NAV-02 | P1: Navigation dies with the session | Design | In Tasks |
| NAV-03 | P1: Navigation dies with the session | Design | In Tasks |
| NAV-04 | P1: Navigation dies with the session | Design | In Tasks |
| NAV-05 | P1: The catalog is always reachable from Screen 2 | Design | In Tasks |
| NAV-06 | P1: The catalog is always reachable from Screen 2 | Design | In Tasks |
| NAV-07 | P1: The catalog is always reachable from Screen 2 | Design | In Tasks |
| NAV-08 | P1: The catalog is always reachable from Screen 2 | Design | In Tasks |
| DATA-01 | P1: Session-local data does not survive the session | Design | In Tasks |
| DATA-02 | P1: Session-local data does not survive the session | Design | In Tasks |
| DATA-03 | P1: Session-local data does not survive the session | Design | In Tasks |
| DATA-04 | P1: Session-local data does not survive the session | Design | In Tasks |
| DATA-05 | P1: Session-local data does not survive the session | Design | In Tasks |
| DATA-06 | P1: Session-local data does not survive the session | Design | In Tasks |
| DATA-07 | P1: Session-local data does not survive the session | Design | In Tasks |
| DATA-08 | P1: Session-local data does not survive the session | Design | In Tasks |
| SES-01 | P2: A finished session cannot disturb the current one | Design | In Tasks |
| SES-02 | P2: A finished session cannot disturb the current one | Design | In Tasks |

**ID format:** `[CATEGORY]-[NUMBER]` (categories: `NAV` navigation ownership, `DATA` session-local data, `SES` session/transport integrity)

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 18 total, 18 mapped to tasks, 0 unmapped ✅ (T1 NAV-01…NAV-04 · T2 NAV-04 · T4 NAV-02, NAV-05…NAV-08 · T7 DATA-01…DATA-04 · T9 DATA-05…DATA-07 · T8 DATA-08 · T11 SES-01 · T12 SES-02; enablers: T3→NAV-05, T5/T6→DATA-01, T10→DATA-01)

---

## Implicit-requirement sweep

| Dimension | Resolution |
| --------- | ---------- |
| Input validation & bounds | N/A — no new inputs; all inputs are existing session actions (logout activation) |
| Failure / partial-failure states | DATA-03 (cleanup failures never block logout), DATA-06 (cancel path keeps everything intact) |
| Idempotency / retry / duplicate handling | NAV-07 (exactly one Screen 2 push per transition), DATA-08 (at most one `/auth/logout`) |
| Auth boundaries & rate limits | NAV-01 … NAV-04 (session-owned surfaces), SES-02 (transport guard); rate limits N/A — backend unchanged |
| Concurrency / ordering | SES-01 … SES-02 (in-flight auth work across a login); FE6-19 already pins late refresh after logout |
| Data lifecycle / expiry | DATA-01, DATA-02 (clear on definitive session end), DATA-04 (keep on transient restore failure) |
| Observability | N/A — no new logging/metrics requirement; the existing logger is untouched |
| External-dependency failure | N/A beyond DATA-03 — no new external dependency; transport and retry policy unchanged |
| State-transition integrity | NAV-01 … NAV-04 (navigation torn down and rebuilt with the session), SES-01 |

---

## Success Criteria

- [ ] Bug 2 repro fixed: log in with an account that owns a sheet → Screen 2 opens over the catalog and back reaches the catalog; the user is never trapped.
- [ ] Bug 2 / bug 1 (screen vector) repro fixed: logout from a pushed SheetScreen → only the auth screen is mounted; login as another account → zero routes survive.
- [ ] Bug 1 (catalog vector) repro fixed: an account without a sheet logs in → "Minhas fichas" renders no entry from the previous session (`sheet_library.json` empty on disk).
- [ ] Dirty logout shows the confirmation; cancel preserves session, edits, and files; confirm ends the session and clears them.
- [ ] A late `401`/refresh from the previous session leaves the new session authenticated with its stored refresh token intact.
- [ ] `flutter test` green (race-clean for the new tests) and `make lint-specs` clean — every requirement ID cited in code/tests is defined in this spec.
