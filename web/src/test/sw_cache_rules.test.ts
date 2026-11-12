import { describe, expect, it } from 'vitest';
import {
  isAuthPath,
  isRuntimeCacheable,
  LEGACY_RUNTIME_CACHE_NAMES,
  RUNTIME_CACHE_NAME,
} from '../pwa/cache_rules';

const APP_ORIGIN = 'https://app.legnaris.test';

const STATIC_GET = {
  url: `${APP_ORIGIN}/images/hero-lava-1920.webp`,
  method: 'GET',
  hasAuthorization: false,
  isNavigation: false,
};

describe('service-worker cache exclusion rules (FND-09)', () => {
  const cases: ReadonlyArray<{
    name: string;
    request: typeof STATIC_GET;
    expected: boolean;
  }> = [
    {
      name: 'Happy path: a same-origin static GET is cacheable',
      request: STATIC_GET,
      expected: true,
    },
    {
      name: 'Happy path: a relative same-origin path is cacheable',
      request: { ...STATIC_GET, url: '/assets/index-abc.js' },
      expected: true,
    },
    {
      name: 'Edge: the API sheet list on another origin is never cacheable',
      request: { ...STATIC_GET, url: 'http://localhost:8080/sheets' },
      expected: false,
    },
    {
      name: 'Edge: the public template catalog on the API origin is never cacheable',
      request: { ...STATIC_GET, url: 'https://api.legnaris.test/templates' },
      expected: false,
    },
    {
      name: 'Edge: same host on another port is another origin',
      request: { ...STATIC_GET, url: 'https://app.legnaris.test:8443/sheets' },
      expected: false,
    },
    {
      name: 'Edge: the runtime config is read fresh',
      request: { ...STATIC_GET, url: '/config.json' },
      expected: false,
    },
    {
      name: 'Edge: /auth/login is never cacheable',
      request: { ...STATIC_GET, url: '/auth/login' },
      expected: false,
    },
    {
      name: 'Edge: /auth/refresh is never cacheable',
      request: { ...STATIC_GET, url: '/auth/refresh' },
      expected: false,
    },
    {
      name: 'Edge: a request carrying an authorization header is never cacheable',
      request: { ...STATIC_GET, hasAuthorization: true },
      expected: false,
    },
    {
      name: 'Edge: POST is never cacheable',
      request: { ...STATIC_GET, method: 'POST' },
      expected: false,
    },
    {
      name: 'Edge: PATCH is never cacheable',
      request: { ...STATIC_GET, method: 'PATCH' },
      expected: false,
    },
    {
      name: 'Edge: navigations are handled by the shell route, not the runtime cache',
      request: { ...STATIC_GET, isNavigation: true },
      expected: false,
    },
    {
      name: 'Edge: an unparseable URL is never cacheable',
      request: { ...STATIC_GET, url: 'http://[::1' },
      expected: false,
    },
  ];

  for (const tc of cases) {
    it(tc.name, () => {
      expect(isRuntimeCacheable(tc.request, APP_ORIGIN)).toBe(tc.expected);
    });
  }

  it('Edge: the /auth prefix match is exact — a longer path is not excluded', () => {
    expect(isAuthPath('/auth')).toBe(true);
    expect(isAuthPath('/auth/')).toBe(true);
    expect(isAuthPath('/authenticate')).toBe(false);
    expect(isAuthPath('/sheets')).toBe(false);
  });

  it('Edge: the legacy cache that held API responses is not the current cache', () => {
    expect(LEGACY_RUNTIME_CACHE_NAMES).toContain('pwa-runtime-safe');
    expect(LEGACY_RUNTIME_CACHE_NAMES).not.toContain(RUNTIME_CACHE_NAME);
  });
});
