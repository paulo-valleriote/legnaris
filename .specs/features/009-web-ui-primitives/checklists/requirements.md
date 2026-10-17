# Specification Quality Checklist: Web UI Primitives

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-10-17
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

## Requirement Completeness

- [x] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Success criteria are technology-agnostic (no implementation details)
- [x] All acceptance scenarios are defined
- [x] Edge cases are identified
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

## Feature Readiness

- [x] All functional requirements have clear acceptance criteria
- [x] User scenarios cover primary flows
- [x] Feature meets measurable outcomes defined in Success Criteria
- [x] No implementation details leak into specification

## Notes

- No question needed a person: every ambiguity the plan leaves open is answered by
  `docs/plan-ui-refactor.md` (F2 names exactly the five primitives; §3.8 gives the field family;
  §3.12 asks for the extended scans; §6 asks for the base-layer rule to be replaced) and by
  `web/DESIGN.md`'s component specifications. The two derived choices are written down in
  Assumptions instead of hidden: the field family stops at the text input, the select and the
  multi-line text area (the radio and the switch land with the screen that needs them), and the
  notice owns no dismissal schedule.
- The requirements name token roles, class conventions and DOM-level semantics (a live region, modal
  semantics, tab order) because those *are* the behaviour being specified — a component-primitive
  contract whose whole purpose is to be asserted by rendering tests. The stack
  (React/Vite/TypeScript, token CSS, Vitest) is confined to Assumptions, matching the sibling specs
  (`005`, `007`, `008`).
- Scope is deliberately the *layer* only: the screen-refactor phase switches the existing surfaces
  onto the primitives. Every requirement therefore has a rendering or source-scan assertion that
  needs no mounted route, exactly as the schema-driven engine's spec was shaped when it landed with
  no screen.
- The "no base component layer" prohibition from `web/AGENTS.md` is replaced by a requirement that
  the developer-rules document sanction `web/src/ui/` and keep the "three real usages" rule for every
  other extraction (`UIP-03`), so the rule change is specified and tested, not argued in review.
