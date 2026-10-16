# Quickstart: validate the typed web API client

**Feature**: `.specs/features/002-web-api-client/` | **Date**: 2026-10-16

Runnable scenarios that prove the feature works end to end. Each lists what to run and the outcome that
means it passed. Prerequisites: the F1 client toolchain (`web/` with `npm install` already done);
scenario 6 additionally needs the local backend stack.

Paths referenced here are defined in [plan.md](./plan.md) (source layout),
[data-model.md](./data-model.md) (shapes) and the four documents under [contracts/](./contracts/).

---

## 1. The suite runs with no backend at all

```bash
cd web && npm test
```

**Outcome**: the whole suite passes with the API and MongoDB stopped; the live contract cases report as
skipped rather than failing (API-34, API-37). This is the state continuous integration runs in.

---

## 2. Every operation is typed, and the envelope is decoded once

```bash
cd web && npx vitest run src/test/api_client.test.ts src/test/api_envelope.test.ts
```

**Outcome**: six cases — one per operation — assert the method, path, headers and body the client sent,
and the payload it returned. A fake transport records every request, so the same suite shows all six
went through the injected transport and that no operation built its own (API-01, API-02, API-05, API-06,
API-07). Envelope cases cover a 2xx with `data`, a failure with `error.code`, a 2xx without `data`, an
empty body and invalid JSON (API-03, API-04, API-22).

---

## 3. Failures become the kinds the interface branches on

```bash
cd web && npx vitest run src/test/api_error_mapper.test.ts
```

**Outcome**: a table-driven case per backend code asserts the kind — five dedicated kinds, seven codes
falling back to `unknown` with the raw code and message preserved — plus the transport-failure kind
`offline` with no status (API-15…API-21, API-24). One case per kind asserts the message shown is
byte-equal to the entry in the translation catalog, which is how "no hardcoded copy" is proven rather
than reviewed (API-23).

---

## 4. Models keep the contract, and paths behave

```bash
cd web && npx vitest run src/test/api_models_template.test.ts src/test/api_models_sheet.test.ts
```

**Outcome**: the template case asserts the leaf/group tree, that an omitted `formula`/`depends_on` is
absent rather than empty, and that the payload's `default` key is read (API-08, API-09). The sheet case
asserts the nested `data` shape, that whole and fractional numbers survive decoding unchanged, a
resolving dotted path, a missing segment and a non-object intermediate returning "no value" without
throwing, and an empty or double-dotted path raising the typed path failure (API-10…API-14).

---

## 5. Attempts are bounded, and writes never repeat

```bash
cd web && npx vitest run src/test/api_client_retry.test.ts
```

**Outcome**: the transport is called twice for a read that fails at the transport level and once for a
read answered with a failure status; once for any write failure; the injected delay shows the fixed
retry interval; and under fake timers the attempt is aborted at the five-second timeout
(API-30…API-33).

---

## 6. The contract holds against the real backend (optional, needs the stack)

```bash
make dev-db                                     # terminal 1
go run ./cmd/api.go                             # terminal 2
cd web && API_BASE_URL=http://localhost:8080 npm test
```

**Outcome**: the live cases now run instead of skipping and exercise the four real flows — creating a
sheet returns derived values, updating an editable field returns only recalculated effects, updating a
derived or unknown field is refused as an invalid edit or an unknown field, and a missing sheet reports
as not found (API-38). Pointing `API_BASE_URL` at a wrong address makes the suite **fail** naming the
status it saw: a configured base address that does not answer is never a silent skip (API-39).

---

## 7. The captures are real, and drift breaks the build

```bash
cd web && npx vitest run src/test/api_contract_captures.test.ts
git diff --stat web/src/api/fixtures   # only after a deliberate re-capture
```

**Outcome**: every success capture decodes through its model, and the drift case — a capture copy with
one key altered — fails the decode instead of passing silently (API-34…API-36). Re-capturing is the
documented script in [contracts/capture-provenance.contract.md](./contracts/capture-provenance.contract.md),
never a hand edit.

---

## 8. The base address is configuration

```bash
cd web && npx vitest run src/test/api_config.test.ts && npm run build
```

**Outcome**: no override resolves to the public API over HTTPS with `source: 'default'`; an override is
used verbatim; an empty override falls back to the default; a malformed value (a bare host, a relative
path, an empty string that is not treated as an override) raises a configuration failure before any
request is issued; a plaintext address reports `secure: false` while an HTTPS one reports `secure: true`
(API-25…API-28). The production build contains no plaintext API address.

---

## 9. Nothing sensitive is logged or attached implicitly

```bash
cd web && npx vitest run src/test/api_no_logging.test.ts
```

**Outcome**: a source scan shows `web/src/api/` contains no `console.*` call and imports no logger, and
every request the fake transport recorded carries `credentials: 'omit'` (API-29).

---

## 10. The gates stay green

```bash
cd web && npm run format:check && npm run lint && npm run typecheck && npm test && npm run build
cd .. && make lint-specs
```

**Outcome**: all client gates pass and the spec lint reports clean, meaning every `API-*` citation in
the delivered sources resolves to `spec.md` and no assertion in the client suite is vacuous.

---

## Traceability

| Scenario | Requirements |
|---|---|
| 1 | API-34, API-37 |
| 2 | API-01…API-07, API-22 |
| 3 | API-15…API-24 |
| 4 | API-08…API-14 |
| 5 | API-30…API-33 |
| 6 | API-38, API-39 |
| 7 | API-34…API-36 |
| 8 | API-25…API-28 |
| 9 | API-29 |
| 10 | API-23, API-34, all `API-*` (lint) |
