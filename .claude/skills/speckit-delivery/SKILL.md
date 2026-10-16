---
name: speckit-delivery
description: Delivers what a program plan asks for by running this repository's Spec Kit cycle end to end — locate the plan, scope the features, then constitution → specify → clarify → plan → tasks → analyze → implement per feature, using the repo's own scripts, templates, lints, and gates. Use when the user says "segue com o plan", "usa speckit para entregar", "executa o plano", "deliver F2", "run the spec kit cycle", "continue with the plan", "especifica a próxima feature", or references a plan document and asks for the next slice. Do NOT use to write the initial program plan (use program-plan-creator), to only review an existing plan, or to implement work that has no spec/plan/tasks artifacts yet.
license: CC-BY-4.0
metadata:
  version: 1.0.0
---

# Spec Kit Delivery

Execute a program plan through this repository's Spec Kit installation: one feature per cycle, in
dependency order, stopping at every gate and never inventing a requirement.

## Critical rules

- **Read the plan first.** The program plan (`docs/plan-*.md`) is the source of the feature
  sequence, the prerequisite order, and the requirement-ID prefixes. If the user names a feature,
  still read the plan's row for it before specifying.
- **The commands are prompt files.** `.opencode/commands/speckit.<step>.md` is the authoritative
  procedure for each step: read the file and follow its Outline. Do not improvise a shortcut, and do
  not skip a step silently — a consumed step is reported, not assumed.
- **Run the bash scripts with `bash`.** Everything under `.specify/scripts/bash/` uses bash-isms;
  invoking them with `sh` (or through a shell that does not honour the shebang) fails with
  `Bad substitution`. Use `bash .specify/scripts/bash/<script>.sh`.
- **Specs are the requirement-ID corpus.** `.specs/features/NNN-<slug>/spec.md` defines the IDs
  (`.specs/STATE.md` defines `AD-*`). The requirement prefix must be unique to the feature —
  `grep -r '<PREFIX>-' .specs` before choosing it.
- **Never invent a requirement.** Requirements come from the plan and the Jira card
  (`LGN-\d+`), never from convenience. If the plan is silent on something that changes scope,
  ask.
- **Tests ship with the behavior.** The constitution makes this non-negotiable: edge cases first,
  then one happy path, with the typed error asserted on failure paths. A task list with no tests is
  incomplete.
- **`/speckit.analyze` is read-only.** Report findings, offer remediation, and apply it only after
  the user approves. Never edit artifacts during analysis.
- **Do not push.** Commits are local unless the user explicitly authorizes a push; never print,
  log, or commit a token.

## Workflow

### Step 0 — Locate the plan and scope the work

1. Find the program plan: the path the user gave, else the newest `docs/plan-*.md`.
2. Read its feature sequence, prerequisite notes, and requirement-ID prefixes.
3. Read the repository rules: `AGENTS.md` and companions, `.specify/memory/constitution.md`,
   `.specs/STATE.md`, and the latest feature under `.specs/features/` (for numbering and style).
4. Confirm scope with the user in one short question when it is ambiguous (which features, or how
   far this session goes). Batching: one question, recommended option first. Do not ask what the
   plan already answers.
5. Confirm the cycle target: artifacts only, or artifacts and implementation.

### Step 1 — Constitution (only when the plan changes the rules)

Run only if the plan amends principles, the stack, or the workflow. Follow
`.opencode/commands/speckit.constitution.md`; resolve the template with
`bash .specify/scripts/bash/resolve-template.sh constitution-template --json`, write only
`.specify/memory/constitution.md`, and include the Sync Impact Report plus the semver bump rationale.
Commit separately: `docs(specs): amend constitution to vX.Y.Z (<what changed>)`.

### Step 2 — Specify

Follow `.opencode/commands/speckit.specify.md`, then:

```bash
bash .specify/scripts/bash/create-new-feature.sh --json --short-name <slug> "<one-line feature description>"
```

This creates `.specs/features/NNN-<slug>/spec.md` from the resolved template and persists
`.specify/feature.json` (machine-local, gitignored) so later steps find the feature. Then write the
spec:

- Keep the template's section order; fill every mandatory section; delete optional sections that do
  not apply instead of writing "N/A".
- Give the feature its plan-declared prefix; number requirements sequentially (`XXX-01`,
  `XXX-02`, …) and make each testable.
- State user stories with priorities, an independent test per story, edge cases, measurable success
  criteria, and assumptions.
- Write `checklists/requirements.md` and mark it honestly; iterate the spec (max 3 times) until the
  checklist passes.
- Verify: `make lint-specs` (requirement IDs must resolve).

### Step 3 — Clarify

Follow `.opencode/commands/speckit.clarify.md`: scan the spec for ambiguity, then ask only the
questions whose answers change architecture, scope, security, or acceptance — at most three per
round, each with a recommended option first and its consequence stated. Integrate each accepted
answer into a `## Clarifications` section **and** the section it changes, re-validate the checklist,
and stop asking as soon as the critical ambiguities are gone. If there are none, say so explicitly
and move on.

### Step 4 — Plan

Follow `.opencode/commands/speckit.plan.md`:

```bash
bash .specify/scripts/bash/setup-plan.sh --json
```

Fill `plan.md` (Technical Context with versions verified in the environment — check the real
toolchain, do not guess versions), run the Constitution Check as a gate with a post-design
re-check, and produce `research.md` (decision / rationale / rejected alternatives),
`data-model.md`, `contracts/`, and `quickstart.md`. Commit the research and design artifacts with
the plan.

### Step 5 — Tasks

Follow `.opencode/commands/speckit.tasks.md`:

```bash
bash .specify/scripts/bash/setup-tasks.sh --json
```

Write `tasks.md` in the strict format `- [ ] T0NN [P?] [US?] <action with file path>`, organized by
user story in priority order, with tests first (they must fail before the implementation they
cover), a dependency section, parallel examples, and an implementation strategy. Every task names
its file; every requirement from the spec maps to at least one task.

### Step 6 — Analyze

Follow `.opencode/commands/speckit.analyze.md`:

```bash
bash .specify/scripts/bash/check-prerequisites.sh --json --require-spec --require-tasks --include-tasks
```

Produce the read-only report: findings table with severities, coverage table (requirement → tasks),
constitution alignment, unmapped tasks, and metrics. Then offer remediation of the top findings and
**wait for approval** before touching any artifact. Re-run the analysis after applying it.

### Step 7 — Implement (only on explicit authorization)

When the user authorizes implementation, execute `tasks.md` in order, following
`.opencode/commands/speckit.implement.md`:

- One atomic Conventional Commit per task or logical group, tests in the same commit.
- Keep requirement-ID citations in the code referencing the spec that defines them.
- Run the repository gates before reporting: `make fmt && make lint && make vet && make
  lint-specs`, then `make test-race`; for client work also the client targets the plan declares
  (`make lint-web && make test-web && make sec-scan`).
- Never weaken or delete an assertion to make a gate pass — fix the behavior or record an approved
  amendment in the spec.
- Tick the checkbox for each completed task in `tasks.md`.
- Report the branch, the commits, the gate results, and what remains unchecked.

## Reporting and commits

After each step, report in the command's own completion-report shape: artifact paths, counts, gate
results, and what is ready next. Commit conventions:

| Work | Message |
|---|---|
| Constitution amendment | `docs(specs): amend constitution to vX.Y.Z (<what changed>)` |
| Spec, plan, research, design, tasks | `docs(specs): spec <the feature> (<what it covers>)` |
| Workflow / script / CI changes | `<type>(<scope>): <what changed>` |
| Feature implementation | Conventional Commits, one logical change per commit |

Local commits only; state explicitly that nothing was pushed.

## Stop gates

Pause and ask before: amending the constitution's principles, resolving a scope ambiguity the plan
does not answer, applying analyze's remediation, implementing code, pushing, or deleting anything
the user authored. Never pause to ask for information the plan or spec already contains.

## References

- `references/step-playbook.md` — per-step commands, artifacts, and repository-specific quirks.
