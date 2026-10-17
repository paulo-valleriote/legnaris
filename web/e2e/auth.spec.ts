import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { seedSession } from './auth_seed';

// The service worker is irrelevant to the accessibility measurement and would
// sit between the page and the intercepted API requests.
test.use({ serviceWorkers: 'block' });

const TEMPLATE_ID = 'fantasy_v1';
const SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd1d';

const CATALOG = [{ id: TEMPLATE_ID, name: 'Fantasy', default: true, fields: {} }];

const SHEET = {
  id: SHEET_ID,
  template_id: TEMPLATE_ID,
  name: 'Aragorn',
  data: {},
  created_at: '2026-10-01T21:38:54.635Z',
  updated_at: '2026-10-02T09:00:00.000Z',
};

function criticalViolations(results: Awaited<ReturnType<AxeBuilder['analyze']>>) {
  return results.violations.filter((violation) => violation.impact === 'critical');
}

test.describe('auth surface, signed-in shell and own-sheet accessibility (SC-010, AD-014)', () => {
  test('the auth surface has no critical violations and names its controls', async ({ page }) => {
    await page.route('**/auth/refresh', (route) =>
      route.fulfill({
        status: 401,
        json: { error: { code: 'UNAUTHORIZED', message: 'no cookie' } },
      }),
    );
    await page.goto('/');

    await expect(page.getByRole('heading', { name: 'Sign in to Legnaris' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Email' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Sign in' })).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(criticalViolations(results)).toEqual([]);
  });

  test('the signed-in shell has no critical violations and names the session controls', async ({
    page,
  }) => {
    await seedSession(page);
    await page.route('**/templates', (route) => route.fulfill({ json: { data: CATALOG } }));
    await page.goto('/');

    await expect(page.getByRole('navigation', { name: 'Primary' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Your sheet' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Sign out' })).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(criticalViolations(results)).toEqual([]);
  });

  test('the own-sheet surface has no critical violations and names its controls', async ({
    page,
  }) => {
    await seedSession(page);
    await page.route('**/sheets', (route) => route.fulfill({ json: { data: SHEET } }));
    await page.goto('/your-sheet');

    await expect(page.getByRole('heading', { name: 'Your character sheet' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Open sheet' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Delete sheet' })).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(criticalViolations(results)).toEqual([]);
  });
});
