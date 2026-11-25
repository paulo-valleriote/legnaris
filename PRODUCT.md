# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

- **Primary: the player at a live session.** A fifth-edition player, often holding a phone in one hand at the table, who needs to read, roll, and change their character mid-play without doing arithmetic.
- **Secondary: the Game Master.** Runs a Campaign, reads Players' Sheets read-only (polled every minute, labelled as possibly stale), and links and reveals NPC and enemy Sheets.
- **Visitor.** Arrives at the home, tries the public `/demo` live sheet, and decides whether to sign up.

## Product Purpose

Legnaris keeps tabletop character sheets alive: an installable PWA that opens, edits, previews and saves one Sheet at a time, with every derived value recalculating itself. Success is a player who trusts the Sheet during play and never calculates a modifier by hand.

## Positioning

A generic, code-defined Template engine (typed Fields, declarative formulas, a compiled dependency graph) rather than one hardcoded game or a spreadsheet. Every Template follows the coherence rule: it automates every value its rules derive, and anything else is plain entry, never an approximation. Roll modes stay the player's choice.

## Operating Context

- Live play at the table: phone-first, one-handed, quick glances between turns; desktop also served.
- Edits go through preview and save; the stored Sheet is server-owned with a local draft per device so a crash never costs the evening.
- Campaigns are the only sharing mechanism: join by invite code (optional password), link one character; GMs never edit a Player's Sheet.
- AI generation (Paid only) is an asynchronous draft the player reviews and accepts.
- Full product detail, flows and contracts: `context/product.md` (the authoritative narrative; this file summarizes it for design work).

## Capabilities and Constraints

- First Game system is fifth edition via SRD 5.2.1 (CC-BY-4.0): active 5E Character (catalog default) and 5E Stat Block Templates. `fantasy_v1` is retired but its existing Sheets keep working.
- Sheet kinds: character, NPC, enemy; kind is fixed at creation.
- Plans: Free (1/2/2 Sheets, 1 Campaign, no AI, ads on passive screens only) and Paid (USD 6.79/month, 10/50/50 Sheets, 5 Campaigns, 30 generations/day, no ads).
- Ads never appear on a sheet, dice, play session, generation, plans or checkout; no Auto ads or interstitials.
- Self-hosted, no third-party scripts, styles or fonts (except the Free-plan AdSense units); strict same-origin policy.
- English-only interface; no offline editing, no real-time collaboration, no export/import, no user-authored Templates.

## Brand Commitments

- The name is Legnaris.
- Say "5E" / "5E compatible"; never the publisher's trademarks outside the `/srd` attribution page.
- No voice guide exists; copy voice is undecided and must not be invented.
- Visual values and rules live in `DESIGN.md`, the sole visual authority.

## Evidence on Hand

- The public `/demo` live 5E sheet (an invented level 3 Cleric) is the product's proof.
- Brand assets: `web/public/icons/` (favicon, PWA icons), `web/public/images/hero-lava-*.webp`, Geist font in `web/public/fonts/geist/`.
- No testimonials, usage data, customer logos or published benchmarks exist; none may be invented.

## Product Principles

1. **Trust at the table beats breadth.** A Sheet that derives every number for one system outranks shallow support for many.
2. **The phone in one hand is the design target.** Every play surface must work one-handed on a small screen.
3. **Never lose the player's evening.** Drafts survive crashes; changes are previewed before they are saved.
4. **Proof before sign-up.** The live demo carries the pitch; claims stay factual.
5. **Calm during play.** No ads, interruptions or upsells on play surfaces.

## Accessibility & Inclusion

AA contrast in both light and dark themes, a visible focus ring on every focusable control, and a keyboard-only path through every surface (contract in `DESIGN.md`). Accessibility beyond that baseline is undecided.
