# Phase 1 Data Model: Web UI Foundation

**Feature**: `008-web-ui-foundation` | **Date**: 2026-10-17 | **Spec**: [spec.md](./spec.md)

This feature's "data" is the design contract: the token sets, the theme, the stored preference, the
shell regions and the two written documents. Each entity lists its fields, the rules that constrain
it, and the requirements that make those rules testable. There is no server entity and no migration.

---

## 1. Design token

| Field | Meaning | Rule |
|---|---|---|
| `name` | CSS custom property, `--lg-kebab-case` | unique across the file; referenced by name from `tokens.ts` (`UIF-01`) |
| `category` | layer: `primitive` \| `semantic` \| `component` | a component token exists only where no semantic covers the need (`UIF-01`) |
| `value` (per theme) | the declared literal (primitive) or the alias to a primitive (semantic) | every semantic resolves in **both** themes (`UIF-02`) |

**Layers**

1. **Primitives** — `--lg-ink-*` (warm neutral ramp), `--lg-amber-*` (accent ramp),
   `--lg-danger-*`, `--lg-space-2/4/8/12/16/24/32/48`, `--lg-radius-sm/md/lg/pill`,
   `--lg-font-family/size-*/weight-*/line-*`, `--lg-motion-120/180/240` and the easing pair, plus the
   size tokens that survive (`--lg-size-hairline`, `--lg-size-row`, `--lg-size-control`,
   `--lg-size-nav-min/max`, `--lg-size-content-min/max`).
2. **Semantics** — the roles components consume:

   | Role | Token |
   |---|---|
   | Page background | `--lg-color-bg` |
   | Card / region surface | `--lg-color-surface`, `--lg-color-surface-hover` |
   | Raised layer (popover, card above surface) | `--lg-color-raised` |
   | Inset / recessed (wells, code) | `--lg-color-inset` |
   | Overlay scrim (modal backdrop) | `--lg-color-overlay` |
   | Text intensities | `--lg-color-text`, `--lg-color-text-muted`, `--lg-color-text-subtle` |
   | Borders | `--lg-color-border`, `--lg-color-border-strong` |
   | Accent | `--lg-color-accent`, `--lg-color-accent-hover`, `--lg-color-accent-text` |
   | Danger | `--lg-color-danger`, `--lg-color-danger-hover`, `--lg-color-danger-text` |
   | System states | `--lg-color-info`, `--lg-color-success`, `--lg-color-warning` |
   | Focus | `--lg-color-focus` (+ `--lg-focus-width`, `--lg-focus-offset`) |
   | Elevation | `--lg-shadow-1`, `--lg-shadow-2`, `--lg-shadow-3` |

3. **Component** — only where no semantic covers the need (e.g. `--lg-color-nav-bg` if the nav
   needs its own surface role). None is required to land in this phase.

**Type scale** (`UIF-07`): `--lg-type-12/13/15/16/18/20/24/30`; weights
`--lg-font-weight-regular/medium/semibold/bold` (400/500/600/700); line heights
`--lg-line-tight` (1.2) and `--lg-line-normal` (1.5). Tabular figures are applied by the components
that render numbers (`UIF-08`), not by a token.

**Motion** (`UIF-12`, `UIF-13`): `--lg-motion-120/180/240` (120/180/240 ms),
`--lg-ease-standard`, `--lg-ease-emphasized`; transitions are limited to background, color,
border-color, box-shadow, transform and opacity, and are zeroed under
`prefers-reduced-motion: reduce`.

---

## 2. Theme

| Field | Meaning | Rule |
|---|---|---|
| `name` | `light` \| `dark` | the closed accepted set (`UIF-17`, `UIF-19`, `UIF-22`) |
| `selector` | `:root` (light) \| `[data-theme='dark']` (dark) | declared in `tokens.css` (`UIF-02`, `R7`) |
| `colorScheme` | `light` \| `dark` | set by the theme block so native controls match (`UIF-03`) |
| `surfaceModel` | `bg → surface → raised → overlay` | at most two levels visible at once (`UIF-11`) |
| `accent` | one accent per theme | accent reserved to action/active/selection; a fill always pairs with `--lg-color-accent-text` (`UIF-05`, `UIF-06`) |

**Personality** (`UIF-05`)

- Light — parchment: `bg #EFE6D4`, `surface #FBF6EA`, `inset #E4D8BF`, `text #2A2118`,
  `text-muted #6B5B47`, `border #D6C9AF`, `border-strong #B9A886`, accent `#B45309`,
  accent hover `#92400E`, accent text `#FFFFFF`, danger `#A83232`.
- Dark — charcoal + amber: `bg #17181B`, `surface #212327`, `inset #141518`, `text #ECE9E4`,
  `text-muted #A8A29B`, `border #33363B`, `border-strong #4A4E54`, accent `#F59E0B`,
  accent text `#201A10`, danger `#F87171`.

Values are the starting point; the both-theme contrast assertion is authoritative (`R10`).

**Contrast contract** (`UIF-28`, `UIF-31`): every foreground/background pair is declared per theme
with its minimum ratio; body text pairs 4.5:1, the focus ring 3:1. The pair set covers, at minimum:
text/bg, text/surface, text-muted/bg, text-muted/surface, text-subtle/bg, accent-text/accent,
accent-text/accent-hover, danger-text/danger, focus/bg, focus/surface — for each theme.

---

## 3. Theme preference

| Field | Meaning | Rule |
|---|---|---|
| `key` | `lg.theme` in `localStorage` | the only persisted theme state (`UIF-19`) |
| `value` | `light` \| `dark` | a value outside the set is ignored in favour of the system preference (`UIF-22`) |
| `fresh` | derived: no valid stored value | resolve from `prefers-color-scheme`, and listen for changes (`UIF-17`, `UIF-23`) |
| `explicit` | derived: a valid stored value exists | it wins, and a system change is ignored (`UIF-19`) |

**Resolution order** (the state machine)

```text
read stored value
  ├─ unreadable (throws) ───────────────► system preference  (UIF-21)
  ├─ absent ────────────────────────────► system preference  (UIF-17), live (UIF-23)
  ├─ not in {light, dark} ──────────────► system preference  (UIF-22)
  └─ light | dark ──────────────────────► that theme          (UIF-19)
```

**Transitions**

| From | Event | To | Side effect |
|---|---|---|---|
| fresh (`light` or `dark` from system) | system preference changes | the new system theme | re-render only, nothing written (`UIF-23`) |
| fresh | person activates the control | the other theme | write the explicit value (`UIF-19`) |
| explicit | system preference changes | unchanged | nothing (`UIF-19`) |
| explicit | person activates the control | the other theme | overwrite the stored value (`UIF-19`) |
| any | storage write throws | unchanged | swallow; the session-only choice still applies (`UIF-21`) |

**Contract**: read/write never throw into the interface; the resolved theme is applied to
`document.documentElement` before first paint by the head script (`UIF-20`).

---

## 4. Shell region

| Field | Meaning | Rule |
|---|---|---|
| `order` | `1` navigation, `2` content | exactly two, in that order, on every route (`UIF-24`) |
| `landmark` | `nav` (with an accessible name), `main` | asserted by role (`UIF-24`) |
| `persistence` | navigation stays visible at every width | content keeps a minimum width and never overlaps the nav (`UIF-25`) |
| `footer` | the navigation's last block | hosts the theme control beside the session control (`UIF-27`) |

**Removed entity** (`UIF-26`): the contextual panel — the `right_panel.tsx` module, `.lg-panel*`
rules, `--lg-size-panel-min/max`, the `SHELL_NARROW_QUERY` breakpoint and the shell's narrow
variant. "Removed" is a testable state: no source reference remains and no route renders a third
region.

---

## 5. Vendored font asset

| Field | Value | Rule |
|---|---|---|
| `family` | `Geist` | one self-hosted family (`UIF-14`) |
| `file` | `public/fonts/geist/Geist-Variable.woff2` (69,492 B) | committed artifact, same-origin (`UIF-14`, `UIF-15`) |
| `license` | `public/fonts/geist/Geist-LICENSE.txt` (OFL-1.1) | committed beside the artifact (`UIF-15`) |
| `declaration` | `@font-face` in `src/theme/fonts.css`: `format('woff2')`, `font-weight: 100 900`, `font-display: swap` | imported by `base.css` (`UIF-16`) |
| `preload` | `<link rel="preload" as="font" type="font/woff2" crossorigin>` in `index.html` | same-origin path (`UIF-16`) |
| `fallback` | `--lg-font-family: 'Geist', system-ui, -apple-system, …` | text renders without the file (`UIF-16`) |

**No second family**: no monospace is added; tabular figures cover numeric alignment (`UIF-08`).

---

## 6. Written documents

| Field | Value | Rule |
|---|---|---|
| `web/DESIGN.md` | the visual source of truth | mandatory sections: principles, token architecture, colour, typography, spacing/layout, radius/border/elevation, motion, component specs, accessibility, security posture, per-screen patterns, enforcement, anti-patterns (`UIF-33`) |
| `web/AGENTS.md` | development, security and architecture rules | visual values are deferred to `DESIGN.md`, and a disagreement is a defect (`UIF-33`) |
| `.specs/STATE.md` | the decision log | records the three-layer two-theme tokens, the `localStorage` preference, the same-origin no-flash script and the panel removal (`UIF-34`) |
