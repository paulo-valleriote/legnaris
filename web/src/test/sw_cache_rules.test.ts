import { describe, expect, it } from 'vitest';
import { isAuthPath, isRuntimeCacheable } from '../pwa/cache_rules';

const PUBLIC_GET = {
  url: 'https://www.api-legnaris.paulovalleriote.io/templates',
  method: 'GET',
  hasAuthorization: false,
  isNavigation: false,
};

describe('service-worker cache exclusion rules (FND-09)', () => {
  it('Happy path: a public GET is runtime-cacheable', () => {
    expect(isRuntimeCacheable(PUBLIC_GET)).toBe(true);
  });

  it('Edge: /auth/* requests are never cacheable', () => {
    expect(isRuntimeCacheable({ ...PUBLIC_GET, url: '/auth/login' })).toBe(false);
    expect(isRuntimeCacheable({ ...PUBLIC_GET, url: '/auth/refresh' })).toBe(false);
  });

  it('Edge: a request carrying an authorization header is never cacheable', () => {
    expect(isRuntimeCacheable({ ...PUBLIC_GET, hasAuthorization: true })).toBe(false);
  });

  it('Edge: non-GET requests are never cacheable', () => {
    expect(isRuntimeCacheable({ ...PUBLIC_GET, method: 'POST' })).toBe(false);
    expect(isRuntimeCacheable({ ...PUBLIC_GET, method: 'PATCH' })).toBe(false);
  });

  it('Edge: navigations are handled by the shell route, not the runtime cache', () => {
    expect(isRuntimeCacheable({ ...PUBLIC_GET, isNavigation: true })).toBe(false);
  });

  it('Edge: the /auth prefix match is exact — a longer path is not excluded', () => {
    expect(isAuthPath('/auth')).toBe(true);
    expect(isAuthPath('/auth/')).toBe(true);
    expect(isAuthPath('/authenticate')).toBe(false);
    expect(isAuthPath('/sheets')).toBe(false);
  });
});
