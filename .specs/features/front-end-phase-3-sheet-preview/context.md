# Front End Phase 3 — Sheet Preview Context

**Gathered:** 2026-10-01
**Spec:** `.specs/features/front-end-phase-3-sheet-preview/spec.md`
**Status:** Design amended (UAT 2026-10-01)

---

## Feature Boundary

Read-only preview of the template's fields with their base values, derived entirely from the already-loaded `FieldSchema`, rendered **inside the creation dialog** (Tela 3). No editing, no `POST /sheets`, no per-system hardcode, no new endpoint, no shell change.

---

## Implementation Decisions

### Surface (revised after the first round)

- Round 1 chose the right panel with the dialog closing — the maintainer rejected it on seeing the real cost: **the typed sheet name was discarded**.
- Round 2 chose the preview **inside the same dialog**, in a **card grouping to the right** of the dialog information (two columns).
- Round 3 (UAT 2026-10-01): on the running dialog the maintainer found the two columns odd and asked for the preview **below the sheet-name field** (single column, 560px content), an **always-visible scrollbar**, a **"Read-only preview"** header and the **template id beside the name** (`nome | id`). This supersedes Round 2's two-column decision.
- Final decision: the preview lives **inside the same dialog, below the sheet-name input**, at the full content width.
- The dialog never closes for previewing; no second dialog, no push route, no right-panel section, no `ShellSurface` change.

### Visibility

- The preview card is **always visible** once `GET /templates/:id` settles — no "Preview" action, no toggle.
- Loading / mapped error + Retry / no-fields render **inside the preview card** (the card is the only part that reacts to loading).

### Scroll & read-only affordances (UAT 2026-10-01)

- The card body keeps a bounded height (`previewCardMaxHeight = 420`) and wraps its scroll view in a `Scrollbar(thumbVisibility: true)` — the overflow is visible, not implicit.
- The card header reads **"Read-only preview"** (`createSheetPreviewLabel`), so the no-edit contract (PRV-06) is stated, not inferred from greyed controls.
- Template identity renders **name + id on one row**, separated by a hairline divider; empty name drops the divider (id only).

### Phase-2 text summary → removed

- The maintainer chose to **drop the text summary** (`Stats: Constitution, Dexterity`) since the preview card shows the same grouping plus values — listing every field twice in one dialog is noise.
- Consequence: `desktop/lib/catalog/template_summary.dart` and its tests are deleted (dead code); `fieldDisplayName` stays in `desktop/lib/engine/field_grouping.dart`.
- The Phase-2 spec gets an amend note rewriting TPL-02/03/04/05 — same pattern as its UAT amend.

### Reserved groups (`config` / `derived` / `modifiers`)

- **Shown, read-only** — the point of the card is "see what the sheet contains" and the ticket's example (`Armor class`) is derived.
- LGN-99 (backend group flag, still To Do) is **not** a dependency; its flag may later own visibility.

### Base values & presentation

- Every leaf shows `FieldSchema.default`, **including derived fields** (no formula evaluation — that is LGN-103's dry-run).
- Reuse `SheetForm` + `SheetFieldControl` with `onChanged: null`. The card is a flat bordered surface (hairline + `AppRadius.panel`, no elevation) with a localized caption header.

### Agent's Discretion

- Exact dialog content width (locked at 560 in the UAT amend) and the card's max height / internal scroll bound (must keep the name field and actions reachable — mandatory layout review).
- Position of the caption header and its test key; whether the card wraps `SheetForm` directly or a thin preview widget around it.
- How the identity/name block renders now that loading/error states live in the card.
- Deleting `template_summary.dart` + `template_summary_test.dart` and the summary assertions in `create_sheet_dialog_test.dart`.

### Declined / Undiscussed Gray Areas → Assumptions

- Right panel surface (round 1) and nested preview dialog — both declined in favor of the in-dialog card; recorded in the spec's Assumptions.
- Blocking on LGN-99 — declined; recorded (show all groups now).
- Pushing a read-only Tela 2 route — recorded in Out of Scope.

---

## Specific References

- "Renderizar, no diálogo de criação (ou numa superfície de detalhe), uma prévia somente-leitura dos campos do template com seus valores default" — LGN-100.
- Ticket example values: `Armor class: 10`, `Constitution: 10`.
- Maintainer's layout direction: "expand inside the same dialog in another 'card grouping' on right of our current dialog information".
- `desktop/AGENTS.md`: cards only for genuinely bounded objects, no elevation, mandatory critical layout review.

---

## Deferred Ideas

- LGN-99 group flag replacing the fixed `config`/`derived`/`modifiers` visibility.
- LGN-103 dry-run so the preview could show computed derived values instead of schema defaults.
- LGN-102 granular rebuild (only meaningful once editing exists — Phase 4).
- Right-panel "field details" usage for the sheet preview — shelved with this round's decision; revisit only if the maintainer asks again.
