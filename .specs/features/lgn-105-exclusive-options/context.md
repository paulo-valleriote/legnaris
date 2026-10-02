# LGN-105 — Context (User Decisions)

Recorded from the specification discussion before implementation. Each entry is a
locked decision; the spec's Assumptions table references them.

## Area 1 — Contract model: metadata, not a select field

**Decision:** Option (b) — add a scalar `exclusive_group` to the leaf `FieldSchema`; keep the
0/1 proxy leaves (`config.acUsesDex`, `config.acUsesCon`) and the existing `armorClass` formula.

**Alternatives considered:** Option (a) replaced the pair with one select/enum field.

**Why:** Option (a) rewrites the formula into string comparison, extends the numeric-only
formula compile env, changes `SeedValue` and the `default float64` contract, and requires a
migration for every stored sheet — while removing two of the anti-copyright 0/1 proxies that
the project's strategy is built on (`docs/TDD.md` §anti-copyright). Option (b) is additive and
keeps every existing sheet valid.

## Area 2 — Key name: `exclusive_group`

**Decision:** The metadata key is `exclusive_group` (snake_case, matching `depends_on` / `is_editable`).

**Why:** `one_of` collides with JSON Schema's `oneOf` keyword (an array of schemas, "exactly one
matches"). The contract here is a string group id and "at most one active", so `one_of` would
mislead readers and tooling.

## Area 3 — Enforcement: backend normalizes and advertises

**Decision:** The contract advertises the group AND the backend enforces it. `DAG.NormalizeExclusiveUpdates`
runs inside `PATCH /sheets/:id` after key validation: a non-zero member zeroes its siblings; two or
more non-zero members in the same payload is a typed error mapped to `400 INVALID_REQUEST`.

**Why:** Without enforcement any API client (not just the UI) can set both proxies and silently
double-count `armorClass`. The UI still renders a single control, so the happy path never produces
a conflicting payload.

## Area 4 — Desktop control: segmented with radio fallback

**Decision:** The UI Engine renders an exclusive group as a `SegmentedButton` when the group has
≤ 3 options and every option label is ≤ 30 characters; otherwise it renders a `Radio` group. A
single-member group degrades to the member's normal per-field control; a read-only surface renders
the group disabled with the active option selected.

**Why:** Segmented is the densest correct control and fits the 2-option `armorClassAbility` case;
radio survives longer option lists/labels without overflow.

## Area 5 — Tracking: implement under LGN-105

**Decision:** Implement backend + desktop directly on branch `feat/lgn-105-exclusive-options` under
LGN-105, record the decision as AD-002 in `.specs/STATE.md`, and summarize on the Jira card instead
of creating separate implementation cards.

**Why:** The card's acceptance criteria ask for the decision and follow-up cards; the user chose to
ship the change from the same card rather than open new ones, since the scope is small (one metadata
key plus one control).

## Area 6 — Spellcasting ability is exclusive too, and `spells.ability` goes away

**Decision:** The six `config.spellUses*` leaves join one exclusive group `spellcastingAbility` (default
all `0` = no ability selected). The free-text `spells.ability` leaf is removed; its "Spellcasting
ability" label becomes the exclusive group's row label.

**Why:** The user confirmed the six flags have the same defect as the ac pair — the `spells.attackBonus`
formula sums all six multipliers, so two abilities on double-counts. Making them exclusive turns the six
switches into one radio group. The `spells.ability` text field is then a redundant second control for the
same concept, so it is removed. Overrides the original card note that `spellUses*` were "loose flags".

**Trade-off:** Removing a leaf changes the template shape. The project is pre-launch MVP with no stored
sheets to preserve (user decision), so `fantasy_v1` is mutated in place rather than versioned to
`fantasy_v2`. A stored `spells.ability` on an old sheet would simply be ignored.

## Area 7 — Standardize the Config controls (increment 3)

**Decision:** One control per concept.
- Every exclusive group renders as a **single dropdown** (segmented and radio both removed). A group
  with no active default gets a **`None`** entry that clears it; a group with a default does not.
- Every independent 0/1 flag renders as a **checkbox** (Switch removed).
- Option labels come from a new schema field **`option_label`** (short: `Dexterity`, `Charisma`, …);
  when absent, the option falls back to the member's `label`.

**Why:** In one Config section the user saw a segmented button (AC), switches (proficiencies) and a
radio list (spellcasting) — three visual languages, two of them for the same concept. Standardizing to
"dropdown = choose one, checkbox = on/off" makes the section predictable. Short labels remove the
redundant "Spellcasting uses X" next to the group label "Spellcasting ability".

**Trade-off:** A dropdown needs one click to reveal options (less "at a glance" than a segmented
control) and `None` becomes an explicit entry for optional groups. Accepted for consistency and
density. This revises the earlier widget decision (segmented ≤3 / radio), which is superseded.
