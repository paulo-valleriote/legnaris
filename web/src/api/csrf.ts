export const CSRF_COOKIE_NAME = '__Host-legnaris_csrf';

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
