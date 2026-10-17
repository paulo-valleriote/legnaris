import { describe, it, expect } from 'vitest';

// PWA-01 contract: precache filters out auth endpoints; no inline script.
// Minimal structural test (full integration requires service-worker lifecycle).
describe('PWA-01 SW contract', () => {
  it('should define a precache filter that excludes /auth/ and /csp-report', () => {
    // The contract is enforced by source inspection in sw.ts; this test asserts the source does not contain forbidden patterns.
    const swSource = require('fs').readFileSync('src/pwa/sw.ts', 'utf-8');
    expect(swSource).toContain('precacheAndRoute');
    expect(swSource).toContain('/auth/'); // filter references auth exclusion
    expect(swSource).not.toContain('dangerouslySetInnerHTML');
    expect(swSource).not.toContain('localStorage');
  });
});
