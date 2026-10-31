import { expect, test } from '@playwright/test';
import { seedSession } from './auth_seed';

test.use({ serviceWorkers: 'block' });

const TEMPLATE_ID = 'fantasy_v1';
const SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd1d';
const CONTROL_HEIGHT = 44;

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
        guarded: { schema: { type: 'bool', label: 'Guarded', is_editable: true, default: 0 } },
      },
    },
  },
};

const SHEET = {
  id: SHEET_ID,
  template_id: TEMPLATE_ID,
  name: 'Aragorn',
  data: { identity: { characterName: 'Aragorn', guarded: 0 } },
  created_at: '2026-10-01T21:38:54.635Z',
  updated_at: '2026-10-02T09:00:00.000Z',
};

const PHONES = [
  { name: 'a 320 px phone', width: 320, height: 640 },
  { name: 'a 390 px phone', width: 390, height: 844 },
];

test.describe('narrow viewport (the top bar and full-width content)', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await page.route('**/templates/*', (route) => route.fulfill({ json: { data: DETAIL } }));
    await page.route('**/templates', (route) => route.fulfill({ json: { data: CATALOG } }));
    await page.route('**/sheets/*', (route) =>
      route.request().resourceType() === 'document'
        ? route.fallback()
        : route.fulfill({ json: { data: SHEET } }),
    );
    await page.route('**/sheets', (route) => route.fulfill({ status: 201, json: { data: SHEET } }));
  });

  for (const phone of PHONES) {
    test(`the sheet fits ${phone.name} with its actions reachable`, async ({ page }) => {
      await page.setViewportSize({ width: phone.width, height: phone.height });
      await page.goto(`/sheets/${SHEET_ID}`);
      await expect(page.getByRole('heading', { name: 'Aragorn' })).toBeVisible();

      const overflow = await page.evaluate(() => ({
        scroll: document.documentElement.scrollWidth,
        client: document.documentElement.clientWidth,
      }));
      expect(overflow.scroll).toBe(overflow.client);

      for (const name of ['Save', 'Preview', 'Arrange']) {
        const box = await page.getByRole('button', { name, exact: true }).boundingBox();
        expect(box).not.toBeNull();
        expect(box!.x).toBeGreaterThanOrEqual(0);
        expect(box!.x + box!.width).toBeLessThanOrEqual(phone.width);
        expect(box!.height).toBeGreaterThanOrEqual(CONTROL_HEIGHT);
      }
    });

    test(`the navigation is a reachable top bar of full-height controls on ${phone.name}`, async ({
      page,
    }) => {
      await page.setViewportSize({ width: phone.width, height: phone.height });
      await page.goto('/');

      const nav = page.getByRole('navigation', { name: 'Primary' });
      const navBox = await nav.boundingBox();
      const mainBox = await page.getByRole('main').boundingBox();
      expect(navBox!.width).toBe(phone.width);
      expect(navBox!.y + navBox!.height).toBeLessThanOrEqual(mainBox!.y);

      for (const control of [
        nav.getByRole('link', { name: 'Catalog' }),
        nav.getByRole('link', { name: 'Your sheet' }),
        nav.getByRole('button', { name: 'Sign out' }),
      ]) {
        const box = await control.boundingBox();
        expect(box).not.toBeNull();
        expect(box!.height).toBeGreaterThanOrEqual(CONTROL_HEIGHT);
        expect(box!.x + box!.width).toBeLessThanOrEqual(phone.width);
      }
    });

    test(`the creation dialog fits ${phone.name} with a clickable action`, async ({ page }) => {
      await page.setViewportSize({ width: phone.width, height: phone.height });
      await page.goto('/');
      await page.getByRole('button', { name: /Fantasy/ }).click();

      const dialog = page.getByRole('dialog');
      await expect(dialog).toBeVisible();
      await page.getByRole('textbox', { name: 'Sheet name' }).fill('Aragorn');

      const box = await dialog.boundingBox();
      expect(box!.x).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width).toBeLessThanOrEqual(phone.width);
      await page.getByRole('button', { name: 'Create sheet' }).click({ trial: true });

      const scroll = await page.evaluate(
        () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
      );
      expect(scroll).toBe(0);
    });
  }

  test('the switch input is at least the control height', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`/sheets/${SHEET_ID}`);

    const box = await page.getByRole('checkbox', { name: 'Guarded' }).boundingBox();
    expect(box!.height).toBeGreaterThanOrEqual(CONTROL_HEIGHT);
  });

  test('a desktop viewport keeps the left navigation column', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/');

    const navBox = await page.getByRole('navigation', { name: 'Primary' }).boundingBox();
    const mainBox = await page.getByRole('main').boundingBox();
    expect(navBox!.width).toBeLessThanOrEqual(240);
    expect(mainBox!.x).toBeGreaterThanOrEqual(navBox!.x + navBox!.width);
  });
});

const LONG_LABEL = 'Ancestry, lineage and heritage of the northern clans';

const MARKED_DETAIL = {
  ...DETAIL,
  fields: {
    identity: {
      label: 'Identity',
      children: {
        ancestry: { schema: { type: 'text', label: LONG_LABEL, is_editable: true, default: 0 } },
        origin: { schema: { type: 'text', label: 'Origin', is_editable: true, default: 0 } },
      },
    },
  },
};

const MARKED_SHEET = { ...SHEET, data: { identity: { ancestry: '', origin: '' } } };

test.describe('preview marks never move rows', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await page.route('**/templates/*', (route) => route.fulfill({ json: { data: MARKED_DETAIL } }));
    await page.route('**/templates', (route) => route.fulfill({ json: { data: CATALOG } }));
    await page.route('**/sheets/*/preview', (route) =>
      route.fulfill({ json: { data: { identity: { ancestry: 'Dwarf' } } } }),
    );
    await page.route('**/sheets/*', (route) =>
      route.request().resourceType() === 'document'
        ? route.fallback()
        : route.fulfill({ json: { data: MARKED_SHEET } }),
    );
  });

  for (const viewport of [
    { name: '320 px', width: 320, height: 700 },
    { name: '390 px', width: 390, height: 844 },
    { name: '1280 px', width: 1280, height: 800 },
  ]) {
    test(`a wrapping label keeps its row height when it is marked at ${viewport.name}`, async ({
      page,
    }) => {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      await page.goto(`/sheets/${SHEET_ID}`);
      const row = page.locator('[data-row-key="identity.ancestry"]');
      await expect(row).toBeVisible();
      const before = (await row.boundingBox())!.height;

      await page.getByRole('textbox', { name: LONG_LABEL }).fill('Dwarf');
      await page.getByRole('button', { name: 'Preview', exact: true }).click();
      await expect(row.getByText('Modified')).toBeVisible();

      expect((await row.boundingBox())!.height).toBe(before);
    });
  }
});

const PIN_FIELDS = Object.fromEntries(
  ['One', 'Two', 'Three', 'Four', 'Five', 'Six', 'Seven'].map((label) => [
    label.toLowerCase(),
    {
      schema: {
        type: 'number',
        label: `${label} points of long label`,
        is_editable: true,
        default: 0,
      },
    },
  ]),
);

const PIN_DETAIL = {
  ...DETAIL,
  fields: { core: { label: 'Core', children: PIN_FIELDS } },
};

const SHOTS = process.env.PIN_SHOTS_DIR;

test.describe('the sticky strip leading slot', () => {
  for (const viewport of [
    { name: '320', width: 320, height: 640 },
    { name: '390', width: 390, height: 844 },
    { name: '1280', width: 1280, height: 800 },
  ]) {
    test(`the sticky strip keeps the star, the sheet name and the links inside ${viewport.name} px`, async ({
      page,
    }) => {
      await seedSession(page);
      await page.route('**/templates/*', (route) => route.fulfill({ json: { data: PIN_DETAIL } }));
      await page.route('**/templates', (route) => route.fulfill({ json: { data: CATALOG } }));
      await page.route('**/sheets/*', (route) =>
        route.request().resourceType() === 'document'
          ? route.fallback()
          : route.fulfill({
              json: { data: { ...SHEET, name: 'Aragorn son of Arathorn', data: { core: {} } } },
            }),
      );
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      await page.goto(`/sheets/${SHEET_ID}`);

      const bar = page.getByTestId('section-strip');
      const filter = bar.getByRole('button', { name: 'Pinned only' });
      const chip = bar.getByRole('button', { name: /back to top/ });
      for (const control of [filter, chip]) {
        const box = (await control.boundingBox())!;
        expect(box.height).toBeGreaterThanOrEqual(CONTROL_HEIGHT);
        expect(box.width).toBeGreaterThanOrEqual(CONTROL_HEIGHT - 4);
        expect(box.x + box.width).toBeLessThanOrEqual(viewport.width);
      }
      const textWidth = await filter
        .locator('.lg-sectionbar__filter-text')
        .evaluate((el) => el.getBoundingClientRect().width);
      expect(textWidth <= 1).toBe(viewport.width < 640);
      if (SHOTS) {
        await bar.screenshot({ path: `${SHOTS}/strip-${viewport.name}.png` });
      }
    });
  }
});

test.describe('the footer after a Preview', () => {
  const FOOTER_BUDGET = 104;
  const PREVIEW_DETAIL = {
    ...DETAIL,
    fields: {
      core: {
        label: 'Core',
        children: {
          level: { schema: { type: 'number', label: 'Level', is_editable: true, default: 1 } },
          bonus: {
            schema: {
              type: 'number',
              label: 'Bonus',
              formula: 'level + 1',
              is_editable: false,
              default: 0,
            },
          },
        },
      },
    },
  };

  for (const phone of PHONES) {
    test(`the footer stays within two rows and ${FOOTER_BUDGET} px on ${phone.name}`, async ({
      page,
    }) => {
      await seedSession(page);
      await page.route('**/templates/*', (route) =>
        route.fulfill({ json: { data: PREVIEW_DETAIL } }),
      );
      await page.route('**/templates', (route) => route.fulfill({ json: { data: CATALOG } }));
      await page.route('**/sheets/*/preview', (route) =>
        route.fulfill({ json: { data: { core: { bonus: 5 } } } }),
      );
      await page.route('**/sheets/*', (route) =>
        route.request().resourceType() === 'document'
          ? route.fallback()
          : route.fulfill({ json: { data: { ...SHEET, data: { core: { level: 3 } } } } }),
      );
      await page.setViewportSize({ width: phone.width, height: phone.height });
      await page.goto(`/sheets/${SHEET_ID}`);

      await page.getByRole('textbox', { name: 'Level' }).fill('9');
      await page.getByRole('button', { name: 'Preview', exact: true }).click();
      await expect(page.getByTestId('sheet-status')).toHaveText('Unsaved, 1 recalculated');

      const footer = (await page.locator('.lg-sheet__footer').boundingBox())!;
      expect(footer.height).toBeLessThanOrEqual(FOOTER_BUDGET);
      for (const name of ['Preview', 'Save']) {
        const box = (await page.getByRole('button', { name, exact: true }).boundingBox())!;
        expect(box.height).toBeGreaterThanOrEqual(CONTROL_HEIGHT);
      }
      const jump = (await page
        .getByRole('button', { name: 'Jump to first recalculated value' })
        .boundingBox())!;
      const status = (await page.getByTestId('sheet-status').boundingBox())!;
      expect(Math.abs(jump.y + jump.height / 2 - (status.y + status.height / 2))).toBeLessThan(
        jump.height,
      );
    });
  }
});
