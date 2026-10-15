# Contract: Requirement-ID Traceability for Client Sources

**Applies to**: `scripts/check_requirement_ids.py`, `Makefile` (`lint-specs`),
`.github/workflows/ci-spec-lint.yml`.
**Requirements**: SEC-07 (and the constitution's spec-corpus clause).

## Rule

Every requirement-ID-shaped token (`\b[A-Z][A-Z0-9]*-\d+\b`) cited in a client source file MUST be
defined in the spec corpus:

| Prefix | Must be defined in |
|---|---|
| `AD-*` | `.specs/STATE.md` |
| `LGN-*` | skipped — Jira card key |
| everything else (`FND-*`, `SEC-*`, `RND-*`, …) | `.specs/**/spec.md` |

A citation that resolves to nothing is a defect and fails the pipeline.

## Scope

- Source files scanned: `*.go`, `*.dart` (existing) and `*.ts`, `*.tsx` (added by this feature).
- Excluded directories keep the current list plus `node_modules`, `dist`, `.vite`, and Playwright
  report/output directories.
- Spec corpus read: `.specs/**/spec.md` for requirement IDs and `.specs/STATE.md` for `AD-*`; test
  fixtures that intentionally cite a non-existent ID live under a directory excluded from the scan
  or are asserted by the script's own self-test.

## Output

- Clean: `check_requirement_ids: clean (<N> spec IDs, <M> decision IDs)`.
- Violation: `path:line: <ID> not defined in <source>` with a non-zero exit, which is what
  `make lint-specs` and `ci-spec-lint` surface.

## Verification

| Check | Method |
|---|---|
| `*.tsx` fixture with a valid ID passes | `scripts/test_lints.py` self-test case |
| `*.ts` fixture with an unknown ID fails | `scripts/test_lints.py` self-test case |
| `node_modules` and build output are ignored | self-test case using a nested fixture directory |
| CI triggers on client sources | path-filter assertion |
