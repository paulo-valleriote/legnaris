# LGN-102 Granular Rebuild Context

**Gathered:** 2026-10-01
**Spec:** `.specs/features/lgn-102-granular-rebuild/spec.md`
**Status:** Ready for design

---

## Feature Boundary

In-memory, per-field edit state on Screen 2 with granular rebuild (only the edited control re-renders), plus the user-decided local-edits marker and the leave/refresh discard guards. No save, no PATCH, no cascade, no persistence.

---

## Implementation Decisions

### Area 1 — Edit lifetime on navigation

- Leaving the sheet with pending edits shows a destructive-confirmation dialog stating the local edits will be discarded.
- Confirm → discard the edit state and navigate back. Cancel → stay with edits intact.
- Leaving with no pending edits navigates directly, no dialog.
- Discard is absolute: reopening the template reseeds from schema defaults (no session cache, no persistence).

### Area 2 — Reload semantics

- The sheet screen gains an F5 refresh entry point that refetches the template detail.
- With pending edits pending, refresh shows the same discard-confirmation before any request; cancel issues zero requests.
- Confirm → refetch; the form unmounts during loading (edit state disposes) and reseeds from the fresh schema on success.
- A failed refetch after confirm shows the mapped error + Retry; the edits stay discarded (the user accepted that when confirming).

### Area 3 — No-save affordance

- Small "local edits" caption marker in the sheet header, visible only while the edit state differs from the seeded defaults.
- No Save button, no footer bar, no PATCH — those stay in Phase 4.

### Agent's Discretion

- Marker wording: English l10n caption "Local edits"; hidden when state equals defaults.
- "Pending" definition: deep comparison against the seeded defaults, so reverting a field to its default clears it.
- F5 as the refresh trigger: `desktop/AGENTS.md` already declares F5 refresh as the V1 default binding; the registry gains a sheet-surface refresh path.
- Dialog copy follows the destructive-confirmation pattern from `desktop/AGENTS.md` (explicit consequence, explicit buttons, never confirm-by-Enter alone).
- Granularity mechanism: edit state as an autoDispose Riverpod family keyed by template id; each control subscribes through `select` on its own dotted path; the screen itself never watches the edit state.

### Declined / Undiscussed Gray Areas → Assumptions

- App/window close is not guarded (out of scope in the spec; neither the card nor the discussion covered leaving the app).
- Reload-failure semantics, marker placement (header vs the AGENTS footer preference), and the pending-edit definition are logged in the spec's Assumptions table with rationale.

---

## Specific References

- Card ACs are the anchor: "< 100ms after API response" and "editing a field does not re-render the other fields".
- User overrode both agent defaults in the discussion: warning on leave (default was silent discard) and a visible marker (default was silent local edit).

---

## Deferred Ideas

- Dirty indicator in a fixed footer + Save button + PATCH delta + cascade merge (Phase 4).
- Reapplying pending edits when the schema changes under a reload (merge logic, Phase 4).
- Guarding app close with unsaved local edits (rides with Phase 4's dirty cycle).
