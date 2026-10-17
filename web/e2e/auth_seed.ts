import type { Page } from '@playwright/test';

/**
 * Seeds the silent refresh the auth gate performs on cold start, so a gated
 * surface renders in the browser specs (WAS-03, WAS-08).
 *
 * The built client points at the preview origin (`VITE_API_BASE_URL`), so the
 * refresh request is answered locally rather than by a real cookie session.
 */
export async function seedSession(page: Page): Promise<void> {
  await page.route('**/auth/refresh', (route) =>
    route.fulfill({
      json: { data: { access_token: 'e2e-access-token', token_type: 'Bearer', expires_in: 900 } },
    }),
  );
}
