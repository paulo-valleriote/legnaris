import { expect, test } from '@playwright/test';
import { seedSession } from './auth_seed';
import { SHEET, SHEET_ID, TEMPLATE } from './fantasy_play';

test.use({ serviceWorkers: 'block' });

const TAP_TARGET = 44;
const COMPACT_ROW = 40;
const PAGE_HEIGHT_BEFORE = 2750;

test.describe('Play tap targets on a phone', () => {
  test.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await seedSession(page);
    await page.route('**/templates/*', (route) => route.fulfill({ json: { data: TEMPLATE } }));
    await page.route('**/templates', (route) => route.fulfill({ json: { data: [TEMPLATE] } }));
    await page.route('**/sheets/*', (route) =>
      route.request().resourceType() === 'document'
        ? route.fallback()
        : route.fulfill({ json: { data: SHEET } }),
    );
    await page.goto(`/sheets/${SHEET_ID}`);
    await expect(page.getByTestId('sheet-play')).toBeVisible();
  });

  test('measures one tap target of each kind', async ({ page }) => {
    const save = page.getByTestId('sheet-save-strength');
    const skill = page.getByTestId('sheet-skill-athletics');
    const attack = page.getByTestId('sheet-attack-1');
    const slots = page.getByTestId('sheet-slot-1');

    for (const target of [
      save.locator('.lg-prof-dot'),
      save.locator('.lg-save__total'),
      skill.locator('.lg-prof-dot'),
      skill.locator('.lg-skill__total'),
      attack.locator('.lg-attack-card__roll').first(),
      attack.locator('.lg-attack-card__roll').last(),
      attack.locator('.lg-attack-card__edit').first(),
      slots.locator('.lg-spells__pip').first(),
      page.getByTestId('sheet-initiative').locator('.lg-combat-stat__edit'),
    ]) {
      await expect(target).toBeVisible();
      const box = await target.boundingBox();
      expect(box).not.toBeNull();
      expect(box!.width).toBeGreaterThanOrEqual(TAP_TARGET);
      expect(box!.height).toBeGreaterThanOrEqual(TAP_TARGET);
    }

    await page.getByTestId('sheet-ac-shield').click();
    const armor = page.getByTestId('sheet-ac-breakdown').locator('.lg-ac-breakdown__base');
    await expect(armor).toBeVisible();
    const armorBox = await armor.boundingBox();
    expect(armorBox!.width).toBeGreaterThanOrEqual(TAP_TARGET);
    expect(armorBox!.height).toBeGreaterThanOrEqual(TAP_TARGET);
  });

  test('keeps the save and skill rows compact and their targets apart', async ({ page }) => {
    const rows = page.locator('.lg-save, .lg-skill');
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);
    for (let index = 0; index < count; index += 1) {
      const box = await rows.nth(index).boundingBox();
      expect(box).not.toBeNull();
      expect(box!.height).toBeLessThan(COMPACT_ROW);
    }

    const row = page.getByTestId('sheet-save-strength');
    const dot = (await row.locator('.lg-prof-dot').boundingBox())!;
    const total = (await row.locator('.lg-save__total').boundingBox())!;
    expect(dot.x + dot.width).toBeLessThanOrEqual(total.x);
  });

  test('shortens the page at 390', async ({ page }) => {
    const height = await page.evaluate(() => document.documentElement.scrollHeight);
    expect(height).toBeLessThan(PAGE_HEIGHT_BEFORE);
  });
});
