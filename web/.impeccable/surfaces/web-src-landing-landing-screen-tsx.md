---
version: 1
slug: "web-src-landing-landing-screen-tsx"
primary_target: "web/src/landing/landing_screen.tsx"
related_targets: []
---

# Landing (public `/` for visitors)

Mode: Persuade. Visitors: tabletop RPG players arriving cold. Job: understand what Legnaris is and take one of two actions (create account, sign in). Constraints: English via l10n, no invented stats/testimonials/prices, one photo only (lava crater, supplied by the owner, hero only), self-hosted, token-only CSS, Geist only. DESIGN.md gets a scoped "Landing surface" exception approved by the user; the logged-in app is untouched.

## Direction contract

THESIS: A character sheet that does its own math, sold by showing it do the math. Refuses the Arix-clone template of stat tiles, icon cards and testimonial rows; keeps only its rhythm: full-bleed photo hero, statement, ordered process, list, FAQ, plain closing call to action.

OWN-WORLD: Teal-black ground and bone text from the `--lg-color-stage-*` roles, with a single lava-orange accent sampled from the volcano photo. Geist 600 at display scale with tight tracking, hairline-divided rows instead of cards, flat opaque surfaces, one scrim over the photo. Recognizable with content removed: dark lava-crater photo, lava-orange primary button, flat teal-black ground.

STORY: The visitor sees a living sheet, not a spreadsheet: a stated promise (it does its own math), a live demo where one edit cascades into derived values, three truthful differentiators (zero licensed content, computed not typed, portable data), then three steps and honest FAQ answers. Action: create an account or sign in.

FIRST VIEWPORT: Full-height lava-crater photo, dark scrim weighted bottom and left. Top bar: sigil + wordmark left, a ghost Sign in right; no in-page navigation. Bottom-left: H1 at display scale in three lines over the dark rock. Bottom-right: one sentence of subcopy and the primary "Start your archive"; "Sign in" lives in the top bar. Crater and lava stay unobstructed in the upper-middle.

FORM: Own-world direction from the owner's reference (Arix) with a volcano photo, position 1 of 1 (owner-pinned). seed: none (unavailable)

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
