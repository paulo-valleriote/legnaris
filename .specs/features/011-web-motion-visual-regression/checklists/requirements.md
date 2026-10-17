# Specification Quality Checklist: Motion, Polish and Visual Regression (F4)

**Purpose**: Validate spec completeness and quality before planning/implementation.
**Created**: 2026-10-17
**Feature**: `.specs/features/011-web-motion-visual-regression/` (prefix `UIM`)

## Content Quality

- [x] No implementation details leak — tokens, CSS, Playwright only.
- [x] Focused on user value (deterministic motion + stable visuals).
- [x] Written for non-technical stakeholders (user scenarios describe the journey).
- [x] All mandatory sections completed (`User Scenarios`, `Requirements`, `Success Criteria`, `Assumptions`).

## Requirement Completeness

- [x] No `[NEEDS CLARIFICATION]` markers remain — all decisions are in plan/spec/assumptions.
- [x] Requirements are testable and unambiguous (`UIM-01` through `UIM-16`).
- [x] Success criteria are measurable (`SC-001` through `SC-007`) and technology-agnostic (Playwright is gate tooling, not feature tech).
- [x] All acceptance scenarios defined for 3 user stories (motion, dark polish, visual regression) + edge cases.
- [x] Scope clearly bounded (no new deps, existing suites must stay green, Playwright already present).
- [x] Dependencies and assumptions identified (requires F1–F3 delivered; Playwright exists).

## Feature Readiness

- [x] All functional requirements have clear acceptance criteria.
- [x] User scenarios cover primary flows (reduced-motion honored, dark shadow separation, baselines stable).
- [x] Success criteria are verifiable (token scan, reduced-motion measurement, lint, baseline diff).
- [x] No implementation details leak.

## Notes

- Motion rules live in `web/DESIGN.md` §3.7 and `src/theme/tokens.css`; they are enforced by scan and by Playwright measurement.
- The visual-regression gate runs under two motion states (off + on); both are baselined.
- `web/AGENTS.md` is restated as non-visual (`DESIGN.md` is the visual contract).
