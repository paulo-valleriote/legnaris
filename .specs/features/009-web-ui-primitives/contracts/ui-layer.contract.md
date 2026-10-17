# Contract: Shared UI Layer

**Applies to**: every primitive under `web/src/ui/`, and every later screen that consumes one.
**Requirements**: UIP-01 … UIP-06, UIP-03 (documents), SC-002, SC-004, SC-006, SC-007.
**Supersedes**: the `web/AGENTS.md` bullet "No 'base component' layers, no wrapper-for-wrapper's
sake" — replaced by the sanction below, which keeps the three-usages rule for every other extraction.

## The layer

| Field | Value |
|---|---|
| Path | `web/src/ui/` |
| Primitives | `button.tsx`, `field.tsx`, `checkbox.tsx`, `dialog.tsx`, `toast.tsx` |
| Helper | `focus_trap.ts` (the dialog's focus confinement) |
| Styles | `ui.css` — one token-only stylesheet |
| Class convention | `lg-<block>__<element>--<modifier>` |

Rules:

- Exactly these five primitives and their helper live in the layer (`UIP-01`).
- The layer is a leaf: no import specifier may name `api/`, `app/`, `auth/`, `catalog/`, `engine/`,
  `library/`, `pwa/`, `sheet/` or `shell/`. The only client import allowed is `../l10n/strings`, and
  every visual value arrives as a `--lg-*` custom property resolved from `src/theme/tokens.css`
  (`UIP-02`).
- No barrel module: a consumer imports the primitive it uses.

## The rule change

- `web/AGENTS.md` states that `web/src/ui/` is the one place a base component may live, and that the
  **three real usages** rule still governs every other extraction (`UIP-03`).
- `web/DESIGN.md` names the layer, its primitive set and the class convention (`UIP-03`).
- The prohibition is *replaced*: neither document may still forbid a base component layer
  (`SC-007`).
- The decision is recorded as `AD-019` in `.specs/STATE.md`.

## Enforcement

| Check | Method |
|---|---|
| The primitive set exists, and nothing in the layer imports a feature directory | `ui_layer.test.ts` |
| No literal colour or pixel value; every referenced token resolves | `tokens_used.test.tsx` (`web/src/ui/` block) |
| Every rendered string resolves through the catalog; no JSX text literal | `l10n_strings.test.ts` (`web/src/ui/` block) |
| Both documents carry the layer rule | `design_guidelines.test.ts` |
| Requirement citations in the stylesheet resolve | `scripts/check_requirement_ids.py` walks `.css`; `make lint-specs` |

## Out of scope

Screen migration. The catalog, sheet, auth and state surfaces still render their own `lg-button` /
`lg-overlay` / `lg-dialog*` markup and styles; the screen-refactor phase switches them onto this
layer. This feature therefore adds no route and no browser spec.
