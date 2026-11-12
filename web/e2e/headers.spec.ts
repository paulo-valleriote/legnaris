import { expect, test } from '@playwright/test';
import { PRODUCTION_API_ORIGIN } from '../config/security_headers';

// POL-02 / F9-T010: the document headers the origin must deliver. In production
// Traefik emits them (`deploy/traefik/dynamic.yml` `web-security-headers`); the
// dev and preview servers mirror the same list from `vite.config.ts`, so this
// gate fails the moment the policy disappears from either origin. It replaced
// the vacuous `src/test/headers.spec.ts`, which asserted nothing about a
// response (D3).
test.use({ serviceWorkers: 'block' });

/** literal matches the exact text, so the origin stays sourced from the config. */
function literal(text: string): RegExp {
  return new RegExp(text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
}

const REQUIRED_HEADERS: ReadonlyArray<readonly [name: string, pattern: RegExp]> = [
  ['content-security-policy', /default-src 'self'/],
  ['content-security-policy', /frame-ancestors 'none'/],
  ['content-security-policy', /require-trusted-types-for 'script'/],
  ['reporting-endpoints', literal(`csp-endpoint="${PRODUCTION_API_ORIGIN}/csp-report"`)],
  ['strict-transport-security', /max-age=63072000/],
  ['x-frame-options', /^DENY$/],
  ['x-content-type-options', /^nosniff$/],
  ['referrer-policy', /^strict-origin-when-cross-origin$/],
  ['permissions-policy', /camera=\(\)/],
  ['cross-origin-opener-policy', /^same-origin$/],
  ['cross-origin-resource-policy', /^same-origin$/],
];

test.describe('document security headers (POL-02)', () => {
  test('Happy path: the served document carries the enforced policy and its companions', async ({
    page,
  }) => {
    const response = await page.goto('/');
    expect(response, 'the document response must be observable').not.toBeNull();

    const headers = response!.headers();
    for (const [name, pattern] of REQUIRED_HEADERS) {
      expect(headers[name], `missing ${name}`).toBeDefined();
      expect(headers[name], `${name} does not match ${pattern}`).toMatch(pattern);
    }
  });

  test('Edge: the policy is enforced, never report-only', async ({ page }) => {
    const response = await page.goto('/');

    expect(response!.headers()['content-security-policy-report-only']).toBeUndefined();
  });
});
