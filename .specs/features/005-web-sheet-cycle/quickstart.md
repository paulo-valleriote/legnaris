# Quickstart: validate the web sheet cycle

**Feature**: `.specs/features/005-web-sheet-cycle/` | **Date**: 2026-10-16

Runnable scenarios that prove the feature works end to end. Each lists what to run and the outcome that
means it passed. Prerequisites: the F1 client toolchain (`web/` with `npm install` already done). The
Vitest suites need no backend and no MongoDB: every request is driven through the injected fake
transport and every local record through the in-memory `SheetStore` double. Only the accessibility
scenario (9) needs a built client.

Paths referenced here are defined in [plan.md](./plan.md) (source layout),
[data-model.md](./data-model.md) (shapes) and the four documents under [contracts/](./contracts/).

---

## 1. The suite runs with no backend at all

```bash
cd web && npm test
```

**Outcome**: the whole suite passes with the API and MongoDB stopped. The sheet suites drive the F2
client through `fake_transport.ts` and the `SheetStore` through the in-memory double, so nothing touches
the network or a database — the state continuous integration runs in.

---

## 2. A sheet opens through every situation

```bash
cd web && npx vitest run src/test/sheet_open.test.tsx
```

**Outcome**: opening a sheet issues exactly one `GET /sheets/:id` and one `GET /templates/:id`, then
renders the sheet's name and the template's field tree (WSC-05, WSC-08); a slow response shows a
loading state; a `NOT_FOUND` shows the mapped missing message with no Retry, deletes the sheet and its
draft from the store, and a throwing delete still shows the message (WSC-06); any other failure —
including a malformed payload — shows the mapped message with a Retry that re-fetches exactly once
(WSC-07); a template failure shows its own mapped message and Retry (WSC-09); a template with no fields
shows the empty-fields line; and an open writes the sheet to the store under `schemaVersion 1` (WSC-11).

---

## 3. A sheet is created from the catalog and opened without a second fetch

```bash
cd web && npx vitest run src/test/create_sheet_flow.test.tsx
```

**Outcome**: a non-empty trimmed name issues exactly one `POST /sheets` with the template id and the
name (WSC-01); while in flight the primary action is disabled and a repeat activation issues no second
request (WSC-03); on success the flow navigates to the created sheet and the screen renders it with no
`GET /sheets/:id` because the cache was seeded (WSC-02); a failure keeps the surface open with the typed
name and the mapped message inline (WSC-04); a whitespace-only name keeps the action disabled and issues
no request.

---

## 4. The delta discipline holds

```bash
cd web && npx vitest run src/test/sheet_edit.test.ts
```

**Outcome**: an edit records its path as pending and reverting it un-pends (WSC-12, WSC-13); Save sends
the nested delta of only the changed editable leaves — no unchanged field and no derived field (WSC-14);
an exclusive selection writes one member on and its siblings off in one change, and "None" writes all
off (WSC-15); a clean sheet's Save issues no request (WSC-19) and a second Save in flight issues none
(WSC-18); a save merges the returned cascade, advances the baseline and clears pending, while an edit
made during the request stays pending (WSC-16); a failed save keeps every edit and issues no automatic
retry (WSC-17).

---

## 5. The dry-run previews and marks

```bash
cd web && npx vitest run src/test/sheet_preview.test.tsx
```

**Outcome**: Preview sends one `POST /sheets/:id/preview` with the same delta and merges the returned
cascade **without** advancing the baseline (WSC-21, WSC-22); the changed editable rows are labeled
"Modified" and the recalculated derived rows "Recalculated", both tinted, including a marked row whose
value is empty, and the labels come from the engine's mark seam (WSC-23); a new edit clears the marks
and re-enables Preview (WSC-24); a failed preview keeps every edit and the mapped message and leaves
Preview available (WSC-25); a repeat preview with no new edit issues no request.

---

## 6. Drafts survive a crash and are restored

```bash
cd web && npx vitest run src/test/draft_backup.test.ts
```

**Outcome**: while pending, the injected cadence writes the pending editable delta to the store and
never a derived value (WSC-26); while clean it writes nothing (WSC-27); reopening a sheet overlays only
the draft keys that are editable leaves of the loaded template and marks them pending (WSC-28);
a save success, a confirmed discard and a `notFound` eviction each delete the draft (WSC-29); a corrupt,
unsupported-version or foreign draft is ignored so the sheet seeds cleanly and nothing throws (WSC-30);
an inspection of the stored records finds no credential (WSC-31).

---

## 7. The guards and the throttle protect the work and the server

```bash
cd web && npx vitest run src/test/sheet_guards.test.tsx
```

**Outcome**: `Ctrl+S` runs the same save as the footer and is a no-op with no sheet mounted (WSC-20);
`F5` with a clean sheet re-fetches the template detail once, with pending edits confirms a discard
first, and cancel issues zero requests (WSC-36); leaving with pending edits confirms and only a confirm
discards the edits and deletes the draft (WSC-37); a fourth Save or Preview inside the rolling minute
issues no request and the action re-enables when the window frees (WSC-38); an in-flight preview or save
makes `F5` and leaving inert.

---

## 8. No copy and no visual value is authored in the feature

```bash
cd web && npx vitest run src/test/l10n_strings.test.ts src/test/tokens_used.test.tsx && npm run lint
```

**Outcome**: every string the screen, the open state and the library render resolves through
`src/l10n/strings.ts` — removing a referenced key turns the suite red (WSC-32, SC-006) — and the token
scan finds no literal hex or px in `web/src/sheet/` or `web/src/library/` (WSC-33). The ESLint
anti-sink rules cover the new `.tsx` sources, and no value is rendered as markup (WSC-35, SC-007).

---

## 9. The mounted surface has no critical accessibility violation

```bash
cd web && npm run build && npx playwright test e2e/sheet.spec.ts
```

**Outcome**: the mounted sheet screen and the creation surface report zero critical automated
accessibility violations from `@axe-core/playwright` (WSC-39, SC-010), and every rendered control
exposes an accessible role and name. No new axe dependency is added: F1 installed it and F3 established
the measurement pattern (`AD-014`).

---

## 10. The gates stay green

```bash
cd web && npm run format:check && npm run lint && npm run typecheck && npm test && npm run build
cd .. && make lint-specs
```

**Outcome**: all client gates pass and the spec lint reports clean, so every `WSC-*` citation in the
delivered sources resolves to `spec.md` and no suite assertion is vacuous. `idb` is already resolved in
the lockfile (a direct declaration is added), so the security gates see no new package to vet; the
secret, SCA, SAST and lockfile gates stay green.

---

## Traceability

| Scenario | Requirements |
|---|---|
| 1 | all `WSC-*` (no backend) |
| 2 | WSC-05…WSC-11 |
| 3 | WSC-01…WSC-04 |
| 4 | WSC-12…WSC-20 |
| 5 | WSC-21…WSC-25 |
| 6 | WSC-26…WSC-31 |
| 7 | WSC-20, WSC-36…WSC-38 |
| 8 | WSC-32…WSC-35 |
| 9 | WSC-39 |
| 10 | all `WSC-*` (lint) |

## Validation outcome (2026-10-16)

Run in the delivered tree with the API and MongoDB stopped; every scenario passed. The feature added no
new runtime package: `idb` was already resolved in the lockfile (a direct declaration was added) and
`fake-indexeddb` is a dev-only double, so the bundle and the security gates are unchanged.

| Scenario | Result |
|---|---|
| 1 — suite runs with no backend | PASS — `npm test`: 46 files, 394 passed, 5 live-suite skips |
| 2 — a sheet opens through every situation | PASS — `sheet_open.test.tsx` (11) |
| 3 — created from the catalog, opened without a second fetch | PASS — `create_sheet_flow.test.tsx` (4) |
| 4 — the delta discipline holds | PASS — `sheet_edit.test.ts` (18) |
| 5 — dry-run previews and marks | PASS — `sheet_preview.test.tsx` (8) |
| 6 — drafts survive a crash and restore | PASS — `draft_backup.test.ts` (11) |
| 7 — guards and throttle protect work and server | PASS — `sheet_guards.test.tsx` (16) |
| 8 — no copy or visual value authored | PASS — `l10n_strings.test.ts` + `tokens_used.test.tsx` (26) + `npm run lint` |
| 9 — mounted surface has no critical a11y violation | PASS — `e2e/sheet.spec.ts` (3 axe cases: creation, mounted screen, leave dialog) |
| 10 — the gates stay green | PASS — `format:check`, `lint`, `typecheck`, `npm test`, `npm run build`, `make lint-specs` |

The browser-level case (9) mounts the real build through the F1 Playwright harness; the leave
confirmation is exercised there as well, since it renders one surface-local `SheetConfirmation` on both
the refresh and the leave paths. Per the recorded deviations, only in-app navigation is guarded (browser
back is not interceptable without a data router), which no scenario above requires.
