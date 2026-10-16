# Spec Kit step playbook (this repository)

Repository-specific detail for each cycle step. The authoritative procedure is always the matching
command file under `.opencode/commands/`; this playbook lists the commands, the artifacts, and the
quirks that bite.

## Installation facts

| Fact | Value |
|---|---|
| Spec Kit version / integration | see `.specify/integration.json` (integration `opencode`, script `sh` → run with **bash**) |
| Command procedures | `.opencode/commands/speckit.{constitution,specify,clarify,plan,tasks,analyze,implement,checklist,converge,taskstoissues}.md` |
| Scripts | `.specify/scripts/bash/{common,resolve-template,create-new-feature,setup-plan,setup-tasks,check-prerequisites}.sh` |
| Templates | `.specify/templates/{spec,plan,tasks,checklist,constitution}-template.md` |
| Constitution | `.specify/memory/constitution.md` |
| Spec corpus | `.specs/features/NNN-<slug>/` (`SPECS_DIR` is hard-coded to `$REPO_ROOT/.specs/features`) |
| Feature pointer | `.specify/feature.json` — machine-local, **gitignored**, rewritten by specify |
| Extension hooks | `.specify/extensions.yml` — checked by every command; absent here, so skip silently |

### Quirks

- **Always `bash`.** `bash .specify/scripts/bash/<script>.sh`. Under `sh` the scripts die with
  `Bad substitution` / `source: not found`.
- **`--json` everywhere.** `create-new-feature`, `setup-plan`, `setup-tasks`, and
  `check-prerequisites` all take `--json` and print a machine-readable payload; parse it instead of
  guessing paths.
- **Numbering is sequential** (`.specify/init-options.json` → `feature_numbering: sequential`), so
  slugs come out as `001-<slug>`, `002-<slug>`, …. Pass `--short-name` to control the slug; run
  `--dry-run` first when unsure.
- **One feature per specify invocation.** Never create two feature directories at once.
- **`create-new-feature.sh` writes the spec file from the template** — you then fill it in place.
- **Templates resolve through a stack** (`resolve-template.sh`), not a fixed path, so use the
  resolver when the constitution flow needs the template content.

## Per-step command and output

| Step | Command | Output |
|---|---|---|
| Constitution | `bash .specify/scripts/bash/resolve-template.sh constitution-template --json` | `.specify/memory/constitution.md` + Sync Impact Report |
| Specify | `bash .specify/scripts/bash/create-new-feature.sh --json --short-name <slug> "<desc>"` | `.specs/features/NNN-<slug>/spec.md`, `checklists/requirements.md`, `.specify/feature.json` |
| Clarify | none (edits the spec) | `## Clarifications` section + updated requirements + re-validated checklist |
| Plan | `bash .specify/scripts/bash/setup-plan.sh --json` | `plan.md` (from template), then `research.md`, `data-model.md`, `contracts/`, `quickstart.md` |
| Tasks | `bash .specify/scripts/bash/setup-tasks.sh --json` | `tasks.md` (from template) |
| Analyze | `bash .specify/scripts/bash/check-prerequisites.sh --json --require-spec --require-tasks --include-tasks` | chat report only — **no file writes** |
| Implement | none (executes `tasks.md`) | code + tests + ticked checkboxes |

## Requirement IDs

- Shape: `[A-Z][A-Z0-9]*-\d+`. `AD-*` → `.specs/STATE.md`; `LGN-*` → Jira (skipped); everything else
  → `.specs/**/spec.md`.
- One unique prefix per feature, declared in the program plan. Check before using:
  `grep -rho '\b[A-Z][A-Z0-9]*-[0-9]\+' .specs | sort -u | cut -d- -f1 | sort -u`
- Gate: `make lint-specs` runs `scripts/test_lints.py`, `check_test_assertions.py`,
  `check_requirement_ids.py`, `check_validation_citations.py`.
- Today `check_requirement_ids.py` scans `*.go`/`*.dart` only. If a feature adds another source
  language (TypeScript, …), extending that suffix list is part of the feature, not an afterthought —
  otherwise the citations have no gate.

## Test conventions the tasks must encode

- Edge cases first (nil/empty/zero, boundaries, invalid input, missing keys, idempotency,
  concurrency), then a representative happy path.
- Case names prefixed `Happy path:` / `Edge:` / `Error:`; table-driven with `t.Run`.
- `testify`: `require` for preconditions, `assert` otherwise. Mocks are mockery-generated
  (`NewMockX(t)`) and never hand-written; regenerate with `make mocks` after an interface change.
- `go test -race` stays clean; integration tests self-skip without MongoDB.
- Client work inherits the same spirit: logic tests plus tests that render the interface
  requirements.

## Definition of done for one cycle

- [ ] `spec.md` validated against its quality checklist.
- [ ] Ambiguities either resolved with the user or explicitly reported as none.
- [ ] `plan.md` passes the Constitution Check, before and after design.
- [ ] `tasks.md` covers every requirement, tests included and ordered first.
- [ ] `analyze` report has no CRITICAL finding, and approved remediation is applied.
- [ ] `make lint-specs` clean; every cited requirement ID resolves.
- [ ] Artifacts committed (`docs(specs): …`), nothing pushed.

## Handy checks

```bash
# Current feature pointer
cat .specify/feature.json

# Existing features and their prefixes
ls .specs/features
grep -rho '\b[A-Z][A-Z0-9]*-[0-9]\+' .specs/features | cut -d- -f1 | sort -u

# Spec corpus lint (also what CI runs)
make lint-specs

# Program plan
ls docs/plan-*.md
```
