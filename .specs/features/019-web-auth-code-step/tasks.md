# Tasks: The Code Step in the Client — Registration and Sign-in Hand Back a Code Screen, Not a Session

**Input**: Design documents from `.specs/features/019-web-auth-code-step/`

**Prerequisites**: `plan.md`, `spec.md` (`WCS-01`…`WCS-18`), `research.md` (`R1`…`R10`),
`data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Required, not optional — the constitution makes behavior ship with tests in the same commit
(`III. Behavior Ships With Tests`). Every story's tests are written **first**, must fail against the
current tree, and land in the same commit as the implementation they cover; the existing suites that
assume "registration and sign-in return a session" are moved in that same commit.

**Organization**: tasks are grouped by user story so each story is independently verifiable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: the user story the task serves (`US1`, `US2`, `US3`)
- Every task names the exact file it changes.

## Path Conventions

All paths are under `web/` (the React/TypeScript PWA). The Go backend is not touched by this feature.

---

## Phase 1: Setup

**Purpose**: confirm the change needs nothing installed.

- [ ] T001 [P] Confirm no dependency is added and the toolchain is unchanged — `web/package.json` and `web/package-lock.json` stay byte-identical (WCS-16)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the shared taxonomy, copy and pinned captures every story consumes. The refusal kinds and
the code-surface copy are shared by US1's surface and US3's mapping, so they land first — mirroring how
the delivered features put a shared seam before the stories that build on it.

- [ ] T002 [P] Add `invalidCode`, `emailUnavailable` and `conflict` to the `FailureKind` union — `web/src/api/errors.ts` (WCS-11)
- [ ] T003 [P] Add the code-surface copy keys and the `api.error.invalidCode` / `api.error.emailUnavailable` entries to the catalog; the `CONFLICT` copy lands with US3 — `web/src/l10n/strings.ts` (WCS-13, WCS-16)
- [ ] T004 Add the `OTP_INVALID` → `invalidCode` and `EMAIL_UNAVAILABLE` → `emailUnavailable` entries to the mapping table, keeping every existing entry unchanged and the unmapped fallback intact — `web/src/api/error_mapper.ts` (WCS-11, WCS-12) *(depends on T002, T003)*
- [ ] T005 [P] Make the capture script branch on the answer it actually received (session body versus code step) and document running it once per gate state, leaving the other family's files untouched — `web/src/api/fixtures/capture.sh`, `web/src/api/fixtures/README.md` (WCS-14, R8)
- [ ] T006 Capture the seven code-step fixtures from the real backend, bytes verbatim — `web/src/api/fixtures/auth_register_challenge.json`, `auth_login_challenge.json`, `auth_challenge_verify.json`, `auth_challenge_resend.json`, `error_otp_invalid.json`, `error_email_unavailable.json`, `error_conflict.json` (WCS-14) *(depends on T005)*
- [ ] T007 Extend the pinned suite to decode the seven fixtures with no backend running and to fail on a hand-edited key — `web/src/test/api_contract_captures.test.ts` (WCS-14) *(depends on T006)*

**Checkpoint**: the taxonomy, the copy and the pinned contract are in place; the stories can begin.

---

## Phase 3: User Story 1 — Registering or signing in now asks for a code (Priority: P1) 🎯 MVP

**Goal**: the credential operations accept either answer without knowing the gate; a code step becomes
an in-memory pending state rendered by a code surface; submitting the code establishes the session; a
refusal keeps the step.

**Independent Test**: drive `register`/`login` with a challenge answer and assert no session, no token
and the code surface; submit the correct code and assert the session is established exactly as a
session answer does; submit a wrong code and assert the surface stays with the mapped copy; abandon and
assert no request.

### Tests for User Story 1 (write first, ensure they FAIL) ⚠️

- [ ] T008 [P] [US1] Decode both outcomes from `register` and `login`, the shape discrimination, the ambiguous and malformed payloads, the two new operations' method/path/body/credentials and absent headers, and leading zeros preserved — `web/src/test/auth_api.test.ts` (WCS-01, WCS-03, WCS-15)
- [ ] T009 [P] [US1] The outcome branch, the `pending !== null` iff `pendingCode` invariant, submit success and refusal, the version counter's unchanged semantics, the stale-handle guard and abandonment — `web/src/test/auth_session.test.ts` (WCS-02, WCS-03, WCS-04, WCS-06)
- [ ] T010 [P] [US1] The gate renders the code surface while pending, never the shell, and never treats a code-step answer as a session — `web/src/test/auth_gate.test.tsx` (WCS-02)
- [ ] T011 [P] [US1] The surface's origin-specific copy, the code field's `inputMode`/`autoComplete`/label, the in-flight submit, the refusal copy with the code kept, and "start over" clearing and issuing nothing — `web/src/test/auth_code_screen.test.tsx` (WCS-03, WCS-04, WCS-05, WCS-06, WCS-16)
- [ ] T012 [P] [US1] Move the credential-form and session-boundary cases that assumed a session answer; keep the legacy gate-off path asserted beside the code-step path — `web/src/test/auth_screen.test.tsx`, `web/src/test/session_boundary.test.ts` (WCS-01, WCS-17)

### Implementation for User Story 1

- [ ] T013 [US1] Add `SignInOutcome`, `ChallengeStep`, `ResendResult`, `decodeSignInOutcome` and the `verifyChallenge` / `resendChallenge` operations; keep `refresh`/`logout` unchanged — `web/src/api/auth_api.ts` (WCS-01, WCS-03, WCS-15)
- [ ] T014 [US1] Add the `pendingCode` state, the snapshot's `pending` projection, the outcome branch in `register`/`login`, `submitCode` with the stale-handle guard, and `cancelCodeStep`; keep the handle module-private — `web/src/auth/auth_session.ts` (WCS-02, WCS-03, WCS-04, WCS-06)
- [ ] T015 [US1] Build the code surface — origin copy, the one-time-code field, the submit with its progress state, the inline mapped refusal, the "start over" action, and its token-only styles — `web/src/auth/auth_code_screen.tsx`, `web/src/auth/auth.css` (WCS-03, WCS-05, WCS-16)
- [ ] T016 [US1] Render the code surface in place of the credential form while a step is pending, leaving the URL and the route table unchanged — `web/src/auth/auth_gate.tsx` (WCS-02)

**Checkpoint**: US1 is exercisable end to end against a gated stack; US1 is the MVP.

---

## Phase 4: User Story 2 — The code never arrived (Priority: P1)

**Goal**: the surface offers a resend action governed by an honest countdown, issues one request per
activation, and never rotates the handle or discards the code.

**Independent Test**: assert the action is disabled with the remaining seconds; elapse the window and
activate once (one request, countdown re-seeded); activate twice (one request); answer unavailable and
assert the copy, the kept handle and the re-armed action; submit the same code afterwards and assert it
still verifies.

### Tests for User Story 2 (write first, ensure they FAIL) ⚠️

- [ ] T017 [P] [US2] The countdown's enable boundary under fake timers, one request per activation, the unavailable copy re-arming at the window's end, and the entered code untouched — `web/src/test/auth_code_screen.test.tsx` (WCS-07, WCS-08, WCS-09, WCS-10)
- [ ] T018 [P] [US2] `resendCode` single-flight with concurrent callers and the window re-seeded from the answer, leaving the handle and the pending state intact on failure — `web/src/test/auth_session.test.ts` (WCS-08, WCS-10)

### Implementation for User Story 2

- [ ] T019 [US2] Add `resendCode` as a single-flight operation that re-seeds the advertised window on success and rethrows without touching the pending state on failure — `web/src/auth/auth_session.ts` (WCS-08, WCS-10)
- [ ] T020 [US2] Add the resend action with its local countdown seeded from `pending.resendAfter`, disabled while the window runs and while a request is in flight, re-arming at the window's end after an unavailable answer — `web/src/auth/auth_code_screen.tsx` (WCS-07, WCS-08, WCS-09)

**Checkpoint**: US1 and US2 both work independently; a person whose mail fails has a path forward.

---

## Phase 5: User Story 3 — The failure the person sees names the real cause (Priority: P2)

**Goal**: the duplicate-address conflict gets its own kind and copy at the credential form, and the
mapping is proven total over the auth surface's codes with the unmapped fallback intact.

**Independent Test**: classify each of the three new codes and assert its kind and resolved copy; feed
an unknown code and assert the generic kind with its raw code preserved; register a duplicate address
and assert the conflict copy naming the address and offering sign-in.

### Tests for User Story 3 (write first, ensure they FAIL) ⚠️

- [ ] T021 [P] [US3] Each new code's kind and resolved copy, the copy coming from the catalog rather than the server message, the unknown fallback, and the existing entries unchanged — `web/src/test/api_error_mapper.test.ts` (WCS-11, WCS-12, WCS-13)
- [ ] T022 [P] [US3] The duplicate-address conflict copy at registration names the address and offers sign-in, keeping the typed values — `web/src/test/auth_screen.test.tsx` (WCS-11)

### Implementation for User Story 3

- [ ] T023 [US3] Add the `CONFLICT` → `conflict` entry with its copy key and the catalog copy that names the address and offers sign-in, recording that the server emits `CONFLICT` in one place only — `web/src/api/error_mapper.ts`, `web/src/l10n/strings.ts` (WCS-11, WCS-13)

**Checkpoint**: all three stories are independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T024 [P] Add the code-step flow end to end (intercept a challenge answer, render the surface, complete against a verification answer) and the axe sweep over the code surface — `web/e2e/auth.spec.ts` (WCS-17)
- [ ] T025 [P] Reconcile the corpus: mark `WAS-02`'s premise as delivered by this feature's requirements, with a pointer to the superseding requirement — `.specs/features/007-web-auth-session/spec.md` (WCS-18)
- [ ] T026 Run the full gate set and the quickstart scenarios — `npm run lint && npm run typecheck && npm test && npm run test:e2e` under `web/`, and `make lint-specs` at the root; record the results (WCS-16, WCS-17, WCS-18)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependency.
- **Foundational (Phase 2)**: depends on Setup; **blocks every story** — the refusal kinds, the copy and
  the pinned contract are consumed by all three.
- **US1 (Phase 3)**: depends on Phase 2. It is the MVP and the largest slice.
- **US2 (Phase 4)**: depends on Phase 2; integrates with US1's surface (`T015`) and session (`T014`),
  so in a sequential run it follows US1.
- **US3 (Phase 5)**: depends on Phase 2 (the taxonomy); its copy lands in the shared catalog, so
  sequential runs follow US1.
- **Polish (Phase 6)**: depends on every story.

### Requirement → task coverage

| Requirement | Tasks |
|---|---|
| `WCS-01` | T008, T012, T013 |
| `WCS-02` | T009, T010, T014, T016 |
| `WCS-03` | T008, T009, T011, T013, T014, T015 |
| `WCS-04` | T009, T011, T014 |
| `WCS-05` | T011, T015 |
| `WCS-06` | T009, T011, T014 |
| `WCS-07` | T017, T020 |
| `WCS-08` | T017, T018, T019, T020 |
| `WCS-09` | T017, T020 |
| `WCS-10` | T017, T018, T019 |
| `WCS-11` | T002, T004, T021, T022, T023 |
| `WCS-12` | T004, T021 |
| `WCS-13` | T003, T021, T023 |
| `WCS-14` | T005, T006, T007 |
| `WCS-15` | T008, T013 |
| `WCS-16` | T001, T003, T011, T015, T026 |
| `WCS-17` | T012, T024, T026 |
| `WCS-18` | T025, T026 |

Every requirement is covered by at least one task; no task is unmapped.

### Within each story

- Tests are written and must fail before the implementation they cover.
- Types before the table (T002 before T004); the table before the surface that shows its copy.
- The session state before the surface that reads it (T014 before T015).
- A story's tests and its implementation land in the same commit (constitution III).

### Parallel Opportunities

- `T002`, `T003`, `T005` touch different files and can run together; `T004` waits on `T002`/`T003`.
- `T008`–`T012` are five different files and can be written together.
- `T017` and `T018` are different files and can be written together.
- `T021` and `T022` are different files and can be written together.
- `T024` and `T025` are independent and can run together.

---

## Parallel Example: User Story 1

```bash
# Write the five US1 suites together (they must fail first):
Task: "auth_api.test.ts — both outcomes, the discrimination, the new operations"
Task: "auth_session.test.ts — the pending state, submit, the stale-handle guard"
Task: "auth_gate.test.tsx — the gate renders the code surface while pending"
Task: "auth_code_screen.test.tsx — the surface's rendering, refusal copy, start over"
Task: "auth_screen.test.tsx + session_boundary.test.ts — the moved cases"

# Then the implementation in dependency order:
Task: "auth_api.ts — the outcome and the two new operations"
Task: "auth_session.ts — the pending state and submitCode"
Task: "auth_code_screen.tsx + auth.css — the surface"
Task: "auth_gate.tsx — render the surface while pending"
```

---

## Implementation Strategy

### MVP first (User Story 1 only)

1. Phase 1 Setup, then Phase 2 Foundational — the taxonomy, the copy and the pinned captures.
2. Phase 3 `US1`: the outcome, the pending state, the code surface and the gate.
3. **STOP and validate**: run the US1 independent test and the quickstart scenarios 1, 2, 3, 6 and 7.
   At this point a person on a gated deployment can register, receive a code and sign in — the MVP.

### Incremental delivery

4. Add `US2` (resend) → validate scenario 4 → the mail-failure path is closed.
5. Add `US3` (the conflict copy) → validate scenario 5 → the credential form names the real cause.
6. Polish → the e2e flow, the corpus reconciliation, scenario 8 and the full gate set.

Each increment adds value without breaking the previous one, and the gate-off path stays asserted
throughout so the deployed behavior cannot regress.

---

## Notes

- `[P]` marks different files with no incomplete dependency; tasks that touch the same file
  (`auth_session.ts`, `auth_code_screen.tsx`, `auth_screen.test.tsx`, `strings.ts`,
  `error_mapper.ts`) are deliberately sequential even across stories.
- The handle never leaves `auth_session.ts` (T014) — no test, fixture or surface holds it.
- The refusal copy arrives from the catalog, never from the server message (T021).
- Tests cannot be deferred: the constitution makes code without tests in the same commit incomplete.
- No task adds a dependency, a route, a local store or a Go change; a diff that does is out of scope.
