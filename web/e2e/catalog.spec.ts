import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { seedSession } from './auth_seed';

// The service worker is irrelevant to the accessibility measurement and would
// sit between the page and the intercepted catalog request; its offline-ready
// notice would also race the screenshot of the catalog.
test.use({ serviceWorkers: 'block' });

const CATALOG = [
  {
    id: 'dnd5e',
    name: 'Dungeons & Dragons 5e',
    default: true,
    fields: {},
  },
  {
    id: 'pbta',
    name: 'Powered by the Apocalypse',
    default: false,
    fields: {},
  },
];

const DETAIL = {
  id: 'dnd5e',
  name: 'Dungeons & Dragons 5e',
  default: true,
  fields: {},
};

function criticalViolations(results: Awaited<ReturnType<AxeBuilder['analyze']>>) {
  return results.violations.filter((violation) => violation.impact === 'critical');
}

test.describe('catalog accessibility (SC-005, AD-014)', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await page.route('**/templates/*', (route) => route.fulfill({ json: { data: DETAIL } }));
    await page.route('**/templates', (route) => route.fulfill({ json: { data: CATALOG } }));
  });

  test('the catalog has no critical accessibility violations', async ({ page }) => {
    await page.goto('/');

    await expect(page.getByRole('button', { name: /Dungeons & Dragons 5e/ })).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(criticalViolations(results)).toEqual([]);
    await expect(page).toHaveScreenshot('catalog.png');
  });

  test('the creation surface has no critical accessibility violations', async ({ page }) => {
    await page.goto('/');

    await page.getByRole('button', { name: /Dungeons & Dragons 5e/ }).click();
    await expect(page.getByRole('dialog')).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Sheet name' })).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(criticalViolations(results)).toEqual([]);
  });
});
