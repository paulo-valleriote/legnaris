# Program plan skeleton

Copy this skeleton into `docs/plan-<program-slug>.md` and fill every bracket. Delete a section only
when it is genuinely inapplicable, and say why in one line instead of leaving a placeholder.

---

```markdown
# [Program name] — [v1]

> **Program**: [one sentence: what is being built or replaced, and for whom].
> **Status**: [v1 draft | v1 FINAL] ([date]). Planning document — execution happens through the
> Spec Kit cycle (see `.specs/features/`).
> **Scope of this file**: this is the program plan. It defines **no** requirement IDs — those live
> in `.specs/**/spec.md` and architecture decisions in `.specs/STATE.md` (`AD-*`). [pt-BR is allowed
> here because `docs/` is the constitution's exception; everything outside `docs/` is English.]
>
> Changelog: [v1 — initial plan].

## 1. Verified context (facts checked against the code)

- [Fact, with the file that proves it.]
- [Fact, with the file that proves it.]

### 1.1 Upstream documents that contradict the code

| Document | Claim | Reality (proof) |
|---|---|---|
| [docs/...] | [what it says] | [what the code does — file:line] |

## 2. Closed decisions

| Theme | Decision | Decided with the user on |
|---|---|---|
| [scope / stack / auth / topology] | [decision] | [date or round] |

## 3. Architecture and stack, with trade-offs

| Dimension | Choice | Trade-off / rejected alternatives |
|---|---|---|
| Build/UI | | |
| State | | |
| Storage | | |
| Routing | | |
| Testing | | |
| Security tooling | | |

### 3.1 Rejected alternatives

[One paragraph per serious alternative that was evaluated, and why it lost.]

## 4. Threat model

| Threat | Vector | Mitigation | Feature |
|---|---|---|---|
| | | | [F#] |

### 4.1 Inviolable project rules

- [e.g. forbidden API/sink, credential storage rule, allowed origins.]

## 5. Feature sequence (Spec Kit)

| # | Slug | Delivers | Blocks |
|---|---|---|---|
| F1 | `[slug]` | | — |
| F2 | `[slug]` | | [F3] |

Per feature the cycle is: `/speckit.specify` → `/speckit.clarify` → `/speckit.plan` →
`/speckit.tasks` → `/speckit.analyze` (→ `/speckit.implement` when authorized).

Each feature claims a unique requirement-ID prefix: [F1 → `XXX-*`, F2 → `YYY-*`, …].

## 6. Per-feature task sketch

### F1 — `[slug]`

- [ ] [task, with the file it touches]
- [ ] [task]

### F2 — `[slug]`

- [ ] [task, with the file it touches]

## 7. Test strategy

| Type | Scope | How it is verified |
|---|---|---|
| Logic | | |
| Interface | | |
| Security | | |
| Contract | | |
| End-to-end | | |
| Accessibility | | |

**Cross-cutting acceptance**: [which gates are blocking, and that no gate may `continue-on-error`, no
citation may go untraced, and no assertion may be vacuous.]

## 8. Risks

| Risk | Impact | Mitigation |
|---|---|---|
| | | |

## 9. Definition of done

- [ ] [Exit criterion that marks the program complete.]
- [ ] [Exit criterion.]

## 10. Next steps

1. [Review and approve this plan.]
2. [Run the Spec Kit cycle for F1, in the order of section 5, with the declared prerequisites.]

## Appendix [letter] — [deep dive]

[Optional: worked detail for a decision the feature cycle will not re-open, e.g. a wire format or a
reporting pipeline.]
```
