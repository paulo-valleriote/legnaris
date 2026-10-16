# Quickstart: validate the web catalog

**Feature**: `.specs/features/003-web-catalog/` | **Date**: 2026-10-16

Runnable scenarios that prove the feature works end to end. Each lists what to run and the outcome
that means it passed. Prerequisites: the F1 client toolchain (`web/` with `npm install` already done).
No scenario needs a backend: every request is scripted through the F2 fake transport.

Paths referenced here are defined in [plan.md](./plan.md) (source layout),
[data-model.md](./data-model.md) (shapes) and the three documents under [contracts/](./contracts/).

---

## 1. The suite runs with no backend at all

```bash
cd web && npm test
```

**Outcome**: the whole suite passes with the API and MongoDB stopped. The catalog suites never touch
the network — the injected fake transport answers every request — so this is the state continuous
integration runs in (WCT-02, WCT-12).

---

## 2. The catalog replaces the empty workspace and its five states are distinct

```bash
cd web && npx vitest run src/test/catalog_screen.test.tsx src/test/routing.test.tsx
```

**Outcome**: the root route renders the catalog, not `EmptyWorkspace` (WCT-01). Five cases prove the
five situations separately (SC-001): while the request is in flight the loading copy shows and the
empty copy does not (WCT-03); a two-entry response renders two rows in the response order with their
names (WCT-04); an empty response renders one plain line and no decorative art (WCT-05); a failure
renders the mapped copy with Retry (WCT-06); a malformed payload renders the parse-failure copy with
Retry and neither a blank screen nor a partial list (WCT-09). An entry whose name is `""` renders
without throwing (WCT-10).

---

## 3. The catalog loads exactly once per app load

```bash
cd web && npx vitest run src/test/catalog_queries.test.tsx
```

**Outcome**: the first mount issues exactly one request (WCT-02); remounting the route, firing a
window focus event and firing a reconnect issue none, because the query client disables retry,
refetch-on-focus and refetch-on-mount (contract [server-state](./contracts/server-state.contract.md));
activating Retry from an error state issues exactly one further request (WCT-07); activating it again
while that request is in flight issues none (WCT-08). Transport attempt counts, not the DOM, are the
proof.

---

## 4. Opening a template loads its own detail, once per opening

```bash
cd web && npx vitest run src/test/create_sheet_dialog.test.tsx
```

**Outcome**: activating an entry opens the surface for that identifier and issues exactly one detail
request (WCT-11, WCT-12); while it is in flight the loading copy shows and no previously opened
template's detail is rendered (WCT-13). On success the surface shows the template's display name
(WCT-14). A failure shows the mapped copy inline and keeps the surface open (WCT-15); a missing
template shows the missing-template copy inline without closing or crashing (WCT-17); an unreadable
response shows the parse-failure copy inline and never a half-filled detail (WCT-18). Retry re-requests
exactly once (WCT-16). Opening the same template again issues exactly one further request and starts at
`loading` (WCT-21). Opening two different entries in turn loads each one's own detail.

---

## 5. Dismissing returns to the catalog unchanged

```bash
cd web && npx vitest run src/test/catalog_keyboard.test.tsx
```

**Outcome**: Esc (and the close action) dismisses the surface, the catalog list is unchanged, and no
catalog request is issued (WCT-19); navigation stays shallow — the surface is never a route and the
catalog is still on the same URL (WCT-20). Focus moves to the sheet-name input on open and returns to
the triggering entry on close, and every control is reachable by Tab with a visible focus state
(WCT-27).

---

## 6. The sheet name gates the primary action, and nothing mutates

```bash
cd web && npx vitest run src/test/create_sheet_dialog.test.tsx
```

**Outcome**: the field starts empty with a localized label and placeholder (WCT-22); with `""` or
`"   "` the primary action is disabled and its tooltip states a sheet name is required (WCT-23); with
`"  Aragorn  "` or `"Aragorn"` it is enabled (WCT-24); activating it with a valid name issues **zero**
transport attempts (WCT-25, SC-002) — the surface performs no creation in this feature.

---

## 7. No copy and no visual value is authored in the components

```bash
cd web && npx vitest run src/test/l10n_strings.test.ts && npm run lint
```

**Outcome**: every visible string on both surfaces resolves through `src/l10n/strings.ts` (WCT-26,
SC-003), and the source scan finds no literal hex or px in `web/src/catalog/` (WCT-28): every value
comes from the shared tokens. The copy suite goes red if a pinned catalog key is removed, so a missing
key cannot pass silently (SC-004).

---

## 8. Zero critical automated accessibility violations

```bash
cd web && npm run test:e2e -- catalog.spec.ts
```

**Outcome**: the Playwright case loads the real build, opens the catalog and the creation surface and
asserts zero critical `axe-core` violations on each (SC-005), using the `@axe-core/playwright`
dependency F1 already installs. The broader axe sweep of every main flow is F9-T007.

---

## 9. The gates stay green

```bash
cd web && npm run format:check && npm run lint && npm run typecheck && npm test && npm run build
cd .. && make lint-specs
```

**Outcome**: all client gates pass and the spec lint reports clean, meaning every `WCT-*` citation in
the delivered sources resolves to `spec.md` and no assertion in the client suite is vacuous.

---

## Traceability

| Scenario | Requirements |
|---|---|
| 1 | WCT-02, WCT-12 |
| 2 | WCT-01, WCT-03…WCT-06, WCT-09, WCT-10 |
| 3 | WCT-02, WCT-07, WCT-08 |
| 4 | WCT-11…WCT-18, WCT-21 |
| 5 | WCT-19, WCT-20, WCT-27 |
| 6 | WCT-22…WCT-25 |
| 7 | WCT-26, WCT-28 |
| 8 | WCT-27 (a11y) |
| 9 | all `WCT-*` (lint) |
