import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { seedSession } from './auth_seed';

// The service worker is irrelevant to the accessibility measurement and would
// sit between the page and the intercepted API requests; its offline-ready
// notice would also race the screenshot of the auth surface.
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
  test('the landing has no critical violations, names its landmarks and leads to the auth paths', async ({
    page,
  }) => {
    await page.route('**/auth/refresh', (route) =>
      route.fulfill({
        status: 401,
        json: { error: { code: 'UNAUTHORIZED', message: 'no cookie' } },
      }),
    );
    await page.goto('/');

    await expect(
      page.getByRole('heading', { level: 1, name: 'A character sheet that does its own math.' }),
    ).toBeVisible();
    await expect(page.getByRole('main')).toBeVisible();
    await expect(page.getByRole('link', { name: 'Start your archive' }).first()).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(criticalViolations(results)).toEqual([]);

    await page.getByRole('link', { name: 'Start your archive' }).first().click();
    await expect(page).toHaveURL(/\/sign-up$/);
    await expect(page.getByRole('textbox', { name: 'Username' })).toBeVisible();
  });

  test('the auth surface has no critical violations and names its controls', async ({ page }) => {
    await page.route('**/auth/refresh', (route) =>
      route.fulfill({
        status: 401,
        json: { error: { code: 'UNAUTHORIZED', message: 'no cookie' } },
      }),
    );
    await page.goto('/sign-in');

    await expect(page.getByRole('heading', { name: 'Sign in to your archive' })).toBeVisible();
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

  test('the code step renders the code surface, names the address and the lifetime, and completes (WCS-19, WCS-20, WCS-22, WCS-24)', async ({
    page,
  }) => {
    await page.route('**/auth/refresh', (route) =>
      route.fulfill({
        status: 401,
        json: { error: { code: 'UNAUTHORIZED', message: 'no cookie' } },
      }),
    );
    await page.route('**/auth/login', (route) =>
      route.fulfill({
        status: 202,
        json: {
          data: { challenge_id: 'challenge-1', expires_in: 600, resend_after: 60 },
        },
      }),
    );
    await page.route('**/auth/challenge/verify', (route) =>
      route.fulfill({
        status: 200,
        json: { data: { access_token: 'e2e-access-token', token_type: 'Bearer', expires_in: 900 } },
      }),
    );
    await page.route('**/templates', (route) => route.fulfill({ json: { data: CATALOG } }));

    await page.goto('/sign-in');

    await page.getByLabel('Email').fill('a@b.test');
    await page.getByLabel('Password').fill('secret');
    await page.getByRole('button', { name: 'Sign in' }).click();

    const code = page.getByLabel('6-digit code');
    await expect(code).toBeVisible();
    await expect(code).toBeFocused();
    await expect(page.getByText('We sent a 6-digit code to a@b.test.')).toBeVisible();
    await expect(page.getByText('It expires in 10 minutes.')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Resend code' })).toBeVisible();
    await expect(page.getByText('Resend in 60s')).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(criticalViolations(results)).toEqual([]);

    await code.fill('042917');
    await page.getByRole('button', { name: 'Verify' }).click();

    await expect(page.getByTestId('catalog')).toBeVisible();
  });
});
