---
name: program-plan-creator
description: Creates the initial program plan for a broad, multi-feature request before any Spec Kit artifacts exist — grounds every claim in the repository, closes open decisions with the user, and writes one versioned plan document under docs/ that maps the work into Spec Kit features. Use when the user says "faz um plano", "cria o plan inicial", "planeja esse projeto", "escreve o plan", "planejar antes de implementar", "define as fases", or hands over a broad goal with constraints and no plan yet. Do NOT use to implement code, to create or update .specs/features/* artifacts (use speckit-delivery), or to write a single-feature design document (use technical-design-doc-creator).
license: CC-BY-4.0
metadata:
  version: 1.0.0
---

# Program Plan Creator

Turn a broad request ("build X like Y, with constraints A and B") into **one reviewable program
plan**: a versioned document in the repository that states verified facts, closed decisions,
trade-offs, the feature sequence, and the definition of done. The plan is an input to the Spec Kit
cycle; it is not a spec, a design, or code.

## Critical rules

- **Never assert a repository fact you did not verify.** Read the file. Documents in `docs/` are
  often stale — the previous plan's most valuable section was the list of upstream docs the code had
  already invalidated. Quote the file and line that proves each fact.
- **The plan document lives inside the repository and is committed.** Default location:
  `docs/plan-<program-slug>.md`. A plan left in a scratch directory is unversioned, cannot be cited
  in a PR or card, and disappears with the container.
- **Close decisions with the user, do not assume them.** Ask in batches of at most three, each with
  a recommended option and the consequence of the alternatives. Prioritise scope > security/privacy
  > user experience > technical detail.
- **The plan defines no requirement IDs.** `AD-*` live in `.specs/STATE.md`; every other ID lives in
  a `.specs/**/spec.md`, which `/speckit.specify` writes. The plan only names the *prefixes* each
  feature will use.
- **Plan, do not build.** No application code, no `.specs/features/*` artifacts, no CI changes. If
  the user asks for implementation, finish the plan and hand off to `speckit-delivery`.
- **Write the plan in the language of its readers, not the code.** `docs/` is the constitution's
  explicit exception for pt-BR; everything outside `docs/` (specs, code, comments) stays English.
- **Version the document.** First release `v1`; every revision bumps the version and appends a
  changelog line. Amend sections in place instead of appending contradictions.

## Workflow

### Step 1 — Ground in facts

Read, in this order, before writing a single line of the plan:

1. `AGENTS.md` and every per-directory companion it lists.
2. `.specify/memory/constitution.md` (stack, principles, workflow, governance).
3. `.specs/STATE.md` (decisions `AD-*`) and the feature specs under `.specs/features/`.
4. The CI workflows in `.github/workflows/` (what is gated today, and by what paths).
5. `docs/` design documents, deliberately sceptically.
6. The target code trees for the area the plan touches, plus `Makefile`, `go.mod`, and the relevant
   package manifests/entry points.

Then produce a short list of **verified facts** and a second list of **documents that contradict the
code**. The contradiction list is the first section of the plan and often the most valuable — it
tells implementers which upstream document to stop trusting.

### Step 2 — Interview the user

Ask only what changes the architecture, the scope, or the acceptance criteria. Batch up to three
questions in one message; each question states the stake in one plain sentence and puts the
recommended option first with its reasoning. Categories that usually need an answer:

| Category | Typical question |
|---|---|
| Scope boundary | which of these candidate features are in this program, and which are explicitly out |
| Security posture | credential/token model, where secrets live, which threats are in scope |
| Topology | single origin or split origins, hosting model, environment names |
| Compatibility | target platforms/browser matrix, deprecation of the surface being replaced |
| Delivery | what "done" gates merge, and what remains a follow-up |

Record every answer in the **closed decisions** table. If a decision is genuinely deferred, record
it under **open points** with the reason — never leave it implicit.

### Step 3 — Choose the artifact location

- Default: `docs/plan-<program-slug>.md`.
- Honour an explicit user path, but never leave the plan outside the repository.
- If a previous revision exists, read it first and revise in place.

### Step 4 — Write the plan

Use `references/plan-template.md` as the skeleton. The sections that must exist:

1. **Header** — program, status and version, date, and a scope note stating that this document is
   the program plan, that it defines no requirement IDs (those live in `.specs/**/spec.md`, `AD-*` in
   `.specs/STATE.md`), and which language exception applies.
2. **Verified context and upstream contradictions** — the facts from Step 1, each with its source.
3. **Closed decisions** — the table from Step 2.
4. **Architecture and stack, with trade-offs** — one row per dimension (build, state, storage,
   routing, testing, security), each with the rejected alternatives and why.
5. **Threat model** — whenever the program touches auth, credentials, user data, or a browser
   surface. Threat, vector, mitigation, and the feature that implements the mitigation.
6. **Feature sequence** — a numbered table of features, each with its Spec Kit slug, what it
   delivers, and whether it blocks another feature. Order matters: a prerequisite that changes the
   server must come before the client feature that depends on it.
7. **Per-feature task sketch** — the smallest useful tasks per feature with the files they touch.
   This seeds `/speckit.tasks`; it is not the task list.
8. **Test strategy** — logic, interface, security, contract, end-to-end, accessibility: what is
   verified where, and which gates are blocking.
9. **Risks** — risk, impact, mitigation.
10. **Definition of done** — the exit criteria that mark the program complete.
11. **Next steps** — the exact Spec Kit sequence with the slug order from section 6.

Keep every table scannable and every claim checkable. No padding prose.

### Step 5 — Self-check before committing

- [ ] Every repository fact cites the file (and line, where it matters) that proves it.
- [ ] Every decision in the table was answered by the user — none inferred silently.
- [ ] Every feature has a unique slug, a dependency note, and a requirement-ID prefix that no
      existing feature uses (grep `.specs/**/spec.md` for the prefix).
- [ ] The portion of the plan that the team can build first (the MVP) is identifiable.
- [ ] No application code, no `.specs/features/*` artifact, and no CI edit was made.
- [ ] `make lint-specs` still passes when the repository has that target.
- [ ] The language rule holds: pt-BR only inside `docs/`.

### Step 6 — Commit and hand off

Commit with a `docs(specs):` scope, e.g.
`docs(specs): add the <program> program plan`. Local commit only — do not push without an explicit
go-ahead. Then report: the plan path, version, the feature sequence, and which feature is the MVP,
and hand off to `speckit-delivery` for the first feature.

## Revision protocol

1. Read the current plan and the artifacts produced since (`.specs/features/*`).
2. Amend the affected sections in place; do not append a second version of a section.
3. Bump the version, add one changelog line naming what changed and why.
4. Re-run Step 5, then commit.

## Anti-patterns

| Anti-pattern | Instead |
|---|---|
| Plan written from memory of the docs | Verify each fact in code first |
| A plan in a scratch directory or `/tmp` | `docs/plan-<program-slug>.md`, committed |
| Deciding auth/hosting/scope by inference | Ask, then record the answer in the table |
| Plan that defines requirement IDs | Define the prefixes; the spec defines the IDs |
| A feature list with no dependency order | Number the features and mark what blocks what |
| "Deliver everything" as the sequence | One Spec Kit cycle per feature, in dependency order |
| Implementation sneaking in | Stop at planning; hand off to `speckit-delivery` |

## References

- `references/plan-template.md` — the document skeleton to copy and fill.
