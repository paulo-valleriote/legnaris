# Specification Quality Checklist: The Email Worker

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-10-21
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

- **Validation run 1 (2026-10-21)**: all items pass. No marker remains; the decisions the program
  plan left open for this stage are inherited from the plan itself (D5, D6, O1, O2) and recorded in
  Assumptions.
- **On "no implementation details"**: the spec names four *contract* terms that the requirements are
  defined against, not technologies this feature chooses: the canonical identity shape that Principle
  II fixes (a lowercase UUID v7), the broker's delay tiers (the plan's decision O1, already declared
  by the delivered topology), the dead-letter route (the broker's queue for messages that can never
  succeed) and the idempotency key (the message identity handed to the provider). No language,
  framework, package or file is named, and the requirements are stated as observable outcomes
  ("the message is acknowledged only after the send succeeded", "the original stays unacknowledged",
  "the process fails fast with a typed error").
- **Bounded scope**: this feature ships the delivery *process*; the deployment of it — the compose
  service, the broker's management UI behind the ops. gateway, the metrics scrape and the alert rules
  that read these counters — is the next feature and is named as such in Assumptions. The request-
  serving process is untouched: it still never talks to the broker or the provider.
- **The gap this feature closes**: the preceding feature's research deferred the broker consumer
  ("the broker consumer is the next feature"), and the plan's stage 6 requires the worker as a
  deployed process. Without it, every queued code is a person waiting for an email that never
  arrives.
- **Traceability**: `WKR-01` … `WKR-27` are defined only in this spec. The plan's identifiers
  (`D5`, `D6`, `O1`, `O2`, `F-14`, `F-16`) and the delivered features' identifiers (`EML-*`,
  `OTP-*`) are cited, never redefined, so `scripts/check_requirement_ids.py` stays green.
- **Clarification session (2026-10-21)**: three questions were answered by user decision and
  integrated — the process does not consult the challenge store (`WKR-01`, `WKR-07`), an expired
  message is acknowledged and counted rather than dead-lettered (`WKR-03`, `SC-002`), and no
  dead-letter replay tool is in scope (Assumptions). The items were re-evaluated against the
  amended spec and all still pass: each answer is expressed as an observable outcome, names no
  framework, module or file, and leaves the feature's scope bounded. Pass count unchanged at
  16/16.
- **Analyze remediation (2026-10-21)**: the read-only analysis found two contradictions between MUST
  requirements — `WKR-02` enumerated only two outcomes for a delivery that was not sent successfully,
  which excluded the expired outcome of `WKR-03`, and `WKR-12` required the dead-letter route for a
  message that `WKR-03` forbids it from reaching. Both were amended by user approval: `WKR-02` now
  names all four outcomes, `WKR-12` now states that the expiry is decided first, and the precedence
  is recorded as an edge case. `SC-004` now counts deliveries rather than send attempts; the
  re-registration delay is fixed at 5 seconds; the "route" and "queue" wording is reconciled once in
  the destination contract; the retry window's interaction with the code's lifetime is recorded in
  Assumptions and in the worker contract; the tasks gained the two missing pieces (the contract's
  documentation touch and the self-skipping broker suite) and now cite every measurable outcome.
  The items were re-evaluated against the amended spec and all still pass. Pass count unchanged at
  16/16.
