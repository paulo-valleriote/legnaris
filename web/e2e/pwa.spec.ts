import { expect, test } from '@playwright/test';

test.describe('installable, offline client', () => {
  test('declares a valid web app manifest', async ({ page }) => {
    await page.goto('/');
    const href = await page.getAttribute('link[rel="manifest"]', 'href');
    expect(href).not.toBeNull();

    const manifest = await page.evaluate(async (url) => {
      const response = await fetch(url as string);
      return response.json() as Promise<Record<string, unknown>>;
    }, href as string);

    expect(manifest.name).toBe('Legnaris');
    expect(manifest.display).toBe('standalone');
    expect(manifest.start_url).toBe('/');
  });

  test('registers an external service worker that controls the page', async ({ page }) => {
    await page.goto('/');

    await page.evaluate(() => navigator.serviceWorker.ready);
    await page.reload();
    await expect
      .poll(() => page.evaluate(() => Boolean(navigator.serviceWorker.controller)))
      .toBe(true);
  });

  test('renders the shell offline after a first load', async ({ page, context }) => {
    await page.goto('/');
    await page.evaluate(() => navigator.serviceWorker.ready);
    await page.reload();

    await context.setOffline(true);
    await page.reload();

    await expect(page.getByRole('navigation', { name: 'Primary' })).toBeVisible();
    await expect(page.getByRole('main')).toBeVisible();
    await context.setOffline(false);
  });

  test('serves a declared route offline', async ({ page, context }) => {
    await page.goto('/');
    await page.evaluate(() => navigator.serviceWorker.ready);

    await context.setOffline(true);
    await page.goto('/');

    await expect(page.getByRole('main')).toBeVisible();
    await context.setOffline(false);
  });
});
