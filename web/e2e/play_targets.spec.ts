import { expect, test } from '@playwright/test';
import { seedSession } from './auth_seed';
import { FIFTH_SHEET, FIFTH_SHEET_ID, FIFTH_TEMPLATE } from './fifth_play';

test.use({ serviceWorkers: 'block' });

const TAP_TARGET = 44;

test.describe('preset tap targets on a phone', () => {
  test.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await seedSession(page);
    await page.route('**/templates/*', (route) =>
      route.fulfill({ json: { data: FIFTH_TEMPLATE } }),
    );
    await page.route('**/templates', (route) =>
      route.fulfill({ json: { data: [FIFTH_TEMPLATE] } }),
    );
    await page.route('**/sheets/*', (route) =>
      route.request().resourceType() === 'document'
        ? route.fallback()
        : route.fulfill({ json: { data: FIFTH_SHEET } }),
    );
    await page.goto(`/sheets/${FIFTH_SHEET_ID}`);
    await expect(page.getByTestId('sheet-play')).toBeVisible();
  });

  test('measures one tap target of each preset control', async ({ page }) => {
    await page.getByRole('button', { name: 'Edit sheet' }).click();
    const targets = [
      page.locator('.lg-compose-flag').first(),
      page.locator('.lg-compose-rank').first(),
      page.locator('.lg-inline-edit').first(),
      page.locator('.lg-compose-record__move').first(),
    ];

    for (const target of targets) {
      await expect(target).toBeVisible();
      const box = await target.boundingBox();
      expect(box).not.toBeNull();
      expect(box!.width).toBeGreaterThanOrEqual(TAP_TARGET);
      expect(box!.height).toBeGreaterThanOrEqual(TAP_TARGET);
    }
  });

  test('measures a condition chip once the list is open', async ({ page }) => {
    await page.getByTestId('compose-conditions-toggle').click();

    const condition = page.locator('.lg-compose-condition').first();
    await expect(condition).toBeVisible();
    const box = (await condition.boundingBox())!;
    expect(box.height).toBeGreaterThanOrEqual(TAP_TARGET);
  });

  test('keeps a pip target inside its row and clear of its neighbour', async ({ page }) => {
    const pip = page.locator('.lg-compose-pip').first();
    await expect(pip).toBeVisible();
    const box = (await pip.boundingBox())!;
    expect(box.width).toBeGreaterThanOrEqual(28);
    expect(box.height).toBeGreaterThanOrEqual(28);

    const row = page.locator('.lg-compose-pips__row').first();
    const rowBox = (await row.boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(rowBox.x - 0.5);
    expect(box.x + box.width).toBeLessThanOrEqual(rowBox.x + rowBox.width + 0.5);
  });

  test('keeps the phone preset free of horizontal overflow', async ({ page }) => {
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(overflow).toBe(0);
  });
});
