import { expect, test } from '@playwright/test';
import { seedSession } from './auth_seed';

test.describe('theme preference', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
  });

  test('the stored theme is applied before the interface paints', async ({ page }) => {
    await page.addInitScript(() => window.localStorage.setItem('lg.theme', 'dark'));
    // Block the app bundle: only the same-origin head script can have set the theme.
    await page.route('**/assets/*.js', (route) => route.abort());

    await page.goto('/', { waitUntil: 'commit' });

    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  });

  test('the head script is served same-origin and the document has no inline script', async ({
    page,
  }) => {
    await page.goto('/');

    await expect(page.locator('script[src="/theme-init.js"]')).toHaveCount(1);
    await expect(page.locator('script:not([src])')).toHaveCount(0);
  });
});
