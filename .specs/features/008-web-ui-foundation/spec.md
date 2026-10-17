# Feature Specification: Web UI Foundation

**Feature Branch**: `008-web-ui-foundation`

**Created**: 2026-10-17

**Status**: Draft

**Input**: User description: "Establish the web UI foundation: a three-layer two-theme token system, a persisted light/dark theme toggle with no load flash, a shell without the dead context panel, and a self-hosted variable font, plus the design guidelines that govern every later screen"

## Clarifications

### Session 2026-10-17

- Q: How is the "no flash of the wrong theme" requirement satisfied without breaking the enforced
  same-origin, no-inline-script posture? → A: a tiny blocking same-origin classic script in the
  document head resolves the theme before the first paint — zero flash, allowed by `script-src
  'self'`, no inline script, no policy hash to keep in sync (user decision, 2026-10-17). The
  alternatives were rejected: an inline head script plus a Content-Security-Policy hash would amend
  the constitution's "no inline" clause for a one-line boot, and resolving the theme in the app
  entry accepts one frame of the wrong theme (`UIF-20`, `UIF-34`).
- Q: Where does the theme control live, and is a preference allowed to persist when credentials may
  not? → A: the theme control lives in the navigation footer beside the session control, and the
  preference persists in `localStorage` because it is not a credential; the constitution's ban
  covers credentials only (`UIF-19`, `UIF-27`).
- Q: Can a person return to the system preference after choosing a theme? → A: no (user decision,
  2026-10-17): the control is a two-state light/dark toggle, the system preference is only the
  default until the first explicit choice, and the stored value is always `light` or `dark`
  (`UIF-17`, `UIF-19`, `UIF-23`).
- Q: Which color/typography values are the contract, given the plan calls its palette a starting
  point? → A: the *personality* (warm parchment light, neutral charcoal plus amber dark, single
  accent, expanded scale, elevation in three steps) is the requirement; the concrete values live in
  the plan and the guidelines and are authoritative only through the both-theme contrast assertion
  (`UIF-05`, `UIF-28`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - One foundation that reads the same in light and dark (Priority: P1)

A person opens the app and sees a warm, editorial surface with real hierarchy: a visible type
scale, spacing that groups and separates, radii that soften controls, and depth that tells a
raised card from a page. Everything colored, sized, and spaced comes from one sanctioned source,
so no screen invents its own shade. The same surface has a dark counterpart — neutral charcoal
with an amber accent — that carries the identical structure, is chosen by the person's system
preference until they choose otherwise, and never shows a color whose contrast fails.

**Why this priority**: every later screen (the component primitives, then the catalog, sheet,
auth and states) inherits this contract; without it the redesign has no foundation and the
"token-only" gate has nothing to enforce.

**Independent Test**: read the token source and assert both theme blocks define every semantic
token; run the contrast assertion against both themes; assert the self-hosted font is declared and
preloaded; assert the scanned component directories contain no literal color or pixel value.

**Acceptance Scenarios**:

1. **Given** the token source, **When** it is inspected, **Then** it declares raw primitive
   scales, semantic aliases, and only the component tokens a semantic does not cover. (UIF-01)
2. **Given** a light and a dark theme, **When** each is applied, **Then** every semantic token
   resolves to a value in both, and the same token names describe the same role in both. (UIF-02)
3. **Given** a rendered surface, **When** the theme is dark, **Then** the browser is told the
   document color scheme is dark, so native controls and scrollbars match. (UIF-03)
4. **Given** the semantic color set, **When** a screen needs a background, a surface, a raised or
   inset area, an overlay, text at three intensities, two border weights, an accent with its
   hover and its on-accent text, danger, focus, or one of three elevations, **Then** a token
   exists for it and no screen hardcodes the value. (UIF-04)
5. **Given** the light theme, **When** it renders, **Then** it reads as warm parchment with a
   burnt-amber accent; **given** the dark theme, **Then** it reads as neutral charcoal with an
   amber accent — one accent family in each, no second accent. (UIF-05)
6. **Given** the accent, **When** it is used, **Then** it marks an action, an active state or a
   selection — never decoration — and whenever it fills a surface the text on it comes from the
   on-accent token. (UIF-06)
7. **Given** the type scale, **When** a heading or a section title renders, **Then** it may exceed
   the body size (the old hard cap is gone) and the scale is the declared fixed set, with tight and
   normal line heights and four weights. (UIF-07)
8. **Given** a numeric or derived column, **When** it renders, **Then** its figures are tabular so
   values align. (UIF-08)
9. **Given** the spacing scale, **When** a screen composes a region, **Then** every gap and pad is
   a step of the declared scale, and the readable content measure is preserved. (UIF-09)
10. **Given** a control, a card or an overlay, **When** it renders, **Then** its corner radius and
    its hairline border come from the declared radius and hairline tokens, and no radius is capped
    at the old maximum. (UIF-10)
11. **Given** a raised element, **When** it renders, **Then** its depth comes from one of three
    elevation tokens; the layered model is background → surface → raised → overlay and never shows
    more than two levels at once. (UIF-11)
12. **Given** any transition, **When** it runs, **Then** it uses a declared duration and easing and
    animates only the allowed property set. (UIF-12)
13. **Given** a person who asked their system to reduce motion, **When** they use the app, **Then**
    non-essential animation is zeroed. (UIF-13)
14. **Given** the typography, **When** any text renders, **Then** it uses one self-hosted variable
    family with a system fallback, and no font, style or script is fetched from a third-party
    origin. (UIF-14, UIF-16)
15. **Given** the shipped font, **When** the repository is inspected, **Then** the font file and
    its open-font license are committed and served same-origin. (UIF-15)

---

### User Story 2 - The chosen theme stays chosen, and never flashes (Priority: P1)

Until a person chooses, the app follows their system preference — and keeps following it live if
the system switches. When they choose a theme, the choice sticks across reloads and wins over the
system preference from then on; the system preference is only the starting point, and no control
state hands the choice back to it. The first paint is already the right theme, so the app never
shows a light surface and then snaps to dark. If the browser refuses to store or read the
preference, the app still renders, falls back to the system preference, and never shows an error.

**Why this priority**: a theme switch that resets on reload, or that flashes the wrong palette on
every load, is worse than no switch; the persistence and the no-flash path are what make the dark
theme usable.

**Independent Test**: render with no stored preference under a simulated dark system preference
and assert the dark theme; choose the other theme and assert the document and the storage agree;
re-render with that stored value and assert it wins over a later system change; drive an
unreadable/unknown stored value and a storage that throws, and assert the fallback without a crash.

**Acceptance Scenarios**:

1. **Given** nothing is stored, **When** the app resolves the theme, **Then** it follows the
   system's color-scheme preference. (UIF-17)
2. **Given** the system preference changes while nothing is stored, **When** it changes, **Then**
   the app follows it without a reload. (UIF-23)
3. **Given** a person uses the theme control in the navigation footer, **When** they activate it,
   **Then** the theme switches and the control exposes its accessible name and current state.
   (UIF-18, UIF-27)
4. **Given** a person has chosen a theme, **When** they reload, **Then** the app renders that
   theme before the interface paints, and the stored preference is the single source of the
   choice. (UIF-19, UIF-20)
5. **Given** storage is unavailable or throws, **When** the app resolves the theme, **Then** it
   renders with the system-derived theme and no error. (UIF-21)
6. **Given** the stored value is unknown or corrupt, **When** the app resolves the theme, **Then**
   it ignores the value and falls back to the system preference. (UIF-22)
7. **Given** a person has chosen a theme, **When** the system preference changes, **Then** the app
   keeps rendering the chosen theme. (UIF-19)

---

### User Story 3 - A shell that shows only what has content (Priority: P1)

The application shell is navigation plus content. The contextual panel that every route rendered —
fixed, always present, and always empty — is gone from the markup, not merely hidden; its tokens,
its styles and its narrow-viewport rule go with it. As the window narrows, the navigation stays
put and the content reaches its minimum without ever overlapping it.

**Why this priority**: a permanently empty third region is a defect the redesign must delete, not
restyle; it is also the largest single removal in the foundation and the one existing tests pin
most directly.

**Independent Test**: render every declared route and assert exactly the navigation and the content
regions exist; narrow the viewport and assert both remain and no region is lost or overlapped;
assert the panel module, rules and tokens no longer exist anywhere.

**Acceptance Scenarios**:

1. **Given** any declared route, **When** it renders, **Then** the shell exposes a navigation
   region and a content region, in that order, and no third region. (UIF-24)
2. **Given** a narrowing viewport, **When** the shell re-lays out, **Then** the navigation remains
   visible and the content keeps its minimum width without overlapping the navigation. (UIF-25)
3. **Given** the source tree, **When** it is inspected, **Then** the panel module, its style rules,
   its size tokens and the panel collapse breakpoint no longer exist, and no source references
   them. (UIF-26)

---

### User Story 4 - The guidelines are the one place a visual rule is written (Priority: P2)

A person building the next screen finds the visual rules in one committed document: the principles,
the token architecture, the color, typography, spacing, radius, elevation, motion rules, the
component specifications, the accessibility contract, the anti-patterns and the per-screen
patterns. The developer-rules document keeps only development, security and architecture rules and
points to the design document instead of restating visual values. The decisions this foundation
makes are recorded with their rationale and their rejected alternative.

**Why this priority**: the redesign is delivered over several later phases; without a single
written source, each phase would re-decide the visual rules.

**Independent Test**: read the design document and assert each mandatory section exists; assert
the developer-rules document no longer restates visual values and points to it; assert the
decision record captures the theme persistence, the no-flash strategy, the panel removal and the
token architecture.

**Acceptance Scenarios**:

1. **Given** the design document, **When** it is read, **Then** it covers principles, token
   architecture, color, typography, spacing and layout, radius/border/elevation, motion, component
   specifications, accessibility, security posture, per-screen patterns, enforcement and
   anti-patterns. (UIF-33)
2. **Given** the developer-rules document, **When** it is read, **Then** it carries the
   development, security and architecture rules and defers all visual values to the design
   document. (UIF-33)
3. **Given** this feature's decisions, **When** the decision log is read, **Then** it records the
   three-layer tokens with two themes, the theme preference in `localStorage`, the same-origin
   no-flash head script (with the inline-plus-hash alternative rejected), and the removal of the
   panel. (UIF-34)

---

### Edge Cases

- **Stored theme value is unknown or corrupt**: the app ignores it, follows the system preference,
  and does not crash. (UIF-22)
- **Storage read or write throws** (private mode, disabled storage): the app renders with the
  system-derived theme; a later choice simply does not persist, and no error surfaces. (UIF-21)
- **No system color-scheme signal**: the app resolves to the documented default theme rather than
  rendering undefined colors. (UIF-17)
- **A semantic token declared in one theme only**: the both-theme assertion fails, so a half-migrated
  theme cannot land. (UIF-02, UIF-28)
- **A component references a token that is not declared**: the resolution assertion fails. (UIF-30)
- **The font fails to load or is blocked**: text renders in the system fallback with no layout
  break, and nothing is requested from a third-party origin. (UIF-14, UIF-16)
- **The design document and the developer-rules document disagree on a visual value**: the design
  document wins and the conflict is a defect. (UIF-33)
- **An existing suite asserted the removed third region**: it is retargeted to the two remaining
  regions rather than deleted or weakened. (UIF-32)

## Requirements *(mandatory)*

### Functional Requirements

**Token architecture and themes**

- **UIF-01**: The token source MUST declare three layers: raw primitive scales, semantic aliases,
  and component tokens only where no semantic covers the need. Naming stays `--lg-kebab-case`.
- **UIF-02**: The light theme MUST be declared on the document root and the dark theme under a
  single dark-theme selector, and every semantic token MUST resolve in both.
- **UIF-03**: Each theme MUST set the document's declared color scheme so native controls match it.
- **UIF-04**: The semantic color set MUST include background, surface, raised, inset, overlay,
  three text intensities, two border weights, accent with hover and on-accent text, danger with its
  on-color, focus, and three elevations.
- **UIF-05**: The light theme MUST read as warm parchment with a burnt-amber accent and the dark
  theme as neutral charcoal with an amber accent; each theme MUST have exactly one accent.
- **UIF-06**: The accent MUST be reserved for action, active state or selection, and any accent fill
  MUST pair with the on-accent text token.
- **UIF-07**: The type scale MUST be the declared fixed set (12/13/15/16/18/20/24/30) with no upper
  cap below the largest step, tight and normal line heights, and four weights.
- **UIF-08**: Numeric and derived values MUST be rendered with tabular figures.
- **UIF-09**: The spacing scale MUST be the declared fixed set (2/4/8/12/16/24/32/48) and the
  readable content measure MUST be preserved.
- **UIF-10**: Radius MUST come from the declared set (small 4, medium 8, large 12, pill 999) and
  borders MUST use a one-pixel hairline, with no upper radius cap.
- **UIF-11**: Depth MUST come from exactly three elevation steps, and the layered model
  (background → surface → raised → overlay) MUST NOT show more than two levels at once.
- **UIF-12**: Motion MUST use declared durations (120/180/240 ms) and declared easings, and MUST
  transition only background, color, border-color, box-shadow, transform and opacity.
- **UIF-13**: A reduced-motion preference MUST zero non-essential animation.

**Typography asset**

- **UIF-14**: The client MUST use one self-hosted variable font family with a system fallback, and
  MUST NOT request any font, style or script from a third-party origin.
- **UIF-15**: The font file and its open-font license MUST be committed and served same-origin.
- **UIF-16**: The font MUST be loaded with swap behavior and preloaded, and text MUST render in the
  fallback without a layout break if it does not load.

**Theme preference**

- **UIF-17**: With no stored preference, the theme MUST follow the system color-scheme preference;
  absent any signal it MUST resolve to the documented default.
- **UIF-18**: The navigation footer MUST offer a theme control whose accessible name and current
  state are exposed.
- **UIF-19**: An explicit choice MUST persist across reloads in `localStorage` under the documented
  key as `light` or `dark`, which is permitted because a preference is not a credential; the
  accepted set has exactly those two values.
- **UIF-20**: The first paint MUST already use the resolved theme, resolved by a same-origin
  classic script in the document head that runs before the first paint; no inline script and no
  Content-Security-Policy hash are introduced, and the no-flash decision MUST be recorded.
- **UIF-21**: Reading or writing the preference MUST NOT throw into the interface; a storage failure
  MUST fall back to the system-derived theme.
- **UIF-22**: A stored value outside the accepted set MUST be ignored in favor of the system
  preference.
- **UIF-23**: While no explicit choice is stored, a change in the system preference MUST be followed
  without a reload.

**Shell**

- **UIF-24**: The shell MUST render exactly two regions — navigation then content — on every route,
  with no third region in the markup.
- **UIF-25**: The navigation MUST stay visible as the viewport narrows and the content MUST keep a
  minimum width without overlapping the navigation.
- **UIF-26**: The contextual panel module, its style rules, its size tokens and its collapse
  breakpoint MUST be removed, with no remaining references.
- **UIF-27**: The theme control MUST live in the navigation footer, beside the session control.

**Enforcement and documentation**

- **UIF-28**: The contrast assertion MUST evaluate every declared pair in **both** themes against
  AA (4.5:1, focus 3:1) using the values declared in the token source.
- **UIF-29**: The literal-value scan MUST keep forbidding hexadecimal colors and pixel values in the
  scanned component directories, with the theme directory as the documented exception where the
  literals live.
- **UIF-30**: Every token referenced by a scanned component MUST resolve to a declared token.
- **UIF-31**: Every documented design token MUST resolve in the token source, and every foreground
  token MUST declare its contrast pair per theme.
- **UIF-32**: Existing suites that pinned the removed third region MUST be retargeted to the two
  remaining regions, preserving their intent.
- **UIF-33**: `web/DESIGN.md` MUST be the visual source of truth with the declared sections, and
  `web/AGENTS.md` MUST keep only development, security and architecture rules and point to it.
- **UIF-34**: The feature's decisions MUST be recorded in `.specs/STATE.md`.

### Key Entities

- **Primitive token**: a raw scale value (ink, amber, danger, space, radius, font, motion) with no
  role attached.
- **Semantic token**: a role the components consume (background, surface, text intensities, border
  weights, accent, danger, focus, elevation), resolving per theme.
- **Component token**: added only when no semantic covers a component's need.
- **Theme**: one resolved set of values (light or dark) applied to the document.
- **Theme preference**: the stored explicit choice (`light` or `dark`) that overrides the system
  preference; not a credential.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of declared contrast pairs meet AA in both themes, verified from the token
  source values.
- **SC-002**: 0 literal color or pixel values remain in the scanned component directories.
- **SC-003**: 0 routes render a region without real content; every route exposes exactly the
  navigation and the content region.
- **SC-004**: A stored theme choice survives a reload with 0 flashes: the first paint already uses
  the chosen theme.
- **SC-005**: 0 requests to a third-party origin for fonts, styles or scripts in the built client.
- **SC-006**: 1 committed design document is the sole written source of visual values, referenced by
  the developer-rules document.

## Assumptions

- The stack stays React with Vite and TypeScript under `web/`, and "tokens plus plain CSS, zero
  design-system dependencies" holds: no component library, theme framework or font package is
  added (the font file is vendored, not depended on).
- The redesign is phased: this feature is the foundation. The component primitives under a new
  shared UI layer, the screen-by-screen refactor, and the visual-regression suite are later phases
  and are out of scope here.
- The concrete palette, scale and radius values in the plan are the starting point; the both-theme
  contrast assertion is the authority, so a value may be adjusted to pass AA.
- The theme directory is the sanctioned exception to the literal-value scan because it is where the
  values are declared; every scanned component directory stays literal-free.
- The theme preference is a non-credential preference and may persist in `localStorage`; the
  constitution's ban applies to credentials (the refresh credential stays an `HttpOnly` cookie and
  the access token stays in memory).
- The dark theme selector is `[data-theme='dark']` on the document root, matching the plan; the
  no-flash strategy is a small blocking same-origin classic script in the document head that sets
  the attribute before the first paint, so `script-src 'self'` already permits it and no inline
  script or policy hash is added.
- The theme control is a two-state light/dark toggle: the system preference is only the default
  until the first explicit choice, the stored value always overrides it afterwards, and no control
  state restores the system-following default.
- The panel removal retargets the existing shell, responsiveness and end-to-end assertions to the
  two remaining regions; no assertion is deleted or weakened.
- `SHELL_NARROW_QUERY`, `--lg-size-panel-min`, `--lg-size-panel-max` and the panel style rules are
  removed together with the panel module.
- Geist (OFL-1.1) is the chosen family, self-hosted; no monospace family is added, because tabular
  figures cover the numeric columns.
