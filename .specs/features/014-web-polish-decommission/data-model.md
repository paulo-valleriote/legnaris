# Data Model: 014-web-polish-decommission (F10)

**No new persisted entities.** F10 is closure / enforcement / removal.

## Existing entities used (unchanged schema)

- **Template / Sheet / FieldSchema** (existing MongoDB collections) — not modified.
- **CSP Report (normalized)** — ephemeral payload, not persisted; normalized to struct `{effective_directive, disposition, document_uri_sanitized, blocked_uri_sanitized, original_policy, status_code}`; emitted to Loki + Prometheus only.
- **Rate Limit Counter** — shared in-memory / distributed limiter (existing mechanism); key = `IP[:identity]`; no persistence required.
- **Reference Render (desktop)** — not a data entity; screenshot artifact for Playwright comparison.

## Contracts / interfaces (to produce)

- `cookie-auth.contract.md`: cookie attributes (`__Host-`, `HttpOnly`, `Secure`, `SameSite=Strict`); rotation; family revocation; absence of `X-Refresh-Token`.
- `csp-enforce.contract.md`: header content; report endpoint behavior; rate-limit integration; sanitize rules.
- `parity-gate.contract.md`: Playwright comparison criteria; pass/fail thresholds; removal authorization.
- `desktop-removal.contract.md`: file set; Makefile / CI adjustments; constitution update.
