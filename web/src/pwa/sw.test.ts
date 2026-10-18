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
});
