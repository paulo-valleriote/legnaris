# Tasks: Motion, Polish and Visual Regression (F4)

**Input**: `.specs/features/011-web-motion-visual-regression/`
**Prerequisites**: F1, F2, F3 delivered; `lint-specs` green; `make test-web` green.

## Phase: Token + Motion System

- [ ] T001 [P] Declare `--lg-motion-*` / `--lg-ease-*` in `tokens.css`; mirror in `tokens.ts`; add dark-theme values.
- [ ] T002 [P] Add `@media (prefers-reduced-motion: reduce)` in `base.css`; clamp durations to near-zero; set scroll instant.
- [ ] T003 Apply token-based `transition` rules to F2 primitives (`src/ui/*.css`) — only permitted 6 properties.
- [ ] T004 Assert `tokens_used.test.tsx` passes (zero literal ms outside `theme/`); assert contrast passes both themes.

## Phase: Dark Theme Polish

- [ ] T005 Verify dark shadows (`shadow-1..3`) deeper than light per layer; add borders (`border`, `border-strong`) where needed.
- [ ] T006 Assert contrast passes for all shadow/border pairs in both themes (`tokens_contrast.test.ts`).

## Phase: Visual Regression Gate

- [ ] T007 Confirm `playwright.config.ts` uses preview origin (`VITE_API_BASE_URL`); build passes.
- [ ] T008 Generate approved baselines (`npx playwright exec --save-snapshots`) for catalog/sheet/auth/states in light/dark, motion off/on.
- [ ] T009 Commit baselines; assert `make test-web` passes (no unapproved drift).
- [ ] T010 Update `web/AGENTS.md` — restate `DESIGN.md` as the visual contract; document F4 deviation (no new components; baselines committed locally).

## Phase: Final Gates

- [ ] T011 `make lint-web && make test-web && make test-race` green.
- [ ] T012 `make lint-specs` green.
- [ ] T013 Commit all artifacts locally; state nothing pushed.
