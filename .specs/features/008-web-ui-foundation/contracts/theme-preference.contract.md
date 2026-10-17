# Contract: Theme Preference

**Applies to**: the theme store (`web/src/theme/theme.ts`), its hook, the navigation-footer control,
the pre-paint head script (`web/public/theme-init.js`) and `index.html`.
**Requirements**: UIF-03, UIF-17 … UIF-23, UIF-27.

## Storage

| Fact | Value |
|---|---|
| Key | `lg.theme` |
| Accepted values | `light`, `dark` — exactly these two |
| Kind | a display preference, **not** a credential: it may live in `localStorage` |
| Forbidden | any token, cookie value or credential; the refresh cookie and the in-memory access token are untouched |

## Resolution order

```text
read localStorage['lg.theme']
  1. the read throws ───────────────► prefers-color-scheme  (UIF-21)
  2. absent ────────────────────────► prefers-color-scheme  (UIF-17)
  3. not `light` and not `dark` ────► prefers-color-scheme  (UIF-22)
  4. `light` or `dark` ─────────────► that theme wins       (UIF-19)
```

With no valid stored value the resolved theme follows the system preference **live**: a change in
`prefers-color-scheme` re-applies without a reload (`UIF-23`). With a valid stored value the system
preference is ignored.

## Application

- The resolved theme is written to `document.documentElement.dataset.theme`
  (`light` → the attribute is absent or `light`; `dark` → `dark`).
- The write happens **before the first paint**: `theme-init.js` is a classic, same-origin script in
  the document head, so `script-src 'self'` already permits it and no inline script or policy hash
  exists (`UIF-20`).
- `index.html`'s `theme-color` meta is conditioned on `prefers-color-scheme` for both schemes
  (`UIF-03`).

## Control

| Fact | Value |
|---|---|
| Location | the navigation footer, beside the session control (`UIF-27`) |
| Semantics | a two-state light/dark toggle; no third "system" state (`R6`) |
| Accessible name | resolves through the copy catalog, never a literal (`UIF-18`), and names the state it switches to |
| State | exposed to assistive technology (`aria-pressed`, per the pressed state) |
| Keyboard | reachable in the normal tab order of the navigation (`UIF-18`) |

## Failure handling

- A storage read or write that throws MUST NOT surface: the interface keeps the last resolved theme
  and the app does not error (`UIF-21`).
- A value outside the accepted set MUST be ignored, not rendered and not thrown on (`UIF-22`).

## Verification

| Check | Method |
|---|---|
| Default follows the system preference | `theme_toggle.test.tsx` with a simulated `prefers-color-scheme` |
| Live system follow with nothing stored | change the simulated preference and assert the applied theme |
| Persistence | activate the control, assert `localStorage['lg.theme']` and the applied attribute |
| Stored value wins | seed a value, assert it applies despite a differing system preference |
| Unknown value falls back | seed `purple`, assert the system preference applies |
| Throwing storage | inject a throwing store, assert no throw surfaces and the fallback applies |
| No flash | head script sets the attribute before first paint; browser spec asserts the first painted theme |
| Control state and name | rendering test over the accessible name/state and the tab order |
