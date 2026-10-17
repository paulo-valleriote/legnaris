# Quickstart: validate the web UI primitives

**Feature**: `.specs/features/009-web-ui-primitives/` | **Date**: 2026-10-17

Runnable scenarios that prove the feature works end to end. Each lists what to run and the outcome
that means it passed. Prerequisites: the client toolchain (`web/` with `npm install` already done,
Node 24). Every Vitest suite runs with **no backend, no MongoDB and no network** — the layer mounts no
route and makes no request — so there is no browser scenario: the assembled-surface accessibility
sweep is the catalog/auth phase's and the visual regression is the redesign's last phase.

Paths referenced here are defined in [plan.md](./plan.md) (source layout), [data-model.md](./data-model.md)
(primitives, states, the focus confinement, the notice) and the three documents under
[contracts/](./contracts/).

---

## 1. The suite runs with no backend at all

```bash
cd web && npm test
```

**Outcome**: the whole suite passes with the API and MongoDB stopped. Every new suite renders a
primitive directly and the scans read committed files, so nothing touches the network. No dependency
was added, so the secret, SCA, SAST and lockfile gates see nothing new to vet (SC-005).

---

## 2. The shared layer exists and is a leaf

```bash
cd web && npx vitest run src/test/ui_layer.test.ts
```

**Outcome**: `web/src/ui/` holds exactly the five primitives (`button.tsx`, `field.tsx`,
`checkbox.tsx`, `dialog.tsx`, `toast.tsx`) and the focus helper (`focus_trap.ts`), each in its own
module, plus the one token-only stylesheet `ui.css` (UIP-01); no import specifier in the layer names a
feature directory, the only client import is the copy catalog, and there is no barrel module (UIP-02).
Adding an `import … from '../catalog/…'` to a primitive turns the suite red.

---

## 3. No primitive authors a visual value

```bash
cd web && npx vitest run src/test/tokens_used.test.tsx
```

**Outcome**: the literal scan now covers `web/src/ui/` alongside the feature directories: no literal
hex colour and no `NNpx` value in the layer (UIP-04, SC-002), and every `var(--lg-*)` the layer
references resolves in `tokens.css` (SC-002). Declaring a raw `#fff` in `ui.css` turns the suite red;
`src/theme/` remains the documented exception where the literals live.

---

## 4. No primitive authors a user-facing string

```bash
cd web && npx vitest run src/test/l10n_strings.test.ts
```

**Outcome**: the copy scan now covers `web/src/ui/`: no JSX text node is a user-facing literal and
every copy key the layer renders (`ui.dismiss`) resolves through `src/l10n/strings.ts` (UIP-05,
SC-004). A hard-coded `Dismiss` label in a primitive turns the suite red.

---

## 5. The action renders every variant, size and state

```bash
cd web && npx vitest run src/test/ui_button.test.tsx
```

**Outcome**: the five variants each render their declared class — primary fills with the accent and
pairs the on-accent text, secondary/ghost/outline read as progressively lighter surfaces, and danger
is outlined with the danger border and text, **never filled** (UIP-07); both sizes render and neither
falls below the declared control height (UIP-08); and each state is reachable and distinct — hover and
press change only background or tint, focus-visible shows the tokenised ring, disabled is announced as
disabled and **does not run its handler**, and loading is announced busy, **does not run its handler**
and shows the caller's in-flight label (UIP-09). The element is a real `button` whose default type
never submits a form and whose accessible name comes from its content or an explicit name (UIP-10).

---

## 6. The field carries its label and announces its error

```bash
cd web && npx vitest run src/test/ui_field.test.tsx
```

**Outcome**: a permanent label is associated with the control and a placeholder is never the only
label (UIP-11); the text input, the select and the text area share height, border, radius and focus
values (UIP-12); with an error the message renders inline, the control is marked invalid and is
associated with that message, and with **no** error no invalid marking and no dangling association are
emitted (UIP-13); a disabled control cannot be edited and is announced as disabled (UIP-14); and a
numeric field right-aligns its value with tabular figures (UIP-15).

---

## 7. The checkbox is a labelled boolean

```bash
cd web && npx vitest run src/test/ui_checkbox.test.tsx
```

**Outcome**: the label is permanently visible and associated with the box, and checked, disabled and
invalid are each exposed rather than only styled (UIP-16); the primitive takes a boolean and reports a
boolean, so a value that is not a boolean (a numeric proxy) is left to the caller and never coerced by
the primitive (UIP-17).

---

## 8. The dialog takes the keyboard and gives it back

```bash
cd web && npx vitest run src/test/ui_dialog.test.tsx
```

**Outcome**: the dialog renders the scrim on the overlay colour behind a panel in the modal elevation
and the large radius, and exposes the dialog role with modal semantics — no dependency on the
platform's native modal element (UIP-18); the accessible name comes from the title and the caller's
description is exposed, and focus lands **inside** the dialog on open with the destructive control
never the one that carries focus (UIP-19); Tab at the last control wraps to the first and Shift+Tab at
the first wraps to the last, Escape dismisses the topmost surface only, and on close focus returns to
the control that opened the dialog (UIP-20); an activation whose target is the scrim dismisses while
an activation inside the panel does not, and the caller is notified exactly once per close (UIP-21).

---

## 9. The focus confinement holds with zero, one and many controls

```bash
cd web && npx vitest run src/test/ui_focus_trap.test.ts
```

**Outcome**: driven directly, the standalone module keeps focus inside its container for **zero**
controls (the container itself stays focusable, so the surface is still dismissible), **one** control
(forward and backward both land on it) and **many** (it cycles in both directions, wrapping at both
ends and never escaping even when focus is forced outside) (UIP-22).

---

## 10. The notice informs without interrupting

```bash
cd web && npx vitest run src/test/ui_toast.test.tsx
```

**Outcome**: the notice is a polite live region that takes no focus, renders no scrim and exposes no
modal semantics (UIP-23); its dismiss control's accessible name resolves through the catalog and its
message renders as text, and an empty message renders no notice (UIP-24); the four system states
render as a variant with the meaning carried by the text, never by colour alone (UIP-25); and left
alone it stays rendered — advancing the virtual clock does not remove it, because the primitive owns
no timer and disappears only when the caller removes it or the person dismisses it (UIP-26).

---

## 11. The documented rules are the source of truth, and the rule changed

```bash
cd web && npx vitest run src/test/design_guidelines.test.ts
grep -n 'base component' AGENTS.md DESIGN.md
```

**Outcome**: `web/DESIGN.md` names the layer, its primitive set and the class convention
`lg-<block>__<element>--<modifier>` (UIP-03); `web/AGENTS.md` sanctions `web/src/ui/` as the one place
a base component may live, keeps the **three real usages** rule for every other extraction, and the
old "no base component layer" prohibition is **gone** rather than restated (UIP-03, SC-007). The
decision is recorded as `AD-019` in `.specs/STATE.md`, and a document that still forbids a base layer
turns the suite red.

---

## 12. The layer's stylesheet citations are gated

```bash
cd web && grep -c 'UIP-' src/ui/ui.css
cd .. && python3 scripts/check_requirement_ids.py
```

**Outcome**: `scripts/check_requirement_ids.py` now walks `*.css` as well as the suffixes it already
walks (`.go`, `.dart`, `.ts`, `.tsx`), so every `UIP-*` citation in `ui.css` resolves to `spec.md`;
the current corpus stays clean — every ID already cited in an existing client stylesheet resolves, so
no assertion is weakened and no comment has to be removed (UIP-06, SC-006). `scripts/test_lints.py`
gains the stylesheet-walker case.

---

## 13. The gates stay green

```bash
cd web && npm run format:check && npm run lint && npm run typecheck && npm test && npm run build
cd .. && make fmt && make lint && make vet && make lint-specs
```

**Outcome**: all client gates pass and the spec lint reports clean (`695` spec IDs plus `UIP-*`, `41`
decision IDs plus `AD-019`), so every citation in the delivered sources **and** the stylesheet
resolves and no suite assertion is vacuous (SC-006). No dependency is added (SC-005), so the security
gates are unchanged.

---

## Traceability

| Scenario | Requirements |
|---|---|
| 1 | all `UIP-*` (no backend, no new dependency) |
| 2 | UIP-01, UIP-02, SC-007 |
| 3 | UIP-04, SC-002 |
| 4 | UIP-05, SC-004 |
| 5 | UIP-07…UIP-10, SC-001, SC-003 |
| 6 | UIP-11…UIP-15 |
| 7 | UIP-16, UIP-17 |
| 8 | UIP-18…UIP-21 |
| 9 | UIP-22 |
| 10 | UIP-23…UIP-26 |
| 11 | UIP-03, SC-007 |
| 12 | UIP-06, SC-006 |
| 13 | all `UIP-*`, SC-005, SC-006 |
