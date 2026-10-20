---
name: pr-review-resolver
description: Resolves the AI review comment already posted on a GitHub PR — fetches the latest github-actions[bot] comment, triages every finding as fix, false positive, or left on purpose, commits the fixes to the same PR branch, and posts a categorized English reply comment. Use when the user shares a PR link and says "resolve the review", "analisa a review", "trata o comentário da AI", "fix what the reviewer pointed out", "responde a review do PR", or "apply the review suggestions". Do NOT use for writing the review itself, opening or creating PRs, replying to human reviewers, or code review without an existing review comment.
license: CC-BY-4.0
metadata:
  author: Paulo Valleriote
  version: 1.0.0
---

# PR Review Resolver

Resolve an AI review already posted on a PR: triage every finding, fix the valid ones, push to the same PR branch, and answer the reviewer with what was fixed, what is a false positive, and what was left on purpose.

## Critical rules

- Never print, log, or commit the `OPENCODE_PR` token.
- No code edits before triage (Step 4) is complete — every finding gets a classification first.
- Only findings classified **Fix** get code changes. False positives and left-on-purpose findings get zero edits.
- Every statement in the reply comment must be verifiable — cite `file:line` or command output.
- The reply comment is in English and has exactly three sections: Fixed, False positives, Left on purpose.
- If the PR is from a fork, the working tree is dirty, or no review comment can be found: stop and ask the user instead of improvising.
- The reviewer can be wrong. `AGENTS.md` (repo conventions) is the ground truth whenever the reviewer contradicts it.
- This skill reacts to an existing review comment; it never writes the review itself and never edits the reviewer's comment.

## Workflow

### Step 1 — Preconditions

1. Work from the repository root (verify with `git rev-parse --show-toplevel`); all commands below assume it.
2. Confirm the token exists: the repo-root `.env` contains `OPENCODE_PR=` (or it is already in the environment). Never print its value.
3. `git status --porcelain` must be empty. If it is dirty, stop and ask the user what to do with their uncommitted work — never stash or discard it.
4. Read `AGENTS.md` in the repo root. Its conventions (typed errors, English-only, layering, PR checklist) are the triage ground truth.

Expected output: clean tree, token available, conventions in context.

### Step 2 — Fetch the review

Run from the repo root:

```bash
python3 .claude/skills/pr-review-resolver/scripts/pr_review.py fetch "<pr-url>"
```

Expected output: JSON with PR metadata (`base_ref`, `head_ref`, `head_sha`, `head_is_fork`) and `review` (`comment_id`, `comment_url`, `body`).

- `review` is `null` → no sectioned review comment by the searched author. Do not guess: go to Troubleshooting.
- `head_is_fork` is `true` → STOP and ask the user; fixes cannot be pushed to a fork from this checkout.
- Save the review `body` — it is the input for Step 4.

### Step 3 — Check out the PR head branch

Depends on Step 2 (`head_ref`, `head_sha`). If the branch already exists locally:

```bash
git checkout <head_ref>
git fetch origin <head_ref>
git merge --ff-only origin/<head_ref>
```

`merge --ff-only` fails → stop and ask the user (local branch has diverged).

If the branch does not exist locally:

```bash
git fetch origin <head_ref>
git checkout -b <head_ref> --track origin/<head_ref>
```

Validation: `git rev-parse HEAD` must equal `head_sha` from Step 2. Mismatch → fetch again; still mismatched means the PR moved mid-run — re-run Step 2 and continue from Step 4 (no edits exist yet, nothing is lost).

### Step 4 — Triage every finding (the correction plan)

Do this completely before touching code.

1. Split the review `body` into individual findings — each bullet under a `##` section. Sections reporting "None" are acknowledgements, not findings; skip them.
2. For each finding:
   a. Open the referenced file at HEAD and verify the quoted `file:line` exists and says what the reviewer claims.
   b. Check the flagged code is part of this PR's diff: `git diff origin/<base_ref>...HEAD -- <file>`.
   c. Classify using the decision rules and record a one-line rationale.

Decision rules:

| Classification | Criteria (any one applies) |
|---|---|
| **Fix** | Factually correct, flagged code is in this PR's diff, and the suggested change does not contradict AGENTS.md. |
| **False positive** | Claim is factually wrong, flagged code is not in this PR's diff, finding targets a stale revision the PR already changed, or it contradicts a documented repo convention with no actual defect. |
| **Left on purpose** | Observation is valid but acting on it is out of scope for this PR, would change intended behavior, would break another requirement, or is a style preference conflicting with AGENTS.md. Requires an explicit reason. |

Notes:
- When the reviewer suggests a concrete patch, evaluate the patch itself — a correct observation can carry a wrong suggestion (fix it correctly, or classify by the actual defect).
- Findings whose line vanished because the PR moved on after the review → **False positive** (stale), and say so.
- Tiebreak: false positive = the facts are wrong; left on purpose = the facts are right but the action is rejected.

Expected output: a triage list — finding → classification → rationale → planned edit. This list becomes both the reply comment (Step 8) and your final chat summary.

### Step 5 — Implement the fixes

Only for findings classified **Fix** (depends on Step 4).

- Make the smallest change that resolves the finding. No drive-by refactoring, no reformatting untouched code.
- Follow AGENTS.md conventions: typed errors via `apperr` at service/controller layers and layer-local typed errors elsewhere, English-only strings and comments, `backend/shared/json` for decoding, correct HTTP status mapping for any new error code.
- A fix that adds a comment or doc line must not duplicate text the finding flagged as duplication — reference the canonical location instead.
- A fix that proves impossible without breaking another requirement → reclassify as **Left on purpose** with that reason and edit nothing for it.

### Step 6 — Verify (gate before committing)

Run what applies to the files you changed:

- Go code changed: `go build ./... && go vet ./... && go test ./... && make lint`
- Compose/YAML changed and docker available: `docker compose -f <file> config -q` for each touched compose file
- Docs/Markdown only: re-read your diff; no other gate

Failure handling:
- Failure caused by your edits → fix it. If you truly cannot, revert that specific fix, reclassify it as **Left on purpose** (reason: the change breaks X), and report it honestly.
- Failure unrelated to your edits (confirm the failing test does not touch any file you changed) → record it as pre-existing with the test name, do not block on it, and mention it in the chat summary.

### Step 7 — Commit and push to the same PR

1. `git status --porcelain` — stage only intentionally changed files (`git add <file>` per file), never `git add -A` blindly.
2. Commit in English, one commit preferred: subject `fix: address AI review findings on PR #<N>`, body with one bullet per fix.
3. `git push origin HEAD:<head_ref>`.
4. Push rejected (remote moved) → `git fetch origin <head_ref> && git rebase origin/<head_ref>`, re-run Step 6, push again. Second rejection → stop and ask the user.

### Step 8 — Post the reply comment

1. Write the body to `/tmp/opencode/pr-reply-<N>.md` using the template below. Every bullet comes from Step 4's rationale; reference findings by their review section and `file:line`.
2. Post it:

```bash
python3 .claude/skills/pr-review-resolver/scripts/pr_review.py post "<pr-url>" --body-file /tmp/opencode/pr-reply-<N>.md
```

3. Validate the output contains `comment_url`; if posting failed, fix and retry once, then report.
4. Report to the user: commit sha, push result, `comment_url`, and the same three-section summary you posted.

## Reply comment template

````markdown
## Fixed

- **<review section> — `<file>:<line>`** — <what was wrong> → <what changed>, verified with `<command>`.

## False positives

- **<review section> — `<file>:<line>`** — <why the finding is factually wrong, citing the code or convention that disproves it>.

## Left on purpose

- **<review section> — `<file>:<line>`** — <valid observation> — not doing it because <explicit reason>.
````

Template rules: keep all three headers even when a section is empty (write `None.`); keep the reviewer's original section names; one bullet per finding; English; end every bullet with the decision, never a question.

## Examples

### Example 1 — full cycle

User says: `https://github.com/paulo-valleriote/legnaris/pull/18 — resolve a review`

Actions:

1. `fetch` → `head_ref: review/1-deploy-security`, `head_is_fork: false`, review body with 6 sections and 9 findings.
2. Checkout `review/1-deploy-security`, confirm `HEAD == head_sha`.
3. Triage: missing `${MONGO_APP_PASSWORD:?}` guard → **Fix**; "duplicate `compactor:` key" claim disproven by reading `deploy/loki/loki-config.yml` → **False positive**; "move the observability stack to a dedicated network" is valid but a redesign outside this PR → **Left on purpose**.
4. Apply only the Fix edits.
5. Compose-only diff → `docker compose -f deploy/docker-compose.yml config -q`.
6. Commit `fix: address AI review findings on PR #18`, push to `review/1-deploy-security`.
7. Post the three-section reply; report sha + `comment_url`.

Result: PR updated with the fix commit and an English reply classifying all 9 findings.

### Example 2 — no review comment found

User says: `analisa a review do PR 22`

Actions: `fetch` returns `review: null` with `bot_comment_count: 3` (CI status logs).

Result: no code changes, nothing posted. Report that no sectioned review comment exists from `github-actions[bot]`, and ask whether the review came from another AI login (re-run fetch with `--author <login>`) or was posted as an inline review (unsupported by this skill).

## Troubleshooting

### fetch exits 2 — "token not found"

Cause: no `OPENCODE_PR` in the environment or repo-root `.env`.
Solution: add `OPENCODE_PR=<token>` to `.env` (gitignored); never echo the token.

### fetch exits 2 — HTTP 404

Cause: wrong PR url, private repo, or token lacks `repo` scope.
Solution: confirm the url opens in the browser; use a token with access to the repo.

### `review` is `null`

Cause: no sectioned (`## ...`) comment by the searched author — stale CI output, a different AI account, or an inline review.
Solution: ask the user; re-run fetch with `--author <login>` if they name another account. Never triage CI log output as a review.

### `head_is_fork` is `true`

Cause: the PR branch lives in a fork; this checkout cannot push it.
Solution: stop and ask the user how they want to proceed (e.g. they push the branch themselves). Present the triage plan so no analysis is wasted.

### Push rejected (non-fast-forward)

Cause: the PR branch moved since Step 3.
Solution: Step 7.4 — fetch, rebase, re-run Step 6, push once more; then stop on a second failure.

### `HEAD != head_sha` after checkout

Cause: PR updated mid-run.
Solution: Step 3 validation — re-run Step 2 and continue from Step 4.

### Working tree is dirty

Cause: the user has uncommitted work in the repo.
Solution: stop and show `git status --porcelain`; ask what to do. Never stash or discard.

### Tests or lint fail after your fixes

Cause: either your edit broke something, or the failure pre-exists.
Solution: Step 6 failure handling — fix, or revert + reclassify, or record as pre-existing.
