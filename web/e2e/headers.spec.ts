import { expect, test } from '@playwright/test';
import { PRODUCTION_API_ORIGIN } from '../config/security_headers';

test.use({ serviceWorkers: 'block' });

function literal(text: string): RegExp {
  return new RegExp(text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
}

const REQUIRED_HEADERS: ReadonlyArray<readonly [name: string, pattern: RegExp]> = [
  ['content-security-policy', /default-src 'self'/],
  ['content-security-policy', /script-src 'self'[^;]*pagead2\.googlesyndication\.com/],
  ['content-security-policy', /frame-src[^;]*googleads\.g\.doubleclick\.net/],
  ['content-security-policy', /object-src 'none'/],
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

test.describe('document security headers', () => {
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
