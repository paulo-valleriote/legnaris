import { describe, it, expect } from 'vitest';
import swSource from '../pwa/sw.ts?raw';

describe('PWA-01 SW contract', () => {
  it('should define a precache filter that excludes /auth/ and /csp-report', () => {
    expect(swSource).toContain('precacheAndRoute');
    expect(swSource).toContain('/auth/');
    expect(swSource).not.toContain('dangerouslySetInnerHTML');
    expect(swSource).not.toContain('localStorage');
  });

  it('Edge: never activates a waiting worker mid-session', () => {
    expect(swSource).not.toContain('skipWaiting');
  });

  it('Edge: routes runtime caching through the shared same-origin rule', () => {
    expect(swSource).toContain('isRuntimeCacheable(');
    expect(swSource).toContain('self.location.origin');
  });

  it('Edge: deletes the legacy runtime cache that held API responses on activation', () => {
    expect(swSource).toContain('LEGACY_RUNTIME_CACHE_NAMES');
    expect(swSource).toContain('caches.delete');
  });
});
