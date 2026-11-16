export const AUTH_PATH_PREFIX = '/auth';

export const RUNTIME_CONFIG_PATH = '/config.json';

export const RUNTIME_CACHE_NAME = 'pwa-runtime-static';

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
