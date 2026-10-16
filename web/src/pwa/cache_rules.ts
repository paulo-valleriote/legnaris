/**
 * Cache-exclusion rules shared by the service worker and its unit test
 * (FND-09). Authenticated requests — `/auth/*` or anything carrying an
 * `Authorization` header — must never be cached or served from cache.
 */

export const AUTH_PATH_PREFIX = '/auth';

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
 * A request is runtime-cacheable only when it is a non-navigation GET with no
 * authorization header and a non-auth path.
 */
export function isRuntimeCacheable(request: CacheableRequest): boolean {
  return (
    request.method === 'GET' &&
    !request.isNavigation &&
    !request.hasAuthorization &&
    !isAuthPath(request.url)
  );
}
