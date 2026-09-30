# front-end-phase-1-api-client Validation

**Date**: 2026-09-30
**Spec**: `.specs/features/front-end-phase-1-api-client/spec.md`
**Diff range**: `fbcd309..468631e` (9 task commits: `bf2bfdb`, `ee2952c`, `47aeeeb`, `75d946e`, `031b5da`, `eddb5ba`, `fea6d1e`, `4c7aee9`, `468631e`; planning docs `93f0ac3` spec, `fbcd309` tasks)
**Verifier**: independent sub-agent (author ≠ verifier)

**Verdict: PASS ✅** — 37/37 ACs matched spec-defined outcomes, 0 spec-precision gaps, gate green (82 passed / 0 failed / 4 self-skipped by design), sensor 6/6 mutants killed, real tree byte-identical after scratch disposal.

---

## Task Completion

| Task | Status | Notes |
| ---- | ------ | ----- |
| T1: Base URL config | ✅ Done | 9 tests (`bf2bfdb`) |
| T2: Envelope decode | ✅ Done | 10 tests (`ee2952c`) |
| T3: apperr mapping + l10n | ✅ Done | 11 tests (`47aeeeb`) |
| T4: Template models | ✅ Done | 6 tests (`75d946e`) |
| T5: Sheet model + dotted path | ✅ Done | 8 tests (`031b5da`) |
| T6: Typed client | ✅ Done | 16 tests (`eddb5ba`) |
| T7: Timeout/retry/offline | ✅ Done | 9 tests (`fea6d1e`) |
| T8: Contract fixtures | ✅ Done | 8 tests (`4c7aee9`) |
| T9: Live self-skip suite | ✅ Done | 6 tests — 4 scenarios self-skip without base + 2 mechanism tests (`468631e`) |

All 9 tasks marked complete in `tasks.md`; claimed test counts match actual counts per file (83 feature tests).

---

## Spec-Anchored Acceptance Criteria

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| FE1-01: one typed method per consumed endpoint, each returning decoded payload | 6 methods: health, templates, template detail, POST/GET/PATCH sheets | `test/api/api_client_test.dart:44` `expect(await client.health(), 'healthy')`; `:60-65`, `:78-83`, `:99-101`, `:123-126`, `:141-144` decoded payloads; `:188-198` exact route list `['GET /health','GET /templates','GET /templates/fantasy_v1','POST /sheets','GET /sheets/sheet-1','PATCH /sheets/sheet-1']` | ✅ PASS |
| FE1-02: 2xx `{"data"}` → decoded payload | payload in endpoint type | `test/api/envelope_test.dart:14` `expect(status, 'healthy')`; `:23` `expect(created, [1,2,3])`; wiring `test/api/api_client_test.dart:44` | ✅ PASS |
| FE1-03: 4xx/5xx error envelope → domain error with `code`+`message` | carries code and message | `test/api/envelope_test.dart:37-42` `isA<ApiDomainException>().having(code,'NOT_FOUND').having(message,'sheet not found')` | ✅ PASS |
| FE1-04: 2xx invalid JSON / no `data` → defensive parse error, never empty success | parse error, not success | `test/api/envelope_test.dart:71-78` (invalid JSON → `ApiParseException`), `:83-90` (no `data`), `:95-102` (empty body); wiring `test/api/api_client_test.dart:350-353` | ✅ PASS |
| FE1-05: POST body `{"template_id","sheet_name"}` | exact body map | `test/api/api_client_test.dart:106-109` `expect(jsonDecode(request.body), {'template_id': 'fantasy_v1', 'sheet_name': 'Aragorn'})` | ✅ PASS |
| FE1-06: PATCH body `{"data": {...}}` only the delta | exact body map | `test/api/api_client_test.dart:149-156` `expect(jsonDecode(request.body), {'data': {'stats': {'dexterity': 16}}})` | ✅ PASS |
| FE1-07: all requests through injected transport; no method creates its own | injected `http.Client` | `test/api/api_client_test.dart:188-198` all six calls recorded by the injected `MockClient`; structural: transport only accepted at construction (`lib/api/api_client.dart:17,31`) | ✅ PASS |
| FE1-08: template exposes `id`,`name`,`default`,`fields`, leaf=`schema` / group=`children` | field list + tree shape | `test/api/template_model_test.dart:63-71` (catalog id/name/default/fields), `:80-83` (detail, `isDefault, isTrue`), `:92-98` (`group.schema isNull`, `leaf.children isNull`, leaf schema notNull) | ✅ PASS |
| FE1-09: leaf schema exposes `type`,`label`,`formula`,`depends_on`,`is_editable`,`default`; omitted optionals → absent | full decode + null when omitted | `test/api/template_model_test.dart:116-123` (`formula`/`dependsOn`/`isEditable`/`defaultValue` on value), `:133-135` `formula isNull`, `dependsOn isNull` when omitted | ✅ PASS |
| FE1-10: sheet exposes `id`,`template_id`,`name`,`data`,`created_at`,`updated_at`, nested `data` preserved | all fields + nesting | `test/api/sheet_model_test.dart:30-34` (fields incl. `DateTime.parse` equality), `:40-44` (`data['stats'] isA<Map>`, nested value 16, `tags` list) | ✅ PASS |
| FE1-11: numbers in `data` keep int/double | 16 int, 16.5 double | `test/api/sheet_model_test.dart:54-59` `expect(intValue, isA<int>())`, `expect(doubleValue, isA<double>())` + exact values | ✅ PASS |
| FE1-12: resolvable dotted path returns the value | `stats.dexterity` → 16 | `test/api/sheet_model_test.dart:67` `expect(sheet.valueAt('stats.dexterity'), 16)` | ✅ PASS |
| FE1-13: missing segment / non-map intermediate → null, no throw | `isNull`, never throws | `test/api/sheet_model_test.dart:75-76` (`valueAt('stats.strength')` → `isNull`, direct call so a throw fails the test), `:82-83` (scalar intermediate → `isNull`) | ✅ PASS |
| FE1-14: empty path / empty segment → typed client error | typed exception | `test/api/sheet_model_test.dart:89-96` `throwsA(isA<InvalidPathException>())` for `stats..dexterity` and `.stats`; `:102-105` for `""` | ✅ PASS |
| FE1-15: `INVALID_JSON` → `clientFault` | exact kind | `test/api/error_mapper_test.dart:23` `expect(apperrKind('INVALID_JSON'), ApiErrorKind.clientFault)`; wiring `test/api/api_client_test.dart:301` | ✅ PASS |
| FE1-16: `INVALID_REQUEST` → `invalidEdit` | exact kind | `test/api/error_mapper_test.dart:28`; wiring `test/api/api_client_test.dart:262` | ✅ PASS |
| FE1-17: `UNKNOWN_FIELD` → `renderMismatch` | exact kind | `test/api/error_mapper_test.dart:33`; wiring `test/api/api_client_test.dart:282` | ✅ PASS |
| FE1-18: `NOT_FOUND` → `notFound` | exact kind | `test/api/error_mapper_test.dart:38`; wiring `test/api/api_client_test.dart:241` | ✅ PASS |
| FE1-19: `INTERNAL_ERROR` → `serverError` | exact kind | `test/api/error_mapper_test.dart:43`; wiring `test/api/api_client_test.dart:317` | ✅ PASS |
| FE1-20: transport failure after retries exhausted → `offline` | kind `offline` | `test/api/api_client_retry_test.dart:61-62` `expect(error.kind, ApiErrorKind.offline)` + `expect(calls, 2)`; POST/PATCH single-attempt offline `:131`, `:147` | ✅ PASS |
| FE1-21: unmapped code → `unknown`, raw code + server message preserved | kind `unknown` + code + message | `test/api/error_mapper_test.dart:56` (6 unmapped codes → `unknown`), `:79-83` (code/message/status preserved); wiring `test/api/api_client_test.dart:337-339` (`kind unknown`, `code 'CONFLICT'`, `message 'name taken'`) | ✅ PASS |
| FE1-22: 4xx/5xx invalid JSON or no `error.code` → defensive parse, not mapped domain error | parse error | `test/api/envelope_test.dart:109-124` (no `error.code` → `ApiParseException`), `:129-142` (HTML body → `ApiParseException` with status), `:147-154` (`error` not a map); wiring `test/api/api_client_test.dart:364-367` | ✅ PASS |
| FE1-23: visible error copy via localization, `en` only, no hardcoded UI literals | copy from l10n layer | `test/api/error_mapper_test.dart:100-102` `expect(errorCopy(kind), copyByKind[kind])` for every kind (ARB values), `:106-109` `supportedLocales == [Locale('en')]`, `:123-131` no `lib/api` source contains any ARB copy string | ✅ PASS |
| FE1-24: domain error carries status, apperr code, server message | all three on error | `test/api/envelope_test.dart:39-41` `.having(statusCode, 404).having(code, 'NOT_FOUND').having(message, 'sheet not found')`; wiring `test/api/api_client_test.dart:242-244` | ✅ PASS |
| FE1-25: default base `https://api.legnaris.com.br` | exact URL | `test/api/api_config_test.dart:8` `expect(resolveApiBaseUrl(), 'https://api.legnaris.com.br')`; `:74` same via `ApiConfig()` | ✅ PASS |
| FE1-26: `API_BASE_URL` define wins in any build mode | override used | `test/api/api_config_test.dart:13-16` `expect(resolveApiBaseUrl(rawOverride: 'https://tunnel.example.dev'), 'https://tunnel.example.dev')`, `:23` http override accepted; build-mode independence is structural — no debug gate anywhere in `lib/api/api_config.dart:30-44` | ✅ PASS |
| FE1-27: non-absolute base → config error before any request | typed config error | `test/api/api_config_test.dart:40-47` (`api:8080`, `localhost:8080` → `throwsA(isA<ApiConfigException>())`), `:52-55` (`ftp://`), `:59-66` (scheme-less, host-less), `:82-85` constructor rejects → client cannot be constructed, so no request can be emitted | ✅ PASS |
| FE1-28: 5s timeout per attempt | 5s constant + applied per attempt | `test/api/api_client_retry_test.dart:26` `expect(ApiClient.requestTimeout, Duration(seconds: 5))`; `:79-80` hanging transport → `calls == 2` and `elapsed >= 10s` (two 5s cuts + 300ms) | ✅ PASS |
| FE1-29: GET transport failure → exactly 1 retry after 300ms | 2 calls, ≥300ms gap | `test/api/api_client_retry_test.dart:44-48` `expect(callTimes, hasLength(2))` + `difference >= 300ms`; `:27` `retryDelay == 300ms`; exhausted → 2 calls (`:62`) | ✅ PASS |
| FE1-30: GET 4xx/5xx → no retry, mapped error exposed | 1 call | `test/api/api_client_retry_test.dart:96-98` `expect(calls, 1)` + `kind notFound` + `statusCode 404`; `:114-115` `calls == 1` + `kind serverError` | ✅ PASS |
| FE1-31: POST/PATCH never retried for any reason | 1 call | `test/api/api_client_retry_test.dart:130-131` (POST transport → `calls == 1`, offline), `:146-147` (PATCH → 1), `:165-166` (POST 4xx → 1, `kind invalidEdit`) | ✅ PASS |
| FE1-32: `flutter test` green without reachable API, exit 0 | green + exit 0 | Gate run (this validation): 82 passed / 0 failed / exit 0; fixture suite runs solely on `MockClient` — `test/fixtures/contract_fixtures_test.dart:25-27` `fixtureClient` never touches network | ✅ PASS |
| FE1-33: fixtures for 6 endpoints + error envelope + PATCH delta pair | all 8 pinned | `test/fixtures/contract_fixtures_test.dart:31-50` requires + validates all 8 files (`health`, `templates`, `template_detail`, `sheet_create`, `sheet_get`, `patch_delta_request`, `patch_delta_response`, `error_not_found`); each decoded on value `:56`, `:62-71`, `:77-93`, `:99-111`, `:117-123`, `:127-148`, `:158-161` | ✅ PASS |
| FE1-34: fixture drift → suite fails | drift breaks suite | Strict typed decode asserts exact fixture values (`test/fixtures/contract_fixtures_test.dart:106`, `:118-123`, `:159-161`); proven by sensor M6: renaming `template_id` in `sheet_get.json` made the suite fail | ✅ PASS (sensor-confirmed) |
| FE1-35: no API base → self-skip, exit 0 | skip + exit 0 | `test/contract/live_contract_test.dart:199-204` `resolveLiveApiBase(rawDefine:'', environment:{})` → `isNull`, `contractSkip(null)` `isA<String>`; `skip: contractSkip(base)` at `:120`, `:149`, `:175`, `:194`; gate run: exactly these 4 skipped, exit 0 | ✅ PASS |
| FE1-36: base configured → 4 TDD scenarios execute (201 / 200 cascade-only / 400 `INVALID_REQUEST`\|`UNKNOWN_FIELD` / 404 `NOT_FOUND`) | scenarios run, exact statuses | Skip gate: `test/contract/live_contract_test.dart:215-218` `expect(contractSkip('http://localhost:8080'), isFalse, reason: '...never a silent skip')` → configured base always runs; scenario bodies assert exact contract outcomes: `:107-112` (201), `:134-139` (200) + cascade-only keys `:140-146`, `:168-174` (400 + accepted codes), `:187-193` (404 + `NOT_FOUND`). Live execution self-skipped in this environment (no base configured — spec-mandated behavior); scenario outcomes await a run against a local instance | ✅ PASS (mechanism + bodies asserted; live run self-skipped by design) |
| FE1-37: configured base + unexpected status/code → loud failure with observed values | fail with observed status and code | `test/contract/live_contract_test.dart:254-258` `expect(reported, hasLength(2))`, `contains('500')`, `contains('INTERNAL_ERROR')`, `contains('404')`, `contains('FORBIDDEN')`; failure text built at `:86-91` includes `observed status` and `observed apperr code` | ✅ PASS |

**Status**: ✅ All 37 ACs covered with `file:line` evidence and spec-matching asserted values · 0 spec-precision gaps · 0 surviving mutants.

### Spec-precision review

Every AC states a checkable outcome (exact URL, exact durations, exact kind names, exact status/code sets, exact JSON bodies); no AC required a vague assertion to pass, so no ⚠️ spec-precision gaps were flagged. Two structural (non-executable-in-unit) qualifiers were verified by code inspection and noted above: FE1-26 "any build mode" (no debug gate in `api_config.dart`) and FE1-27 "before any request" (validation at construction, `api_config_test.dart:71-79`).

### Payload/conjunction rule

Payload fields are asserted on value, not call counts: POST/PATCH request bodies as decoded maps (`api_client_test.dart:106-109`, `:149-156`), fixture delta pair (`contract_fixtures_test.dart:128-132`), error payloads carry status+code+message (`envelope_test.dart:39-41`), retry tests assert counts **and** resulting kind/status (`api_client_retry_test.dart:61-62`, `:96-98`), PATCH responses asserted cascade-only by key absence **and** cascade values (`contract_fixtures_test.dart:139-148`).

---

## Discrimination Sensor

Scratch: temporary git worktree `/tmp/opencode/fe1-sensor` at `468631e` (never `git stash`). One behavior-level mutation at a time, covering test run per mutation, restored after each; worktree removed after the run.

| Mutation | File (scratch) | Description | Killed? |
| -------- | -------------- | ----------- | ------- |
| M1 | `desktop/lib/api/envelope.dart:35` | Envelope success side returns the whole body instead of `parsed['data']` (`decode(parsed['data'])` → `decode(parsed)`) | ✅ Killed — `envelope_test.dart` happy paths fail |
| M2 | `desktop/lib/api/error_mapper.dart:46` | `NOT_FOUND` maps to `ApiErrorKind.unknown` instead of `notFound` | ✅ Killed — `error_mapper_test.dart:36-39` + client wiring fail |
| M3 | `desktop/lib/api/api_client.dart:87-99` | GET loses its single retry (attempt fails → immediate `offline`) | ✅ Killed — `api_client_retry_test.dart` FE1-29 test fails (`hasLength(2)`) |
| M4 | `desktop/lib/api/api_config.dart:7` | Compiled default base changed to `https://api.example.com` | ✅ Killed — `api_config_test.dart:8` fails |
| M5 | `desktop/lib/api/models/sheet.dart:69-71` | Non-map intermediate guard removed (cast crash instead of `null`) | ✅ Killed — `sheet_model_test.dart:79-84` (FE1-13) fails |
| M6 | `desktop/test/fixtures/sheet_get.json` | Contract drift: `template_id` key renamed to `templateId` | ✅ Killed — `contract_fixtures_test.dart` fails (proves FE1-34) |

**Sensor depth**: extended (6 mutations — orchestrator-mandated ≥5 across envelope decode, error mapping, retry policy, base-URL resolution, dotted-path accessor, plus fixture drift)
**Result**: 6/6 killed, 0 survived — ✅ PASS

**Isolation verified**: `git status --porcelain` of the real tree before sensor work == after scratch disposal (only pre-existing noise: ` M .agents/.skill-lock.json*`, untracked `.claude/`, `.cursor/`, `.windsurf/` skill dirs); `git diff HEAD -- desktop/` empty — real tree byte-identical.

---

## Code Quality

| Principle | Status |
| --------- | ------ |
| Minimum code (no features beyond spec, no extra abstraction) | ✅ |
| Surgical changes (only `desktop/lib/api/**`, l10n, `desktop/test/**`, `pubspec`) | ✅ |
| No scope creep (Riverpod, auth, multi-locale, widgets all deferred per §Out of Scope) | ✅ |
| Matches patterns (`flutter analyze` clean; English identifiers/comments; colocated tests; AGENTS.md test conventions) | ✅ |
| Spec-anchored outcome check (asserted values match spec, 37/37) | ✅ |
| Per-layer coverage: domain 1:1 AC mapping; client happy+edge+error per endpoint | ✅ |
| Every feature test maps to an AC or spec edge — no unclaimed tests (all 83 cite FE1-NN or a spec edge) | ✅ |
| Documented guidelines: root `AGENTS.md`, `docs/TDD-desktop-mvp.md` §Estratégia de Testes, `desktop/analysis_options.yaml` | ✅ |

---

## Edge Cases

- [x] Empty/invalid 2xx body → defensive parse error (`envelope_test.dart:93-103`)
- [x] Empty `API_BASE_URL` override → falls back to default https (`api_config_test.dart:28-36`)
- [x] `http://` accepted only via explicit override; default is https (`api_config_test.dart:6-9`, `:19-26`)
- [x] PATCH fixture/response is cascade-only — echo keys absent (`contract_fixtures_test.dart:139-148`)
- [x] JSON key `default` mapped explicitly (`template_model_test.dart:74-84`; fixture `contract_fixtures_test.dart:68`)
- [x] Missing path segment / non-map intermediate → null, empty segment → typed error (`sheet_model_test.dart:72-106`)
- [x] 2xx JSON without `data`, 4xx/5xx without `error.code`, non-envelope `error` → parse error (`envelope_test.dart:68-156`)
- [x] Timeout measured: 2 hanging attempts cut at 5s each → ≥10s then `offline` (`api_client_retry_test.dart:65-81`)

---

## Gate Check

- **Gate command**: `cd desktop && flutter analyze && flutter test` (Full gate from tasks.md)
- **Result**: analyze — 0 issues; tests — **82 passed, 0 failed**, exit 0
- **Test count before feature**: 3 (2 files: `app_title_provider_test.dart`, `widget_test.dart` @ `fbcd309`)
- **Test count after feature**: 86 (83 feature + 3 pre-existing) — **Delta: +83**
- **Skipped tests**: 4 — the FE1-36 live scenarios (`live_contract_test.dart:98`, `:122`, `:151`, `:177`), self-skipped with visible reason because no `API_BASE_URL` is configured — this is the spec-mandated behavior of FE1-35, not a suppression
- **Failures**: none

---

## Fix Plans

None. No surviving mutants, no uncovered ACs, no spec-precision gaps.

---

## Requirement Traceability Update

| Requirement | Previous Status | New Status |
| ----------- | --------------- | ------------ |
| FE1-01 … FE1-37 | Execute | ✅ Verified (37/37) |

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 37/37 ACs matched spec outcome | 0 spec-precision gaps
**Sensor**: 6/6 mutations killed
**Gate**: 82 passed, 0 failed, 4 self-skipped (by design)

**What works**: typed 6-endpoint client on injected transport; single-point envelope decode with defensive parse on both sides; full `apperr` mapping table + `unknown` fallback + `offline` classification; l10n-only English copy; faithful template/sheet models with int/double preservation and the dotted-path accessor (null vs typed-error semantics); compiled default base + dev override + construction-time validation; 5s timeout / 1-retry-300ms GET-only policy; 8 pinned contract fixtures with drift detection; self-skipping live contract suite with loud-failure mechanism.

**Issues found**: none blocking. Note for follow-up (non-blocking): the four live scenarios (FE1-36) have never been executed against a real instance in this validation — run `API_BASE_URL=http://localhost:PORT flutter test test/contract` once a local backend is up to complete the live evidence.

**Next steps**: feature is done — route to M1 milestone sign-off.
