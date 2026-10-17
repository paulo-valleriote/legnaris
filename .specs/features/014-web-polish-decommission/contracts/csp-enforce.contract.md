# Contract: CSP Enforce (F10 / POL-02)

- Header delivered by origin `app.legnaris.com.br`: `Content-Security-Policy` (enforce, not Report-Only only).
- Policy: `default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self' https://api.legnaris.com.br; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; require-trusted-types-for 'script';` plus `report-uri https://api.legnaris.com.br/csp-report`.
- Report endpoint (`POST /csp-report`, no auth): sanitize URLs (strip query/hash); normalize payloads (`application/csp-report` and `application/reports+json`); cap body; respond 204; structured Loki log; Prometheus metric with labels (`effective-directive`, `disposition`); rate-limited (shared limiter).
