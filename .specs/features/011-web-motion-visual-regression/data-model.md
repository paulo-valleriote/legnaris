# Data Model (F4 — Motion / Polish / Visual Regression)

No new storage entities. F4 touches only presentation-layer tokens and visual artifacts.

## Entities (same as F3; unchanged)

- Template (`id`, `name`, `schema`)
- Sheet (`id`, `template_id`, `name`, `data`, `updated_at`)
- Auth State (`isAuthenticated`, `accessToken`, `user`)
- FieldSchema (`key`, `type`, `label`, `kind`, `options`, `required`, etc.)

## New / Modified Artifacts

### Motion Token (presentation only)

Declared in `src/theme/tokens.css`, mirrored in `tokens.ts`. Not a server-side entity.

| Token | Light value | Dark value | Role |
|---|---|---|---|
| `--lg-motion-120` | `120ms` | `120ms` | short transitions |
| `--lg-motion-180` | `180ms` | `180ms` | medium transitions |
| `--lg-motion-240` | `240ms` | `240ms` | long transitions |
| `--lg-ease-standard` | `cubic-bezier(0.4, 0, 0.2, 1)` | same | standard easing |
| `--lg-ease-emphasized` | `cubic-bezier(0.0, 0, 0.2, 1)` | same | emphasized easing |

### Visual Regression Baseline (presentation only, committed)

Stored under `web/e2e/baselines/<theme>/<screen>.png`; each is a PNG comparison artifact.
Not accessed by the server or client at runtime; used only by Playwright.

### No data-flow changes

- Catalog, Sheet, Auth, States screens use the same API contracts (`GET /templates`, `GET /sheets/:id`, `POST /auth/login`, etc.).
- No new fields, no new endpoints, no new storage.
- No links to `FieldSchema` or `Sheet` at this layer.

## Relationships

No new relationships. The F4 layer depends only on:
- `src/theme/` (tokens, base.css, fonts)
- `src/ui/` (primitives — existing from F2; motion styles applied to them if needed)
- `web/e2e/` (Playwright baselines, screenshots, gate)

The screen entities (`Template`, `Sheet`, `Auth`) remain unchanged; only their visual output changes.