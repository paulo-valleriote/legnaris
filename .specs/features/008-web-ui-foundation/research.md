# Phase 0 Research: Web UI Foundation

**Feature**: `008-web-ui-foundation` | **Date**: 2026-10-17

Each decision resolves an unknown the spec left to design, and records the alternatives that were
rejected. Every fact about the environment was verified in the repository or against the real
artifacts (versions, font files, existing suites), not assumed.

---

## R1 — Three-layer tokens, with `theme/` as the single literal exception

**Decision**: `web/src/theme/tokens.css` stays the only file that declares literal values. It is
organised in two declared layers in one file — primitive scales (`--lg-ink-*`, `--lg-amber-*`,
`--lg-danger-*`, `--lg-space-*`, `--lg-radius-*`, `--lg-font-*`, `--lg-motion-*`) and semantic
aliases the components consume (`--lg-color-bg|surface|raised|inset|overlay`, `--lg-color-text*`,
`--lg-color-border*`, `--lg-color-accent*`, `--lg-color-danger*`, `--lg-shadow-1..3`,
`--lg-focus-*`), with a component token added only when no semantic covers a need. Components
consume `--lg-*` only; the token-usage scan keeps `shell/ catalog/ engine/ sheet/ library/ auth/`
literal-free and treats `theme/` as the documented exception where the values must live.

**Rationale**: the scan's whole value is that beauty work is confined to one file: no component can
encode a shade, so a theme change is a token change. Splitting the layers across files would break
the "one token file" reading and add an import order to reason about, for no gain at this size.

**Rejected alternatives**: (a) a `tokens/primitives.css` + `tokens/semantics.css` pair — two files
in the sanctioned directory, an import order in `base.css`, and no gate improvement; (b) declaring
component tokens for every component — the spec forbids it (`UIF-01`: component tokens only when no
semantic covers the need).

## R2 — The test harness parses `tokens.css` per theme block

**Decision**: `web/src/test/tokens_files.ts` gains a per-theme parser: it splits the file into the
`:root` (light) block and the `[data-theme='dark']` block and returns a map *per theme*, keeping
`readTokenValues()` shape-compatible where a flat map is still needed. `tokens_contrast.test.ts`
iterates both themes and asserts each declared pair against the AA threshold.

**Rationale**: the current parser flattens the whole file into one map, so with two blocks the
second declaration of a name silently wins and a token declared in only one theme would read as
present — the exact failure the spec must catch (`UIF-02`, `UIF-28`). Parsing per block turns a
one-theme omission into a red test.

**Rejected alternatives**: (a) split the tokens into one file per theme — the RGB-literal
environment does not support `var()` indirection across files without duplicating names, and the
constitution's "one source" instinct favours one file; (b) assert the two blocks have equal key sets
by a separate scan — a weaker check that does not feed the contrast math, which needs the per-theme
values anyway.

## R3 — No flash via a blocking same-origin head script

**Decision**: `web/public/theme-init.js` is a tiny classic (non-module) script loaded in
`index.html`'s `<head>` before the stylesheets' consumers paint. It reads `localStorage['lg.theme']`,
validates it against the accepted set, falls back to `prefers-color-scheme`, and sets
`document.documentElement.dataset.theme` accordingly. `src/theme/theme.ts` performs the same
resolution for runtime changes and for the toggle, and writes the preference back.

**Rationale**: a classic head script blocks parsing, so the attribute is set before first paint (the
spec's `UIF-20`), and it is **same-origin**, so the enforced `script-src 'self'` already permits it.
No inline script means no hash to maintain and no amendment to constitution VI's "no inline"
clause. Verified: CSP `script-src 'self'` covers same-origin classic scripts; the file is served
from `public/` and is ~0.3 kB.

**Rejected alternatives**: (a) **inline head script + CSP hash** — zero extra request, but it
requires amending a NON-NEGOTIABLE principle for a boot snippet and keeping a hash in sync with the
file, and the user chose against it; (b) **resolve in the app entry** — no extra request and no
policy question, but an explicit choice that differs from the system preference shows one frame of
the wrong theme, violating `UIF-20`; (c) **CSS-only `@media (prefers-color-scheme)`** — cannot
express an explicit override that disagrees with the system.

## R4 — Vendor the official Geist variable artifact and its license

**Decision**: commit `web/public/fonts/geist/Geist-Variable.woff2` (69,492 bytes) and
`web/public/fonts/geist/Geist-LICENSE.txt` (OFL-1.1), both extracted from the official
`geist@1.8.1` release on the npm registry (`dist/fonts/geist-sans/Geist-Variable.woff2`,
`LICENSE.txt`; published by Vercel). `src/theme/fonts.css` declares one `@font-face` with
`font-family: 'Geist'`, `src: url('/fonts/geist/Geist-Variable.woff2') format('woff2')`,
`font-weight: 100 900`, `font-style: normal`, `font-display: swap`. `--lg-font-family` becomes
`'Geist', system-ui, -apple-system, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif`.
`index.html` gains `<link rel="preload" as="font" type="font/woff2" crossorigin>`.

**Rationale**: the official variable file is a single artifact with full glyph coverage, so the
subsetting decisions Fontsource makes per block do not have to be reproduced by hand, and the OFL
text travels with it. It is vendored, not installed, so "no design-system dependency" holds
(`UIF-14`), and `font-src 'self'` is satisfied.

**Rejected alternatives**: (a) **`@fontsource-variable/geist`** — a real dependency and a
five-block `unicode-range` set (latin, latin-ext, cyrillic, cyrillic-ext, vietnamese) whose CSS
would have to be re-implemented anyway, since the plan wants one self-hosted family; (b) a **CDN /
Google Fonts link** — a third-party origin, forbidden by `UIF-14` and by the CSP; (c) **declaring
all the static per-weight files** — 18 requests and a manual weight map, worse than one variable
file.

**Deviation**: the source plan named the artifact `Geist[wght].woff2`. Brackets in a `url()` token
and in an HTML attribute require percent-encoding, and the official file is already named
`Geist-Variable.woff2`, so the vendored name follows the artifact (`Geist-Variable.woff2`) to avoid
an encoding landmine in the CSS and the preload link. The file's *role* is unchanged.

## R5 — Remove the contextual panel rather than hide it

**Decision**: delete `web/src/shell/right_panel.tsx`, its `.lg-panel*` rules from `shell.css`, the
`--lg-size-panel-min`/`--lg-size-panel-max` tokens (CSS and `tokens.ts`), the `SHELL_NARROW_QUERY`
breakpoint and the `--lg-shell--narrow` variant. `app_shell.tsx` renders `LeftNav` and `main`
only, and `shell.css` becomes a two-column grid. `SHELL_NARROW_QUERY`'s export disappears from
`tokens.ts`; the narrow behaviour is now expressed by the content's own minimum width plus the
persistent nav.

**Rationale**: the module's own doc comment called it temporary, and `AD-013` already established
the precedent (a placeholder whose job is finished is deleted with the suites retargeted, not kept
hidden). A hidden-but-rendered region would keep the dead CSS and the extra landmark in the a11y
tree, which is precisely the defect (`UIF-24`, `UIF-26`, `UIF-32`).

**Rejected alternatives**: (a) keep the module and render it only at wide viewports — dead code
with a permanent maintenance cost and an empty landmark; (b) keep the tokens "for a future panel" —
unused tokens fail the resolution assertion or sit as speculation, and the plan explicitly asks for
their removal.

## R6 — Two-state toggle, system preference only as the default

**Decision**: the theme store holds exactly `light` or `dark`. With nothing stored, the resolved
theme follows `prefers-color-scheme` and a `change` listener keeps it live; the first activation of
the control writes an explicit value that wins from then on. There is no third "system" state, so
the control's label and pressed state are a simple two-way switch.

**Rationale**: user decision (2026-10-17). It matches the approved plan's wording ("Claro + escuro
+ toggle persistido") and keeps the storage contract at the two values the spec declares
(`UIF-17`, `UIF-19`, `UIF-23`).

**Rejected alternatives**: a three-state cycle system → light → dark — friendlier for switching
between day/night setups, but it adds a third copy key, a third pressed state and a tri-state
storage contract the plan did not ask for.

## R7 — `data-theme` on the document element, plus `color-scheme`

**Decision**: the light palette is declared under `:root` and the dark palette under
`[data-theme='dark']`; each block also sets `color-scheme: light` / `dark`. The attribute is on
`<html>` (the element `theme-init.js` can reach before the body exists).

**Rationale**: `[data-theme]` is an explicit override that the CSS-only media query cannot express
(`R3`), and putting it on the root means the whole cascade — including `body` background — flips
atomically. `color-scheme` makes native controls, scrollbars and form widgets follow the theme
(`UIF-03`).

**Rejected alternatives**: (a) `@media (prefers-color-scheme: dark)` alone — cannot honour an
explicit choice; (b) a class on `<body>` — the pre-paint script would then depend on the body
existing, and root is where the CSS custom properties are declared.

## R8 — Keep the readable content measure; expand everything else

**Decision**: `--lg-size-content-max` (~960 px) stays, because readability is unchanged by having
more viewport; the rest of the scale is replaced: type `12/13/15/16/18/20/24/30`, spacing
`2/4/8/12/16/24/32/48`, radius `sm 4 / md 8 / lg 12 / pill 999`, elevation `shadow-1..3`, motion
`--lg-motion-120/180/240` with `standard`/`emphasized` easings.

**Rationale**: the plan locks "content with a readable measure (keep `--lg-size-content-max`)" and
identifies the 17 px cap, the 4 px radius cap and the missing elevation as the defects. Widening the
measure because the panel is gone would trade a real readability rule for filling space, and the
spec does not ask for it (`UIF-09`).

**Rejected alternatives**: (a) raise the content measure to fill the freed column — no requirement
supports it and it regresses scanning; (b) keep `--lg-type-17` as a step — the scale is a fixed
declared set, and a 17 px orphan between 16 and 18 helps nobody.

## R9 — Contrast pairs become per theme

**Decision**: `contrastPairs` gains a `theme` field (`'light' | 'dark'`) and is declared for both
themes; the contrast suite iterates the pairs and resolves each pair's foreground/background from
that theme's map, asserting AA (4.5:1) and 3:1 for the focus ring.

**Rationale**: the pairs exist so a colour used as a foreground cannot be added without a declared
contrast target; with two themes, a pair is meaningless without naming the theme it was measured
in, and the plan's risk list calls this out ("needs to separate light/dark for the contrast test").

**Rejected alternatives**: (a) two arrays (`contrastPairsLight`/`contrastPairsDark`) — every
consumer must remember to walk both, and the "declare your pair" check gets two homes; (b) compute
pairs only for light and trust dark — the constitution's AA posture applies to both themes and the
plan's DoD says so.

## R10 — Dark palette starts from the plan's values, then answers to AA

**Decision**: ship the plan's dark palette (`bg #17181B`, `surface #212327`, `inset #141518`,
`text #ECE9E4`, `text-muted #A8A29B`, `border #33363B`, `border-strong #4A4E54`, accent `#F59E0B`,
`accent-text #201A10`, danger `#F87171`) and the light parchment palette
(`bg #EFE6D4`, `surface #FBF6EA`, `inset #E4D8BF`, `text #2A2118`, `text-muted #6B5B47`,
`border #D6C9AF`, `border-strong #B9A886`, accent `#B45309` with hover `#92400E`,
`accent-text #FFFFFF`, danger `#A83232`) as the starting point, adjusting any value the AA
assertion rejects in either theme.

**Rationale**: the spec's Clarifications make the personality a requirement and the concrete values
subject to the both-theme contrast assertion; the plan itself says "the values above are a starting
point — the contrast test is what rules".

**Rejected alternatives**: (a) hard-code the plan's hexes as immutable — the plan says otherwise and
AA wins; (b) redesign the palettes from scratch — no requirement asks for it and it would discard
reviewed choices.

## R11 — The guidelines move to `web/DESIGN.md`; `AGENTS.md` keeps the rest

**Decision**: create `web/DESIGN.md` with the plan's §3 sections (principles, token architecture,
colour, typography, spacing/layout, radius/border/elevation, motion, component specifications,
accessibility, security posture, per-screen patterns, enforcement, anti-patterns) and rewrite
`web/AGENTS.md` so its visual sections become a pointer to it, keeping the development, security,
architecture, componentization and dependency rules plus the existing delivered-deviations log.

**Rationale**: `AGENTS.md` is loaded into every agent's context; a design document that grows with
each phase should not be duplicated into it. The plan's F0 deliverable is exactly this split, and
`UIF-33` makes it testable by section.

**Rejected alternatives**: (a) keep everything in `AGENTS.md` — it already restates visual values
that this feature invalidates (17 px cap, 4 px radius, panel), so the stale rules must go anyway;
(b) two documents with overlapping visual rules — the spec's edge case makes a disagreement a
defect.

## R12 — Suites that pinned the panel are retargeted, not deleted

**Decision**: `shell.test.tsx` asserts the region order of navigation then content and that no
third region exists; `shell_responsive.test.tsx` asserts both regions survive a narrowed viewport
and that the nav stays visible; `web/e2e/shell.spec.ts` drops the complementary-region assertions
and keeps the order, deep-link and a11y checks. `shell_keyboard.test.tsx` gains the theme control in
its tab walk.

**Rationale**: `UIF-32` and constitution III forbid weakening an assertion to make room for a
change; the assertions' *subject* changed, so their intent (region order, persistence, keyboard
reachability) is preserved on the truthful subject.

**Rejected alternatives**: deleting the panel assertions — that would silently drop the region-order
and narrow-viewport guarantees instead of moving them to the two remaining regions.

## R13 — Copy keys for the theme control

**Decision**: add `theme.toggleLabel` (the accessible name, e.g. "Switch theme"), `theme.switchToDark`
and `theme.switchToLight` (the action copy per target) to `web/src/l10n/strings.ts`; the control's
accessible name and its `aria-pressed`/`title` resolve through `message()`.

**Rationale**: `l10n_strings.test.ts` forbids user-facing literals in components, and the control's
action label depends on the current state, so both targets need keys.

**Rejected alternatives**: a single key with a static label — the control's state would be invisible
to a screen reader except as a pressed state, and the plan asks the control to expose its current
state (`UIF-18`).
