# Contract: Cookie Auth (F10 / POL-03 / POL-04)

- Refresh token delivered ONLY via `Set-Cookie: __Host-legnaris_refresh=<opaque>; HttpOnly; Secure; SameSite=Strict; Path=/; Max-Age=<AUTH_REFRESH_TTL>`.
- `POST /auth/refresh` and `/auth/logout` read refresh from cookie ONLY (`credentials: 'include'`); header `X-Refresh-Token` no longer accepted.
- Access token lives in memory only (no `localStorage` / `sessionStorage`); sent as `Authorization: Bearer`.
- Rotation: new refresh cookie + new access token on refresh; old family revoked on reuse detection.
- CSRF defense: `SameSite=Strict` + double-submit token (`__Host-legnaris_csrf`, non-httpOnly, read by JS, sent as `X-CSRF-Token`) + `Origin`/`Referer` validation; only on `/auth/refresh` / `/auth/logout`.
