# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Tabletop RPG players, primarily at the table during a live session: reading and editing their character sheet quickly, often on a phone. Players outside session time (building and levelling a character) are served by the same surfaces but are not the design priority. Game Masters are secondary users: in a Campaign they read their Players' sheets (read-only, refreshed about once a minute) to run the game, but the design priority stays the player at the table.

## Product Purpose

Legnaris is a flexible character-sheet manager for tabletop RPGs. Instead of encoding one game system, it models generic templates (typed fields with declarative formulas and a dependency graph), so any system can be expressed and derived values (modifiers, skills) recalculate on their own. Success is a player who trusts the sheet during play and never calculates by hand.

## Positioning

- Zero licensed content: the product ships calculation mechanics and generic structure, never a publisher's game content. Templates are code-defined and reviewed.
- Automatic cascading recalculation: a living sheet, unlike spreadsheets or editable PDFs.
- Portable user data: self-contained JSON export/import and a self-hostable deployment, so sheets outlive the service.

## Operating Context

- Active client is the installable React PWA in `web/` (replaces the deprecated Flutter desktop app); backend is a Go REST API with MongoDB.
- Sheet cycle: open, edit, preview, save, with crash-safe local drafts and guards before leaving unsaved work.
- Browser cookie session (sign-in, registration, silent refresh, sign-out); credentials never live in JavaScript-readable storage.
- Self-hosted, no third-party scripts, styles or fonts.

## Capabilities and Constraints

- V1: auth, one sheet per user, read-only code-defined template catalog, JSON import/export with strict validation.
- Out of scope V1: user-defined templates, sheet sharing, multiple sheets per user, real-time collaboration.
- All identifiers, comments and user-facing strings in English via `src/l10n/`. Multi-language (pt-BR) is a possible future direction, not decided.
- Visual rules already exist and are authoritative in `web/DESIGN.md` and `web/src/theme/`; this record does not restate them.

## Brand Commitments

Name: Legnaris. The existing token system and rules in `web/DESIGN.md` are binding for web work. No separate voice guide has been defined.

## Evidence on Hand

- `docs/TDD.md` (pt-BR) for problem, scope and architecture; `web/DESIGN.md`, `web/AGENTS.md`.
- No user testimonials, usage data or published benchmarks exist; future work must not fabricate them.

## Product Principles

- Table speed over ceremony: the sheet must be fast to read and edit mid-session.
- Never bake in a game system: structure is generic, content is the user's.
- The user owns their data: it must stay exportable and portable.
- Derived values are computed, not typed: the engine, not the player, does the arithmetic.
- Offline resilience: a lost connection or crash must not lose work.

## Accessibility & Inclusion

Baseline is the contract in `web/DESIGN.md`: AA contrast in both themes, visible focus ring, keyboard-only path through every surface. The user indicated additional accessibility requirements exist but did not specify them; they are undecided and should be captured before accessibility-focused work.
