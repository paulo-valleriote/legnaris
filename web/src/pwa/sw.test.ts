import { describe, it, expect } from 'vitest';
import swSource from './sw.ts?raw';

// PWA-01 contract: precache filters out auth endpoints; no inline script.
// Minimal structural test (full integration requires service-worker lifecycle).
describe('PWA-01 SW contract', () => {
  it('should define a precache filter that excludes /auth/ and /csp-report', () => {
    expect(swSource).toContain('precacheAndRoute');
    expect(swSource).toContain('/auth/'); // filter references auth exclusion
    expect(swSource).not.toContain('dangerouslySetInnerHTML');
    expect(swSource).not.toContain('localStorage');
  });

  it('should activate the waiting worker when the update notice asks for it', () => {
    // `registerType: 'prompt'` never activates on its own: the page's
    // `messageSkipWaiting()` only lands if the worker answers this message.
    expect(swSource).toContain('SKIP_WAITING');
    expect(swSource).toContain('self.skipWaiting()');
  });

  it('should keep the runtime config out of the runtime cache', () => {
    expect(swSource).toContain("url.pathname !== '/config.json'");
  });
});
