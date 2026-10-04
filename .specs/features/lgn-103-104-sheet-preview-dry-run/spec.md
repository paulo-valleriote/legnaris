# Sheet Preview Dry-Run Specification

Source: [LGN-103](https://paulovalleriotedev.atlassian.net/browse/LGN-103) — "Backend — Preview stateless de recálculo da ficha (dry-run)" and [LGN-104](https://paulovalleriotedev.atlassian.net/browse/LGN-104) — "[Front End] Fase 4 — Preview de valores calculados via dry-run com rate-limit e backup do rascunho", the latter under epic [LGN-93](https://paulovalleriotedev.atlassian.net/browse/LGN-93). LGN-103 blocks LGN-104. Both cards ship in one pull request: the backend endpoint and the desktop consumer land together.

## Problem Statement

The sheet lifecycle commits edits only through an explicit Save (`PATCH /sheets/:id`), and the response carries just the derived cascade. There is no way to see the recalculated result **before** committing: the only path that runs the DAG persists its result, so the desktop cannot offer a preview without writing. Two gaps follow: the backend needs a stateless dry-run that reuses the exact PATCH recalculation with persistence off, and the desktop needs a Preview action — enabled once per edit, single-flight, rate-limited — that merges the dry-run cascade into the local draft without persisting, plus a periodic draft backup so unsaved edits survive a restart. This feature ships backend and desktop in one PR.

## Goals

- [ ] Add `POST /sheets/:id/preview`: same recalculation as `PATCH /sheets/:id`, zero persistence, cascade identical to an equivalent PATCH, same typed errors.
- [ ] Add `POST /templates/:id/preview`: the same dry-run seeded from a template's fields, for creation-time evaluation with no stored sheet.
- [ ] Reuse one recalculation helper so persistence lives only in `PartialUpdate` and no logic is duplicated.
- [ ] Add a footer Preview action on Screen 2 that runs one dry-run per edit, merges the cascade into the draft, and never issues a `PATCH`.
- [ ] Enforce single-flight and a 3-per-minute limit on preview and on save, with clear disabled affordances.
- [ ] Back up unsaved drafts every ~1 minute and restore them when the sheet is reopened, clearing the backup on save, discard, eviction, or library removal.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| ------- | ------ |
| Auto-save, debounce, save-on-keystroke | Card and TDD: commit stays explicit via Save; `PATCH` only on Save |
| Label humanization and `config.*` exclusivity | Separate cards |
| Desktop UI that consumes `POST /templates/:id/preview` | LGN-104 only previews a stored sheet; the template variant is an API capability shipped for the creation flow to adopt later |
| A recovery dialog or prompt for a restored draft | The card asks for the backup; restore is automatic over the stored sheet. A prompt is a follow-up |
| Server-side rate limiting of preview | LGN-103 explicitly leaves rate-limit to the frontend card |
| Humanizing or logging draft files | Best-effort backup; no user-facing surface beyond restore |
| Multi-sheet, tabs, concurrent open sheets | V2 (TDD Questão #5); one open sheet at a time keeps draft handling single-writer |
| Auth, JWT, permissions on the new endpoints | V1 endpoints stay open, consistent with the rest of the API |
| Changing the `PATCH` request/response contract | The new endpoints mirror it; PATCH behavior is untouched |
| New packages (rate limiter, scheduler, storage) | `desktop/AGENTS.md`: no new dependency without strict necessity; existing seams suffice |

---

## Assumptions & Open Questions

Every ambiguity is resolved or recorded here - nothing is left silently unclear.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | -------------- | --------- | ---------- |
| Sheet preview contract | `POST /sheets/:id/preview` with `{"data": <delta>}` → `200 {"data": <cascade>}` | Card's primary proposal; mirrors the PATCH body and the existing success envelope | y |
| Preview response shape | The derived cascade only — the same body `PATCH /sheets/:id` returns | Card: "devolve apenas a cascata" and "cascata idêntica à de um PATCH equivalente"; the client already merges this shape | y |
| Template preview contract | `POST /templates/:id/preview` with `{"data": <delta>}` → `200 {"data": <cascade>}`, evaluated over the template's seeded fields (base at seed value, derived recalculated) | User decision (2026-10-04): expose the variant; seeding mirrors `Create` so the dry-run starts from the same state a new sheet would have | y |
| Reuse of the PATCH path | `PartialUpdate` and both previews call one unexported helper that fetches, flattens, normalizes and recalculates; persistence and `updated_at` stay in `PartialUpdate` only | Card: "Reusar o mesmo caminho de recálculo do PATCH … sem duplicar a lógica" | y |
| Preview side effects | Zero writes, no `updated_at` change, no repository write call | Card: "Nenhuma escrita no banco no caminho de preview" | y |
| Preview idempotency key | Preview is blocked while the current pending delta equals the delta of the last **successful** preview; a failure does not record | Card: "sem campo alterado desde o último preview, não re-dispara"; recording only success keeps a failed preview retryable | y |
| Preview/Save mutual exclusion | One in-flight request per sheet blocks both Preview and Save | Prevent an older dry-run cascade from merging over a newer committed baseline; a single server call per sheet keeps merges ordered | y |
| Rate limit | Separate rolling 60-second windows, 3 previews and 3 saves each, with an injectable clock | Card: "Máximo 3 previews por minuto" and "Máximo 3 por minuto" for save; a rolling window is the natural reading of "por minuto" and the clock seam keeps it testable | y |
| Blocked-action feedback | Action disabled with an l10n tooltip naming the reason (nothing new to preview / request in progress / limit reached) | User decision (2026-10-04); `desktop/AGENTS.md`: disabled primary actions explain themselves via tooltip | y |
| Preview action placement and copy | Secondary `TextButton` in the existing Screen 2 footer, label `Preview changes`, left of Save | User decision (2026-10-04); keeps a single primary action (Save) and reuses the footer surface | y |
| Preview merge target | Merge the cascade into the in-memory `data` only; do not advance `defaults` and do not clear `editedPaths` | Card: preview updates "Calculated" fields "sem persistir"; the baseline advances only on a real save | y |
| Draft backup scope | A per-sheet file under the app-support directory, written atomically, holding only the pending editable delta; restored automatically on reopen | User decision (2026-10-04); a delta is sufficient to overlay the stored sheet and avoids writing derived values | y |
| Draft backup cadence | Every 60 seconds while the sheet has pending edits; timer stops when the sheet is clean | Card: "a cada ~1 min enquanto houver edições não salvas"; an injectable scheduler keeps tests deterministic | y |
| Draft backup clearing | Delete on save success, on confirmed discard (leave/refresh), on `NOT_FOUND` eviction, and on library removal | Without clearing, an explicitly discarded or saved draft would resurrect on reopen | y |
| Draft restore filtering | Overlay only keys that are editable leaves of the loaded template; ignore the rest | A template can change between runs; unknown/derived draft keys must never enter the delta or the rendered data | y |
| Draft backup failure | Best-effort: a read or write failure never blocks editing and never shows an error | Backup is a safety net, not a required step of the commit cycle | y |
| App-support directory seam | Drafts reuse `appSupportDirectoryProvider` (AD-003); no new storage mechanism | `desktop/AGENTS.md`: reuse existing infrastructure; the directory is already injected at bootstrap | y |
| Backend conflict/cycle errors | Preview maps the same DAG errors as PATCH (unknown field, not editable, exclusive conflict, value type, nesting depth, formula eval, cycle) through the existing translators | Card: "Mesmos erros do PATCH … com o mesmo mapeamento" | y |
| Unknown template on preview | Missing/malformed template id → `400 INVALID_REQUEST`; unknown slug → `404 NOT_FOUND` | Mirrors `GET /templates/:id` and `POST /sheets` (LGN-86) | y |

**Open questions:** none - all resolved or logged above (required before the spec is confirmed).

---

## User Stories

### P1: Stateless sheet preview endpoint ⭐ MVP

**User Story**: As the desktop client, I want a dry-run recalculation of a delta against a stored sheet so that I can show computed values before committing.

**Why P1**: LGN-104 cannot exist without it; it is the blocking backend half of the vertical slice.

**Acceptance Criteria** (each line is one EARS pattern):

1. WHEN `POST /sheets/:id/preview` receives a valid delta for an existing sheet, the system SHALL respond `200` with a `data` object holding the recalculated cascade.  <!-- event-driven -->
2. The system SHALL compute the preview cascade through the same recalculation path `PATCH /sheets/:id` uses.  <!-- ubiquitous -->
3. WHEN a `PATCH /sheets/:id` and a preview receive the same stored sheet and the same delta, the system SHALL return the same cascade for both.  <!-- event-driven -->
4. The system SHALL persist no change during a preview request.  <!-- ubiquitous -->
5. WHEN the same preview request is repeated against an unchanged stored sheet, the system SHALL return the same cascade.  <!-- event-driven -->
6. WHEN a preview request carries an empty delta, the system SHALL respond `200` with an empty `data` object and persist nothing.  <!-- event-driven -->
7. IF the sheet id is missing or is not a canonical UUID v7, THEN the system SHALL respond `400 INVALID_REQUEST` with the same message the `PATCH` uses.  <!-- unwanted-behavior -->
8. IF the sheet does not exist, THEN the system SHALL respond `404 NOT_FOUND`.  <!-- unwanted-behavior -->
9. IF the delta names a key the DAG does not know, THEN the system SHALL respond `400 UNKNOWN_FIELD`.  <!-- unwanted-behavior -->
10. IF the delta names a derived (non-editable) leaf, THEN the system SHALL respond `400 INVALID_REQUEST`.  <!-- unwanted-behavior -->
11. IF the delta turns on two members of one exclusive group, THEN the system SHALL respond `400 INVALID_REQUEST` with the same conflict mapping the `PATCH` uses.  <!-- unwanted-behavior -->
12. IF the delta nests deeper than the supported depth, THEN the system SHALL respond `400 INVALID_REQUEST`.  <!-- unwanted-behavior -->

**Independent Test**: With a mocked repository, post a valid delta to `/sheets/:id/preview` and assert `200`, the cascade shape, and zero `PartialUpdate` calls; post the same body to `PATCH` on an identical sheet and assert equal cascades; repeat the preview and assert identical output; post an unknown key, a derived key and a too-deep body and assert the mapped codes; post an empty delta and assert `{}` with no write.

---

### P2: Stateless template preview endpoint

**User Story**: As the creation flow, I want to dry-run a delta against a template with no stored sheet so that I can evaluate computed values before a sheet exists.

**Why P2**: The card leaves the variant open and no desktop surface consumes it yet; the API capability ships now so creation can adopt it without a second backend change.

**Acceptance Criteria**:

1. WHEN `POST /templates/:id/preview` receives a valid delta for an existing template, the system SHALL respond `200` with a `data` object holding the recalculation cascade computed from the template's seeded fields.  <!-- event-driven -->
2. The system SHALL seed the template preview with the same initial data `POST /sheets` builds for that template.  <!-- ubiquitous -->
3. IF the template id is missing or is not a valid slug, THEN the system SHALL respond `400 INVALID_REQUEST`.  <!-- unwanted-behavior -->
4. IF the template does not exist, THEN the system SHALL respond `404 NOT_FOUND`.  <!-- unwanted-behavior -->
5. IF the delta names a key the template's DAG does not know or names a derived leaf, THEN the system SHALL respond with the same `UNKNOWN_FIELD` or `INVALID_REQUEST` mapping the sheet preview uses.  <!-- unwanted-behavior -->

**Independent Test**: Post a valid delta to `/templates/:id/preview` for a known template and assert `200` with the cascade seeded from its fields; post a missing and a malformed id and assert `400`; post an unknown slug and assert `404`; post an unknown and a derived key and assert the mapped codes.

---

### P1: Preview the recalculated cascade without persisting ⭐ MVP

**User Story**: As the app user, I want to preview the recalculated result of my current edits so that I can check derived values before I commit.

**Why P1**: This is the frontend half of the card's vertical slice; without it the new endpoint has no consumer.

**Acceptance Criteria**:

1. WHEN Screen 2 shows a loaded sheet whose pending delta differs from the delta recorded at the last successful preview, the system SHALL enable the Preview action.  <!-- event-driven -->
2. WHILE the pending delta equals the delta recorded at the last successful preview, the system SHALL keep the Preview action disabled and SHALL expose a tooltip stating there is nothing new to preview.  <!-- state-driven -->
3. WHEN the Preview action is activated, the system SHALL issue exactly one `POST /sheets/:id/preview` carrying only the pending editable delta.  <!-- event-driven -->
4. WHEN the Preview action is activated, the system SHALL issue no `PATCH /sheets/:id`.  <!-- event-driven -->
5. WHEN a preview responds `200`, the system SHALL merge the returned cascade into the in-memory draft and SHALL update only the controls whose value changed.  <!-- event-driven -->
6. WHEN a preview responds `200`, the system SHALL keep every local edit pending and SHALL NOT advance the baseline.  <!-- event-driven -->
7. WHEN a preview responds `200`, the system SHALL record the current pending delta as previewed so a repeat activation with no new edit is blocked.  <!-- event-driven -->
8. IF a preview fails for any reason, THEN the system SHALL keep every local edit and render the mapped error copy inline in the footer.  <!-- unwanted-behavior -->
9. IF a preview fails, THEN the system SHALL NOT record the delta as previewed, so Preview stays available.  <!-- unwanted-behavior -->
10. WHEN a save succeeds, the system SHALL clear the recorded preview delta so a new edit enables Preview again.  <!-- event-driven -->
11. WHERE Screen 2 is loading, in an error state, or has no fields, the system SHALL NOT render the Preview action.  <!-- optional-feature -->

**Independent Test**: Open a sheet, edit a field and assert Preview is enabled; activate it once and assert exactly one preview request, zero PATCH requests, and the cascade merged without clearing the dirty flag; activate again with no new edit and assert no request; edit again and assert a new request is allowed; fail a preview and assert the edit stays and Preview is still enabled; save and assert Preview is disabled until a new edit; assert the footer omits Preview while loading.

---

### P1: Preview and Save are single-flight and rate-limited ⭐ MVP

**User Story**: As the app user, I want Preview and Save to refuse overlapping and rapid-fire requests so that the draft and the server never drift.

**Why P1**: The card requires single-flight and a hard 3-per-minute cap on both actions; without them a burst corrupts the draft or spams the API.

**Acceptance Criteria**:

1. WHILE a preview or a save request is in flight, the system SHALL keep both Preview and Save disabled so neither issues a second request.  <!-- state-driven -->
2. WHEN Preview is activated while three preview requests were issued within the last 60 seconds, the system SHALL keep Preview disabled and SHALL expose a tooltip stating the preview limit.  <!-- event-driven -->
3. WHEN Save is activated while three save requests were issued within the last 60 seconds, the system SHALL keep Save disabled and SHALL expose a tooltip stating the save limit.  <!-- event-driven -->
4. WHEN an action's rolling window frees up, the system SHALL re-enable that action when it is otherwise allowed.  <!-- event-driven -->
5. The system SHALL count preview requests and save requests in separate rolling 60-second windows.  <!-- ubiquitous -->
6. WHEN a save request is issued, the system SHALL record it in the save window.  <!-- event-driven -->
7. The system SHALL send a `PATCH /sheets/:id` only on an explicit Save activation, never automatically.  <!-- ubiquitous -->

**Independent Test**: With an injectable clock, issue three previews and assert the fourth activation issues no request and Preview shows the limit tooltip; advance 60 seconds and assert Preview is enabled again; repeat for Save with the save window independent of the preview window; start a preview and assert Save is disabled while it is in flight; assert no request is issued without an explicit activation.

---

### P1: Unsaved drafts survive a restart ⭐ MVP

**User Story**: As the app user, I want my unsaved edits to be backed up periodically and restored when I reopen the sheet so that a crash or restart does not lose work.

**Why P1**: The card requires a periodic draft backup; without restore the backup has no value.

**Acceptance Criteria**:

1. WHILE a sheet has pending edits, the system SHALL write that sheet's draft backup every 60 seconds.  <!-- state-driven -->
2. WHEN a sheet transitions from no pending edits to pending edits, the system SHALL start the periodic draft backup.  <!-- event-driven -->
3. WHEN a sheet has no pending edits, the system SHALL stop the periodic draft backup.  <!-- event-driven -->
4. The system SHALL persist each draft backup with an atomic replace, leaving any prior backup intact when a write fails.  <!-- ubiquitous -->
5. The system SHALL persist only the pending editable delta in the draft backup, never a derived value.  <!-- ubiquitous -->
6. WHEN Screen 2 opens a stored sheet that has a draft backup, the system SHALL seed the edit state from the stored sheet overlaid with the backup delta and SHALL mark the restored fields pending.  <!-- event-driven -->
7. WHEN restoring a draft, the system SHALL overlay only keys that are editable leaves of the loaded template.  <!-- event-driven -->
8. WHEN a save succeeds, the system SHALL delete that sheet's draft backup.  <!-- event-driven -->
9. WHEN the user confirms discarding edits on leave or refresh, the system SHALL delete that sheet's draft backup.  <!-- event-driven -->
10. WHEN a sheet returns `NOT_FOUND` and its local entry is evicted, the system SHALL delete that sheet's draft backup.  <!-- event-driven -->
11. WHEN a library entry is removed, the system SHALL delete that sheet's draft backup.  <!-- event-driven -->
12. IF a draft backup cannot be read or decoded, THEN the system SHALL ignore it and seed from the stored sheet.  <!-- unwanted-behavior -->
13. IF a draft backup write fails, THEN the system SHALL keep accepting edits without showing an error.  <!-- unwanted-behavior -->
14. The system SHALL store draft backups under the injected app-support directory.  <!-- ubiquitous -->

**Independent Test**: With a temp directory and a fake clock, edit a sheet, advance 60 seconds and assert one backup file holding only the editable delta; assert a clean sheet writes none and the timer stops; reopen the sheet and assert the stored data is overlaid and the fields pending; assert save, confirmed discard, `NOT_FOUND` eviction and library removal each delete the file; corrupt the file and assert a clean seed; make the file unwritable and assert editing continues with no error.

---

## Edge Cases

Edge cases are usually unwanted-behavior (IF/THEN) or boundary (WHEN) criteria:

- WHEN the preview cascade nests deeper than one level, the system SHALL merge every nested key without flattening it.
- WHEN the preview cascade is empty, the system SHALL keep the pending edits and leave the rendered values unchanged.
- WHEN an edit is made while a preview is in flight, the system SHALL merge the returning cascade without discarding the newer edit.
- WHEN the rate-limit window holds exactly three entries, an additional activation SHALL issue no request.
- WHEN a draft backup holds a key that is no longer an editable leaf, the system SHALL ignore that key on restore.
- WHEN a draft backup holds a key whose value is derived by the template, the system SHALL never overlay it on the rendered data.
- IF the preview request is issued while the sheet is still loading, THEN the system SHALL issue no request because the action is not rendered.
- WHEN a preview fails with `NOT_FOUND`, the system SHALL keep the edits and render the mapped copy without evicting the local entry.
- IF a save is attempted while a preview is in flight, THEN the system SHALL issue no `PATCH`.
- WHEN the app is closed after a backup was written and the sheet is reopened, the system SHALL restore the backed-up delta.

---

## Requirement Traceability

Each requirement gets a unique ID for tracking across design, tasks, and validation.

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------- |
| DRY-01 | P1: Stateless sheet preview endpoint | Design | Pending |
| DRY-02 | P1: Stateless sheet preview endpoint | Design | Pending |
| DRY-03 | P1: Stateless sheet preview endpoint | Design | Pending |
| DRY-04 | P1: Stateless sheet preview endpoint | Design | Pending |
| DRY-05 | P1: Stateless sheet preview endpoint | Design | Pending |
| DRY-06 | P1: Stateless sheet preview endpoint | Design | Pending |
| DRY-07 | P1: Stateless sheet preview endpoint | Design | Pending |
| DRY-08 | P1: Stateless sheet preview endpoint | Design | Pending |
| DRY-09 | P1: Stateless sheet preview endpoint | Design | Pending |
| DRY-10 | P1: Stateless sheet preview endpoint | Design | Pending |
| DRY-11 | P1: Stateless sheet preview endpoint | Design | Pending |
| DRY-12 | P1: Stateless sheet preview endpoint | Design | Pending |
| DRY-13 | P2: Stateless template preview endpoint | - | Pending |
| DRY-14 | P2: Stateless template preview endpoint | - | Pending |
| DRY-15 | P2: Stateless template preview endpoint | - | Pending |
| DRY-16 | P2: Stateless template preview endpoint | - | Pending |
| DRY-17 | P2: Stateless template preview endpoint | - | Pending |
| PRV-01 | P1: Preview the recalculated cascade without persisting | Design | Pending |
| PRV-02 | P1: Preview the recalculated cascade without persisting | Design | Pending |
| PRV-03 | P1: Preview the recalculated cascade without persisting | Design | Pending |
| PRV-04 | P1: Preview the recalculated cascade without persisting | Design | Pending |
| PRV-05 | P1: Preview the recalculated cascade without persisting | Design | Pending |
| PRV-06 | P1: Preview the recalculated cascade without persisting | Design | Pending |
| PRV-07 | P1: Preview the recalculated cascade without persisting | Design | Pending |
| PRV-08 | P1: Preview the recalculated cascade without persisting | Design | Pending |
| PRV-09 | P1: Preview the recalculated cascade without persisting | Design | Pending |
| PRV-10 | P1: Preview the recalculated cascade without persisting | Design | Pending |
| PRV-11 | P1: Preview the recalculated cascade without persisting | Design | Pending |
| SVE-01 | P1: Preview and Save are single-flight and rate-limited | Design | Pending |
| SVE-02 | P1: Preview and Save are single-flight and rate-limited | Design | Pending |
| SVE-03 | P1: Preview and Save are single-flight and rate-limited | Design | Pending |
| SVE-04 | P1: Preview and Save are single-flight and rate-limited | Design | Pending |
| SVE-05 | P1: Preview and Save are single-flight and rate-limited | Design | Pending |
| SVE-06 | P1: Preview and Save are single-flight and rate-limited | Design | Pending |
| SVE-07 | P1: Preview and Save are single-flight and rate-limited | Design | Pending |
| DRB-01 | P1: Unsaved drafts survive a restart | Design | Pending |
| DRB-02 | P1: Unsaved drafts survive a restart | Design | Pending |
| DRB-03 | P1: Unsaved drafts survive a restart | Design | Pending |
| DRB-04 | P1: Unsaved drafts survive a restart | Design | Pending |
| DRB-05 | P1: Unsaved drafts survive a restart | Design | Pending |
| DRB-06 | P1: Unsaved drafts survive a restart | Design | Pending |
| DRB-07 | P1: Unsaved drafts survive a restart | Design | Pending |
| DRB-08 | P1: Unsaved drafts survive a restart | Design | Pending |
| DRB-09 | P1: Unsaved drafts survive a restart | Design | Pending |
| DRB-10 | P1: Unsaved drafts survive a restart | Design | Pending |
| DRB-11 | P1: Unsaved drafts survive a restart | Design | Pending |
| DRB-12 | P1: Unsaved drafts survive a restart | Design | Pending |
| DRB-13 | P1: Unsaved drafts survive a restart | Design | Pending |
| DRB-14 | P1: Unsaved drafts survive a restart | Design | Pending |

**ID format:** `DRY-[NUMBER]` for the backend dry-run endpoints, `PRV-[NUMBER]` for the desktop preview cycle, `SVE-[NUMBER]` for single-flight and rate limit, `DRB-[NUMBER]` for draft backup and restore.

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 49 total, mapped to tasks at Tasks.

---

## Success Criteria

- [ ] `go test ./...` green, including: preview returns the same cascade as an equivalent PATCH, zero repository writes on preview, idempotent repeat, empty-delta `200 {}`, and the mapped errors for missing/malformed/unknown ids, unknown fields, derived fields, exclusive conflicts and over-deep nesting — for both `/sheets` and `/templates`.
- [ ] `make lint && make vet` clean on the backend.
- [ ] `flutter test` green, including: one dry-run per edit with zero PATCH, blocked repeat with no new edit, cascade merged without clearing dirty, single-flight across Preview and Save, three-per-minute windows with an injectable clock, and draft backup write/restore/clear paths.
- [ ] `flutter analyze` clean.
- [ ] Manual check: edit a field on an open sheet, press **Preview changes**, and see the derived values update without the dirty flag clearing; kill the app mid-edit, reopen the sheet, and see the edits restored.
