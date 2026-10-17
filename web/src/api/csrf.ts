/**
 * The CSRF cookie reader (WAS-18, R5).
 *
 * The refresh cookie is `HttpOnly` and never readable by script; the CSRF cookie
 * is deliberately script-readable so the page can echo its value in
 * `X-CSRF-Token` on the state-changing auth routes. This parser is total: an
 * absent, empty or malformed cookie string yields `undefined`, never a throw,
 * and the value is read per request so a rotation is picked up automatically.
 */

/** The contract-fixed, script-readable CSRF cookie name (WAS-18). */
export const CSRF_COOKIE_NAME = '__Host-legnaris_csrf';

/**
 * Resolves the CSRF token from a `document.cookie`-style string, or `undefined`.
 *
 * Only the named cookie is considered; the returned value is percent-decoded
 * when it is a valid escape sequence and otherwise returned verbatim. No
 * credential is stored, logged or placed in a URL (WAS-18, WAS-32).
 */
export function readCsrfToken(cookieString: string): string | undefined {
  if (typeof cookieString !== 'string' || cookieString === '') {
    return undefined;
  }

  for (const pair of cookieString.split(';')) {
    const separator = pair.indexOf('=');

    if (separator === -1) {
      continue;
    }

    const name = pair.slice(0, separator).trim();

    if (name !== CSRF_COOKIE_NAME) {
      continue;
    }

    const raw = pair.slice(separator + 1).trim();

    if (raw === '') {
      return undefined;
    }

    try {
      return decodeURIComponent(raw);
    } catch {
      return raw;
    }
  }

  return undefined;
}
