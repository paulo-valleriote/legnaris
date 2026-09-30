# AGENTS.md — desktop/ (Flutter Desktop App)

Visual and interaction rules that every agent must follow when building screens in
this directory. Design decisions are made **once, here** — not per screen.

## Design Principles

- **Functional before decorative.** Every pixel must carry data, hierarchy, or an
  affordance. Decoration that helps scanning (dividers, section headers) is allowed;
  decoration that only looks "designed" is not.
- **Dense yet legible.** Prefer tables, aligned rows, and compact controls over
  whitespace-driven layouts. Never go below the token sizes below to gain density.
- **Lowest cognitive load possible.** Plain labels first; captions/tooltips only when
  the interface would otherwise be ambiguous.
- **Desktop-first.** Mouse + keyboard, hover states, wide windows, persistent
  navigation. No mobile patterns (bottom bars, swipe-only actions, FABs).
- **Schema-driven UI.** Screens render whatever `FieldSchema` the template returns —
  never hardcode layout, labels, or field order for a specific RPG system.
- The wireframes in `docs/TDD-desktop-mvp.md` (Appendix) are the structural
  reference for V1 screens; this document defines how they are built.

## Avoid

| Avoid | Instead |
| --- | --- |
| Cards for everything | Dividers + section headers; cards only for genuinely bounded objects (e.g. a template entry in the catalog) |
| Gradients | Flat surfaces |
| Glassmorphism / backdrop blur everywhere | Opaque surfaces with hairline borders |
| Corner radius > 4px | Radius 2px or 4px (see Tokens) |
| Large headings | Type scale caps at 17px |
| Floating action buttons | Primary action in a fixed footer bar or toolbar |
| Decorative illustrations (incl. empty-state art) | One line of plain text |
| Icon overload | Text labels; icons only for universally recognized actions (save, delete, back) |
| "AI SaaS" aesthetic | Purple-blue gradients, glass panels, hero text, pill buttons everywhere, emoji as icons, glow effects — all prohibited |

## Layout

- **Left navigation is persistent** (never collapses into a hamburger/overlay on
  desktop); fixed width 200–240px.
- **Main content is width-constrained** to a readable measure (~960px for forms);
  do not stretch fields across a 2560px window.
- **Secondary information goes to a right panel** (280–320px): connection status,
  field details, derived/cascade preview, template summary.
- **Prefer horizontal alignment and tables** whenever the data is tabular (stats,
  modifiers, derived values).
- Shell order: `left nav | content | right panel (optional)`.
- **Resize behavior**: when the window narrows, hide the right panel first, then let
  content grow back to its min width; content never overlaps the nav.

## Tokens (concrete values)

These are fixed. Widgets must not contain magic numbers — every size, radius, and
color comes from the app theme (single source: `lib/theme/`, created with the first
screen phase).

| Token | Values |
| --- | --- |
| Type scale | 12 (caption/label) · 13 (base body & inputs) · 15 (emphasis) · 17 (max: app/section title) — **never above 17px** |
| Font family | Material defaults — no custom font in V1 (brand identity is V2) |
| Spacing | 4 · 8 · 12 · 16 · 24 only (multiples of 4) |
| Radius | 2px (inputs, buttons) · 4px (panels, dialogs) — **never > 4px** |
| Row height | 32px (tables, list rows) |
| Control height | 32–36px (inputs, buttons) |
| Left nav width | 200–240px fixed |
| Content max width | ~960px |
| Right panel width | 280–320px |
| Borders | 1px hairline using `ColorScheme.outlineVariant` |
| Elevation | 0 everywhere except dialogs |
| Colors | Derived from `ColorScheme` only — **no hardcoded hex in widgets**; destructive = `colorScheme.error`; disabled = 0.38 opacity |
| Tables | Numeric columns right-aligned; header row in 12px caption style |

## Interaction (desktop patterns)

- **Keyboard-first**: shortcuts are centralized in a single rebindable registry
  (a plain map of action → shortcut, consumed by Flutter's `Shortcuts` /
  `CallbackShortcuts` / `MaterialApp.shortcuts`) — so they stay configurable without
  an API change. No package needed for this. V1 defaults: `Ctrl+S` save,
  `Ctrl+N` new sheet, `Ctrl+1`/`Ctrl+2` navigate, `Esc` close panel/dialog,
  `F5` refresh.
- **Visible focus**: focus ring on every focusable control; tab order follows visual
  order; no keyboard traps.
- **Hover states are mandatory** on anything clickable: hover changes background/tint
  only — never layout or size (no shifting on hover).
- **Destructive actions require explicit confirmation** — a dialog stating the exact
  consequence (e.g. "Remove from local library — the sheet stays in the API").
  Never a one-click destroy, never confirm-by-Enter alone.
- **Avoid dialog overuse.** Prefer inline surfaces: footer bar for dirty state and
  Save, right panel for details, inline messages for errors. Dialogs are for exactly
  three cases: creation flows, destructive confirmation, and blocking errors.
- **Feedback**: every async action shows progress and an inline result (success or
  mapped error); disabled primary actions explain themselves via tooltip.

## Per-Screen Design Workflow (mandatory, in order)

Before writing widget code for a new screen, answer all nine — in this order:

1. **Information hierarchy** — what is read first, second, third; what can be
   demoted to the right panel or a tooltip.
2. **Primary actions** — at most one per screen (usually the footer bar action).
3. **Secondary actions** — toolbar/menu level; must not visually compete with #2.
4. **Navigation structure** — where this screen sits in left nav / back stack.
5. **Responsive behavior** — what happens at min window width (panel collapse
   order per Layout above).
6. **Desktop interaction patterns** — shortcuts, hover affordances, focus order
   for this screen.
7. **Reusable components** — candidates only if usage already exists ≥ 3 times;
   **defer extraction until then** (no premature abstraction).
8. **Components that must NOT be abstracted** — schema-driven field rendering
   (UI Engine), screen-specific layouts, one-off dialogs, wireframe structures.
9. **Consistency** — same patterns as existing screens (footer bars, section
   headers, error mapping, l10n strings).

## Critical Layout Review (mandatory)

After proposing a layout (and again before opening the PR), review it critically:

- Does it look **AI-generated / template-like**? (uniform card grid, giant hero
  heading, everything centered, gradient buttons)
- Is it **modern without reason**? (animation, blur, elevation carrying no meaning)
- Is it **visually repetitive**? (the same block repeated where aligned rows or a
  table would do)
- Is it **unnecessarily decorative**? (icons, accent colors, illustrations that
  carry no data)

When you find one, **make a concrete, local fix** — change that block, do not
redesign the whole screen.

## Componentization

- Extract a shared widget only when all hold: **≥ 3 real usages**, same shape and
  purpose, and the extraction does not leak schema-specific logic into it.
- Never abstract: the schema-driven UI Engine internals, screen-specific layouts,
  one-off dialogs, wireframe structures (per the table above).
- No "base widget" layers, no wrapper-for-wrapper's-sake.

## Dependencies

- **No new package without strict necessity.** Design-system dependencies
  (font packages, theme frameworks, component libraries) are prohibited in V1 —
  Material defaults plus the tokens above are sufficient.
- If a dependency is ever unavoidable, justify it in the PR description.

## Consistency

- All identifiers, comments, and user-facing strings in **English** (UI strings go
  through `lib/l10n/*.arb`).
- Reuse existing app infrastructure: errors via `lib/api/error_mapper.dart`, state
  via Riverpod, JSON via the API layer — never a second mechanism for the same job.
- **These are construction rules, not brand identity.** Palette and custom
  typography (branding) remain V2 per `docs/TDD-desktop-mvp.md`; V1 uses Material
  defaults within the tokens above.
