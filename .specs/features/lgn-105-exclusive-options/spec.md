# LGN-105 — Mutually Exclusive Options (`exclusive_group`) Specification

Source: [LGN-105](https://paulovalleriotedev.atlassian.net/browse/LGN-105) — "[Front End] Fase 3 follow-up — Opções mutuamente exclusivas no schema (front/back)". Parent epic: LGN-92 (Fase 3 — UI Engine).

## Problem Statement

`config.acUsesDex` and `config.acUsesCon` are two independent 0/1 proxy leaves, so the UI Engine renders them as two independent toggles even though they express a single choice ("armor class uses dexterity, constitution, or nothing") and the `armorClass` formula consumes both as multipliers. The six `config.spellUses*` flags have the same defect: they read as "spellcasting uses <ability>" but render as six independent switches, when spellcasting should use exactly one ability (the `spells.attackBonus` formula sums all six multipliers). Nothing in the contract says "one or the other", and a prefix heuristic on the client would be wrong (`save*Prof`, `skill*Prof` are genuinely independent flags). Without a contract signal, the sheet can double-count modifiers (two flags on) and users see nonsensical switch pairs. This feature keeps the 0/1 proxy strategy intact and adds the missing exclusivity signal, enforced backend-side and rendered as one control in the desktop UI Engine.

## Goals

- [ ] The template contract exposes, per leaf, whether it belongs to a mutually exclusive group.
- [ ] `config.acUsesDex` and `config.acUsesCon` declare the same group and never both carry a non-zero value.
- [ ] A `PATCH` that turns one member on zeroes its siblings; a `PATCH` that sets two members on is rejected with a typed 400.
- [ ] The desktop renders each group as one control (a single dropdown) that can never leave two members on, and each 0/1 flag as a checkbox.
- [ ] Read-only surfaces (sheet preview) render the same group control disabled with the active option shown.
- [ ] The six spellcasting-ability flags (`config.spellUses*`) form one exclusive group, and the redundant `spells.ability` text leaf is gone.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| ------- | ------ |
| Replacing the proxies with a single string/enum field (card's option a) | User decision: the 0/1 proxy strategy is the anti-copyright keystone; option (b) keeps formulas, storage and existing sheets valid |
| Humanizing / shortening option labels | Card lists it as a separate follow-up card ("Humanização/labels … card próprio") |
| Group-level display labels beyond the humanized group id | Same follow-up; the group id is the only label source for now |
| Marking `spellUses*` / `save*Prof` / `skill*Prof` as exclusive | Card: those are genuinely independent flags, not a choice — **superseded for `spellUses*` by user decision (see Assumptions); `save*Prof` / `skill*Prof` stay independent** |
| New field types (`select`/`enum`) or formula-engine changes | Not needed by option (b); the engine stays numeric |
| Data migration for existing sheets | No stored shape changes; old sheets remain valid |
| LGN-99 (group-visibility flag in the DTO) | Separate card; orthogonal to exclusivity |
| Server-side enforcement for other clients of derived effects | Out of scope; only the exclusive siblings are normalized on PATCH |

---

## Assumptions & Open Questions

Every ambiguity is resolved or recorded here - nothing is left silently unclear.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | --------------------------- | --------- | ---------- |
| Contract model: select field (a) vs exclusivity metadata (b) | Option (b): add `exclusive_group` to the leaf schema, keep the 0/1 proxies | User decision (discuss): (a) rewrites the numeric formula env, `SeedValue`/`default` and needs a data migration, and removes anti-copyright proxies; (b) keeps formulas/storage/sheets valid | y |
| Metadata key name | `exclusive_group` (json/bson, snake_case like `depends_on`/`is_editable`) | User decision (discuss): `one_of` collides with JSON Schema's array-of-schemas keyword and promises "exactly one" while the contract is "at most one" | y |
| Membership model | A leaf carries a scalar group id; members sharing a non-empty id under the same parent form the group | Least invasive (b) shape; no group node, no path change, formulas untouched | y |
| Enforcement location | The DAG normalizes updates (`NormalizeExclusiveUpdates`) after key validation and before recalculation; the service calls it | DAG already owns leaf schema and is the single authority; keeps the service thin and the rule testable in isolation | y |
| What happens when two members are set on in one PATCH | Reject with a typed error mapped to 400 `INVALID_REQUEST` | Ambiguous intent must not be silently resolved; the UI never sends this | y |
| Turning a member off | Allowed; siblings are left unchanged (the group may end with no active member) | "At most one", not "exactly one"; a sheet may legitimately use neither (AC falls back to base) | y |
| Minimum group size | Fewer than two editable members is not an exclusive group | A one-member group has nothing to exclude; it degrades to the normal control in the UI | y |
| Derived / non-editable members | Never join a group | Derived values are computed, not chosen; including them would make normalization write a computed field | y |
| Desktop widget choice | A single dropdown for every exclusive group, with a `None` entry when the group has no active default; independent 0/1 flags render as checkboxes | User decision (increment 3): one control per concept — the earlier segmented/radio split confused users | y |
| Single-member group on the desktop | Renders with the member's normal per-field control | Consistent with the backend minimum-group-size rule | y |
| Fixture source | Contract fixtures are updated by hand to the frozen shape (`exclusive_group` on the pair) | The contract shape is frozen in `design.md`; a live capture adds a running-service dependency with no extra signal | y |
| Read-only rendering | The group renders disabled with the active option selected | Matches the existing rule that a schema-editable field keeps its control type in read-only surfaces | y |
| Which config flags are exclusive | The armor-class pair AND the six `config.spellUses*` (group `spellcastingAbility`); `save*Prof` / `skill*Prof` stay independent | User decision (increment): spellcasting must be a single choice; overrides the original card note that `spellUses*` were loose flags | y |
| The redundant `spells.ability` text leaf | Removed; its "Spellcasting ability" label becomes the exclusive group's row label | User decision: no duplicate control; MVP pre-launch, no stored-sheet compatibility concern | y |
| Default active option for `spellcastingAbility` | All six stay `0` (no ability selected) | User decision: keeps seeded values and derived `spellAttackBonus` unchanged | y |

**Open questions:** none - all resolved or logged above (required before the spec is confirmed).

---

## User Stories

### P1: Contract advertises exclusive groups (backend) ⭐ MVP

**User Story**: As a client rendering a template, I want each field schema to tell me when it belongs to a mutually exclusive set so I can render a single choice without guessing from field names.

**Why P1**: The UI Engine cannot express exclusivity without a contract signal; this is the card's option (b).

**Acceptance Criteria** (each line is one EARS pattern):

1. WHEN a template field schema declares an exclusive group, the API response SHALL include that field's `exclusive_group` value  <!-- event-driven -->
2. WHERE a field schema declares no exclusive group, the API response SHALL omit the `exclusive_group` key  <!-- optional-feature -->
3. WHEN the fantasy_v1 template is built, `config.acUsesDex` and `config.acUsesCon` SHALL each declare the exclusive group `armorClassAbility`  <!-- event-driven -->

**Independent Test**: `GET /templates/fantasy_v1` contains `exclusive_group: "armorClassAbility"` on both ac leaves and no `exclusive_group` on any other leaf.

---

### P1: Backend enforces at most one active member (backend)

**User Story**: As the owner of the sheet data, I want the backend to guarantee that two mutually exclusive members never carry non-zero values at once so the derived formulas never double-count.

**Why P1**: Without enforcement a raw `PATCH` (not the UI) can set both flags and silently corrupt `armorClass`.

**Acceptance Criteria**:

1. WHEN a PATCH sets one member of an exclusive group to a non-zero value, the system SHALL set every sibling member of that group to zero  <!-- event-driven -->
2. IF a PATCH sets two or more members of the same exclusive group to non-zero values, THEN the system SHALL reject the request with a 400 `INVALID_REQUEST` error  <!-- unwanted-behavior -->
3. WHEN a PATCH sets an exclusive member to zero, the system SHALL leave its siblings unchanged  <!-- event-driven -->
4. WHEN a PATCH changes an exclusive member, the system SHALL recompute every dependent derived field so no exclusive multiplicand is double-counted  <!-- event-driven -->
5. WHERE an exclusive group has fewer than two editable members, the system SHALL treat its members as non-exclusive  <!-- optional-feature -->

**Independent Test**: PATCH `{"config":{"acUsesDex":1}}` onto a sheet whose `acUsesCon` is 1 → both flags on the response are (1, 0) and `derived.armorClass` reflects only dexterity; PATCH both to 1 → 400 `INVALID_REQUEST`.

---

### P1: Desktop renders a group as one control (frontend)

**User Story**: As a user configuring a sheet, I want mutually exclusive options shown as one consistent control so I cannot turn on two conflicting flags by accident and I do not have to learn a different widget per group.

**Why P1**: The card's frontend objective; the original segmented/radio split made the same concept look different per group (increment 3 standardizes on one control).

**Acceptance Criteria**:

1. WHEN the UI Engine renders any exclusive group, the system SHALL render it as a single dropdown control (the same control for a 2-option and a 6-option group)  <!-- event-driven -->
2. WHEN an exclusive group has no active default option, the system SHALL include a `None` option in the dropdown that clears the group; WHEN the group has a default active option, the system SHALL NOT include `None`  <!-- event-driven -->
3. WHEN the desktop renders a read-only sheet, the system SHALL render each exclusive group as its dropdown disabled with the current value (or `None`) shown  <!-- event-driven -->
4. WHERE an exclusive group has a single member, the UI Engine SHALL render that member with its normal per-field control  <!-- optional-feature -->

**Independent Test**: A template with the `armorClassAbility` pair renders one dropdown (key `sheet-exclusive-armorClassAbility`) showing `Dexterity` and no `None`; the 6-option `spellcastingAbility` group renders the same dropdown control with a `None` entry (no default); the preview renders the dropdown disabled with the active value.

---

### P1: Desktop selection keeps at most one member on (frontend)

**User Story**: As a user choosing an exclusive option, I want the choice stored immediately and consistently so the sheet reflects exactly one active member.

**Why P1**: Completes the loop: the control must write the exclusivity rule into the edit state, in memory, with no network call.

**Acceptance Criteria**:

1. WHEN the user selects a dropdown value (a member or `None`), the system SHALL set the selected member to 1 and every sibling member of that group to 0 (all members to 0 for `None`) in a single edit-state emission  <!-- event-driven -->
2. WHEN the user selects the option that is already active, the system SHALL leave the edit state unchanged  <!-- event-driven -->
3. WHEN the edit state initializes from schema defaults, the system SHALL reflect the group's default active option  <!-- event-driven -->
4. WHEN the user edits an exclusive group, the system SHALL issue no network request  <!-- event-driven -->

**Independent Test**: Select the constitution option → `config.acUsesDex == 0` and `config.acUsesCon == 1` after one state emission; re-selecting it leaves the state instance identical; the recorded requests stay GET-only.

---

### P1: Spellcasting ability is a single choice (backend + frontend) ⭐ MVP

**User Story**: As a user configuring spellcasting, I want to pick exactly one ability so the sheet never sums two ability modifiers into the spell attack bonus.

**Why P1**: The six `spellUses*` flags have the same defect as the ac pair; the user confirmed they must be exclusive.

**Acceptance Criteria**:

1. WHEN the fantasy_v1 template is built, each `config.spellUses<Ability>` leaf SHALL declare the exclusive group `spellcastingAbility`  <!-- event-driven -->
2. WHEN the fantasy_v1 template is built, it SHALL NOT expose a `spells.ability` text leaf (the exclusive group is the single spellcasting-ability control)  <!-- event-driven -->
3. WHEN the desktop renders the spellcasting group, the system SHALL render it as the same single dropdown control used for every exclusive group, including a `None` entry  <!-- event-driven -->
4. WHEN the user selects a spellcasting ability, the system SHALL set the selected member to 1 and the other five to 0 in a single edit-state emission  <!-- event-driven -->
5. WHEN the desktop renders the spellcasting group read-only (preview), the system SHALL render the dropdown disabled with the current value (or `None`) shown  <!-- event-driven -->

**Independent Test**: `GET /templates/fantasy_v1` has no `spells.ability` and tags all six `spellUses*` with `spellcastingAbility`; the screen shows the shared dropdown with a `None` entry and selecting one member zeroes the other five; the preview shows the same dropdown disabled.

---

### P1: Standardized option labels and flag controls (backend + frontend) ⭐ MVP

**User Story**: As a user, I want each kind of field to look the same everywhere — a dropdown for a choice, a checkbox for an on/off flag — and short option labels, so the Config section is predictable instead of a mix of switches, segmented buttons and radios.

**Why P1**: User decision (increment 3): the visual variation across equivalent fields was confusing.

**Acceptance Criteria**:

1. WHEN a field schema declares an option label, the API response SHALL include that field's `option_label` value  <!-- event-driven -->
2. WHERE a field schema declares no option label, the API response SHALL omit the `option_label` key  <!-- optional-feature -->
3. WHEN the fantasy_v1 template is built, the `armorClassAbility` and `spellcastingAbility` members SHALL each declare an `option_label` naming the ability (e.g. `Dexterity`, `Charisma`)  <!-- event-driven -->
4. WHEN the desktop parses a template field schema that declares an `option_label`, the parsed schema SHALL expose it  <!-- event-driven -->
5. WHEN the desktop renders a dropdown option, the system SHALL use the member's `option_label` when present and fall back to the member's label otherwise  <!-- event-driven -->
6. WHEN the UI Engine renders an editable 0/1 flag, the system SHALL render a checkbox (never a switch)  <!-- event-driven -->
7. WHEN the user toggles an editable flag checkbox, the system SHALL emit the value in the field's existing type (true/false for `bool`, 1/0 for the numeric `config.*` proxies)  <!-- event-driven -->
8. WHEN a flag is read-only, the system SHALL render its checkbox disabled  <!-- event-driven -->

**Independent Test**: `GET /templates/fantasy_v1` carries `option_label` on the ac pair and the six spellUses and nowhere else; the dropdown shows `Dexterity`/`Constitution`; the proficiency flags render as checkboxes; a numeric proxy checkbox emits `0`/`1` and a read-only flag renders disabled.

---

## Edge Cases

- IF a template declares an exclusive group id on a non-editable (derived) leaf, THEN the DAG SHALL exclude that leaf from the group.
- IF an exclusive group ends up with a single editable member, THEN normalization SHALL NOT alter any update for that group.
- WHEN a PATCH touches an exclusive group whose siblings are not present in the payload, THEN the missing siblings SHALL be written as zero only when a sibling is turned on.
- WHEN a PATCH sets an exclusive member to zero, THEN the group SHALL be allowed to have no active member.
- WHEN a template detail arrives with `exclusive_group` absent, THEN the desktop SHALL parse a null group and render every field with its normal control (backward compatible).
- WHEN the desktop renders a read-only sheet whose exclusive group has no active member, THEN the dropdown SHALL show `None` and stay disabled.
- WHEN an exclusive group has a default active member, THEN the dropdown SHALL NOT offer a `None` option.
- WHEN an exclusive dropdown option declares no `option_label`, THEN the option SHALL fall back to the member's schema label.
- WHEN the user selects a dropdown value, THEN only the affected controls SHALL rebuild (the group row and any derived consumers), never the whole form.
- WHEN the spellcasting group has all six members off, THEN the dropdown SHALL show `None` and stay selectable.
- WHEN a numeric `config.*` flag checkbox is toggled, THEN the emitted value SHALL be `1` or `0` (never a bool).
- WHEN a `bool` flag checkbox is toggled, THEN the emitted value SHALL be `true` or `false`.
- WHEN a flag's value is null or absent, THEN the checkbox SHALL render unchecked.
- WHEN the user selects `None`, THEN every member of the group SHALL be set to 0 in a single emission.
- WHEN `spells.ability` is removed, THEN the spells section SHALL render the remaining leaves with no gap.
- IF a stored sheet still carries `spells.ability`, THEN the API SHALL ignore it (no template leaf) and the desktop SHALL not render it.
- IF a PATCH sets an exclusive member to a non-numeric value, THEN the system SHALL reject the request with a 400 `INVALID_REQUEST` (the group is numeric 0/1 proxies).
- IF a PATCH payload nests deeper than `maputils.MaxNestingDepth`, THEN the system SHALL reject it with a 400 `INVALID_REQUEST` before the recursive flatten walks the body (DoS guard).

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------- |
| EXC-01 | P1: Contract advertises exclusive groups | Execute | Verified |
| EXC-02 | P1: Contract advertises exclusive groups | Execute | Verified |
| EXC-03 | P1: Contract advertises exclusive groups | Execute | Verified |
| EXC-04 | P1: Backend enforces at most one active member | Execute | Verified |
| EXC-05 | P1: Backend enforces at most one active member | Execute | Verified |
| EXC-06 | P1: Backend enforces at most one active member | Execute | Verified |
| EXC-07 | P1: Backend enforces at most one active member | Execute | Verified |
| EXC-08 | P1: Backend enforces at most one active member | Execute | Verified |
| EXC-09 | P1: Desktop renders a group as one control | Execute | Implementing |
| EXC-10 | P1: Desktop renders a group as one control | Execute | Implementing |
| EXC-11 | P1: Desktop renders a group as one control | Execute | Implementing |
| EXC-12 | P1: Desktop renders a group as one control | Execute | Verified |
| EXC-13 | P1: Desktop selection keeps at most one member on | Execute | Implementing |
| EXC-14 | P1: Desktop selection keeps at most one member on | Execute | Verified |
| EXC-15 | P1: Desktop selection keeps at most one member on | Execute | Verified |
| EXC-16 | P1: Desktop selection keeps at most one member on | Execute | Verified |
| EXC-17 | P1: Contract advertises exclusive groups | Execute | Verified |
| EXC-18 | P1: Spellcasting ability is a single choice | Execute | Verified |
| EXC-19 | P1: Spellcasting ability is a single choice | Execute | Verified |
| EXC-20 | P1: Spellcasting ability is a single choice | Execute | Verified |
| EXC-21 | P1: Spellcasting ability is a single choice | Execute | Verified |
| EXC-22 | P1: Spellcasting ability is a single choice | Execute | Verified |
| EXC-23 | P1: Standardized option labels and flag controls | Execute | Implementing |
| EXC-24 | P1: Standardized option labels and flag controls | Execute | Implementing |
| EXC-25 | P1: Standardized option labels and flag controls | Execute | Implementing |
| EXC-26 | P1: Standardized option labels and flag controls | Execute | Implementing |
| EXC-27 | P1: Standardized option labels and flag controls | Execute | Implementing |
| EXC-28 | P1: Standardized option labels and flag controls | Execute | Implementing |
| EXC-29 | P1: Standardized option labels and flag controls | Execute | Implementing |
| EXC-30 | P1: Standardized option labels and flag controls | Execute | Implementing |

**ID format:** `EXC-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 30 total, 30 mapped to tasks, 0 unmapped

---

## Success Criteria

- [ ] `go test -race ./...` green, including DTO omission, seeder tag, normalization edge cases, and the service 400 path.
- [ ] `flutter analyze && flutter test` green, including dropdown selection (member and `None`), single-member degradation, read-only rendering, checkbox emission, and batch edit emission.
- [ ] `GET /templates/fantasy_v1` shows `exclusive_group: "armorClassAbility"` on exactly `config.acUsesDex` and `config.acUsesCon`.
- [ ] `PATCH {"config":{"acUsesDex":1}}` on a sheet with `acUsesCon:1` yields (1, 0) and a recomputed `armorClass`; PATCHing both to 1 returns 400.
- [ ] The desktop shows one dropdown per exclusive group and a checkbox per flag, in both edit and read-only surfaces.
- [ ] `GET /templates/fantasy_v1` tags all six `config.spellUses*` with `spellcastingAbility` and exposes no `spells.ability`.
- [ ] The desktop renders the spellcasting group as the shared dropdown with a `None` entry; selecting a member zeroes the other five in one emission.
- [ ] `option_label` is present on the ac pair and the six spellUses and absent everywhere else.
- [ ] `go test -race ./...` and `flutter analyze && flutter test` stay green.
