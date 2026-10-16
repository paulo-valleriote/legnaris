---
description: "Task list for the Typed Web API Client feature"
---

# Tasks: Typed Web API Client

**Input**: Design documents from `.specs/features/002-web-api-client/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: REQUIRED here. The constitution's "behavior ships with tests" principle, the feature's own
API-33, API-34, API-35, API-36, API-37, API-38, API-39, and the repo's edge-case-first rule all demand test tasks; they are written first and
must fail before the implementation they cover.

**Organization**: tasks are grouped by user story so each story is independently implementable,
testable, and demonstrable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on incomplete tasks)
- **[Story]**: US1 … US7, per spec.md
- Every task names its exact file path

## Path Conventions

The module lives under `web/src/api/`, the translation catalog under `web/src/l10n/`, and every test
under `web/src/test/` with an `api_*` (or `l10n_*`) prefix, matching the flat layout F1 established.
Pinned captures live inside the module at `web/src/api/fixtures/`, next to the script that produced
them.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: the three shared pieces every story consumes — the copy catalog, the error shapes, and the
test transport.

- [ ] T001 [P] Create the typed English translation catalog `web/src/l10n/strings.ts` with the eight
      `api.error.*` keys from `contracts/error-mapping.contract.md` and a `message(key)` accessor
      (API-23), plus `web/src/test/l10n_strings.test.ts` asserting every key resolves to a non-empty
      string, that no two keys share a value by accident, and that `message` returns the catalog entry
      itself
- [ ] T002 [P] Create the error shapes `web/src/api/errors.ts` — the `FailureKind` union, `DomainFailure`
      carrying kind/status/code/message, `DefensiveParseFailure` carrying reason/status, and
      `PathFailure` carrying the offending path (API-03, API-04, API-14, API-24) — plus
      `web/src/test/api_errors.test.ts` covering the edge cases: a non-string server message becomes an
      empty string, an absent status stays absent, and the two failure classes are not interchangeable
- [ ] T003 [P] Create the shared test transport `web/src/test/fake_transport.ts`: records every request
      and is scripted with responses or transport failures per call, so the suites can assert method,
      path, headers, body, `credentials`, and the number of attempts (test infrastructure — no behavior
      of its own)

**Checkpoint**: `cd web && npm test` passes with the new catalog, error shapes and helper in place.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the transport seam and the single envelope decoder every story builds on.

**⚠️ CRITICAL**: US1, US3, US5, US6 and US7 all consume these two modules.

### Tests for the foundation ⚠️ (write first; they must fail before the implementation)

- [ ] T004 [P] Transport test `web/src/test/api_transport.test.ts` (API-07, API-29): the native adapter
      passes url, method, headers, body, `credentials: 'omit'` and the abort signal through unchanged;
      it reads the status and raw text back; a rejection propagates as a transport failure; an empty
      body arrives as an empty string
- [ ] T005 Envelope test `web/src/test/api_envelope.test.ts` (API-02, API-03, API-04, API-22): a 2xx with
      `data` decodes through the per-operation callback; a failure with `error.code` raises a domain
      failure carrying status, code and message; a 2xx without `data`, an empty body and invalid JSON on
      either branch raise the defensive parse failure; a failure body with a missing or non-string
      `error.code` raises the defensive parse failure and never a domain failure

### Implementation for the foundation

- [ ] T006 Implement `web/src/api/transport.ts`: the request/response/transport types per
      `contracts/transport.contract.md` and the native adapter that maps them onto the platform request
      (API-07, API-29)
- [ ] T007 Implement `web/src/api/envelope.ts`: the single `decodeEnvelope({ status, body, decode })`
      decoder (API-02, API-03, API-04, API-22)

**Checkpoint**: the transport and the decoder pass their suites; no endpoint exists yet.

---

## Phase 3: User Story 1 - One typed call per consumed endpoint (Priority: P1) 🎯 MVP

**Goal**: six named operations, every request through the injected transport, the envelope decoded
once, and the two write bodies exactly as the backend expects.

**Independent Test**: with the recording fake transport, call all six operations and assert method,
path, headers and body for each; return a success envelope and a failure envelope; return broken JSON
and assert a defensive failure instead of an empty success.

### Tests for User Story 1 ⚠️

- [ ] T008 [P] [US1] Client test `web/src/test/api_client.test.ts` (API-01, API-02, API-05, API-06,
      API-07) covering, edge cases first: the six operations' method and path; the creation body
      carrying exactly `template_id` and `sheet_name`; the update body carrying exactly `data`; the
      update change set not being mutated; a success envelope decoding per operation; a failure
      envelope raising the mapped domain failure; broken JSON raising the defensive parse failure; and
      a structural case proving every request went through the injected transport (never the platform
      one)

### Implementation for User Story 1

- [ ] T009 [US1] Implement `web/src/api/client.ts` with the six typed operations over an injected
      transport and a resolved base address, taking the operation path, the body and the payload
      decoder from `contracts/endpoints.contract.md` (API-01, API-02, API-05, API-06, API-07)

**Checkpoint**: the client's six operations are demonstrable with a fake transport; the payload types
for templates and sheets are still the decoded envelopes' raw values until US2 lands.

---

## Phase 4: User Story 2 - Models faithful to the contract (Priority: P1)

**Goal**: the template field tree and the sheet's dynamic data decoded faithfully, with the dotted-path
accessor the later features build on.

**Independent Test**: decode the pinned template and sheet captures; read a valid path, a missing one, a
path through a non-object, and an empty path; decode whole and fractional numbers and confirm neither is
altered.

### Tests for User Story 2 ⚠️

- [ ] T010 [P] [US2] Template model test `web/src/test/api_models_template.test.ts` (API-08, API-09): a
      leaf is recognised by its `schema` key and a group by its `children` key; a leaf's `default`,
      `type`, `label` and `is_editable` are exposed; an omitted `formula`/`depends_on` is absent rather
      than `''`/`[]`; the optional `exclusive_group`/`option_label`/`column_label` metadata is carried
      through; a node carrying neither key is a defensive parse failure; the catalog entry's `default`
      boolean is read by key
- [ ] T011 [P] [US2] Sheet model test `web/src/test/api_models_sheet.test.ts` (API-10, API-11, API-12,
      API-13, API-14): the sheet's identifiers, name, data and timestamps decode; a whole number and a
      fractional number survive decoding unchanged; a resolving dotted path returns its value; a missing
      segment and a non-object intermediate return no value without raising; an empty path and a
      double-dotted path raise the typed path failure; the accessor never mutates the data

### Implementation for User Story 2

- [ ] T012 [P] [US2] Implement `web/src/api/models/template.ts`: the catalog/detail parse functions, the
      field node union and the field schema shape (API-08, API-09)
- [ ] T013 [P] [US2] Implement `web/src/api/models/sheet.ts`: the sheet parse function and the
      `valueAtPath` accessor (API-10, API-11, API-12, API-13, API-14)
- [ ] T014 [US2] Plug the model parsers into `web/src/api/client.ts` as the payload decoders of the
      template and sheet operations, and decode the update payload as the recalculated effect map
      (API-01, API-02, API-08, API-10)

**Checkpoint**: the six operations return typed payloads and the dotted-path accessor is usable by the
later engine and sheet features.

---

## Phase 5: User Story 3 - Every failure becomes a state the interface can render (Priority: P1)

**Goal**: the complete code→kind mapping, the copy resolved from the catalog, and every failure leaving
the client already classified.

**Independent Test**: drive a table of captured failure bodies through the mapper and assert each kind;
send an unmapped code and assert the generic kind keeps the raw code and message; send a transport
failure and assert the offline kind; render a copy and assert it equals the catalog entry.

### Tests for User Story 3 ⚠️

- [ ] T015 [P] [US3] Mapper test `web/src/test/api_error_mapper.test.ts` (API-15, API-16, API-17, API-18, API-19, API-20, API-21, API-22, API-23, API-24): one case per
      code in `contracts/error-mapping.contract.md` — the five dedicated kinds, and each of the seven
      fallback codes taking `unknown` with the raw code and message preserved — plus a code the backend
      does not define, and one case per kind asserting the resolved copy is byte-equal to the catalog
      entry (API-23)
- [ ] T016 [P] [US3] End-to-end failure test `web/src/test/api_failure_states.test.ts` (API-20, API-21,
      API-24): a transport failure with attempts exhausted leaves the client as `offline` with no status;
      a failed response leaves the client with the mapped kind, the status, the raw code and the server
      message

### Implementation for User Story 3

- [ ] T017 [US3] Implement `web/src/api/error_mapper.ts`: `apperrKind(code)`, the kind→key table and the
      copy resolver that reads `web/src/l10n/strings.ts` (API-15, API-16, API-17, API-18, API-19, API-20, API-21, API-22, API-23)
- [ ] T018 [US3] Attach the mapper inside `web/src/api/client.ts` so every domain failure leaving the
      client carries its kind, and a transport failure after its attempts maps to `offline` (API-20,
      API-21, API-24)

**Checkpoint**: every failure path out of the client carries a kind the interface can branch on.

---

## Phase 6: User Story 4 - The base address is configuration (Priority: P1)

**Goal**: a secure default, an explicit build-time override, validation before any request, and an
insecure state reported on the resolved configuration.

**Independent Test**: resolve with no override, with an override, with an empty override, with a
malformed value, and with a plaintext value; assert the address, its provenance and the security flag
each time.

### Tests for User Story 4 ⚠️

- [ ] T019 [P] [US4] Configuration test `web/src/test/api_config.test.ts` (API-25, API-26, API-27,
      API-28): no override resolves to the public API over HTTPS with `source: 'default'` and
      `secure: true`; an override is used verbatim with `source: 'override'`; an empty override falls
      back to the default; a bare host, a relative path and a non-HTTP scheme raise a configuration
      failure before any request; a plaintext override reports `secure: false` and `isDevOverride: true`

### Implementation for User Story 4

- [ ] T020 [P] [US4] Implement `web/src/api/config.ts`: the pure `resolveApiConfig(raw)` resolver and the
      `ApiConfiguration` shape (API-25, API-26, API-27, API-28), reading the build variable only at the
      module's boundary so the resolver stays testable without browser globals
- [ ] T021 [US4] Document and, where the app wires the client, use `resolveApiConfig` as the single
      source of the base address so the insecure state is reported rather than rendered (API-28)

**Checkpoint**: the base address is configuration, secure by default, and validated before use.

---

## Phase 7: User Story 5 - Predictable timeout, safe retry (Priority: P1)

**Goal**: a five-second fail-fast timeout per attempt, one read retry on transport failure only, and no
automatic write retry.

**Independent Test**: a transport failing the first read then succeeding; a transport failing twice; a
read answered with a failure status; a write that fails; and assertions of the retry delay and the
timeout.

### Tests for User Story 5 ⚠️

- [ ] T022 [P] [US5] Retry test `web/src/test/api_client_retry.test.ts` (API-30, API-31, API-32, API-33):
      a read failing at the transport is attempted exactly twice with the injected delay called once with
      the fixed interval; a read failing twice ends as `offline`; a read answered with a failure status
      is attempted once; a write failing at the transport or with a failure status is attempted once;
      and under fake timers an attempt is aborted at the five-second timeout and the default values are
      asserted

### Implementation for User Story 5

- [ ] T023 [US5] Implement the bounded attempts in `web/src/api/client.ts`: one abort controller per
      attempt bounded by the timeout, read-only single retry through the injectable sleep, and no retry
      for writes (API-30, API-31, API-32, API-33)

**Checkpoint**: the worst case until `offline` is bounded and asserted; writes can never double up.

---

## Phase 8: User Story 6 - The contract is pinned (Priority: P1)

**Goal**: real captures taken from the running backend, and a suite that fails when they drift.

**Independent Test**: run the suite with no backend; then alter a key in a capture copy and confirm the
suite turns red.

### Tests for User Story 6 ⚠️

- [ ] T024 [P] [US6] Capture-driven test `web/src/test/api_contract_captures.test.ts` (API-34, API-35,
      API-36): every success capture decodes through its operation's model and asserts the declared
      shape; the update pair asserts the request carries a single change and the response only the
      recalculated effects; the drift case mutates a capture copy and asserts the decode fails; the
      failure captures are driven through the mapper

### Implementation for User Story 6

- [ ] T025 [US6] Write the capture script `web/src/api/fixtures/capture.sh` and its provenance note
      `web/src/api/fixtures/README.md` per `contracts/capture-provenance.contract.md`: start the local
      stack, register a user, exercise the six operations and the failure cases, and write each raw
      response body plus its request into a JSON capture without persisting any credential (API-35)
- [ ] T026 [US6] Produce and pin the captures into `web/src/api/fixtures/` by running the script against
      the local stack (`make dev-db`, `go run ./cmd/api.go`); never hand-edit a capture to make a test
      pass (API-35)
- [ ] T027 [US6] Run the suite with the API and MongoDB stopped and confirm it is green, then confirm the
      drift case fails when a capture key is altered (API-34, API-36)

**Checkpoint**: the contract milestone is met — the suite validates real captures with no backend.

---

## Phase 9: User Story 7 - The real contract can be validated (Priority: P2)

**Goal**: a live suite that exercises the four real flows when a base address is configured and steps
aside when none is.

**Independent Test**: run with no `API_BASE_URL` and confirm the live cases skip and the run exits
successfully; run against a local backend and confirm the four flows; point it at a wrong address and
confirm it fails loudly.

### Tests for User Story 7 ⚠️

- [ ] T028 [P] [US7] Live suite `web/src/test/api_live_contract.test.ts` (API-37, API-38, API-39): gated on
      `API_BASE_URL` being present, exercising creating a sheet with derived values, updating an editable
      field to receive only recalculated effects, the refusal of a derived or unknown field, and a
      missing sheet; plus the two mechanism cases — that it skips without a base address, and that a
      configured-but-unreachable address fails naming the observed status instead of skipping

**Checkpoint**: the client is provably able to consume the real backend.

---

## Phase 10: Polish & Cross-Cutting Concerns

- [ ] T029 [P] Add the credential-hygiene source scan `web/src/test/api_no_logging.test.ts` (API-29):
      `web/src/api/` contains no `console.*` call and imports no logger, and every request the fake
      transport recorded carries `credentials: 'omit'`
- [ ] T030 [P] Extend `web/README.md` with how to run the module's suites, how to re-capture the contract
      fixtures, and how to run the live suite against a local backend; extend `web/AGENTS.md` with the
      module's rules (no logging, no implicit credentials, no user-facing literal outside the catalog,
      captures are never hand-edited)
- [ ] T031 Run the full gate set from `web/` (`npm run format:check`, `npm run lint`, `npm run typecheck`,
      `npm test`, `npm run build`) and `make lint-specs` from the repo root, and confirm every `API-*`
      citation in the delivered sources resolves to `spec.md`
- [ ] T032 Run the ten `quickstart.md` scenarios and record the outcome with the feature
- [ ] T033 Record any plan-versus-delivered deviation (dependency, pin or path that changed) in
      `web/AGENTS.md` under the deviations section rather than weakening an assertion (constitution III)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies — starts immediately.
- **Foundational (Phase 2)**: depends on Setup (the error shapes and the fake transport); blocks every
  story.
- **US1 (Phase 3)**: depends on Foundational; delivers the MVP.
- **US2 (Phase 4)**: depends on Foundational and on US1's client file (T014 extends T009).
- **US3 (Phase 5)**: depends on Setup (the catalog) and US1's client file (T018 extends T009).
- **US4 (Phase 6)**: depends on Setup only — the resolver is independent of the client, and T021 wires
  it where the app constructs the client.
- **US5 (Phase 7)**: depends on US1's client file (T023 extends T009).
- **US6 (Phase 8)**: depends on US1–US5 for the model and mapping suites it drives; produces the
  captures the other suites may reuse.
- **US7 (Phase 9)**: depends on the client being complete (US1, US3, US5); the captures are not a
  prerequisite.
- **Polish (Phase 10)**: depends on all desired stories.

### User Story Dependencies

- **US1 (P1)**: no dependency on other stories — the MVP.
- **US2 (P1)**: independent of US3/US4/US5; it types the payloads US1 already returns.
- **US3 (P1)**: independent of US2/US4/US5; it classifies the failures US1 already raises.
- **US4 (P1)**: independent of every other story; it resolves where the client points.
- **US5 (P1)**: independent of US2/US3/US4; it bounds the attempts US1 already makes.
- **US6 (P1)**: depends on US1–US3 and US5 for what it asserts, but each of those is independently
  testable without it.
- **US7 (P2)**: last, because it validates the finished client against a real service.

### Within Each User Story

- Tests are written first and must fail before the implementation lands (constitution III).
- Error shapes and the transport before the client; the client before the mapper is attached to it.
- Story complete before moving to the next priority.

### Parallel Opportunities

- T001, T002, T003 (setup) — different files.
- T004 and T005 — independent test files.
- T010 and T011, T012 and T013 — independent test/module pairs.
- T015 and T016 — independent test files.
- T019/T020 and T022 are independent of the model and mapper work.
- T029 and T030 — independent files.

---

## Parallel Example: User Story 2

```bash
# Tests first (independent files, must fail before T012/T013):
Task: "Template model test in web/src/test/api_models_template.test.ts"
Task: "Sheet model test in web/src/test/api_models_sheet.test.ts"

# Then the models:
Task: "Implement the template models in web/src/api/models/template.ts"
Task: "Implement the sheet model and dotted-path accessor in web/src/api/models/sheet.ts"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1 Setup → Phase 2 Foundational.
2. Phase 3 US1: six typed operations, envelope decoded once, payloads returned.
3. **STOP and VALIDATE** with `quickstart.md` scenarios 1 and 2.
4. The layer is already usable by a screen feature that does its own decoding — but only US2 makes it
   correct, so the MVP is a milestone, not a release.

### Incremental Delivery

1. Setup + Foundational → the transport seam and the decoder work.
2. US1 → the six operations exist (MVP).
3. US2 → the payloads are typed and faithful to the contract.
4. US3 → failures arrive classified, with copy from the catalog.
5. US4 → the base address is configuration, secure by default.
6. US5 → attempts are bounded and writes never repeat.
7. US6 → the contract is pinned and drift fails the build (the milestone the program plan calls for).
8. US7 → the real backend can be validated on demand.
9. Polish → documentation, the credential-hygiene scan, and the final gate sweep.

### Parallel Team Strategy

With two contributors: one takes US1 → US5 (the client and its transport behaviour) while the other
takes US2 and US3 (models and mapping), which share no file; US4 can be done by either at any point
because it shares no file with the client. US6's capture run needs the local stack and should be done by
one person, on the finished client.

---

## Notes

- [P] tasks touch different files and have no dependency on incomplete work.
- Requirement IDs cited in task descriptions (`API-*`) are defined in `spec.md`; the delivered source
  must cite the same IDs so `make lint-specs` can trace them.
- Write tests first (T004, T005, T008, T010, T011, T015, T016, T019, T022, T024, T028) and confirm they
  fail before implementing.
- Commit per task or per logical group, with the tests in the same commit (constitution III).
- Do not weaken an assertion to make a gate pass; amend the spec and record the decision instead.
- The module must never log a request body, a credential or an identifier-bearing URL, and must never
  send an ambient credential — check that rule on every task that touches `web/src/api/client.ts` or
  `web/src/api/transport.ts`.
- The capture set is generated, never hand-edited: a capture that stops matching the models is a backend
  contract change and the suite is expected to go red on it.
