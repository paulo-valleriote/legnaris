import { expect, test } from '@playwright/test';

test.describe('application shell', () => {
  test('renders navigation, content and panel in order with the empty workspace', async ({
    page,
  }) => {
    await page.goto('/');

    const nav = page.getByRole('navigation', { name: 'Primary' });
    const main = page.getByRole('main');
    const panel = page.getByRole('complementary');

    await expect(nav).toBeVisible();
    await expect(main).toBeVisible();
    await expect(panel).toBeVisible();
    await expect(main.getByText('No workspace open')).toBeVisible();
    await expect(page.getByRole('row')).toHaveCount(0);
  });

  test('collapses the contextual panel first at a narrow viewport', async ({ page }) => {
    await page.setViewportSize({ width: 600, height: 900 });
    await page.goto('/');

    await expect(page.getByRole('complementary')).toBeHidden();
    await expect(page.getByRole('navigation', { name: 'Primary' })).toBeVisible();
    await expect(page.getByRole('main')).toBeVisible();
  });

  test('deep link to the root renders the shell', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('main')).toBeVisible();
    await expect(page.getByText('No workspace open')).toBeVisible();
  });

  test('unknown route renders the designed not-found state, not a blank page', async ({ page }) => {
    await page.goto('/does-not-exist');

    await expect(page.getByRole('heading', { name: 'Page not found' })).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Primary' })).toBeVisible();
  });
});
