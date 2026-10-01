# LGN-105 — Mutually Exclusive Options (`exclusive_group`) Specification

Source: [LGN-105](https://paulovalleriotedev.atlassian.net/browse/LGN-105) — "[Front End] Fase 3 follow-up — Opções mutuamente exclusivas no schema (front/back)". Parent epic: LGN-92 (Fase 3 — UI Engine).

## Problem Statement

`config.acUsesDex` and `config.acUsesCon` are two independent 0/1 proxy leaves, so the UI Engine renders them as two independent toggles even though they express a single choice ("armor class uses dexterity, constitution, or nothing") and the `armorClass` formula consumes both as multipliers. Nothing in the contract says "one or the other", and a prefix heuristic on the client would be wrong (`spellUses*`, `save*Prof`, `skill*Prof` are genuinely independent flags). Without a contract signal, the sheet can double-count modifiers (both flags on) and users see a nonsensical pair of switches. This feature keeps the 0/1 proxy strategy intact and adds the missing exclusivity signal, enforced backend-side and rendered as one control in the desktop UI Engine.

## Goals

- [ ] The template contract exposes, per leaf, whether it belongs to a mutually exclusive group.
- [ ] `config.acUsesDex` and `config.acUsesCon` declare the same group and never both carry a non-zero value.
- [ ] A `PATCH` that turns one member on zeroes its siblings; a `PATCH` that sets two members on is rejected with a typed 400.
- [ ] The desktop renders each group as one control (segmented, radio fallback) that can never leave two members on.
- [ ] Read-only surfaces (sheet preview) render the same group control disabled with the active option shown.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| ------- | ------ |
| Replacing the proxies with a single string/enum field (card's option a) | User decision: the 0/1 proxy strategy is the anti-copyright keystone; option (b) keeps formulas, storage and existing sheets valid |
| Humanizing / shortening option labels | Card lists it as a separate follow-up card ("Humanização/labels … card próprio") |
| Group-level display labels beyond the humanized group id | Same follow-up; the group id is the only label source for now |
| Marking `spellUses*` / `save*Prof` / `skill*Prof` as exclusive | Card: they are genuinely independent flags, not a choice |
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
| Desktop widget choice | Segmented control when ≤3 options and every label is ≤30 characters; otherwise a radio group | User decision (discuss): "segmented, radio fallback" — segmented is denser, radio survives long labels | y |
| Single-member group on the desktop | Renders with the member's normal per-field control | Consistent with the backend minimum-group-size rule | y |
| Fixture source | Contract fixtures are updated by hand to the frozen shape (`exclusive_group` on the pair) | The contract shape is frozen in `design.md`; a live capture adds a running-service dependency with no extra signal | y |
| Read-only rendering | The group renders disabled with the active option selected | Matches the existing rule that a schema-editable field keeps its control type in read-only surfaces | y |

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

**User Story**: As a user configuring a sheet, I want mutually exclusive options shown as one choice so I cannot turn on two conflicting flags by accident.

**Why P1**: The card's frontend objective; the current two switches are the defect.

**Acceptance Criteria**:

1. WHEN the UI Engine renders a section whose leaves share an exclusive group of at most three options with labels no longer than the segmented limit, the system SHALL render one segmented control for the group  <!-- event-driven -->
2. WHEN an exclusive group has more than three options or an option label longer than the segmented limit, the system SHALL render the group as a radio control instead  <!-- event-driven -->
3. WHEN the desktop renders a read-only sheet, the system SHALL render each exclusive group as its control disabled with the active option selected  <!-- event-driven -->
4. WHERE an exclusive group has a single member, the UI Engine SHALL render that member with its normal per-field control  <!-- optional-feature -->

**Independent Test**: A template with the `armorClassAbility` pair renders one `SegmentedButton` (key `sheet-exclusive-armorClassAbility`) instead of two switches; a 4-option fixture renders a radio group; the preview renders the segmented control disabled.

---

### P1: Desktop selection keeps at most one member on (frontend)

**User Story**: As a user choosing an exclusive option, I want the choice stored immediately and consistently so the sheet reflects exactly one active member.

**Why P1**: Completes the loop: the control must write the exclusivity rule into the edit state, in memory, with no network call.

**Acceptance Criteria**:

1. WHEN the user selects an option in an exclusive group, the system SHALL set the selected member to 1 and every sibling member of that group to 0 in a single edit-state emission  <!-- event-driven -->
2. WHEN the user selects the option that is already active, the system SHALL leave the edit state unchanged  <!-- event-driven -->
3. WHEN the edit state initializes from schema defaults, the system SHALL reflect the group's default active option  <!-- event-driven -->
4. WHEN the user edits an exclusive group, the system SHALL issue no network request  <!-- event-driven -->

**Independent Test**: Select the constitution option → `config.acUsesDex == 0` and `config.acUsesCon == 1` after one state emission; re-selecting it leaves the state instance identical; the recorded requests stay GET-only.

---

## Edge Cases

- IF a template declares an exclusive group id on a non-editable (derived) leaf, THEN the DAG SHALL exclude that leaf from the group.
- IF an exclusive group ends up with a single editable member, THEN normalization SHALL NOT alter any update for that group.
- WHEN a PATCH touches an exclusive group whose siblings are not present in the payload, THEN the missing siblings SHALL be written as zero only when a sibling is turned on.
- WHEN a PATCH sets an exclusive member to zero, THEN the group SHALL be allowed to have no active member.
- WHEN a template detail arrives with `exclusive_group` absent, THEN the desktop SHALL parse a null group and render every field with its normal control (backward compatible).
- WHEN the desktop renders a read-only sheet whose exclusive group has no active member, THEN the control SHALL render with no option selected and stay disabled.
- IF an exclusive option label is exactly at the segmented limit, THEN the segmented control SHALL be used (limit is inclusive).
- WHEN the user selects a different option, THEN only the affected controls SHALL rebuild (the group row and any derived consumers), never the whole form.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------- |
| EXC-01 | P1: Contract advertises exclusive groups | Execute | Implementing |
| EXC-02 | P1: Contract advertises exclusive groups | Execute | Implementing |
| EXC-03 | P1: Contract advertises exclusive groups | Execute | Implementing |
| EXC-04 | P1: Backend enforces at most one active member | Execute | Implementing |
| EXC-05 | P1: Backend enforces at most one active member | Execute | Implementing |
| EXC-06 | P1: Backend enforces at most one active member | Execute | Implementing |
| EXC-07 | P1: Backend enforces at most one active member | Execute | Implementing |
| EXC-08 | P1: Backend enforces at most one active member | Execute | Implementing |
| EXC-09 | P1: Desktop renders a group as one control | Execute | Implementing |
| EXC-10 | P1: Desktop renders a group as one control | Execute | Implementing |
| EXC-11 | P1: Desktop renders a group as one control | Execute | Implementing |
| EXC-12 | P1: Desktop renders a group as one control | Execute | Implementing |
| EXC-13 | P1: Desktop selection keeps at most one member on | Execute | Implementing |
| EXC-14 | P1: Desktop selection keeps at most one member on | Execute | Implementing |
| EXC-15 | P1: Desktop selection keeps at most one member on | Execute | Pending |
| EXC-16 | P1: Desktop selection keeps at most one member on | Execute | Pending |
| EXC-17 | P1: Contract advertises exclusive groups | Execute | Implementing |

**ID format:** `EXC-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 17 total, 17 mapped to tasks, 0 unmapped

---

## Success Criteria

- [ ] `go test -race ./...` green, including DTO omission, seeder tag, normalization edge cases, and the service 400 path.
- [ ] `flutter analyze && flutter test` green, including segmented vs radio selection, single-member degradation, read-only rendering, and batch edit emission.
- [ ] `GET /templates/fantasy_v1` shows `exclusive_group: "armorClassAbility"` on exactly `config.acUsesDex` and `config.acUsesCon`.
- [ ] `PATCH {"config":{"acUsesDex":1}}` on a sheet with `acUsesCon:1` yields (1, 0) and a recomputed `armorClass`; PATCHing both to 1 returns 400.
- [ ] The desktop shows one control for the pair in both edit and read-only surfaces.
