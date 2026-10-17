# Contract: Motion System (tokens)

**Applies to**: `web/src/theme/tokens.css`, `web/src/theme/tokens.ts`, every component declaring a transition.
**Requirements**: UIM-01, UIM-02, UIM-03, UIM-04, UIM-05, UIM-08, SC-001, SC-002, SC-003
**Supersedes**: none (new surface — F1 declared `--lg-motion-120/180/240` names but no per-property motion contract).

## Declared tokens

| Token | Value | Role |
|---|---|---|
| `--lg-motion-120` | `120ms` | short hover / state transitions |
| `--lg-motion-180` | `180ms` | dialog / panel open |
| `--lg-motion-240` | `240ms` | page-level transitions |
| `--lg-ease-standard` | `cubic-bezier(...)` | standard easing |
| `--lg-ease-emphasized` | `cubic-bezier(...)` | emphasized easing |

## Rules

- Only the six properties may appear in a `transition`: `background`, `color`, `border-color`, `box-shadow`, `transform`, `opacity`.
- A hover state changes background/tint only — never layout, size or position.
- No literal ms value appears outside `src/theme/`.
- Under `prefers-reduced-motion: reduce`, all non-essential transitions/animations clamp to near-zero and scroll becomes instant.

## Enforcement

| Check | Method |
|---|---|
| Motion durations/easings resolve to tokens | `tokens_used.test.tsx` + source scan |
| No literal ms outside `src/theme/` | source scan in `tokens_used.test.tsx` |
| Six-property transition restriction | lint / code review gate |
| Reduced-motion clamped | Playwright measurement (`SC-002`) |
