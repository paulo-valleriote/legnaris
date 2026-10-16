# Feature Specification: Typed Web API Client

**Feature Branch**: `002-web-api-client`

**Created**: 2026-10-16

**Status**: Draft

**Input**: User description: "Web API client: a typed fetch wrapper covering the six consumed endpoints with envelope decoding, dynamic-data models, and apperr-to-domain error mapping"

## Clarifications

### Session 2026-10-16

- Q: Should this API-client feature also define the authentication credential models (the access/refresh token pair) that the later auth feature will consume? → A: Exclude them — the authentication feature owns credential models and endpoints; this feature builds the seam only.
- Q: Where should the visible warning for a non-secure (plaintext HTTP) base address live? → A: This feature exposes the insecure state on the resolved configuration; the interface warning is owned by the connection-indicator feature (no UI here).
- Q: Should this feature also ship the live contract suite that hits a real backend and steps aside when none is reachable? → A: Include it — pinned captures stay blocking in continuous integration, and the live suite runs only when a base address is configured.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - One typed call per consumed endpoint, one place that decodes the envelope (Priority: P1)

A contributor building a screen needs data from the backend. They call one named method per
endpoint the product consumes and receive a payload of the shape that endpoint documents. They never
write a URL, never branch on a status code, and never unwrap the response envelope themselves. Later,
when the product gains authentication, the credential is attached at the one place every call already
goes through, not in forty call sites.

**Why this priority**: it is the layer every screen feature mounts on. Without it, URL building,
envelope unwrapping and error branching spread across the interface and every later feature pays for
them again.

**Independent Test**: with a fake transport injected, call each of the six methods and confirm the
request method, path and body; return a success envelope and a failure envelope; return broken JSON
and confirm a defensive failure instead of a silent empty success.

**Acceptance Scenarios**:

1. **Given** the client is constructed with a transport, **When** each of the six consumed
   operations is invoked, **Then** exactly one typed method exists per operation — health, template
   catalog, template detail, sheet creation, sheet retrieval, sheet update — and each returns the
   payload decoded for that operation. (API-01)
2. **Given** a successful response carrying the data envelope, **When** the client decodes it,
   **Then** the payload is returned in the type that operation documents. (API-02)
3. **Given** a failed response carrying the error envelope, **When** the client decodes it, **Then**
   the call fails with a domain error carrying the server's code and message. (API-03)
4. **Given** a successful response whose body is not valid JSON or has no data key, **When** the
   client decodes it, **Then** the call fails with a defensive parse error and never returns an empty
   success. (API-04)
5. **Given** a sheet creation request, **When** the client sends it, **Then** the body carries exactly
   the template identifier and the sheet name. (API-05)
6. **Given** a sheet update request, **When** the client sends it, **Then** the body carries exactly
   the caller's change set and nothing else. (API-06)
7. **Given** any operation, **When** it issues a request, **Then** it goes through the transport
   supplied at construction — no operation builds its own transport — so one place owns the credential
   attachment point. (API-07)

---

### User Story 2 - Models faithful to the contract, dynamic data and path access (Priority: P1)

A contributor renders whatever schema the template returns. The models hand them the declared field
tree with its leaf and group shape, and the sheet's data as the dynamic structure it actually is,
addressable by the dotted paths the schema declares (`stats.dexterity`). A path that points at
nothing is "no value", not a crash; a path that is malformed is a bug and says so.

**Why this priority**: the product's whole value is rendering an arbitrary schema, so misreading the
contract here contaminates every later feature.

**Independent Test**: decode the pinned template and sheet contract captures; read a valid path, a
missing path, a path whose intermediate is not an object, and an empty path; decode whole and
fractional numbers and confirm neither is altered.

**Acceptance Scenarios**:

1. **Given** a catalog or detail payload, **When** it is decoded, **Then** the model exposes its
   identifier, name, default flag and field tree, preserving the leaf/group distinction (a leaf
   carries its schema, a group carries its children). (API-08)
2. **Given** a leaf schema, **When** it is decoded, **Then** the model exposes type, label, formula,
   dependencies, editability and default, treating formula and dependencies as absent when the payload
   omits them. (API-09)
3. **Given** a sheet payload, **When** it is decoded, **Then** the model exposes identifier, template
   identifier, name, data, and both timestamps, with data preserving its nested dynamic shape.
   (API-10)
4. **Given** a numeric value inside data, **When** it is decoded, **Then** whole numbers and
   fractional numbers are preserved as they arrived, without truncation or forced coercion. (API-11)
5. **Given** a dotted path that resolves, **When** it is read, **Then** the value it points at is
   returned. (API-12)
6. **Given** a dotted path whose segment is missing, or whose intermediate node is not an object,
   **When** it is read, **Then** the result is "no value" and no exception is raised. (API-13)
7. **Given** an empty path or a path with an empty segment, **When** it is read, **Then** a typed
   path error is raised. (API-14)

---

### User Story 3 - Every failure becomes a state the interface can render (Priority: P1)

A contributor handles a failed call by looking at which kind of failure it is — an invalid edit to
revert, a resource that no longer exists, a server fault to retry by hand, no connectivity — instead
of parsing a JSON body in every screen. The wording shown to the person is resolved from the
translation layer, never typed into the client.

**Why this priority**: turning a response into interface state is the contract that makes the later
screen features possible; it is also where a wrong message reaches a person.

**Independent Test**: drive a table of captured failure bodies through the mapper and confirm each
kind; send an unmapped code and confirm the generic kind keeps the raw code and message; send a body
without a code and confirm a defensive parse failure; fail the transport and confirm the offline
kind; render a copy and confirm it comes from the translation layer.

**Acceptance Scenarios**:

1. **Given** the backend reports an invalid client request body, **When** the failure is mapped,
   **Then** its kind is the client-fault kind. (API-15)
2. **Given** the backend reports an invalid edit, **When** the failure is mapped, **Then** its kind
   is the invalid-edit kind. (API-16)
3. **Given** the backend reports an unknown field, **When** the failure is mapped, **Then** its kind
   is the render-mismatch kind. (API-17)
4. **Given** the backend reports a missing resource, **When** the failure is mapped, **Then** its
   kind is the not-found kind. (API-18)
5. **Given** the backend reports an internal fault, **When** the failure is mapped, **Then** its kind
   is the server-error kind. (API-19)
6. **Given** a request fails at the transport after its attempts are exhausted, **When** the failure
   is mapped, **Then** its kind is the offline kind. (API-20)
7. **Given** the backend reports a code outside the mapping — a conflict, a dependency cycle, a
   refusal because the caller is not permitted — **When** the failure is mapped, **Then** its kind is
   the generic kind and the error keeps the raw code and the server message. (API-21)
8. **Given** a failed response whose body is not valid JSON or has no error code, **When** it is
   decoded, **Then** the call fails with a defensive parse error rather than a mapped domain failure.
   (API-22)
9. **Given** any failure that reaches a person, **When** its wording is produced, **Then** it is
   resolved from the translation layer with English as the only locale, and no user-facing literal
   exists in the client. (API-23)
10. **Given** a domain failure is produced, **When** it is inspected, **Then** it carries the HTTP
    status when one was received, the backend code when present, and the server message, so later
    logging never has to repeat the call. (API-24)

---

### User Story 4 - The base address is configuration, secure by default (Priority: P1)

A contributor builds the client for production and it targets the public API over TLS without anyone
setting a value. Running locally against a tunnel or a local backend, they point it somewhere else
with an explicit build-time value, and when that value is not TLS the resolved configuration reports
the connection as not secure, so a later interface feature can warn that an insecure build is running.

**Why this priority**: without it no environment reaches a backend at all; with it wrong, a build can
silently talk to the wrong host or over plaintext.

**Independent Test**: build with no override and confirm the public TLS address; build with an
override and confirm it is used verbatim; supply a malformed or empty value and confirm a
configuration failure before any request is issued; supply a plaintext address and confirm the
configuration reports it as not secure.

**Acceptance Scenarios**:

1. **Given** no base address is configured for the build, **When** the client resolves its base
   address, **Then** it uses the public API over HTTPS. (API-25)
2. **Given** a build-time override is supplied, **When** the client resolves its base address,
   **Then** it uses the supplied value exactly. (API-26)
3. **Given** the resolved base address is not an absolute HTTP or HTTPS address, **When** the client
   is constructed, **Then** it fails with a configuration error before emitting any request. (API-27)
4. **Given** the resolved base address is plaintext HTTP, **When** the configuration is inspected,
   **Then** it reports the connection as not secure so the interface can warn about it. (API-28)

---

### User Story 5 - Predictable timeout, retry only where it is safe (Priority: P1)

A contributor expects the interface to fall to a clear offline state in a bounded time when the
network is gone, and expects that a change the person made is never silently sent twice.

**Why this priority**: it is the precondition for the offline state later screen features render, and
the guarantee that keeps duplicate writes out of the product.

**Independent Test**: a fake transport that fails the first read and succeeds the second; a transport
that fails twice; a read answered with a server fault; a write that fails; and an assertion of the
timeout and the interval between attempts.

**Acceptance Scenarios**:

1. **Given** any request attempt, **When** it is issued, **Then** it is bounded by a fail-fast
   timeout of five seconds. (API-30)
2. **Given** a read attempt fails at the transport level — timeout, refused connection, name
   resolution — **When** the client recovers, **Then** it repeats the read exactly once after a fixed
   delay, and the delay is injectable so its value is asserted deterministically. (API-31)
3. **Given** a read receives any response, including a failure response, **When** the client
   evaluates it, **Then** it does not repeat the request and surfaces the mapped failure immediately.
   (API-32)
4. **Given** a write attempt fails for any reason, **When** the client recovers, **Then** it does not
   repeat the request. (API-33)

---

### User Story 6 - The contract is pinned, so drift breaks the build (Priority: P1)

A contributor changes something in the client or the backend and the pinned, captured responses stop
matching what the client expects — the test suite fails before a screen shows the wrong thing.

**Why this priority**: it is the milestone that declares the contract validated, and the only
protection once the backend and the client evolve separately.

**Independent Test**: run the client suite with no backend reachable; then alter a key in one capture
and confirm the suite turns red.

**Acceptance Scenarios**:

1. **Given** the client test suite, **When** it runs locally and in continuous integration, **Then**
   it executes against pinned captures without requiring a reachable backend and exits successfully.
   (API-34)
2. **Given** the capture set, **Then** it contains at least one real response for each of the six
   consumed operations, the failure envelope, and the update pair whose request is a single change and
   whose response contains only the recalculated effects. (API-35)
3. **Given** a capture whose envelope or field shape diverges from what the models expect, **When**
   the suite runs, **Then** it fails. (API-36)

---

### User Story 7 - The real contract can be validated when a backend is available (Priority: P2)

A contributor with a backend running — local or deployed — asks the suite to check the client against
the real service; with none configured, the check steps aside rather than failing the build.

**Why this priority**: it does not block the contract milestone the captures already cover, but it is
the only direct evidence that the client consumes the real service.

**Independent Test**: run the suite with no base address for live checks and confirm it steps aside
and exits successfully; run it against a local backend and confirm the four real flows; point it at
the wrong address and confirm it fails loudly.

**Acceptance Scenarios**:

1. **Given** no reachable base address is configured for live checks, **When** the suite runs, **Then**
   the live checks step aside and the overall run exits successfully. (API-37)
2. **Given** a reachable base address is configured, **When** the suite runs, **Then** it executes the
   four real flows: creating a sheet returns a created sheet with derived values; updating an editable
   field returns only recalculated effects; updating a derived or unknown field is refused as an
   invalid edit or an unknown field; and retrieving a sheet that does not exist reports it missing.
   (API-38)
3. **Given** a reachable base address is configured and a flow returns an unexpected status or code,
   **When** the suite runs, **Then** it fails reporting the status and code it observed, never
   stepping aside silently. (API-39)

---

### Edge Cases

- **Credential hygiene**: no operation may cause ambient cookies to be attached implicitly, and no
  request body, credential or identifier-bearing URL may reach a log. The one place that will attach
  a credential is the injected transport; this feature guarantees the seam and the default. (API-07,
  API-29)
- **Empty success body**: a successful response with no JSON body is a defensive failure, never an
  empty success (API-04); every consumed backend operation returns a body today, so this is contract
  drift, not a normal case.
- **Empty override value**: an empty build-time override falls back to the secure default rather than
  producing an empty base address (API-27).
- **Plaintext base address**: accepted only as an explicit build-time choice; without it the base
  address is the HTTPS default (API-25, API-28). The insecure state is reported on the resolved
  configuration — the interface feature that will show the warning consumes it, and no interface
  change is part of this feature.
- **Update capture semantics**: the pinned update response contains only the cascading effects; merging
  them into the sheet is a later feature, and this feature only decodes them (API-35).
- **Reserved key in the payload**: the template payload's `default` key must be read explicitly, not
  through an automatic naming convention, so a mapping change cannot silently drop it (API-08).
- **Unmapped backend codes**: authorization and refusal codes exist in the backend today but no
  interface treatment is defined for them in this phase, so they take the generic kind and keep their
  raw code for the feature that will define the treatment (API-21).
- **Dimension sweep** (explicitly covered / not applicable here):
  - *Input validation and bounds* — defensive envelope parsing (API-04, API-22), base address
    validation (API-27), dotted-path validation (API-14).
  - *Failure and partial failure* — the offline kind with no partial state; the client holds no state.
  - *Idempotency and retry* — API-30 … API-33.
  - *Observability* — status, code and message travel on the error (API-24); nothing is logged
    (API-29).
  - *External dependency failure* — retry plus the offline kind (API-20, API-31).
  - *Not applicable*: authentication and rate limits (this feature does not authenticate; the
    transport is the seam the authentication feature attaches to), concurrency and ordering (no shared
    state), data lifecycle and expiry (the client caches nothing).

## Requirements *(mandatory)*

### Functional Requirements

**Typed operations and envelope decoding**

- **API-01**: The client MUST expose exactly one typed method per consumed operation — health,
  template catalog, template detail, sheet creation, sheet retrieval, sheet update — each returning
  the payload decoded for that operation.
- **API-02**: WHEN a successful response carries the data envelope, the client MUST return the payload
  decoded in the type that operation documents.
- **API-03**: WHEN a failed response carries the error envelope, the client MUST fail with a domain
  error carrying the reported code and message.
- **API-04**: IF a successful body is not valid JSON or carries no data key, THEN the client MUST fail
  with a defensive parse error and MUST NOT return an empty success.
- **API-05**: WHEN the client creates a sheet, the request body MUST carry exactly the template
  identifier and the sheet name.
- **API-06**: WHEN the client updates a sheet, the request body MUST carry exactly the change set the
  caller supplied and nothing else.
- **API-07**: The client MUST issue every request through the transport supplied at construction; no
  operation may create its own transport, and the transport MUST be the single documented extension
  point for attaching credentials.

**Contract-faithful models**

- **API-08**: WHEN a catalog or detail payload is decoded, the model MUST expose identifier, name,
  default flag and field tree, preserving the leaf/group distinction; the payload's `default` key MUST
  be mapped explicitly rather than through a naming convention.
- **API-09**: WHEN a leaf schema is decoded, the model MUST expose type, label, formula, dependencies,
  editability and default, and MUST treat omitted formula and dependencies as absent (not as empty
  strings or empty lists).
- **API-10**: WHEN a sheet payload is decoded, the model MUST expose identifier, template identifier,
  name, data, and both timestamps, with data preserving its nested dynamic shape.
- **API-11**: WHEN a number is decoded, whole and fractional values MUST be preserved as received,
  without truncation or forced coercion.
- **API-12**: WHEN a dotted path resolves against sheet data, the accessor MUST return the value it
  points at.
- **API-13**: IF a dotted path segment is missing or an intermediate node is not an object, THEN the
  accessor MUST yield no value and MUST NOT raise.
- **API-14**: IF a dotted path is empty or contains an empty segment, THEN the accessor MUST raise a
  typed client error.

**Failures as interface states**

- **API-15**: WHEN the backend reports an invalid request body, the client MUST map it to the
  client-fault kind.
- **API-16**: WHEN the backend reports an invalid edit, the client MUST map it to the invalid-edit
  kind.
- **API-17**: WHEN the backend reports an unknown field, the client MUST map it to the render-mismatch
  kind.
- **API-18**: WHEN the backend reports a missing resource, the client MUST map it to the not-found
  kind.
- **API-19**: WHEN the backend reports an internal fault, the client MUST map it to the server-error
  kind.
- **API-20**: WHEN a request fails at the transport with its attempts exhausted, the client MUST map
  it to the offline kind.
- **API-21**: IF the backend reports a code outside the mapping, THEN the client MUST map it to the
  generic kind while preserving the raw code and the server message on the error.
- **API-22**: IF a failed body is not valid JSON or carries no error code, THEN the client MUST fail
  with a defensive parse error instead of a mapped domain failure.
- **API-23**: The client MUST resolve every user-visible message through the translation layer, with
  English as the only locale, and MUST NOT contain hardcoded user-facing strings.
- **API-24**: WHEN a domain failure is produced, it MUST carry the HTTP status when received, the
  backend code when present, and the server message.

**Configuration and security of the transport**

- **API-25**: The client MUST use the public API over HTTPS as its base address when the build
  configures none.
- **API-26**: WHEN the build supplies an override, the client MUST use it exactly; an empty override
  MUST fall back to the HTTPS default.
- **API-27**: IF the resolved base address is not an absolute HTTP or HTTPS address, THEN the client
  MUST fail with a configuration error before emitting any request.
- **API-28**: WHEN the resolved base address is plaintext HTTP, the client MUST report the connection
  as not secure on the resolved configuration, so a later interface feature can warn about it; this
  feature ships no such warning itself.
- **API-29**: The client MUST NOT log request bodies, credentials, or identifier-bearing URLs, and
  MUST NOT attach ambient credentials to requests implicitly.

**Bounded attempts**

- **API-30**: The client MUST bound every request attempt with a fail-fast timeout of five seconds.
- **API-31**: WHEN a read fails at the transport level, the client MUST repeat it exactly once after a
  fixed, injectable delay.
- **API-32**: WHEN a read receives any response, the client MUST NOT repeat the request and MUST
  surface the mapped failure immediately.
- **API-33**: WHEN a write fails for any reason, the client MUST NOT repeat the request.

**Pinned contract**

- **API-34**: The client suite MUST run against pinned captures without a reachable backend and MUST
  exit successfully.
- **API-35**: The capture set MUST contain at least one real response per consumed operation, the
  failure envelope, and the update pair whose request carries a single change and whose response
  carries only recalculated effects.
- **API-36**: IF a capture's envelope or field shape diverges from what the models expect, THEN the
  suite MUST fail.
- **API-37**: WHEN no reachable base address is configured for live checks, the live suite MUST step
  aside and the overall run MUST exit successfully.
- **API-38**: WHEN a reachable base address is configured, the live suite MUST exercise the four real
  flows (create with derived values, update returning only recalculated effects, refusal of a derived
  or unknown field, and a missing resource).
- **API-39**: IF a configured live check observes an unexpected status or code, THEN the suite MUST
  fail reporting what it observed and MUST NOT step aside silently.

### Key Entities

- **Response envelope**: the wrapper every backend response uses; carries either a data payload or an
  error with a code and a message.
- **Consumed operation**: one of the six backend operations the product calls, carrying its HTTP
  method, path, request body shape and decoded payload type.
- **Domain failure**: the client's representation of a failed call; carries the failure kind, the HTTP
  status when received, the raw backend code when present, and the server message.
- **Failure kind**: the interface-facing class of a failure (client fault, invalid edit, render
  mismatch, missing resource, server fault, offline, generic) that a screen branches on.
- **Defensive parse failure**: the client's representation of a response that does not match the
  envelope, distinct from a mapped domain failure so contract drift is distinguishable.
- **Template catalog entry / template detail**: the decoded template shape, carrying identifier, name,
  default flag and the field tree.
- **Field node**: a leaf carrying its schema, or a group carrying its children.
- **Field schema**: a leaf's declared contract — type, label, formula, dependencies, editability,
  default, and the exclusive-group and option/column metadata the backend emits.
- **Sheet**: a stored sheet — identifier, template identifier, name, dynamic data, created and
  updated timestamps.
- **Dotted path**: the schema-declared address of a value inside sheet data (`stats.dexterity`).
- **Base address configuration**: the resolved API origin, its provenance (default or build override)
  and whether it is secure.
- **Transport**: the injected request function; the single extension point through which credentials
  will later be attached.
- **Pinned capture**: a recorded real response used as a contract fixture in the suite.

### Requirement Traceability

Every requirement carries a unique identifier so it can be traced through design, tasks and
validation.

| Requirement ID | Story |
| -------------- | ----- |
| API-01 | US1: One typed call per consumed endpoint |
| API-02 | US1: One typed call per consumed endpoint |
| API-03 | US1: One typed call per consumed endpoint |
| API-04 | US1: One typed call per consumed endpoint |
| API-05 | US1: One typed call per consumed endpoint |
| API-06 | US1: One typed call per consumed endpoint |
| API-07 | US1: One typed call per consumed endpoint |
| API-08 | US2: Contract-faithful models |
| API-09 | US2: Contract-faithful models |
| API-10 | US2: Contract-faithful models |
| API-11 | US2: Contract-faithful models |
| API-12 | US2: Contract-faithful models |
| API-13 | US2: Contract-faithful models |
| API-14 | US2: Contract-faithful models |
| API-15 | US3: Failures as interface states |
| API-16 | US3: Failures as interface states |
| API-17 | US3: Failures as interface states |
| API-18 | US3: Failures as interface states |
| API-19 | US3: Failures as interface states |
| API-20 | US3: Failures as interface states |
| API-21 | US3: Failures as interface states |
| API-22 | US3: Failures as interface states |
| API-23 | US3: Failures as interface states |
| API-24 | US3: Failures as interface states |
| API-25 | US4: Base address as configuration |
| API-26 | US4: Base address as configuration |
| API-27 | US4: Base address as configuration |
| API-28 | US4: Base address as configuration |
| API-29 | US4: Base address as configuration |
| API-30 | US5: Predictable timeout, safe retry |
| API-31 | US5: Predictable timeout, safe retry |
| API-32 | US5: Predictable timeout, safe retry |
| API-33 | US5: Predictable timeout, safe retry |
| API-34 | US6: Pinned contract |
| API-35 | US6: Pinned contract |
| API-36 | US6: Pinned contract |
| API-37 | US7: Live contract, self-skipping |
| API-38 | US7: Live contract, self-skipping |
| API-39 | US7: Live contract, self-skipping |

**Coverage:** 39 requirements, each mapped to exactly one user story; 0 unmapped.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of the six consumed operations are reachable through one named method, with the
  envelope decoded in a single place; 0 call sites build a URL or unwrap an envelope themselves.
- **SC-002**: 100% of the mapping table (5 backend codes plus transport failure plus generic fallback
  plus defensive parse) is verified by a captured failure body or a simulated transport failure.
- **SC-003**: 0 user-facing strings are hardcoded in the client; every rendered failure message
  resolves through the translation layer.
- **SC-004**: A changed key in a pinned capture turns the client suite red on the first run
  (0 false negatives).
- **SC-005**: Writes are never repeated automatically and reads repeat exactly once on a transport
  failure; the worst case until the offline kind is bounded and asserted.
- **SC-006**: A build with no override targets the public API over HTTPS, a build with an override
  targets the supplied address, a malformed address fails before any request is issued, and a
  plaintext address is reported as not secure by the resolved configuration.
- **SC-007**: 0 request bodies, credentials or identifier-bearing URLs are emitted to logs; 0 requests
  carry ambient credentials implicitly.
- **SC-008**: The client suite runs green with no backend reachable, and one configured live run
  exercises the four real flows.

## Assumptions

- The backend contract is the source of truth and is already stable: the envelope, the error codes,
  the template and sheet payload shapes and the six consumed operations are taken from
  `shared/response`, `shared/apperr`, `internal/template` and `internal/sheet`, and the pinned
  captures are real recorded responses for those shapes.
- Two backend codes that the older desktop client mapped are already handled as an invalid edit and an
  unknown field by the current service; the mapping table therefore covers every code the backend can
  emit today plus the generic fallback.
- Authentication is a later feature: this feature builds the seam (injected transport, no implicit
  credentials) but attaches no credential and defines no credential model. Refusal and authorization
  codes take the generic kind until that feature defines their interface treatment.
- Only the six operations the product consumes are in scope. The backend also exposes listing the
  caller's own sheet, deleting a sheet and previewing a change; those belong to the sheet-cycle and
  preview features.
- The interface states that consume these kinds, the query cache and the connection indicator belong
  to later features; this feature delivers a state-free module and reports the insecure base address
  on its resolved configuration rather than rendering anything.
- Continuous integration already runs the client suite, so a pinned capture that drifts fails a pull
  request without any new gate.
- All identifiers, comments and user-facing strings remain English-only, per the project
  constitution.
