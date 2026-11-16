import type { Page } from '@playwright/test';

export async function seedSession(page: Page): Promise<void> {
  await page.route('**/auth/refresh', (route) =>
    route.fulfill({
      json: { data: { access_token: 'e2e-access-token', token_type: 'Bearer', expires_in: 900 } },
    }),
  );
}
