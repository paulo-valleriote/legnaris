/**
 * Cache-exclusion rules shared by the service worker and its unit test
 * (FND-09). The API authenticates with a session cookie, so every API response
 * is caller-scoped: only same-origin static assets of the shell may ever be
 * runtime-cached, and `/auth/*`, `/config.json` or anything carrying an
 * `Authorization` header never is.
 */

export const AUTH_PATH_PREFIX = '/auth';

/** The runtime configuration, read fresh so a changed API address applies on the next load. */
export const RUNTIME_CONFIG_PATH = '/config.json';

/** The current runtime cache; any other `pwa-runtime-*` cache is a stale generation. */
export const RUNTIME_CACHE_NAME = 'pwa-runtime-static';

/**
 * The cache an earlier worker filled with cross-origin API responses, including
 * caller-scoped sheets. It must be deleted on activation, never read.
 */
export const LEGACY_RUNTIME_CACHE_NAMES: readonly string[] = ['pwa-runtime-safe'];

export function isAuthPath(url: string): boolean {
  let pathname: string;
  try {
    pathname = new URL(url, 'https://client.invalid').pathname;
  } catch {
    return false;
  }
  return pathname === AUTH_PATH_PREFIX || pathname.startsWith(`${AUTH_PATH_PREFIX}/`);
}

export interface CacheableRequest {
  readonly url: string;
  readonly method: string;
  readonly hasAuthorization: boolean;
  readonly isNavigation: boolean;
}

/**
 * A request is runtime-cacheable only when it is a non-navigation GET to [appOrigin]
 * with no authorization header, outside `/auth` and other than the runtime config.
 * A cross-origin request (the API) is never cacheable: serving a cached
 * `GET /sheets` would resurrect a deleted sheet.
 */
export function isRuntimeCacheable(request: CacheableRequest, appOrigin: string): boolean {
  let url: URL;
  try {
    url = new URL(request.url, appOrigin);
  } catch {
    return false;
  }
  return (
    request.method === 'GET' &&
    !request.isNavigation &&
    !request.hasAuthorization &&
    url.origin === appOrigin &&
    url.pathname !== RUNTIME_CONFIG_PATH &&
    !isAuthPath(url.href)
  );
}
