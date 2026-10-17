# Tasks: Web Screens Refactor

**Input**: Design documents from `.specs/features/010-web-screens-refactor/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: required — constitution III makes behavior ships with tests non-negotiable. Each story
opens with its suites (edge cases first) and they MUST fail before that story's implementation.

**Organization**: tasks are grouped by user story so each story is independently implementable and
testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no incomplete dependency)
- **[Story]**: the user story the task belongs to

## Path Conventions

- Client sources: `web/src/`
- Client tests: `web/src/test/` (flat, `*.test.ts(x)` suffix)
- Feature docs: `web/DESIGN.md`, `web/AGENTS.md`; decisions: `.specs/STATE.md`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: pin down the starting point so a later red is attributable to this feature.

- [x] T001 Run the baseline on the feature branch — `cd web && npm test && npm run lint && npm run typecheck` — and record the green result. No file changes; this is the reference the story checkpoints compare against.

**Checkpoint**: the client suites, lint and typecheck are green before any change.

---

## Phase 2: Foundational (Scan Extensions — required by every story)

**Purpose**: extend the source scans so every refactored directory is covered before any screen is
touched, so a later red is attributable to the screen, not a missing scan rule.

- [x] T002 [P] Extend `web/src/test/tokens_used.test.tsx` to scan `src/ui/`, `src/catalog/`, `src/sheet/`, `src/auth/`, and `src/library/`; keep the literal scan + token-resolution check; assert no hex literal (`#...`) or `\d+px` literal in the new directories, and every `var(--lg-*)` resolves (`UIP-23`).
- [x] T003 [P] Extend `web/src/test/l10n_strings.test.ts` to scan the same directories for user-facing literals (`UIP-24`).
- [x] T004 [P] Run `npm test` to confirm the extended scans pass on the baseline (no new literals introduced yet).

**Checkpoint**: the literal scan covers all refactored directories and the baseline (old screens)
still passes — there are no literal colors/pixels in the existing component code to begin with.

---

## Phase 3: Catalog Refactor (P1)

**Purpose**: refactor catalog onto F1 tokens and F2 primitives so it serves as the reference screen.

- [x] T005 [P] Catalog: extend `web/src/test/catalog_screen.test.tsx` to assert token-only styling and the two-region shell (no contextual panel).
- [x] T006 [P] Catalog: import F2 `Button` primitive (`src/ui/button.tsx`) for the primary "Create Sheet" action; remove the generic `.lg-button` from `catalog.css`.
- [x] T007 [P] Catalog: import F2 `Dialog` primitive (`src/ui/dialog.tsx`) for the create-sheet dialog; remove `.lg-panel*` from `catalog.css`; assert `role="dialog"`, `aria-modal`, Esc closes, focus returns to trigger.
- [x] T008 Catalog: replace all literal or non-token styling in `catalog.css` / `catalog_screen.tsx` with `--lg-*` tokens; assert no hex/pixel literal (`UIP-03`).
- [x] T009 Catalog: assert the contrast assertion passes for catalog content in both themes (`UIP-04`).
- [x] T010 Catalog: run `npm test && npm run lint && npm run typecheck`; fix failures.

**Checkpoint**: catalog renders identically to before but with F2 primitives and `UIP-03`/`UIP-04` green.

---

## Phase 4: Sheet Refactor (P1)

**Purpose**: refactor the sheet screen onto F1 tokens and F2 field/button/toast primitives.

- [x] T011 [P] Sheet: extend `web/src/test/sheet_screen_states.test.tsx` to assert token-only styling and the two-region shell.
- [x] T012 [P] Sheet: replace all form field controls with F2 `Field` primitive (`src/ui/field.tsx`, input variant) — permanent labels, inline error, token border/radius (`UIP-07`).
- [x] T013 Sheet: replace footer action buttons with F2 `Button` primitive; Save disabled when clean/invalid (`UIP-07`).
- [x] T014 Sheet: assert the sticky footer shows dirty state in muted color and the Save button uses `--lg-color-accent` (`UIS-03`).
- [x] T015 Sheet: inline validation errors appear under invalid fields using F2 styling (`--lg-color-border-strong` on border, `--lg-color-focus` on focus) (`UIS-05`).
- [x] T016 Sheet: add/modify `Toast` primitive usage for async action feedback (inline for result of action; toast only for non-blocking) (`UIS-01`).
- [x] T017 Sheet: run `npm test && npm run lint && npm run typecheck`; fix failures (`UIP-08`, `UIP-09`, `UIP-10`).

**Checkpoint**: sheet renders identically but with F2 primitives; dirty tracking, validation, and save flow behave exactly as before; `UIP-08`–`UIP-10` green.

---

## Phase 5: Auth Refactor (P1)

**Purpose**: refactor auth onto F1 tokens and F2 field/button/switch primitives.

- [x] T018 [P] Auth: extend `web/src/test/auth_screen.test.tsx` to assert token-only styling and centered panel (`UIP-13`).
- [x] T019 [P] Auth: replace the mode switch region with F2 `Switch` primitive; replace all `auth.css` literals with `--lg-*` tokens (`UIP-11`).
- [x] T020 Auth: replace form fields with F2 `Field` primitive (permanent labels, inline errors) (`UIP-12`).
- [x] T021 Auth: replace submit button with F2 `Button` primitive; disabled when invalid; centered panel layout preserved (`UIS-04`).
- [x] T022 Auth: run `npm test && npm run lint && npm run typecheck`; fix failures (`UIP-13`, `UIP-14`, `UIP-15`).

**Checkpoint**: auth mode toggle, fields, and submit button all render through F2 primitives; behavior is identical; `UIP-11`–`UIP-14` green.

---

## Phase 6: States Refactor (P2)

**Purpose**: refactor your-sheet, not-found, and update-prompt onto F1 tokens and F2 button primitive.

- [x] T023 [P] States: extend `web/src/test/your_sheet.test.tsx`, `not_found.test.tsx`, `update_prompt.test.tsx` — or their shared `sheet_screen_states.test.tsx` — to assert token-only styling (`UIP-18`).
- [x] T024 [P] Your-sheet: replace `.lg-empty*` styling with `--lg-*` tokens; "No sheet created yet" message uses `--lg-color-text-muted` (`UIP-20`).
- [x] T025 [P] Your-sheet: add F2 `Button` primitive for the "New sheet" action when no sheet exists (`UIP-17`); assert no action button when loading.
- [x] T026 [P] Not-found: add F2 `Button` primitive for "Go to catalog" (`UIP-17`, `UIP-21`); message uses `--lg-color-text-muted`.
- [x] T027 [P] Update-prompt: add F2 `Button` primitive for "Update now" (`UIP-17`, `UIP-22`); message uses `--lg-color-text-muted`.
- [x] T028 States: assert keyboard focus goes to the action button when present, focus ring visible (`SC-003`); `prefers-reduced-motion` honored (`UIP-30`).
- [x] T029 States: run `npm test && npm run lint && npm run typecheck`; fix failures (`UIP-16`–`UIP-19`).

**Checkpoint**: all three state screens render with token-only styling and F2 button primitive; `UIP-16`–`UIP-19` green.

---

## Phase 7: Final Gates & Deviations

**Purpose**: verify all tests green and record the AGENTS.md rule change.

- [x] T030 Run `make lint-web && make test-web && make test-race`; assert green.
- [x] T031 Run `make lint-specs`; assert green (the scan extensions in `tokens_used.test.tsx` / `l10n_strings.test.ts` are covered).
- [x] T032 Update `web/AGENTS.md`: add `src/ui/` as the sanctioned base-component layer and restate that the `≥ 3 real usages` extraction rule governs every other extraction (the `src/ui/` base-layer prohibition is now lifted). Document the deviation: old screens used a generic `.lg-button`; the new screens use F2 primitives.
- [x] T033 Update `web/AGENTS.md` deviations log with a F3 entry recording the refactored screens and the scan extensions.

**Checkpoint**: every test green, `lint-specs` clean, AGENTS.md documents the refactor and the base-layer rule change.

---

## Requirement Coverage

| Requirement | Task IDs |
|---|---|
| UIP-01 | T008, T010 |
| UIP-02 | T006, T007 |
| UIP-03 | T008 |
| UIP-04 | T009 |
| UIP-05 | T010 |
| UIP-06 | T017, T019 |
| UIP-07 | T012, T013 |
| UIP-08 | T017 |
| UIP-09 | T017 |
| UIP-10 | T014, T015, T016 |
| UIP-11 | T019 |
| UIP-12 | T020 |
| UIP-13 | T022 |
| UIP-14 | T022 |
| UIP-15 | T022 |
| UIP-16 | T029 |
| UIP-17 | T025, T026, T027 |
| UIP-18 | T023 |
| UIP-19 | T029 |
| UIP-20 | T024 |
| UIP-21 | T026 |
| UIP-22 | T027 |
| UIP-23 | T002 |
| UIP-24 | T003 |
| UIP-25 | T031 |
| UIP-26 | T001, T030 |
| UIP-27 | T002 |
| UIP-28 | T004 |
| UIP-29 | T035 |
| UIP-30 | T028 |
| SC-001 | T008, T012, T019, T024, T025, T026, T027 |
| SC-002 | T008, T022 |
| SC-003 | T028 |
| SC-004 | T010, T017, T022, T029 |
| SC-005 | T019 |
| SC-006 | T003, T033 |
| SC-007 | T028 |

## Implementation Strategy

- Each screen refactor is independent (catalog, sheet, auth, states) and can reuse the scan extensions (T002, T003).
- Tests first: extend the screen test files (T005, T011, T018, T023) before refactoring the screen itself.
- Token-only styling is enforced by `tokens_used.test.tsx`; the extended scan catches any regression.
- The AGENTS.md dev-rule change (T032) is recorded as `AD-019` in `.specs/STATE.md`.
- No new runtime dependencies; F2 primitives are plain React components.

## Notes

- T032 and T033 are `web/AGENTS.md` edits — outside the token/contrast scan exception, but AGENTS.md is a non-visual dev-rules document (`web/AGENTS.md:3` states "the visual rules live in DESIGN.md"); no token literal is introduced.
- `T035` (no new deps) is a gate assertion, not a code edit.
