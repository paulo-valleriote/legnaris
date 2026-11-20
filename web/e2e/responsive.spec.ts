import { expect, test } from '@playwright/test';
import type { Page } from '@playwright/test';
import { seedSession } from './auth_seed';
import {
  SHEET as FANTASY_SHEET,
  SHEET_ID as FANTASY_SHEET_ID,
  TEMPLATE as FANTASY_TEMPLATE,
} from './fantasy_play';
import { STAT_BLOCK_DETAIL, STAT_BLOCK_SHEET, STAT_BLOCK_SHEET_ID } from './stat_block_fixture';

test.use({ serviceWorkers: 'block' });

const TEMPLATE_ID = 'fantasy_v1';
const SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd1d';
const CONTROL_HEIGHT = 44;
const UNDERLINE_THICKNESS = 1.5;
const UNDERLINE_CLEARANCE = 2;

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

const FREE_PLAN = {
  plan: 'free',
  sheets: {
    character: { used: 0, limit: 1 },
    npc: { used: 0, limit: 2 },
    enemy: { used: 0, limit: 2 },
  },
  owned_campaigns: { used: 0, limit: 1 },
};

const PHONES = [
  { name: 'a 320 px phone', width: 320, height: 640 },
  { name: 'a 390 px phone', width: 390, height: 844 },
];

async function contentEdges(page: Page) {
  return page.locator('.lg-sheet__header').evaluate((element) => {
    const rect = element.getBoundingClientRect();
    const style = getComputedStyle(element);
    return {
      left: rect.left + Number.parseFloat(style.paddingLeft),
      right: rect.right - Number.parseFloat(style.paddingRight),
    };
  });
}

async function expectFullWidthModeSwitch(page: Page, segments: number) {
  const group = page.getByTestId('sheet-header-actions');
  const edges = await contentEdges(page);
  const box = (await group.boundingBox())!;
  expect(Math.abs(box.x - edges.left)).toBeLessThanOrEqual(1);
  expect(Math.abs(box.x + box.width - edges.right)).toBeLessThanOrEqual(1);

  const buttons = group.locator('.lg-button');
  expect(await buttons.count()).toBe(segments);
  const measured = await buttons.evaluateAll((elements) =>
    elements.map((element) => {
      const rect = element.getBoundingClientRect();
      return { width: rect.width, height: rect.height };
    }),
  );
  for (const segment of measured) {
    expect(Math.abs(segment.width - measured[0]!.width)).toBeLessThanOrEqual(1);
    expect(segment.width).toBeGreaterThanOrEqual(CONTROL_HEIGHT);
    expect(segment.height).toBeGreaterThanOrEqual(CONTROL_HEIGHT);
  }

  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflow).toBe(0);
}

async function expectTrailingModeSwitch(page: Page, segments: number) {
  const group = page.getByTestId('sheet-header-actions');
  const edges = await contentEdges(page);
  const box = (await group.boundingBox())!;
  expect(Math.abs(box.x + box.width - edges.right)).toBeLessThanOrEqual(1);
  expect(box.width).toBeLessThan(edges.right - edges.left - 1);
  expect(await group.locator('.lg-button').count()).toBe(segments);
}

test.describe('narrow viewport (the top bar and full-width content)', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await page.route('**/me/plan', (route) => route.fulfill({ json: { data: FREE_PLAN } }));
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
      await page.getByRole('button', { name: 'Edit mode' }).click();
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

    test(`the navigation collapses behind one menu button on ${phone.name}`, async ({ page }) => {
      await page.setViewportSize({ width: phone.width, height: phone.height });
      await page.goto('/');

      const nav = page.getByRole('navigation', { name: 'Primary' });
      const navBox = await nav.boundingBox();
      const mainBox = await page.getByRole('main').boundingBox();
      expect(navBox!.width).toBe(phone.width);
      expect(navBox!.y + navBox!.height).toBeLessThanOrEqual(mainBox!.y);

      const trigger = nav.getByRole('button', { name: 'Menu' });
      await expect(trigger).toHaveAttribute('aria-expanded', 'false');
      const triggerBox = await trigger.boundingBox();
      expect(triggerBox!.width).toBeGreaterThanOrEqual(CONTROL_HEIGHT);
      expect(triggerBox!.height).toBeGreaterThanOrEqual(CONTROL_HEIGHT);
      expect(triggerBox!.x + triggerBox!.width).toBeLessThanOrEqual(phone.width);

      await trigger.click();
      await expect(trigger).toHaveAttribute('aria-expanded', 'true');
      const panelId = await trigger.getAttribute('aria-controls');
      const panel = page.locator(`[id="${panelId}"]`);
      for (const control of [
        panel.getByRole('link', { name: 'Catalog' }),
        panel.getByRole('link', { name: 'Your sheet' }),
        panel.getByRole('button', { name: 'Sign out' }),
      ]) {
        await expect(control).toBeVisible();
        const box = await control.boundingBox();
        expect(box!.height).toBeGreaterThanOrEqual(CONTROL_HEIGHT);
        expect(box!.x + box!.width).toBeLessThanOrEqual(phone.width);
      }

      await page.keyboard.press('Escape');
      await expect(trigger).toHaveAttribute('aria-expanded', 'false');
      await expect(trigger).toBeFocused();
    });

    test(`the sheet title starts within the first 80 px on ${phone.name}`, async ({ page }) => {
      await page.setViewportSize({ width: phone.width, height: phone.height });
      await page.goto(`/sheets/${SHEET_ID}`);

      const playTitle = await page.getByTestId('sheet-name').boundingBox();
      expect(playTitle).not.toBeNull();
      expect(playTitle!.y).toBeLessThanOrEqual(80);

      await page.getByRole('button', { name: 'Edit mode' }).click();

      const editTitle = await page.getByTestId('sheet-name').boundingBox();
      expect(editTitle).not.toBeNull();
      expect(editTitle!.y).toBeLessThanOrEqual(80);
    });

    test(`the creation dialog fits ${phone.name} with a clickable action`, async ({ page }) => {
      await page.setViewportSize({ width: phone.width, height: phone.height });
      await page.goto('/');
      await page.getByRole('button', { name: /Fantasy/ }).click();

      const dialog = page.getByRole('dialog');
      await expect(dialog).toBeVisible();
      await page.getByRole('textbox', { name: 'Sheet name' }).fill('Aragorn');

      const entry = dialog.getByRole('button', { name: 'Generate with AI' });
      await expect(entry.getByText('Pro')).toBeVisible();
      const entryBox = await entry.boundingBox();
      expect(entryBox!.x).toBeGreaterThanOrEqual(0);
      expect(entryBox!.x + entryBox!.width).toBeLessThanOrEqual(phone.width);

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
    await page.getByRole('button', { name: 'Edit mode' }).click();

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

const LONG_FIELDS = Object.fromEntries(
  Array.from({ length: 30 }, (_, index) => [
    `field${index}`,
    {
      schema: {
        type: 'number',
        label: `Field ${index}`,
        is_editable: true,
        default: 0,
      },
    },
  ]),
);

const LONG_DETAIL = {
  ...DETAIL,
  fields: { long: { label: 'Long', children: LONG_FIELDS } },
};

const LONG_SHEET = { ...SHEET, data: { long: {} } };

test.describe('the desktop navigation on a long page', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await page.route('**/me/plan', (route) => route.fulfill({ json: { data: FREE_PLAN } }));
    await page.route('**/templates/*', (route) => route.fulfill({ json: { data: LONG_DETAIL } }));
    await page.route('**/templates', (route) => route.fulfill({ json: { data: CATALOG } }));
    await page.route('**/sheets/*', (route) =>
      route.request().resourceType() === 'document'
        ? route.fallback()
        : route.fulfill({ json: { data: LONG_SHEET } }),
    );
  });

  test('the navigation paints the full scroll height while Sign out stays pinned', async ({
    page,
  }) => {
    const viewport = { width: 1440, height: 900 };
    await page.setViewportSize(viewport);
    await page.goto(`/sheets/${SHEET_ID}`);
    await page.getByRole('button', { name: 'Edit mode' }).click();
    await expect(page.locator('[data-row-key="long.field29"]')).toBeVisible();

    const scrollHeight = await page.evaluate(() => document.documentElement.scrollHeight);
    expect(scrollHeight).toBeGreaterThan(viewport.height);

    const navBox = (await page.getByRole('navigation', { name: 'Primary' }).boundingBox())!;
    expect(navBox.y).toBe(0);
    expect(navBox.height).toBeGreaterThanOrEqual(scrollHeight - 1);

    await page.evaluate(() => window.scrollTo({ top: 300, behavior: 'instant' }));
    await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(300);

    const signOut = (await page.getByRole('button', { name: 'Sign out' }).boundingBox())!;
    expect(signOut.y).toBeGreaterThan(0);
    expect(signOut.y + signOut.height).toBeGreaterThan(viewport.height - 64);
    expect(signOut.y + signOut.height).toBeLessThanOrEqual(viewport.height);
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
      await page.getByRole('button', { name: 'Edit mode' }).click();
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
      await page.getByRole('button', { name: 'Edit mode' }).click();

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
      await page.getByRole('button', { name: 'Edit mode' }).click();

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

test.describe('the enemy stat block', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await page.route('**/me/plan', (route) =>
      route.fulfill({
        json: {
          data: {
            plan: 'free',
            sheets: {
              character: { used: 0, limit: 1 },
              npc: { used: 0, limit: 2 },
              enemy: { used: 0, limit: 2 },
            },
            owned_campaigns: { used: 0, limit: 1 },
          },
        },
      }),
    );
    await page.route('**/templates/*', (route) =>
      route.fulfill({ json: { data: STAT_BLOCK_DETAIL } }),
    );
    await page.route('**/templates', (route) =>
      route.fulfill({
        json: {
          data: [
            { id: STAT_BLOCK_DETAIL.id, name: STAT_BLOCK_DETAIL.name, default: true, fields: {} },
          ],
        },
      }),
    );
    await page.route('**/sheets/*', (route) =>
      route.request().resourceType() === 'document'
        ? route.fallback()
        : route.fulfill({ json: { data: STAT_BLOCK_SHEET } }),
    );
  });

  async function openStatBlock(page: Page) {
    await page.goto(`/sheets/${STAT_BLOCK_SHEET_ID}`);
    const block = page.getByTestId('sheet-stat-block');
    await expect(block).toBeVisible();
    return block;
  }

  for (const viewport of [
    { name: '1440', width: 1440, height: 900, budget: 900 },
    { name: '390', width: 390, height: 844, budget: 1200 },
  ]) {
    test(`the stat block height fits its ${viewport.name} px budget`, async ({ page }) => {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      const block = await openStatBlock(page);

      expect((await block.boundingBox())!.height).toBeLessThanOrEqual(viewport.budget);
    });
  }

  test('a 390 px phone shows armor class, hit points and the first attack at a glance', async ({
    page,
  }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await openStatBlock(page);

    for (const testId of ['sheet-ac', 'sheet-hp-card', 'sheet-attack-1']) {
      const box = (await page.getByTestId(testId).boundingBox())!;
      expect(box.y + box.height).toBeLessThanOrEqual(844);
    }

    const label = page.getByTestId('sheet-ability-strength').locator('.lg-ability__name');
    expect(await label.evaluate((element) => getComputedStyle(element).fontSize)).toBe('12px');

    const grid = page.locator('.lg-ability-grid').first();
    const columns = await grid.evaluate((element) => getComputedStyle(element).gridTemplateColumns);
    expect(columns.split(' ')).toHaveLength(3);
  });

  test('the body splits into two columns from 1100 px and stacks below it', async ({ page }) => {
    await page.setViewportSize({ width: 1100, height: 900 });
    await openStatBlock(page);

    const wideCombat = (await page.getByTestId('sheet-stat-column-combat').boundingBox())!;
    const wideTraits = (await page.getByTestId('sheet-stat-column-traits').boundingBox())!;
    expect(wideTraits.x).toBeGreaterThanOrEqual(wideCombat.x + wideCombat.width);

    await page.setViewportSize({ width: 1099, height: 900 });
    await openStatBlock(page);

    const narrowCombat = (await page.getByTestId('sheet-stat-column-combat').boundingBox())!;
    const narrowTraits = (await page.getByTestId('sheet-stat-column-traits').boundingBox())!;
    expect(narrowTraits.x).toBe(narrowCombat.x);
    expect(narrowTraits.y).toBeGreaterThanOrEqual(narrowCombat.y + narrowCombat.height);
  });

  test('a 360 px phone keeps the stat block free of horizontal overflow', async ({ page }) => {
    await page.setViewportSize({ width: 360, height: 780 });
    await openStatBlock(page);

    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(overflow).toBe(0);
  });

  for (const phone of PHONES) {
    test(`the header switch group spans the content width on ${phone.name}`, async ({ page }) => {
      await page.setViewportSize({ width: phone.width, height: phone.height });
      await openStatBlock(page);

      await expectFullWidthModeSwitch(page, 3);
    });
  }

  test('a 1440 px viewport keeps the header switch group trailing and compact', async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await openStatBlock(page);

    await expectTrailingModeSwitch(page, 3);
  });

  for (const viewport of [
    { name: '1440', width: 1440, height: 900 },
    { name: '1100', width: 1100, height: 900 },
  ]) {
    test(`no stat block ability name is cut at ${viewport.name} px`, async ({ page }) => {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      await openStatBlock(page);

      const names = page.locator('.lg-stat-block .lg-ability__name');
      expect(await names.count()).toBe(6);
      const measured = await names.evaluateAll((elements) =>
        elements.map((element) => ({
          text: element.textContent?.trim(),
          scroll: element.scrollWidth,
          client: element.clientWidth,
        })),
      );
      for (const name of measured) {
        expect(name.text).not.toBe('');
        expect(name.scroll).toBeLessThanOrEqual(name.client);
      }
    });
  }
});

const dieField = (label: string) => ({
  schema: { type: 'number', label, is_editable: true, default: 0 },
});

const DICE_DETAIL = {
  ...DETAIL,
  dice: {
    section: {},
    row: {
      'combat.initiative': '1d20+combat.initiative',
      'combat.damage': '2d6+1d4+3',
      'combat.speed': '1d20',
    },
  },
  fields: {
    combat: {
      label: 'Combat',
      children: {
        armor: dieField('Base armor class'),
        initiative: dieField('Initiative'),
        damage: dieField('Damage'),
        constitution: dieField('Constitution'),
        speed: dieField('Speed'),
      },
    },
  },
};

const DICE_SHEET = {
  ...SHEET,
  data: { combat: { armor: 10, initiative: 2, damage: 0, constitution: 14, speed: 30 } },
  dice: { section: {}, row: { 'combat.speed': null } },
};

test.describe('dice controls keep the value column on one guide', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await page.route('**/templates/*', (route) => route.fulfill({ json: { data: DICE_DETAIL } }));
    await page.route('**/templates', (route) => route.fulfill({ json: { data: CATALOG } }));
    await page.route('**/sheets/*', (route) =>
      route.request().resourceType() === 'document'
        ? route.fallback()
        : route.fulfill({ json: { data: DICE_SHEET } }),
    );
  });

  for (const viewport of [
    { name: '320 px', width: 320, height: 700 },
    { name: '390 px', width: 390, height: 844 },
    { name: '1280 px', width: 1280, height: 800 },
  ]) {
    test(`every Row's value starts on the same guide at ${viewport.name}`, async ({ page }) => {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      await page.goto(`/sheets/${SHEET_ID}`);
      await page.getByRole('button', { name: 'Edit mode' }).click();
      await expect(page.getByRole('button', { name: /^Roll Damage/ })).toBeVisible();

      const starts = await page
        .locator('[data-row-key^="combat."] .lg-field-row__control')
        .evaluateAll((controls) => controls.map((control) => control.getBoundingClientRect().left));

      expect(starts).toHaveLength(5);
      for (const start of starts) {
        expect(Math.abs(start - starts[0]!)).toBeLessThanOrEqual(1);
      }
      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
      );
      expect(overflow).toBe(0);
    });
  }
});

test.describe('the fantasy Play sheet at every width', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await page.route('**/templates/*', (route) =>
      route.fulfill({ json: { data: FANTASY_TEMPLATE } }),
    );
    await page.route('**/templates', (route) =>
      route.fulfill({ json: { data: [FANTASY_TEMPLATE] } }),
    );
    await page.route('**/sheets/*', (route) =>
      route.request().resourceType() === 'document'
        ? route.fallback()
        : route.fulfill({ json: { data: FANTASY_SHEET } }),
    );
  });

  async function openPlay(page: Page) {
    await page.goto(`/sheets/${FANTASY_SHEET_ID}`);
    await expect(page.getByTestId('sheet-hp-card')).toBeVisible();
  }

  for (const phone of PHONES) {
    test(`every hit point control stays inside the card on ${phone.name}`, async ({ page }) => {
      await page.setViewportSize({ width: phone.width, height: phone.height });
      await openPlay(page);

      const card = page.getByTestId('sheet-hp-card');
      const cardBox = (await card.boundingBox())!;
      const controls = card.locator('button, input');
      const count = await controls.count();
      expect(count).toBeGreaterThanOrEqual(11);
      for (let index = 0; index < count; index += 1) {
        const control = controls.nth(index);
        await expect(control).toBeVisible();
        const box = (await control.boundingBox())!;
        expect(box.x).toBeGreaterThanOrEqual(cardBox.x - 0.5);
        expect(box.x + box.width).toBeLessThanOrEqual(cardBox.x + cardBox.width + 0.5);
        expect(box.y).toBeGreaterThanOrEqual(cardBox.y - 0.5);
        expect(box.y + box.height).toBeLessThanOrEqual(cardBox.y + cardBox.height + 0.5);
      }

      for (const name of [
        'Damage 5',
        'Damage 1',
        'Heal 1',
        'Heal 5',
        'Add temporary hit points',
        'Damage',
        'Heal',
      ]) {
        const box = (await card.getByRole('button', { name, exact: true }).boundingBox())!;
        expect(box.width).toBeGreaterThanOrEqual(CONTROL_HEIGHT);
        expect(box.height).toBeGreaterThanOrEqual(CONTROL_HEIGHT);
      }
    });
  }

  for (const viewport of [
    { name: '1440', width: 1440, height: 900 },
    { name: '390', width: 390, height: 844 },
  ]) {
    test(`the initiative underline sits clear of its label at ${viewport.name} px`, async ({
      page,
    }) => {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      await openPlay(page);

      const geometry = await page.getByTestId('sheet-initiative').evaluate((element) => {
        const value = element.querySelector('.lg-combat-stat__value')!;
        const label = element.querySelector('.lg-combat-stat__label')!;
        const text = [...value.childNodes].find(
          (node) => node.nodeType === Node.TEXT_NODE && (node.textContent ?? '').trim() !== '',
        )!;
        const range = document.createRange();
        range.selectNodeContents(text);
        const textBox = range.getBoundingClientRect();
        const style = getComputedStyle(value);
        const context = document.createElement('canvas').getContext('2d')!;
        context.font = `${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
        const metrics = context.measureText(text.textContent ?? '');
        return {
          underlineBottom:
            textBox.top +
            metrics.fontBoundingBoxAscent +
            Number.parseFloat(style.textUnderlineOffset),
          labelTop: label.getBoundingClientRect().top,
        };
      });

      expect(geometry.underlineBottom + UNDERLINE_THICKNESS).toBeLessThanOrEqual(
        geometry.labelTop - UNDERLINE_CLEARANCE,
      );
    });
  }

  for (const phone of PHONES) {
    test(`the Play/Edit switch spans the content width on ${phone.name}`, async ({ page }) => {
      await page.setViewportSize({ width: phone.width, height: phone.height });
      await openPlay(page);

      await expectFullWidthModeSwitch(page, 2);
    });
  }

  test('a 1440 px viewport keeps the Play/Edit switch trailing and compact', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await openPlay(page);

    await expectTrailingModeSwitch(page, 2);
  });
});
