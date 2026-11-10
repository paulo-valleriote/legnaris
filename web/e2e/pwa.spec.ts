import { expect, test } from '@playwright/test';
import { seedSession } from './auth_seed';

test.describe('installable, offline client', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
  });

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

  test('Happy path: serves the favicon and apple-touch links with the right content type', async ({
    page,
  }) => {
    await page.goto('/');
    const hrefs = await page.evaluate(() =>
      [
        ...document.querySelectorAll<HTMLLinkElement>(
          'link[rel="icon"], link[rel="apple-touch-icon"]',
        ),
      ].map((link) => ({ rel: link.rel, href: link.getAttribute('href') ?? '' })),
    );
    expect(hrefs.some((link) => link.href.endsWith('.svg'))).toBe(true);
    expect(hrefs.some((link) => link.rel === 'apple-touch-icon')).toBe(true);

    for (const link of hrefs) {
      const response = await page.request.get(link.href);
      expect(response.status(), link.href).toBe(200);
      const contentType = response.headers()['content-type'] ?? '';
      if (link.href.endsWith('.svg')) {
        expect(contentType).toContain('svg');
      } else if (link.href.endsWith('.ico')) {
        expect(contentType).toMatch(/icon|octet-stream/);
      } else {
        expect(contentType).toContain('image/png');
      }
    }
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
