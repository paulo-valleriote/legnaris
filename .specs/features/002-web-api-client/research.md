# Phase 0 Research: Typed Web API Client

**Feature**: `.specs/features/002-web-api-client/` | **Date**: 2026-10-16

Every decision below answers an open question in the feature's Technical Context or in the program
plan's F2 task list. No `NEEDS CLARIFICATION` remains.

---

## R1. How the client reaches the network

**Decision**: the client depends on an injected *transport* — a function taking a request description
(`url`, `method`, `headers`, `body`, `credentials`, `signal`) and returning `{ status, body }`. The
browser implementation is a thin adapter over the platform `fetch`; tests pass a fake.

**Rationale**: the contract requires that no operation builds its own transport and that the transport
is the single extension point for credentials (API-07). Making it a narrow function instead of the
platform `fetch` signature also makes the security requirements directly assertable: the request
description is an inspectable value, so "no implicit credentials" (API-29) becomes an assertion on
`credentials: 'omit'` rather than an inference about browser behaviour, and the retry suite can count
attempts without a server.

**Alternatives considered**:

- *Platform `fetch` injected directly* — closest to "the platform", but a fake then has to fabricate
  `Response` objects, request inspection goes through the `Request` API, and `credentials` is buried in
  `RequestInit`. Rejected: more friction for the same guarantees.
- *MSW, as the program plan's stack table names* — real network-level interception, valuable for the
  authentication feature's silent-refresh flow. Rejected **here**: it adds a dependency the contract
  does not need, and call-count/retry assertions are less direct through an interceptor. Recorded in
  the plan as a deliberate deferral.
- *A wrapper library (axios and similar)* — rejected: no runtime dependency is justified for six
  operations, and the envelope decoding is the feature's actual work.

## R2. Where the base address comes from

**Decision**: a resolver takes the raw build-time value and returns a resolved configuration
`{ baseUrl, source, secure }`. The default is `https://api.legnaris.com.br`; the override is the Vite
build variable `VITE_API_BASE_URL`; an empty override falls back to the default; a value that is not an
absolute `http://`/`https://` address raises a configuration error at construction, before any request.
`secure` is `true` only when the resolved address uses `https:`.

**Rationale**: the plan asks for "https by default; `http://` only in development (with a warning)".
The desktop phase used a compile-time define because Dart had no equivalent; Vite's `import.meta.env`
is the web equivalent and is likewise build-time, so no user-facing setting exists and no production
build can be pointed at a tunnel without an explicit build argument. Resolving in a pure function (raw
string in, configuration out) keeps the browser globals out of the tested path. Per the clarification
in the spec, the insecure state is reported on the configuration only; the interface warning is the
connection-indicator feature's job.

**Alternatives considered**:

- *Runtime configuration (a `config.json` fetched at boot)* — rejected: it would let a deployment
  environment change the target without a rebuild, which is exactly what the plan's "hardcoded +
  dev-only override" decision excludes, and it adds a request before the first real one.
- *Throwing on `http://` outside a development build* — rejected: the plan wants the local tunnel to
  work from any build, with the risk made visible instead of blocked (the user chose the flag-only
  variant of this).
- *Accepting a relative address (same-origin `/api`)* — rejected: the app and the API are different
  origins in every environment the plan describes, and a relative address would silently pass
  validation while resolving to the frontend host.

## R3. Where the envelope is decoded

**Decision**: one exported decoder, `decodeEnvelope({ status, body, decode })`. It parses the body once;
on 2xx it requires an object with a `data` key and hands `data` to the per-operation `decode` callback;
on non-2xx it requires `error.code` (a non-empty string) and throws a domain error carrying the status,
the raw code and the server message; any deviation — empty body, invalid JSON, a 2xx without `data`, a
failure without `error.code` — throws the defensive parse error, which is a distinct type.

**Rationale**: mirrors the desktop phase's decoder one-to-one (the spec's requirements were written for
it), so the migration is a port rather than a redesign, and it gives the two failure classes the
interface needs: "the server said no" versus "we cannot understand the server" (API-04, API-22).

**Alternatives considered**:

- *Per-operation parsing* — rejected: it duplicates the contract in six places and makes drift silent.
- *A schema-validation library (Zod and similar)* — rejected: a new runtime dependency to re-implement
  checks the decoder already performs; the models' defensive parse (R7) covers drift where it matters.
- *Returning a result union (`{ ok: true, data } | { ok: false, error }`) instead of throwing* —
  rejected: the six operations are already promise-based, and a thrown typed error is what the retry
  and mapping layers branch on; a union would double the branching in every caller.

## R4. How failures are classified

**Decision**: a string-literal union `FailureKind` with seven members — `clientFault`, `invalidEdit`,
`renderMismatch`, `notFound`, `serverError`, `offline`, `unknown` — plus `apperrKind(code)` mapping the
backend code to a kind. The mapping covers the five codes with a dedicated kind; every other code the
backend can emit (conflict, cycle, invalid dependency, invalid path, sheet limit, unauthorized,
forbidden) takes `unknown` while the error keeps the raw code and message. The domain error carries
`kind`, `status`, `code` and `message`.

**Rationale**: this is the contract that lets a screen branch on a failure without parsing JSON, and it
is where an unmapped code must not silently lose information (API-21, API-24). Enumerating all twelve
backend codes in `shared/apperr/err.go` is what makes the fallback a decision rather than an omission —
each of the seven fallback codes is listed in the spec's edge cases with the reason it has no interface
treatment yet.

**Alternatives considered**:

- *An error class hierarchy (one class per kind)* — rejected: `instanceof` chains across module
  boundaries break under duplicate module instances and are harder to assert than a discriminant field;
  the desktop phase used an enum, and this mirrors it.
- *Mapping by HTTP status instead of the backend code* — rejected: the spec's table is code-based, and
  two different codes share a status (400), so status alone cannot distinguish an invalid edit from a
  render mismatch.
- *Surfacing `UNAUTHORIZED` as its own kind now* — rejected: the authentication feature owns the
  session, and inventing its interface treatment here would bake in behaviour nothing implements yet
  (recorded as an edge case).

## R5. Timeout and retry

**Decision**: one `AbortController` per attempt, aborted after `requestTimeoutMs` (default 5000), and a
request-level `timeoutSignal` combined so a caller can cancel too. Reads repeat exactly once after
`retryDelayMs` (default 300) **only** when the transport itself fails; a received response — including
4xx and 5xx — ends the attempt. Writes never repeat. The sleep used for the delay is injectable.

**Rationale**: the plan's fail-fast decision (5 s, one read retry, no write retry) is what bounds the
worst case until the offline kind (≈10.3 s) and keeps a duplicate write out of the product (API-30…API-33). Injecting the sleep keeps the delay assertion exact without making the suite wait.

**Alternatives considered**:

- *Blanket retry with backoff* — rejected: it re-sends non-idempotent writes and multiplies the time
  until the interface can say "offline", both of which the spec forbids.
- *Retrying on 5xx* — rejected: a received response is definitive; retrying it would turn a visible
  server fault into a slow one.
- *Real timers in the retry suite* — rejected: a 300 ms real wait per case makes the suite slow and
  flaky; the injected sleep asserts the value instead. The timeout path does use fake timers, because
  the abort is what is under test there.

## R6. How models are built

**Decision**: plain parse functions returning readonly structural types — `parseTemplateCatalog`,
`parseTemplateDetail`, `parseSheet` — with no classes and no runtime schema library. A field node is
either a leaf (`schema`) or a group (`children`), decided by which key the payload carries. Omitted
`formula` and `depends_on` become `undefined`, not `''`/`[]`. The payload's `default` key is read
explicitly. `data` is `Record<string, unknown>` and is never traversed by anything but the path accessor.

**Rationale**: `data` is intentionally dynamic — the product renders an arbitrary schema — so the type
must stay open while the declared parts stay strict (API-08, API-09, API-10). Structural types keep the
module free of framework and class identity, and explicit key reads are what prevent a contract key
from being dropped by a naming convention (the desktop phase's reserved-word note becomes an explicit
note here).

**Alternatives considered**:

- *Classes with constructors* — rejected: no behaviour to attach, and object identity complicates the
  fixture drift assertions.
- *Zod or a similar validator for the tree* — rejected: same dependency objection as R3; the tree is
  recursive and small enough that an explicit parser is clearer and its failures exact.
- *`unknown` everywhere with no model* — rejected: the later engine features need the declared shape
  (`type`, `is_editable`, `depends_on`) typed, and getting it wrong contaminates every screen.

## R7. Defensive parsing and the "no eval" rule

**Decision**: all decoding is `JSON.parse` plus explicit field reads. A payload that does not match the
expected shape raises the defensive parse error naming the field that failed. No `eval`, no
`new Function`, no `dangerouslySetInnerHTML`, and no string-to-code path anywhere in the module — the
ESLint rules F1 enabled already fail the build on those.

**Rationale**: the program plan's F2-T002 states it explicitly ("dynamic `data` without `eval`"), and
the security gates require that the data path from the network to the interface never becomes an
execution path.

**Alternatives considered**:

- *Trusting the payload and casting* — rejected: contract drift would surface as `undefined` deep in a
  render instead of a named parse failure.
- *Coercing mismatched values into the expected type* — rejected: silent coercion hides drift, which is
  the failure mode API-36 exists to catch.

## R8. Where the pinned captures come from

**Decision**: the capture set is produced by a documented script that starts the local stack
(`make dev-db`, `go run ./cmd/api.go`), registers a user, creates a sheet, exercises the six operations
and the failure cases, and writes the raw response bodies into `web/src/api/fixtures/`. The script and a
provenance note ship with the fixtures; each capture records which operation produced it.

**Rationale**: the desktop phase's milestone was "fixtures of the real contract, passing in CI", and
that is only honest if the captures are real. The local stack is available and the backend is the same
one the client will talk to, so the capture is reproducible rather than transcribed. It also yields the
failure envelopes for the whole mapping table — `NOT_FOUND`, `INVALID_REQUEST`, `UNKNOWN_FIELD`,
`INVALID_JSON`, `INTERNAL_ERROR`, and real `UNAUTHORIZED`/`CONFLICT`-class codes for the fallback — which
a hand-written fixture could only imitate.

**Alternatives considered**:

- *Copying the deprecated desktop client's captures* — attractive (they are real captures of the same
  backend) but they cover only the six operations and one error, and they would drift independently the
  moment the web set is refreshed. Used as prior art and as a cross-check, not as the source.
- *Hand-written fixtures* — rejected: it weakens the milestone to "fixtures match our own JSON".
- *Recording against a deployed API* — rejected: it needs credentials for an environment this feature
  does not have to depend on.

## R9. How the live suite gates itself

**Decision**: the live suite runs only when `API_BASE_URL` is present in the environment of the test
run; otherwise it reports as skipped. When a base address is present it exercises four real flows and
fails, naming the observed status and code, on anything unexpected.

**Rationale**: this is the repository's established convention for anything needing an external service
(integration tests self-skip without MongoDB), so it keeps continuous integration green without a
backend while still allowing a real contract check (API-37…API-39). Using a plain environment variable
rather than a build variable matters: the live suite must be switchable at run time, not baked at build
time like the base address of R2.

**Alternatives considered**:

- *An opt-in flag in the Vitest config* — rejected: an environment variable is the pattern already used
  in this repository and needs no config edit to run.
- *Skipping on a connection failure instead of on absence* — rejected: that is the "silent skip" API-39
  forbids; a configured base address that does not answer is a failure.

## R10. Where user-facing copy lives

**Decision**: one typed English catalog in `web/src/l10n/strings.ts`, keyed by purpose, with a
`message(key)` accessor. The error mapper resolves a kind to a catalog key; no copy literal exists in
`web/src/api/`.

**Rationale**: the contract requires every visible message to come from the translation layer, with
English as the only locale, and no hardcoded user-facing literal in the client (API-23). One catalog is
also the shape the localization feature will extend without rewriting call sites. A test asserts the
resolved copy equals the catalog entry, which is the honest form of "it comes from the catalog" (the
desktop phase asserted the same way).

**Alternatives considered**:

- *`i18next` or another framework* — allowed by the program plan, rejected: one locale and a dozen keys
  do not justify a runtime dependency; the plan's own alternative is "a module of typed keys".
- *Message text on the failure kinds* — rejected: it puts copy in the network layer and makes the
  English-only rule enforceable only by review.
- *A source-scanning test that forbids string literals in `web/src/api/`* — rejected as the primary
  check: it is brittle (paths, headers and keys are literals too) and would assert syntax instead of
  behaviour. A source scan is used only to assert the security rule of R11, where the literal itself is
  the risk.

## R11. Credential hygiene and logging

**Decision**: every request carries `credentials: 'omit'` explicitly, so no ambient cookie is attached
even once the authentication feature exists on another origin; the module contains no logging call, and
a source-scanning test asserts that (`console.*` and any logger import are absent from `web/src/api/`).
The transport request shape carries a `headers` map, which is where the authentication feature will add
its bearer header.

**Rationale**: the plan's F2-T001 is a security requirement — "never log the token or the body" — and
the constitution makes security gates blocking. The cheapest honest guarantee at this layer is to have
no logging path at all, plus the explicit credential posture, both tested at the seam.

**Alternatives considered**:

- *Logging with redaction* — rejected: a redaction routine is code that can be wrong; a layer with no
  logging cannot leak. Observability that needs request data belongs to the feature that owns the
  session.
- *Leaving `credentials` at the browser default* — rejected: the default is origin-dependent and would
  become a silent behaviour change when the app and the API are same-site subdomains. Explicit
  `'omit'` is a decision; the default is an accident.
