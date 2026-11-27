import { expect, test, type Locator, type Page } from '@playwright/test';
import { seedSession } from './auth_seed';
import { FIFTH_SHEET, FIFTH_SHEET_ID, FIFTH_TEMPLATE } from './fifth_play';

test.use({ serviceWorkers: 'block' });

test.beforeEach(async ({ page }) => {
  await seedSession(page);
  await page.route('**/templates/*', (route) => route.fulfill({ json: { data: FIFTH_TEMPLATE } }));
  await page.route('**/templates', (route) => route.fulfill({ json: { data: [FIFTH_TEMPLATE] } }));
  await page.route('**/sheets/*', (route) =>
    route.request().resourceType() === 'document'
      ? route.fallback()
      : route.fulfill({ json: { data: FIFTH_SHEET } }),
  );
  await page.goto(`/sheets/${FIFTH_SHEET_ID}`);
  await expect(page.getByTestId('sheet-play')).toBeVisible();
});

async function openCharacterDetails(page: Page) {
  await page.locator('[data-preset-section="identity"] summary').click();
}

function fieldStyle(locator: Locator) {
  return locator.evaluate((element) => {
    const style = getComputedStyle(element);
    return {
      background: style.backgroundColor,
      border: style.borderTopColor,
      cursor: style.cursor,
    };
  });
}

test('a locked build value reads without the edit-mode box', async ({ page }) => {
  await openCharacterDetails(page);
  const input = page.locator('[data-field-path="identity.level"] input');

  const style = await fieldStyle(input);
  expect(style.background).toBe('rgba(0, 0, 0, 0)');
  expect(style.border).toBe('rgba(0, 0, 0, 0)');
  expect(style.cursor).toBe('default');
});

test('the same value draws the edit box once editing starts', async ({ page }) => {
  await openCharacterDetails(page);
  const input = page.locator('[data-field-path="identity.level"] input');
  const locked = await fieldStyle(input);

  await page.getByRole('button', { name: 'Edit sheet' }).click();
  const editing = await fieldStyle(input);

  expect(editing.border).not.toBe(locked.border);
  expect(editing.background).not.toBe(locked.background);
});
