# Tasks: The Code Step in the Client — Registration and Sign-in Hand Back a Code Screen, Not a Session

**Input**: Design documents from `.specs/features/019-web-auth-code-step/`

**Prerequisites**: `plan.md`, `spec.md` (`WCS-01`…`WCS-27`), `research.md` (`R1`…`R16`),
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

- [X] T001 [P] Confirm no dependency is added and the toolchain is unchanged — `web/package.json` and `web/package-lock.json` stay byte-identical (WCS-16)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the shared taxonomy, copy and pinned captures every story consumes. The refusal kinds and
the code-surface copy are shared by US1's surface and US3's mapping, so they land first — mirroring how
the delivered features put a shared seam before the stories that build on it.

- [X] T002 [P] Add `invalidCode`, `emailUnavailable` and `conflict` to the `FailureKind` union — `web/src/api/errors.ts` (WCS-11)
- [X] T003 [P] Add the code-surface copy keys and the `api.error.invalidCode` / `api.error.emailUnavailable` entries to the catalog; the `CONFLICT` copy lands with US3 — `web/src/l10n/strings.ts` (WCS-13, WCS-16)
- [X] T004 Add the parameterized resolution to the catalog, so the copy that carries the address, the remaining time or the account mode is resolved by the catalog and never concatenated by a surface — `web/src/l10n/strings.ts` (WCS-13)
- [X] T005 [P] Make the capture script branch on the answer it actually received (session body versus code step) and document running it once per gate state, leaving the other family's files untouched — `web/src/api/fixtures/capture.sh`, `web/src/api/fixtures/README.md` (WCS-14, R8)
- [X] T006 Capture the seven code-step fixtures from the real backend, bytes verbatim — `web/src/api/fixtures/auth_register_challenge.json`, `auth_login_challenge.json`, `auth_challenge_verify.json`, `auth_challenge_resend.json`, `error_otp_invalid.json`, `error_email_unavailable.json`, `error_conflict.json` (WCS-14) *(depends on T005)*
- [X] T007 Extend the pinned suite to decode the seven fixtures with no backend running and to fail on a hand-edited key — `web/src/test/api_contract_captures.test.ts` (WCS-14) *(depends on T006)*

**Checkpoint**: the taxonomy, the copy and the pinned contract are in place; the stories can begin.

---

## Phase 3: User Story 1 — Registering or signing in now asks for a code (Priority: P1) 🎯 MVP

**Goal**: the credential operations accept either answer without knowing the gate; a code step becomes
an in-memory pending state rendered by a code surface that names the address and the code's lifetime;
submitting the code establishes the session; a refusal keeps the step.

**Independent Test**: drive `register`/`login` with a challenge answer and assert no session, no token
and the code surface; assert the surface names the address and the stated lifetime and that the code
field holds the focus; submit the correct code and assert the session is established exactly as a
session answer does; submit a wrong code and assert the surface stays with the mapped copy; abandon and
assert the form returns with the address kept and no request issued.

### Tests for User Story 1 (write first, ensure they FAIL) ⚠️

- [X] T008 [P] [US1] Decode both outcomes from `register` and `login`, the shape discrimination, the ambiguous and malformed payloads, the two new operations' method/path/body/credentials and absent headers, and leading zeros preserved — `web/src/test/auth_api.test.ts` (WCS-01, WCS-03, WCS-15)
- [X] T009 [P] [US1] The outcome branch, the `pending !== null` iff `pendingCode` invariant, submit success and refusal, the version counter's unchanged semantics, the stale-handle guard and abandonment — `web/src/test/auth_session.test.ts` (WCS-02, WCS-03, WCS-04, WCS-06)
- [X] T010 [P] [US1] The pending step's projection — the address, the username and the advertised lifetime — and the absence of a password in it — `web/src/test/auth_session.test.ts` (WCS-19, WCS-20, WCS-21)
- [X] T011 [P] [US1] The gate renders the code surface while pending, never the shell, and never treats a code-step answer as a session — `web/src/test/auth_gate.test.tsx` (WCS-02)
- [X] T012 [P] [US1] The surface's origin-specific copy, the instruction naming the address and the stated lifetime, the field's `inputMode`/`autoComplete`/label, the six positions following the field's value, the focus on the field when the surface appears and after a refusal, the refusal associated with the field, the in-flight submit, and "start over" clearing and issuing nothing — `web/src/test/auth_code_screen.test.tsx` (WCS-03, WCS-04, WCS-05, WCS-06, WCS-16, WCS-19, WCS-20, WCS-22, WCS-24)
- [X] T013 [P] [US1] The surface's composition — the origin line in place of the mode switch, the block order, and the transition limited to opacity and transform and zeroed under reduced motion — `web/src/test/auth_code_screen.test.tsx` (WCS-25)
- [X] T014 [P] [US1] The credential form's values restored after the step is abandoned — the address and the account mode kept, the password empty, the way back moving the focus to the form — `web/src/test/auth_screen.test.tsx` (WCS-21, WCS-22)
- [X] T015 [P] [US1] Move the credential-form and session-boundary cases that assumed a session answer; keep the legacy gate-off path asserted beside the code-step path — `web/src/test/auth_screen.test.tsx`, `web/src/test/session_boundary.test.ts` (WCS-01, WCS-17)

### Implementation for User Story 1

- [X] T016 [US1] Add `SignInOutcome`, `ChallengeStep`, `ResendResult`, `decodeSignInOutcome` and the `verifyChallenge` / `resendChallenge` operations; keep `refresh`/`logout` unchanged — `web/src/api/auth_api.ts` (WCS-01, WCS-03, WCS-15)
- [X] T017 [US1] Add the `pendingCode` state, the snapshot's `pending` projection carrying the address, the username and the advertised lifetime, the outcome branch in `register`/`login`, `submitCode` with the stale-handle guard, and `cancelCodeStep`; keep the handle module-private — `web/src/auth/auth_session.ts` (WCS-02, WCS-03, WCS-04, WCS-06, WCS-19, WCS-20, WCS-21)
- [X] T018 [US1] Build the code surface — the origin line, the heading, the instruction that names the address and the stated lifetime, the code field over six single-digit positions, the submit with its progress state, the inline refusal associated with the field, the way back, and its token-only styles — `web/src/auth/auth_code_screen.tsx`, `web/src/auth/auth.css` (WCS-03, WCS-05, WCS-16, WCS-19, WCS-20, WCS-22, WCS-24, WCS-25)
- [X] T019 [US1] Render the code surface in place of the credential form while a step is pending, leaving the URL and the route table unchanged — `web/src/auth/auth_gate.tsx` (WCS-02)
- [X] T020 [US1] Restore the credential form's address and account mode when the step is abandoned, clear the password, and move the focus to the form — `web/src/auth/auth_screen.tsx` (WCS-21, WCS-22)
- [X] T021 [US1] Specify the code field and the code surface in the visual contract — the field's construction, the surface's block order and the transition's rules — `web/DESIGN.md` (WCS-24, WCS-25)

**Checkpoint**: US1 is exercisable end to end against a gated stack; US1 is the MVP.

---

## Phase 4: User Story 2 — The code never arrived (Priority: P1)

**Goal**: the surface offers a resend action governed by an honest countdown that is announced once,
issues one request per activation, confirms itself without invalidating the code the person holds, and
never rotates the handle or discards the code.

**Independent Test**: assert the action is unreachable-but-findable with the remaining seconds; elapse
the window and activate once (one request, countdown re-seeded); activate twice (one request); answer
unavailable and assert the copy, the kept handle and the re-armed action; assert the confirmation names
both codes as valid; submit the same code afterwards and assert it still verifies.

### Tests for User Story 2 (write first, ensure they FAIL) ⚠️

- [X] T022 [P] [US2] The countdown's enable boundary under fake timers, one request per activation, the unavailable copy re-arming at the window's end, and the entered code untouched — `web/src/test/auth_code_screen.test.tsx` (WCS-07, WCS-08, WCS-09, WCS-10)
- [X] T023 [P] [US2] The countdown announced once and never per tick, and the resend action still reachable by keyboard and by assistive technology while it is unavailable — `web/src/test/auth_code_screen.test.tsx` (WCS-07, WCS-23)
- [X] T024 [P] [US2] The resend confirmation — announced once, naming both codes as valid, and replacing an earlier refusal instead of sitting beside it — `web/src/test/auth_code_screen.test.tsx` (WCS-26)
- [X] T025 [P] [US2] `resendCode` single-flight with concurrent callers and the window re-seeded from the answer, leaving the handle and the pending state intact on failure — `web/src/test/auth_session.test.ts` (WCS-08, WCS-10)

### Implementation for User Story 2

- [X] T026 [US2] Add `resendCode` as a single-flight operation that re-seeds the advertised window on success and rethrows without touching the pending state on failure — `web/src/auth/auth_session.ts` (WCS-08, WCS-10)
- [X] T027 [US2] Add the resend action with its local countdown seeded from `pending.resendAfter`, the countdown as static text outside any live region, the action reachable while unavailable, the recorded-resend confirmation, and the action re-arming at the window's end after an unavailable answer — `web/src/auth/auth_code_screen.tsx` (WCS-07, WCS-08, WCS-09, WCS-23, WCS-26)

**Checkpoint**: US1 and US2 both work independently; a person whose mail fails has a path forward.

---

## Phase 5: User Story 3 — The failure the person sees names the real cause (Priority: P2)

**Goal**: the duplicate-address conflict gets its own kind, its own copy and an action that takes the
offered recovery at the credential form, and the mapping is proven total over the auth surface's codes
with the unmapped fallback intact.

**Independent Test**: classify each of the three new codes and assert its kind and resolved copy; feed
an unknown code and assert the generic kind with its raw code preserved; register a duplicate address
and assert the conflict copy naming the address and offering sign-in, and that the offered action
switches the form, keeps the address and moves the focus.

### Tests for User Story 3 (write first, ensure they FAIL) ⚠️

- [X] T028 [P] [US3] Each new code's kind and resolved copy, the copy coming from the catalog rather than the server message, the unknown fallback, and the existing entries unchanged — `web/src/test/api_error_mapper.test.ts` (WCS-11, WCS-12, WCS-13)
- [X] T029 [P] [US3] The duplicate-address conflict copy at registration names the address and offers sign-in, keeping the typed values — `web/src/test/auth_screen.test.tsx` (WCS-11)
- [X] T030 [P] [US3] The conflict refusal's inline action — switching the form to sign-in, keeping the address and moving the focus to the password field — `web/src/test/auth_screen.test.tsx` (WCS-27)

### Implementation for User Story 3

- [X] T031 [US3] Add the `CONFLICT` → `conflict` entry with its copy key and the catalog copy that names the address and offers sign-in, recording that the server emits `CONFLICT` in one place only — `web/src/api/error_mapper.ts`, `web/src/l10n/strings.ts` (WCS-11, WCS-13)
- [X] T032 [US3] Add the conflict refusal's inline sign-in action — switching the mode, keeping the address and moving the focus to the password field — `web/src/auth/auth_screen.tsx` (WCS-27)

**Checkpoint**: all three stories are independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T033 [P] Add the code-step flow end to end (intercept a challenge answer, render the surface, complete against a verification answer) and the axe sweep over the code surface — `web/e2e/auth.spec.ts` (WCS-17)
- [X] T034 [P] Reconcile the corpus: mark `WAS-02`'s premise as delivered by this feature's requirements, with a pointer to the superseding requirement — `.specs/features/007-web-auth-session/spec.md` (WCS-18)
- [X] T035 Run the full gate set and the quickstart scenarios — `npm run lint && npm run typecheck && npm test && npm run test:e2e` under `web/`, and `make lint-specs` at the root; record the results (WCS-16, WCS-17, WCS-18)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependency.
- **Foundational (Phase 2)**: depends on Setup; **blocks every story** — the refusal kinds, the copy
  and the pinned contract are consumed by all three.
- **US1 (Phase 3)**: depends on Phase 2. It is the MVP and the largest slice.
- **US2 (Phase 4)**: depends on Phase 2; integrates with US1's surface (T018) and session (T017),
  so in a sequential run it follows US1.
- **US3 (Phase 5)**: depends on Phase 2 (the taxonomy); its copy lands in the shared catalog, so
  sequential runs follow US1.
- **Polish (Phase 6)**: depends on every story.

### Requirement → task coverage

| Requirement | Tasks |
|---|---|
| `WCS-01` | T008, T015, T016 |
| `WCS-02` | T009, T011, T017, T019 |
| `WCS-03` | T008, T009, T012, T016, T017, T018 |
| `WCS-04` | T009, T012, T017, T018 |
| `WCS-05` | T012, T018 |
| `WCS-06` | T009, T012, T017, T020 |
| `WCS-07` | T022, T023, T027 |
| `WCS-08` | T022, T025, T026, T027 |
| `WCS-09` | T022, T027 |
| `WCS-10` | T022, T024, T025, T026, T027 |
| `WCS-11` | T002, T004, T028, T029, T031 |
| `WCS-12` | T004, T028 |
| `WCS-13` | T003, T004, T028, T031 |
| `WCS-14` | T005, T006, T007 |
| `WCS-15` | T008, T016 |
| `WCS-16` | T001, T003, T012, T018, T035 |
| `WCS-17` | T012, T013, T015, T033, T035 |
| `WCS-18` | T034, T035 |
| `WCS-19` | T010, T012, T017, T018 |
| `WCS-20` | T010, T012, T017, T018 |
| `WCS-21` | T009, T010, T012, T014, T017, T020 |
| `WCS-22` | T012, T014, T018, T020 |
| `WCS-23` | T023, T027 |
| `WCS-24` | T012, T018, T021 |
| `WCS-25` | T013, T018, T021 |
| `WCS-26` | T024, T027 |
| `WCS-27` | T030, T032 |

Every requirement is covered by at least one task; no task is unmapped.

### Within each story

- Tests are written and must fail before the implementation they cover.
- Types before the table (T002 before T004); the table before the surface that shows its copy.
- The session state before the surface that reads it (T017 before T018).
- A story's tests and its implementation land in the same commit (constitution III).

### Parallel Opportunities

- `T002`, `T003`, `T005` touch different files and can run together; `T004` waits on `T003`.
- `T008`–`T015` are eight different files and can be written together.
- `T022`, `T023`, `T024` and `T025` are different files and can be written together.
- `T026`, `T027` and `T028` are different files and can be written together.
- `T033` and `T034` are independent and can run together.

---

## Parallel Example: User Story 1

```bash
# Write the US1 suites together (they must fail first):
Task: "auth_api.test.ts — both outcomes, the discrimination, the new operations"
Task: "auth_session.test.ts — the pending state, the address and the lifetime, submit, the stale-handle guard"
Task: "auth_gate.test.tsx — the gate renders the code surface while pending"
Task: "auth_code_screen.test.tsx — the surface's rendering, the instruction, the six positions, the focus, the refusal, start over"
Task: "auth_screen.test.tsx + session_boundary.test.ts — the moved cases and the restored values"

# Then the implementation in dependency order:
Task: "auth_api.ts — the outcome and the two new operations"
Task: "auth_session.ts — the pending state, the address, the lifetime and submitCode"
Task: "auth_code_screen.tsx + auth.css — the surface"
Task: "auth_gate.tsx — render the surface while pending"
Task: "auth_screen.tsx — the restored values and the focus"
Task: "DESIGN.md — the code field and the surface's visual contract"
```

---

## Implementation Strategy

### MVP first (User Story 1 only)

1. Phase 1 Setup, then Phase 2 Foundational — the taxonomy, the copy, the parameterized resolution and
   the pinned captures.
2. Phase 3 `US1`: the outcome, the pending state, the code surface and the gate.
3. **STOP and validate**: run the US1 independent test and the quickstart scenarios 1, 2, 3, 6 and 7.
   At this point a person on a gated deployment can register, receive a code and sign in — the MVP.

### Incremental delivery

4. Add `US2` (resend) → validate scenario 4 → the mail-failure path is closed.
5. Add `US3` (the conflict copy and its action) → validate scenario 5 → the credential form names the
   real cause and offers a way to take it.
6. Polish → the e2e flow, the corpus reconciliation, scenario 8 and the full gate set.

Each increment adds value without breaking the previous one, and the gate-off path stays asserted
throughout so the deployed behavior cannot regress.

---

## Notes

- `[P]` marks different files with no incomplete dependency; tasks that touch the same file
  (`auth_session.ts`, `auth_code_screen.tsx`, `auth_screen.test.tsx`, `strings.ts`,
  `error_mapper.ts`) are deliberately sequential even across stories.
- The handle never leaves `auth_session.ts` (T017) — no test, fixture or surface holds it.
- The password never enters the pending state (T010, T017); the address and the username do, and they
  are never persisted.
- The refusal copy arrives from the catalog, never from the server message (T026).
- Tests cannot be deferred: the constitution makes code without tests in the same commit incomplete.
- No task adds a dependency, a route, a local store or a Go change; a diff that does is out of scope.

## Delivered deviations

- **The taxonomy landed with the foundation (T002, T003, T031).** `FailureKind` gained
  `invalidCode`, `emailUnavailable` and `conflict` in one change, and the mapping table stays total
  over the kinds — so the `CONFLICT` → `conflict` entry and its catalog copy could not wait for the
  third story without leaving the tree uncompilable. US3 delivers the conflict's *surface*: the copy at
  registration and the action that takes the offered recovery (T029, T030, T032).
- **The stated lifetime's expiry state landed with the resend (T018, T027).** The surface states the
  advertised lifetime from US1 (WCS-20); the *expired* state and the notice that points at the resend
  arrived with US2, because the copy it uses ("Send a new code or start over.") promises a resend that
  only exists from that story on. One interval drives both countdowns (WCS-25).
- **The values the form comes back with live in the session (WCS-21).** The credential form unmounts
  while the step is pending, so the address and the account mode survive the abandonment in the
  session's `restored` projection rather than in a component's local state. The contract and the data
  model were amended in the same change (`docs(specs): record the restored-form projection in the
  session contract`).
- **The pinned suite's credential assertion was tightened, not weakened (API-35, WCS-14).** The
  verification capture carries the session body's shape with a redacted token, so the assertion now
  looks for a credential *value* — a JWT-shaped string, a refresh token, a bearer value, or any
  `access_token` other than the placeholder — instead of the key's name. It is strictly stronger than
  the previous regex, which the key's name satisfied.
- **The capture script reads the code from the send intention (WCS-14).** The code never reaches a log
  line or the sender's output, so the script reads it from the `email_outbox` collection with
  `mongosh`, and the run removes the send intentions it recorded so the day's allowance is restored.
  The README documents the requirement and the quick local configuration.

### Phase 7 — Convergence

- **The abandoned code is cleared by the gate, not by the surface (T037, WCS-06).** `AuthCodeScreen`
  renders nothing while no step is pending, but the component stays mounted, so its local code state
  would survive a cancel if the surface were rendered on its own. The gate unmounts the surface
  instead — `pendingCode` renders `AuthCodeScreen`, every other state renders `AuthScreen` — so the
  entered code dies with the component. The assertion is therefore driven through the real gate: the
  abandoned digits are gone from the document and the next step opens on an empty field. The suite
  proves the product's mount, not a synthetic one.
- **The superseded handle is asserted at the transport seam (T044, WCS-06).** The second sign-in's
  answer must arrive *after* the first verification is already in flight, so the case drives a
  transport whose attempts are settled by index rather than in order; a FIFO transport could not
  express "the stale answer lands late".
- **The gate set ran green except for four pre-existing screenshot baselines.** `npm run lint`,
  `npm run typecheck` and `npm test` pass; `npm run test:e2e` passes 19 of 23, and the four
  failures are the visual-parity baselines (`shell`, `catalog`, `sheet`), which differ on this
  machine's font rendering and fail identically on the unmodified tree. No accessibility assertion
  failed, and the auth spec — the credential surface, the code surface and the signed-in shell —
  is green.

---

## Phase 7: Convergence

**Purpose**: close the gap between the shipped behaviour and the obligations the artifacts still
carry. The implementation satisfies every functional requirement; what remains is the test depth
`WCS-17` demands and the edge-cases-first ordering `SC-005` states. No product code changes here —
each task adds an assertion or reorders an existing one.

- [X] T036 Move the session-boundary suite with the code-step path: add the pending-step case in which a refresh cookie from an earlier session is present, asserting the pending state is never read as a session and the cold-start refresh keeps its behaviour — `web/src/test/session_boundary.test.ts` — per WCS-01, WCS-17 (partial)
- [X] T037 Assert that abandoning the code surface clears the entered code, not only an empty field — `web/src/test/auth_code_screen.test.tsx` — per WCS-06 (partial)
- [X] T038 Assert that neither challenge operation triggers a refresh, by counting the issued requests and asserting no `/auth/refresh` call — `web/src/test/auth_api.test.ts` — per WCS-15 (partial)
- [X] T039 Assert that the instruction line describes the code field, so the address and the stated lifetime are read with it — `web/src/test/auth_code_screen.test.tsx` — per WCS-22 (partial)
- [X] T040 Assert that the six code positions are hidden from assistive technology, so they are never the only way the code is read — `web/src/test/auth_code_screen.test.tsx` — per WCS-24 (partial)
- [X] T041 Pin the refusal's and the resend row's positions in the surface's declared block order — `web/src/test/auth_code_screen.test.tsx` — per WCS-25 (partial)
- [X] T042 Reorder the moved and new suites so the edge and error cases precede the happy path — `web/src/test/auth_api.test.ts`, `web/src/test/auth_session.test.ts`, `web/src/test/auth_code_screen.test.tsx`, `web/src/test/auth_gate.test.tsx`, `web/src/test/auth_screen.test.tsx`, `web/src/test/api_error_mapper.test.ts`, `web/src/test/api_contract_captures.test.ts` — per SC-005, WCS-17 (partial)
- [X] T043 Assert that a corrected code can be submitted after a refused one — `web/src/test/auth_code_screen.test.tsx` — per WCS-04 (partial)
- [X] T044 Assert that a second sign-in starts a new handle and supersedes the first — `web/src/test/auth_session.test.ts` — per WCS-06 (partial)
