import { LANDING_PATH, SIGN_IN_PATH, SIGN_UP_PATH } from '../app/routes';

export const NEXT_PARAM = 'next';

const PROBE_ORIGIN = 'https://legnaris.invalid';
// eslint-disable-next-line no-control-regex
const UNSAFE = /[\u0000-\u001f\u007f\\]/;
const AUTH_PATHS: ReadonlySet<string> = new Set([SIGN_IN_PATH, SIGN_UP_PATH]);

function decodedOnce(value: string): string | null {
  try {
    return decodeURIComponent(value);
  } catch {
    return null;
  }
}

export function sanitizeNextPath(raw: string | null | undefined): string | null {
  if (raw === null || raw === undefined || raw === '') {
    return null;
  }
  const decoded = decodedOnce(raw);
  if (decoded === null) {
    return null;
  }
  for (const candidate of [raw, decoded]) {
    if (!candidate.startsWith('/') || candidate.startsWith('//') || UNSAFE.test(candidate)) {
      return null;
    }
  }
  const resolved = new URL(raw, PROBE_ORIGIN);
  if (resolved.origin !== PROBE_ORIGIN || AUTH_PATHS.has(resolved.pathname)) {
    return null;
  }
  return raw;
}

export function signInPathFor(target: string): string {
  const safe = sanitizeNextPath(target);
  if (safe === null || safe === LANDING_PATH) {
    return SIGN_IN_PATH;
  }
  return `${SIGN_IN_PATH}?${NEXT_PARAM}=${encodeURIComponent(safe)}`;
}

export function isSheetPath(next: string | null): boolean {
  return next !== null && /^\/sheets\/[^/?#]+/.test(next);
}
