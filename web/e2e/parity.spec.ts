import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { seedSession } from './auth_seed';

// POL-01: the visual parity gate compares the rendered surfaces against the
// desktop reference baselines before the desktop client is decommissioned.
// The service worker would sit between the page and the intercepted requests.
test.use({ serviceWorkers: 'block' });

const TEMPLATE_ID = 'fantasy_v1';
const SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd1d';

const CATALOG = [{ id: TEMPLATE_ID, name: 'Fantasy', default: true, fields: {} }];

const DETAIL = {
  id: TEMPLATE_ID,
  name: 'Fantasy',
  default: true,
  fields: {
    identity: {
      label: 'Identity',
      children: {
        characterName: { schema: { type: 'text', label: 'Name', is_editable: true, default: 0 } },
        level: { schema: { type: 'number', label: 'Level', is_editable: true, default: 1 } },
      },
    },
  },
};

const SHEET = {
  id: SHEET_ID,
  template_id: TEMPLATE_ID,
  name: 'Aragorn',
  data: { identity: { characterName: 'Aragorn', level: 3 } },
  created_at: '2026-10-01T21:38:54.635Z',
  updated_at: '2026-10-02T09:00:00.000Z',
};

function criticalViolations(results: Awaited<ReturnType<AxeBuilder['analyze']>>) {
  return results.violations.filter((violation) => violation.impact === 'critical');
}

test.describe('visual parity (POL-01)', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await page.route('**/templates/*', (route) => route.fulfill({ json: { data: DETAIL } }));
    await page.route('**/templates', (route) => route.fulfill({ json: { data: CATALOG } }));
    await page.route('**/sheets/*/preview', (route) => route.fulfill({ json: { data: {} } }));
    await page.route('**/sheets/*', (route) =>
      route.request().resourceType() === 'document'
        ? route.fallback()
        : route.fulfill({ json: { data: SHEET } }),
    );
  });

  test('the shell matches the reference baseline', async ({ page }) => {
    await page.goto('/');

    await expect(page.getByRole('button', { name: /Fantasy/ })).toBeVisible();

    await expect(page).toHaveScreenshot('parity-shell.png');
  });

  test('the sheet screen matches the reference baseline', async ({ page }) => {
    await page.goto(`/sheets/${SHEET_ID}`);

    await expect(page.getByRole('heading', { name: 'Aragorn' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Level' })).toBeVisible();

    expect(criticalViolations(await new AxeBuilder({ page }).analyze())).toEqual([]);
    await expect(page).toHaveScreenshot('parity-sheet.png');
  });
});
