# AGENTS.md — web/ (React PWA client)

Development, security and architecture rules for the client under `web/`. The **visual** rules —
principles, token architecture, colour, typography, spacing/layout, radius/border/elevation,
motion, component specifications, accessibility, security posture, per-screen patterns, enforcement
and anti-patterns — live in **[DESIGN.md](./DESIGN.md)**, which is the sole written source of the
visual values and supersedes any visual statement elsewhere (UIF-33). Design values are made
**once**, there, not per screen.

## Visual rules

Do not encode or restate visual values here or in a component. Read `DESIGN.md` for the token
architecture and the token-addition workflow, and `src/theme/tokens.css` (mirrored in
`src/theme/tokens.ts`) for the concrete values. Every component consumes `--lg-*` tokens only;
`src/theme/` is the sanctioned exception where the literals are declared, and a source scan keeps
the other directories literal-free.

## Interaction wiring

The keyboard model is centralised, not spread across screens: shortcuts live in one registry
(`src/shell/shortcut_registry.ts`) as a plain action → key map, and the shell-level leave registry
(`src/shell/leave_guard.ts`) is what a navigation asks before leaving a surface with unsaved work.
The visual side of interaction — focus ring, hover, dialog avoidance, feedback — is specified in
`DESIGN.md`.

## Security rules (non-negotiable)

- **No DOM injection sinks**: `dangerouslySetInnerHTML`, `innerHTML`, `eval`,
  `new Function`, `document.write` are forbidden — ESLint fails the build on them
  (FND-10, SEC-02). React escapes text automatically; keep data as text.
- **No third-party scripts, styles, or fonts**: everything is self-hosted and
  same-origin, so a strict Content-Security-Policy can be enforced later.
- **No credentials in JavaScript-readable storage**: never put a token in
  `localStorage`, `sessionStorage`, or IndexedDB. The refresh credential is an
  `HttpOnly` cookie; the access token lives in memory only. (A display preference such as the
  theme is not a credential and may persist.)
- All identifiers, comments, and user-facing strings in **English** (UI strings go
  through `src/l10n/`).

## API client (`src/api/`)

The typed network layer every screen mounts on. Its rules are non-negotiable.

- **One transport seam.** Every request goes through the transport injected at
  construction; no operation builds its own, and the transport is the single place a
  credential will ever be attached. Requests carry `credentials: 'omit'` — no ambient
  credential is attached implicitly, in any environment.
- **No logging.** The module contains no `console.*` call and imports no logger; a request
  body, a credential and an identifier-bearing URL never reach a log. A source scan
  (`src/test/api_no_logging.test.ts`) enforces this.
- **No user-facing literal.** Every message a person sees resolves through
  `src/l10n/strings.ts`; the API module emits codes and failure kinds, never copy.
- **Failures are classified, drift is distinct.** A refusal becomes a `DomainFailure` with
  a `FailureKind`; an unreadable response stays a `DefensiveParseFailure`; a malformed
  dotted path raises a `PathFailure`; an unusable base address raises a
  `ConfigurationFailure` at construction. A new backend code must be given a kind in
  `error_mapper.ts` — the mapping table is total over the codes the backend emits.
- **Captures are never hand-edited.** `src/api/fixtures/` holds real responses written
  verbatim by `capture.sh`; a capture that stops matching the models is a backend contract
  change and the pinned suite is expected to go red on it.

## UI engine (`src/engine/`)

The schema-driven rendering core (feature `web-ui-engine`): it turns a decoded
`FieldSchema` tree into labeled sections and controls and owns **no state** — no value,
no request, no mark. It mounts no screen; every case renders its modules directly.

- `field_grouping.ts` — pure derivation of ordered, labeled sections and render rows
  (stacked leaf, declared-column table, exclusive group).
- `field_control.tsx` / `exclusive_control.tsx` — one control per leaf schema, one
  single-choice control per exclusive group.
- `sheet_form.tsx` — the composition; `sheet_defaults.ts` — the draft seed/overlay/merge;
  `sheet_mark.ts` — the preview-mark contract; `engine.css` — token-only styles.
- Every string resolves through `src/l10n/strings.ts`; every visual value is a `--lg-*`
  token. Data is rendered as text; markup in a value or label stays literal (no DOM sink).

**One control source.** `SheetForm` takes exactly one: `data` + `onChanged`, or both
`controlBuilder` and `exclusiveControlBuilder`. Any other combination throws rather than
half-render (ENG-25). In builder mode the composition reads no value, so the caller owns
the per-field subscription and edit reporting through the seams (`controlBuilder`,
`exclusiveControlBuilder`, optional `rowMarkBuilder`) — the web expression of `AD-001`.

**Numeric guard.** The numeric input seeds from its stored value, holds its display text
locally, and reports an edit only when the whole text resolves to a finite number: an
empty field clears the value, `NaN`/`Infinity`/`1e999` leave the stored value unchanged.
The desktop's partial-token parsing and restore-on-blur are deliberately dropped (ENG-12).

**Mark tint.** A marked control reuses `--lg-color-surface` from `tokens.css`; no new
token was added for the preview tint.

## Sheet cycle (`src/sheet/`, `src/library/`)

The open/edit/preview/draft feature over one sheet (feature `web-sheet-cycle`). It reuses
the F2 client and the F4 engine and owns no wire model.

- `src/sheet/sheet_open.ts` — the open situation for the `:id` route: one `GET /sheets/:id`
  then one `GET /templates/:id`, a seeded fresh `['sheet', id]` rendered with no second
  fetch, and a `NOT_FOUND` that evicts the sheet and its draft.
- `src/sheet/sheet_edit.ts` — the in-memory edit store: `editedPaths`, the copy-on-write
  `data`, the pending delta, the save/preview lifecycle, the preview marks, two rolling
  throttle windows and the draft restore. It emits immutable snapshots for
  `useSyncExternalStore` and binds no listener itself.
- `src/sheet/sheet_screen.tsx` — the mounted screen: header, the F4 form in builder mode,
  the sticky footer and the one guard modal.
- `src/library/sheet_store.ts` — the `SheetStore` port, the versioned envelopes and the
  in-memory double every suite injects; `idb_sheet_store.ts` is the only module importing
  `idb`; `sheet_library.ts` and `draft_backup.ts` hold the cache helpers and the injectable
  draft cadence.
- `src/shell/shortcut_registry.ts` — the one action → key map (`Ctrl+S`/`Cmd+S` save, `F5`
  refresh), with the mounted screen registering and unregistering its handlers;
  `src/shell/leave_guard.ts` — the shell-level leave registry the navigation asks before it
  leaves a surface with unsaved work.

**One edit rebuilds one control.** A field control subscribes to its own dotted path
(`store.subscribePath`) and reports edits back through the engine's `controlBuilder`/
`exclusiveControlBuilder` seams; the screen subscribes only to the coarse signals it renders
(footer, marks). This is the web expression of `AD-001`, obtained with
`useSyncExternalStore` — no state library is added.

**Delta, never a snapshot.** A save and a preview send only the pending editable delta,
rebuilt through the engine's `setPathValue`; a save merges the returned cascade and advances
the baseline, a preview merges it **without** advancing it and marks the changed and
recalculated rows through the F4 `rowMarkBuilder`/`markLabel` seam.

**Two guards, one modal.** `F5` refreshes the sheet's template detail and leaving the screen
with pending edits asks to confirm first; both open the screen's single confirmation modal,
so two guards never stack. The safe action carries focus, Escape and the overlay barrier
cancel, and the destructive action is a real button — never a confirm-by-Enter default.

**Best-effort local records.** A cache write, a draft write and every delete are swallowed on
failure; no local record ever holds a credential, and the draft holds only the pending
editable delta, restored over the stored values and filtered to the template's editable
leaves.

## Theme (`src/theme/`)

- `theme.ts` — the theme store over injected seams (storage, media-query reader, root). The
  resolution order, the live system follow while nothing is stored, and every swallowed
  storage failure are specified in `DESIGN.md`; the accepted values are exactly `light` and
  `dark` under `lg.theme`.
- `use_theme.ts` — the `useSyncExternalStore` hook the control and any surface read.
- `public/theme-init.js` — the same-origin classic head script that resolves the theme before
  the first paint; no inline script and no policy hash (`UIF-20`).

## Componentization

- Extract a shared component only when all hold: **≥ 3 real usages**, same shape and
  purpose, and the extraction does not leak schema-specific logic into it.
- Never abstract the schema-driven UI Engine internals, screen-specific layouts,
  one-off dialogs, or wireframe structures.
- One sanctioned base layer at `web/src/ui/` holds the five primitives (`button`, `field`,
  `checkbox`, `dialog`, `toast`) plus the focus trap; it imports no feature directory,
  uses `lg-<block>__<element>--<modifier>` and keeps the `≥ 3 real usages` rule for every
  other extraction (UIP-01, UIP-03, SC-007). No wrapper-for-wrapper's-sake elsewhere.

## Dependencies

- **No new package without strict necessity.** Design-system dependencies (font
  packages, theme frameworks, component libraries) are prohibited — the tokens plus plain
  CSS are sufficient, and the font is vendored, not depended on.
- If a dependency is ever unavoidable, justify it in the PR description.

## Delivered deviations from the feature task lists

Recorded here instead of weakening an assertion (constitution III) or silently
dropping a task.

### F1 — `web-pwa-foundation`

- **No `src/app/providers.tsx`** (T019). V1 has no provider layer: the shell wraps the
  route table directly in `src/app/router.tsx`. The module lands with the first feature
  that genuinely needs shared context (`web-api-client`).
- **Tests are flat in `src/test/`** with a `*.test.ts(x)` suffix rather than co-located
  next to each source module, because the Vitest setup, the `matchMedia` mock, the
  contrast helper, and the render helper are shared by every suite.
- **Toolchain pins differ from the plan's aspirational ones** — TypeScript 5.9.3 and
  ESLint 9.39.5, because `typescript-eslint`, `eslint-plugin-react`, and
  `eslint-plugin-jsx-a11y` do not yet support TypeScript 6 / ESLint 10. See the
  "Toolchain version note" in `web/README.md`.
- **No `@lhci/cli`** (the plan's tooling line named Lighthouse CI). Nothing invoked it:
  no npm script and no CI step consume it, because the Lighthouse gate belongs to F8
  (`F8-T005`). Its dev-only tree carried the only advisories the `sca` gate could see
  (17 findings, 11 high, two of them — `extract-zip` and `sprintf-js` — with no patched
  release), so the gate could not pass while it was installed. `web/lighthouserc.json`
  (T025) stays; the runner arrives with the F8 gate. Decision: `AD-011`.

### F2 — `web-api-client`

- **No MSW** (the program plan's stack table named it). The injected transport satisfies
  API-07 and makes the retry count and the `credentials: 'omit'` posture directly
  assertable, so no network-interception dependency was added. Recorded in the plan as a
  deliberate deferral.
- **No app-level client wiring yet** (T021, reworded). No screen constructs the client in
  this feature, so `resolveApiConfig` is documented in `web/README.md` as the single
  source of the base address rather than wired into an app entry point; the first feature
  that constructs the client consumes it. The insecure state is reported on the resolved
  configuration, not rendered.
- **A shared decoding helper, `src/api/parse.ts`**, is not named in the task list. It
  holds the payload-narrowing helpers (`isPlainObject`, `drift`, `requireString`, …) that
  the envelope and both models share, instead of duplicating them per module.
- **The INTERNAL_ERROR capture comes from an unmatched route.** No consumed endpoint
  deterministically returns 500, so `error_internal.json` is captured from `GET
/no-such-route`, which surfaces through the global handler as
  `{"code":"INTERNAL_ERROR"}` — a real response, never a hand-written stand-in.
- **The live suite's two mechanism cases assert the gate predicate**, not a real skip;
  the API-38 flow cases run only against a configured backend (`API_BASE_URL`).

### F3 — `web-catalog`

- **`@tanstack/react-query` 5.104.3 is added** (`AD-016`). It is the program plan's
  server-state choice (Parte 3, the F3 task "Queries TanStack"), recorded here as an
  inherited plan decision rather than a new one; it is the feature's only new runtime
  dependency, and the query client disables the library's own retry so the retry policy
  stays the F2 client's.
- **`src/shell/empty_workspace.tsx` is deleted** and the root route renders
  `src/catalog/catalog_screen.tsx` (`AD-013`). The F1 shell/routing/e2e assertions that
  pinned the empty workspace are retargeted to the catalog, not deleted.
- **The left navigation's root item is relabelled "Catalog"** through
  `nav.catalog` (`AD-015`, WCT-29), and `src/app/routes.ts` follows.
- **The creation surface is an accessible overlay, not a native `<dialog>`** (R5):
  `jsdom` does not implement `HTMLDialogElement.showModal`, so the platform element could
  not be driven by the rendering tests the constitution requires; the overlay keeps the
  same `role="dialog"` / `aria-modal` / Esc / focus contract.
- **The `.lg-empty*` rules stay in `src/shell/shell.css`** (deviation from T011). T011
  called them "now-unused" and asked for their removal with `empty_workspace.tsx`, but
  `src/shell/not_found.tsx` renders the same `.lg-empty` / `.lg-empty__title` /
  `.lg-empty__text` classes; deleting the rules would strip the not-found screen's
  styling. They are live CSS, so they are kept.
- **The dialog's accessible title is `sheet.dialogTitle`; the template's display name is
  rendered in the detail area.** The contract's title row and the "detail area resolves
  the display name" obligation are both satisfied this way, and the dialog always has a
  non-empty accessible name (WCT-14, WCT-27).
- **`playwright.config.ts` builds the e2e client with `VITE_API_BASE_URL` set to the
  preview origin**, so the browser specs intercept the catalog request locally instead of
  the built client reaching the production API; `e2e/catalog.spec.ts` blocks the service
  worker and scripts the two responses.

### F4 — `web-ui-engine`

- **The engine mounts no surface** (spec Clarifications, 2026-10-16). No screen, no
  `web/e2e/` case and no test-only route were added; every case renders the engine's own
  modules directly, so the feature proves `ENG-01…ENG-29` without touching the shell.
- **The `rowMarkBuilder` seam landed with the composition (T012), not T016.** The task list
  ordered the seam into US3, but the composition's label row is where the seam plugs in, so
  it shipped with `sheet_form.tsx` and US3 only added the mark tests and the shared
  `markLabel` copy helper.
- **`markLabel(mark)` is exported from `sheet_form.tsx`.** The desktop screen held the
  "Modified" / "Recalculated" copy in its own localization; here the engine owns the two
  catalog keys (`engine.modified`, `engine.recalculated`) so a caller's `rowMarkBuilder`
  names no literal. `sheet_mark.ts` stays behavior-free (a type and a class constant).
- **A section uses the group's declared label when non-empty, else a humanized key.** The
  contract (`R7` / data-model) requires the label; the desktop quirk that ignores a
  group's label is not carried over, because the contract is authoritative here.
- **The numeric input holds its display text locally plus a forced re-render** (a
  `useReducer` tick) instead of the desktop's `TextEditingController` + restore-on-blur:
  React would otherwise refuse to visually reject a whole-value edit, so a rejected
  non-finite text re-renders the controlled input back to the last accepted text (ENG-12).
- **Data mode renders exclusive controls inert** (no `onSelect`), matching the desktop's
  read-only preview; exclusive editing flows through `exclusiveControlBuilder`.
- **The engine tests are flat in `src/test/`** with an `engine_*` prefix, matching the F1
  layout, and the token/copy scans (`tokens_used.test.tsx`, `l10n_strings.test.ts`) now
  cover `src/engine/`.

### F5 — `web-sheet-cycle`

- **No state library** (R2, `AD-001`). The plan's stack table named "Zustand + selectors (or
  Jotai per-path)"; the per-path subscription is React's `useSyncExternalStore`, so no state
  package was added. `idb` 7.1.1 is the feature's one new runtime dependency (already resolved
  transitively through `workbox-expiration`), declared directly; `fake-indexeddb` 6.2.6 is
  dev-only, because jsdom ships no IndexedDB.
- **The store reads its draft through `restore()`, not a seed field.** The draft read is
  asynchronous, so `createSheetEditStore` stays synchronous and `store.restore()` overlays the
  stored draft after construction; this keeps `SheetOpenResult.loaded` unchanged (a seed-time
  draft would have forced the draft into the open result).
- **The leave guard is a shell-level registry, not a router blocker.** `useBlocker` requires a
  data router, which the F1 shell does not mount, so the guard is a small module-level registry
  (`src/shell/leave_guard.ts`) that `LeftNav` asks before it navigates. **Limitation:** a
  browser-back gesture is not interceptable this way; only the in-app navigation is guarded.
- **The guard modal is a screen-local component.** The refresh and leave confirmations are two
  uses of one component inside `sheet_screen.tsx`, not a shared `ConfirmDialog`: two usages do
  not meet the `≥ 3` extraction bar, and the single instance is what keeps the two guards from
  stacking.
- **The F3 dialog also renders `sheet.creating`.** T002 added the "Creating sheet" key; the F3
  `CreateSheetDialog` is retargeted from the idle label to that key so the in-flight state has
  its own copy rather than showing "Create sheet".
- **`.lg-button--danger` is an outlined variant.** No `--lg-color-danger-hover` token exists,
  so the destructive button keeps the surface background with a `--lg-color-danger` border and
  text and tints to `--lg-color-surface-hover` on hover — a real hover change with no new
  token (`--lg-color-danger-text` stays declared for a future filled variant).
- **The token and copy scans now cover the feature.** `tokens_used.test.tsx` scans
  `src/sheet/` and `src/library/` for literal colors and pixels, and `l10n_strings.test.ts`
  scans `src/sheet/`, `src/library/` and `src/shell/shortcut_registry.ts` for user-facing
  literals and unresolved keys (WSC-32, WSC-33).

### F7 — `web-auth-session`

- **`session_store.ts` shipped as `auth_session.ts`.** The plan named the session module
  `src/auth/session_store.ts`; it landed as `src/auth/auth_session.ts`, because the module
  is the session itself (state + the four operations + the in-memory token), not a store
  any surface subscribes to through a separate seam.
- **`authenticated_client.ts` shipped as `authenticated_transport.ts`.** The plan named the
  decorator `src/api/authenticated_client.ts`; it landed as
  `src/api/authenticated_transport.ts` and decorates the `Transport`, not the `ApiClient`,
  so the Bearer attach, the single-flight refresh and the one replay sit at the wire seam
  every request already crosses.
- **No token store — the access token is memory-only** (R9, WAS-32). There is no
  `localStorage`/`sessionStorage`/cookie write anywhere in `src/auth/`; the token lives in a
  module-private field and is exposed only to the authenticated transport and the session's
  own snapshot. A reload therefore holds no session until the silent refresh re-establishes
  it, which is the intended "nothing a script can persist" posture.
- **Single-flight refresh lives on the session, not the decorator.** `AuthSession.refresh()`
  coalesces concurrent callers over one `/auth/refresh` (`refreshInFlight`); the
  authenticated transport calls `session.refresh()` instead of owning its own in-flight
  promise, so a burst of `401`s from any surface shares one rotation (WAS-12, WAS-13).
- **Sign-out has no confirmation dialog.** The plan left the confirmation open; sign-out
  drops the token and sweeps the cache immediately, and the control is disabled while the
  request is in flight, so a double activation issues one `/auth/logout` (WAS-19, WAS-23).
  A confirmation was judged unnecessary because the action is recoverable by signing in
  again and the in-flight guard already covers the double-activation edge.
- **The fake transport needed no extension** (T001). `createFakeTransport` records the whole
  `HttpRequest`, so `method`, `credentials`, `url` and `headers` are already readable off each
  recorded request once `T004` widened the type; the scripted `{status, body}` queue was
  already in place, so the seam is unchanged.
- **The browser specs seed `/auth/refresh`.** `playwright.config.ts` builds the e2e client
  with `VITE_API_BASE_URL` pointing at the preview origin, so the built client's cold-start
  refresh targets that origin; `e2e/auth_seed.ts` routes `/auth/refresh` to a session body so
  the previously public shell, catalog, sheet and PWA specs still render behind the session
  (WAS-08, `AD-013`), and `e2e/auth.spec.ts` adds the axe sweep over the auth surface, the
  signed-in shell and the "Your sheet" surface (SC-010).

### F1 — `web-ui-foundation`

- **The vendored font artifact is `Geist-Variable.woff2`**, not the plan's `Geist[wght].woff2`:
  brackets in a `url()` token and an HTML attribute require percent-encoding, and the official
  file is already named `Geist-Variable.woff2`. The file's role is unchanged (`R4`).
- **The theme control lives at `src/shell/theme_toggle.tsx`**, with its logic in
  `src/theme/theme.ts` and `src/theme/use_theme.ts`. `shell/` stays under the literal scan, so
  the control's styles are token-only and it authors no visual value.
- **`--lg-size-content-min` (320 px) is added** to the size primitives. The two-region grid
  needs a content minimum so the content never collapses under the navigation, and the literal
  scan forbids a bare pixel value in `shell.css`.
- **The motion tokens are named `--lg-motion-120/180/240`**, matching their declared durations;
  the source plan spelled the first `--lg-motion-100` while giving it 120 ms.
- **`src/shell/use_media_query.ts` is deleted** with the panel; nothing else used it once the
  shell stopped reading the narrow breakpoint.
- **The retargeted suites.** `shell.test.tsx`, `shell_responsive.test.tsx`,
  `shell_keyboard.test.tsx` and `web/e2e/shell.spec.ts` now assert the two remaining regions
  and the removed panel, and `no_inline_assets.test.ts` accepts the two external scripts (the
  same-origin theme boot script plus the module entry) while still forbidding inline bodies
  (`UIF-32`).

### F8 — `web-auth-code-step`

- **The code step is a state of the auth surface, not a route** (WCS-02). `AuthGate` renders
  `AuthCodeScreen` in place of the credential form while a step is pending; no entry was added
  to `src/app/routes.ts` and the URL never changes. The challenge handle lives in a
  module-private field beside the access token, so no subscriber can read or persist it.
- **The values the form comes back with live in the session** (WCS-21). The credential form
  unmounts while the step is pending, so the address and the account mode survive the
  abandonment in the session's `restored` projection rather than in a component's local state.
  The snapshot gained that field; the contract and the data model were amended in the same
  change.
- **The credential operations decode a discriminated outcome** (WCS-01). `register` and `login`
  answer either a session or a code step, and the discrimination reads the payload's own keys,
  never the status: registration answers `201` in both gate states. A payload carrying both
  keys or neither is a `DefensiveParseFailure`.
- **The copy that carries a value resolves through the catalog** (WCS-13). `messageWith()` is
  the parameterized resolution next to `message()` in `src/l10n/strings.ts`; no surface
  concatenates a sentence with a value.
- **The pinned capture carries a redacted token** (API-35, WCS-14). The verification capture
  keeps the session body's shape with the `redacted-access-token` placeholder, and the pinned
  suite's credential assertion looks for a credential _value_ instead of the key's name.
- **The capture script reads the code from the send intention** (WCS-14). The code never
  reaches a log line or the sender's output, so `capture.sh` reads it from the `email_outbox`
  collection with `mongosh`, and the run removes the send intentions it recorded so the day's
  allowance is restored. The fixtures' README documents the requirement and the quick local
  configuration.
