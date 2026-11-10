/** The production API origin the CSP and reporting endpoint name (POL-02). */
export const PRODUCTION_API_ORIGIN = 'https://api-legnaris.paulovalleriote.io';

/** The API address a developer runs locally when the build configures none. */
export const DEV_API_ORIGIN = 'http://localhost:8080';

/**
 * Reduces a configured API base address to its origin. An empty, relative or
 * non-http(s) value falls back to the local API, so a typo in a dev env file
 * never widens the policy to an arbitrary scheme.
 */
export function resolveDevApiOrigin(raw: string | undefined): string {
  try {
    const url = new URL((raw ?? '').trim());
    if (url.protocol === 'http:' || url.protocol === 'https:') {
      return url.origin;
    }
  } catch {
    // An unparseable value selects the fallback below.
  }
  return DEV_API_ORIGIN;
}

/**
 * The hash of the one inline script `@vitejs/plugin-react` injects in dev (the Fast Refresh
 * preamble). Without it the dev page stays blank under `script-src 'self'`. It changes with the
 * plugin version: copy the new hash from the browser's CSP violation message. Styles cannot be
 * allowed the same way: the dev server injects each stylesheet as a `<style>` element that
 * changes on every edit, so dev alone allows inline styles.
 */
export const REACT_REFRESH_PREAMBLE_HASH = "'sha256-Z2/iFzh9VMlVkEOar1f/oSHWwQk3ve1qk/C2WdsC4Xk='";

/**
 * Builds the document security headers the origin delivers to the web client.
 * In production they come from Traefik (`deploy/traefik/dynamic.yml`
 * `web-security-headers`); the preview server mirrors them so the header gate
 * asserts the real policy (POL-02, B6). The dev server passes its own API origin
 * so `connect-src` admits the local API; HSTS is dropped because it is ignored
 * over plain HTTP. Keep the production list in step with Traefik — the policy is
 * defined once per origin, never in the HTML.
 */
export function buildSecurityHeaders(apiOrigin: string = PRODUCTION_API_ORIGIN, dev = false) {
  const scriptSrc = dev ? `script-src 'self' ${REACT_REFRESH_PREAMBLE_HASH}` : "script-src 'self'";
  const styleSrc = dev ? "style-src 'self' 'unsafe-inline'" : "style-src 'self'";
  const contentSecurityPolicy = [
    "default-src 'self'",
    scriptSrc,
    styleSrc,
    "img-src 'self' data:",
    "font-src 'self'",
    `connect-src 'self' ${apiOrigin}`,
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "frame-ancestors 'none'",
    "require-trusted-types-for 'script'",
    'report-to csp-endpoint',
    `report-uri ${apiOrigin}/csp-report`,
  ].join('; ');

  const headers: Record<string, string> = {
    'Content-Security-Policy': contentSecurityPolicy,
    'Reporting-Endpoints': `csp-endpoint="${apiOrigin}/csp-report"`,
    'X-Frame-Options': 'DENY',
    'X-Content-Type-Options': 'nosniff',
    'Referrer-Policy': 'strict-origin-when-cross-origin',
    'Permissions-Policy':
      'camera=(), microphone=(), geolocation=(), payment=(), usb=(), fullscreen=(self)',
    'Cross-Origin-Opener-Policy': 'same-origin',
    'Cross-Origin-Resource-Policy': 'same-origin',
  };
  if (!dev) {
    headers['Strict-Transport-Security'] = 'max-age=63072000; includeSubDomains; preload';
  }
  return headers;
}
