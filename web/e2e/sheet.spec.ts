import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

// The service worker is irrelevant to the accessibility measurement and would
// sit between the page and the intercepted API requests.
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
    ability: {
      label: 'Ability',
      children: {
        stealth: {
          schema: {
            type: 'number',
            label: 'Stealth',
            is_editable: true,
            default: 0,
            exclusive_group: 'ability',
            option_label: 'Stealth',
          },
        },
        athletics: {
          schema: {
            type: 'number',
            label: 'Athletics',
            is_editable: true,
            default: 0,
            exclusive_group: 'ability',
            option_label: 'Athletics',
          },
        },
      },
    },
  },
};

const SHEET = {
  id: SHEET_ID,
  template_id: TEMPLATE_ID,
  name: 'Aragorn',
  data: {
    identity: { characterName: 'Aragorn', level: 3 },
    ability: { stealth: 1, athletics: 0 },
  },
  created_at: '2026-10-01T21:38:54.635Z',
  updated_at: '2026-10-02T09:00:00.000Z',
};

function criticalViolations(results: Awaited<ReturnType<AxeBuilder['analyze']>>) {
  return results.violations.filter((violation) => violation.impact === 'critical');
}

test.describe('sheet accessibility (WSC-39, SC-010, AD-014)', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/templates/*', (route) => route.fulfill({ json: { data: DETAIL } }));
    await page.route('**/templates', (route) => route.fulfill({ json: { data: CATALOG } }));
    await page.route('**/sheets/*/preview', (route) => route.fulfill({ json: { data: {} } }));
    // The deep link `/sheets/:id` is also a document navigation; only the JSON
    // request is intercepted, so the document still resolves to the SPA shell.
    await page.route('**/sheets/*', (route) =>
      route.request().resourceType() === 'document'
        ? route.fallback()
        : route.fulfill({ json: { data: SHEET } }),
    );
    await page.route('**/sheets', (route) => route.fulfill({ status: 201, json: { data: SHEET } }));
  });

  test('the creation surface is free of critical violations and names every control', async ({
    page,
  }) => {
    await page.goto('/');
    await page.getByRole('button', { name: /Fantasy/ }).click();

    await expect(page.getByRole('dialog')).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Sheet name' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Create sheet' })).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(criticalViolations(results)).toEqual([]);
  });

  test('the mounted sheet screen is free of critical violations and names every control', async ({
    page,
  }) => {
    await page.goto(`/sheets/${SHEET_ID}`);

    await expect(page.getByRole('heading', { name: 'Aragorn' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Name' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Level' })).toBeVisible();
    await expect(page.getByRole('combobox', { name: 'Ability' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Preview' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Save' })).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(criticalViolations(results)).toEqual([]);
  });

  test('the leave confirmation is free of critical violations and names both actions', async ({
    page,
  }) => {
    await page.goto(`/sheets/${SHEET_ID}`);

    const level = page.getByRole('textbox', { name: 'Level' });
    await expect(level).toBeVisible();
    await level.fill('4');
    await page.getByRole('link', { name: 'Catalog' }).click();

    await expect(page.getByRole('dialog')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Keep editing' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Discard and leave' })).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(criticalViolations(results)).toEqual([]);
  });
});
