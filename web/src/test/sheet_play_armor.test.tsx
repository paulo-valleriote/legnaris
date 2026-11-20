import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import {
  FANTASY_SHEET_ID,
  fantasyFieldTree,
  fantasySheetData,
  fantasySheetPayload,
  fantasyTemplatePayload,
} from './fantasy_fixtures';
import type { FakeOutcome } from './fake_transport';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { rollName } from './dice_names';

const BASE_URL = 'http://api.test';
const here = path.dirname(fileURLToPath(import.meta.url));
const stylesheet = path.resolve(here, '../sheet/preset/fantasy/combat_strip.css');

interface OpenOptions {
  readonly data?: Record<string, unknown>;
  readonly template?: Record<string, unknown>;
  readonly preview?: FakeOutcome;
}

async function openFantasy(options: OpenOptions = {}) {
  const data = options.data ?? fantasySheetData();
  const fake = createFakeTransport(
    successOutcome(fantasySheetPayload({ data })),
    successOutcome(fantasyTemplatePayload(options.template ?? {})),
    options.preview ?? successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

type User = ReturnType<typeof userEvent.setup>;

function shield() {
  return screen.getByTestId('sheet-ac-shield');
}

function acValue() {
  return screen.getByTestId('sheet-ac-value');
}

function breakdown() {
  return screen.getByTestId('sheet-ac-breakdown');
}

function breakdownQuery() {
  return screen.queryByTestId('sheet-ac-breakdown');
}

async function openBreakdown(user: User) {
  await user.click(shield());
  return breakdown();
}

function baseArmorButton() {
  return within(breakdown()).getByRole('button', { name: 'Base armor class' });
}

async function editBaseArmor(user: User, text: string) {
  await user.click(baseArmorButton());
  const input = within(breakdown()).getByRole('textbox', { name: 'Base armor class' });
  await user.clear(input);
  if (text !== '') {
    await user.type(input, text);
  }
  await user.keyboard('{Enter}');
}

function savedData(fake: ReturnType<typeof createFakeTransport>) {
  const patch = fake.requests.find((request) => request.method === 'PATCH');
  return patch === undefined ? undefined : JSON.parse(patch.body ?? '{}');
}

function previewRequest(fake: ReturnType<typeof createFakeTransport>) {
  const preview = fake.requests.find((request) => request.url.endsWith('/preview'));
  return preview === undefined ? undefined : JSON.parse(preview.body ?? '{}');
}

async function save(user: User) {
  await user.click(screen.getByRole('button', { name: message('sheet.save') }));
}

async function preview(user: User) {
  await user.click(
    within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
      name: message('sheet.pending.preview'),
    }),
  );
}

describe('the armor class shield', () => {
  it('Edge: a sheet without a derived armor class shows the placeholder and still opens', async () => {
    const { user } = await openFantasy({
      data: fantasySheetData({ derived: { armorClass: null } }),
    });

    expect(acValue()).toHaveTextContent(message('sheet.play.noValue'));

    const panel = await openBreakdown(user);
    expect(within(panel).getByTestId('sheet-ac-total')).toHaveTextContent(
      message('sheet.play.noValue'),
    );
  });

  it('Edge: the shield names itself with the derived value and the breakdown state', async () => {
    const { user } = await openFantasy();

    expect(shield()).toHaveAccessibleName('Armor class 12');
    expect(shield()).toHaveAttribute('aria-expanded', 'false');
    expect(shield()).not.toHaveAttribute('aria-controls');

    await openBreakdown(user);

    expect(shield()).toHaveAttribute('aria-expanded', 'true');
    expect(shield()).toHaveAttribute('aria-controls', 'lg-ac-breakdown');
  });

  it('Edge: a missing base armor class reads as the add placeholder', async () => {
    const { user } = await openFantasy({
      data: fantasySheetData({ combat: { baseArmorClass: null } }),
    });

    await openBreakdown(user);

    expect(baseArmorButton()).toHaveTextContent(
      messageWith('sheet.inlineAdd', { label: 'Base armor class' }),
    );
  });

  it('Happy path: the shield reads the derived armor class from the sheet', async () => {
    await openFantasy({ data: fantasySheetData({ derived: { armorClass: 15 } }) });

    expect(acValue()).toHaveTextContent('15');
    expect(shield()).toHaveAccessibleName('Armor class 15');
  });
});

describe('the armor class breakdown', () => {
  it('Edge: Escape closes the breakdown and returns focus to the shield', async () => {
    const { user } = await openFantasy();
    await openBreakdown(user);

    await user.keyboard('{Escape}');

    expect(breakdownQuery()).toBeNull();
    expect(shield()).toHaveFocus();
  });

  it('Edge: Escape inside the base armor edit cancels the edit but keeps the breakdown open', async () => {
    const { fake, user } = await openFantasy();
    await openBreakdown(user);

    await user.click(baseArmorButton());
    const input = within(breakdown()).getByRole('textbox', { name: 'Base armor class' });
    await user.clear(input);
    await user.type(input, '18{Escape}');

    expect(breakdownQuery()).not.toBeNull();
    expect(baseArmorButton()).toHaveTextContent('10');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Edge: a click outside closes the breakdown', async () => {
    const { user } = await openFantasy();
    await openBreakdown(user);

    await user.click(document.body);

    expect(breakdownQuery()).toBeNull();
  });

  it('Edge: clicking the shield again closes the breakdown', async () => {
    const { user } = await openFantasy();
    await openBreakdown(user);

    await user.click(shield());

    expect(breakdownQuery()).toBeNull();
  });

  it('Edge: the close button closes the breakdown and returns focus to the shield', async () => {
    const { user } = await openFantasy();
    await openBreakdown(user);

    await user.click(within(breakdown()).getByRole('button', { name: message('common.close') }));

    expect(breakdownQuery()).toBeNull();
    expect(shield()).toHaveFocus();
  });

  it('Edge: with both config leaves off neither segment is pressed', async () => {
    const { user } = await openFantasy({
      data: fantasySheetData({ config: { acUsesDex: 0, acUsesCon: 0 } }),
    });
    await openBreakdown(user);

    expect(within(breakdown()).getByRole('button', { name: 'Dexterity' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
    expect(within(breakdown()).getByRole('button', { name: 'Constitution' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
    expect(within(breakdown()).getByTestId('sheet-ac-modifier')).toHaveTextContent(
      message('sheet.play.noValue'),
    );
  });

  it('Edge: with both config leaves on neither segment is pressed', async () => {
    const { user } = await openFantasy({
      data: fantasySheetData({ config: { acUsesDex: 1, acUsesCon: 1 } }),
    });
    await openBreakdown(user);

    expect(within(breakdown()).getByRole('button', { name: 'Dexterity' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
    expect(within(breakdown()).getByRole('button', { name: 'Constitution' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });

  it('Edge: clicking the active segment writes nothing', async () => {
    const { fake, user } = await openFantasy();
    await openBreakdown(user);

    await user.click(within(breakdown()).getByRole('button', { name: 'Dexterity' }));

    expect(screen.queryByTestId('sheet-pending-card')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Edge: an unreadable or empty base armor keeps the stored value', async () => {
    const { fake, user } = await openFantasy();
    await openBreakdown(user);

    await editBaseArmor(user, 'abc');
    await editBaseArmor(user, '');

    expect(baseArmorButton()).toHaveTextContent('10');
    expect(acValue()).toHaveTextContent('12');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Edge: a base armor beyond the range clamps before it reaches the delta', async () => {
    const { user } = await openFantasy();
    await openBreakdown(user);

    await editBaseArmor(user, '99');
    expect(baseArmorButton()).toHaveTextContent('30');

    await editBaseArmor(user, '0');
    expect(baseArmorButton()).toHaveTextContent('1');
  });

  it('Error: a template that marks the base armor non-editable keeps the stored value', async () => {
    const fields = fantasyFieldTree();
    const combat = fields.combat as {
      children: Record<string, { schema: Record<string, unknown> }>;
    };
    combat.children.baseArmorClass = {
      schema: { ...combat.children.baseArmorClass.schema, is_editable: false },
    };
    const { fake, user } = await openFantasy({ template: { fields } });
    await openBreakdown(user);

    await editBaseArmor(user, '18');

    expect(baseArmorButton()).toHaveTextContent('10');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Happy path: the breakdown reads base armor, the active modifier and the total', async () => {
    const { user } = await openFantasy();
    const panel = await openBreakdown(user);

    expect(panel).toHaveAttribute('role', 'dialog');
    expect(panel).toHaveAccessibleName('Armor class');
    expect(baseArmorButton()).toHaveTextContent('10');
    expect(within(panel).getByRole('button', { name: 'Dexterity' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(within(panel).getByRole('button', { name: 'Constitution' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
    expect(within(panel).getByTestId('sheet-ac-modifier')).toHaveTextContent('+2');
    expect(within(panel).getByTestId('sheet-ac-total')).toHaveTextContent('12');
  });

  it('Happy path: the breakdown is keyboard reachable from the shield', async () => {
    const { user } = await openFantasy();

    shield().focus();
    await user.keyboard('{Enter}');

    expect(breakdown()).toBeInTheDocument();
    expect(breakdown()).toHaveFocus();

    await user.tab();
    expect(
      within(breakdown()).getByRole('button', { name: message('common.close') }),
    ).toHaveFocus();

    await user.tab();
    expect(baseArmorButton()).toHaveFocus();

    await user.tab();
    expect(within(breakdown()).getByRole('button', { name: 'Dexterity' })).toHaveFocus();

    await user.tab();
    expect(within(breakdown()).getByRole('button', { name: 'Constitution' })).toHaveFocus();
  });

  it('Happy path: a base armor edit goes to the pending delta and Save sends only that leaf', async () => {
    const { fake, user } = await openFantasy();
    await openBreakdown(user);

    await editBaseArmor(user, '14');

    expect(baseArmorButton()).toHaveTextContent('14');
    expect(acValue()).toHaveTextContent('12');
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent(
      'Base armor class 10 → 14',
    );
    expect(savedData(fake)).toBeUndefined();

    await save(user);
    await waitFor(() =>
      expect(savedData(fake)).toEqual({ data: { combat: { baseArmorClass: 14 } } }),
    );
  });

  it('Happy path: switching to Constitution sends exactly one non-zero member', async () => {
    const { fake, user } = await openFantasy();
    await openBreakdown(user);

    await user.click(within(breakdown()).getByRole('button', { name: 'Constitution' }));

    expect(within(breakdown()).getByRole('button', { name: 'Constitution' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(within(breakdown()).getByRole('button', { name: 'Dexterity' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
    expect(within(breakdown()).getByTestId('sheet-ac-modifier')).toHaveTextContent('+1');

    await save(user);
    await waitFor(() =>
      expect(savedData(fake)).toEqual({ data: { config: { acUsesDex: 0, acUsesCon: 1 } } }),
    );
  });

  it('Happy path: Preview after an ability switch updates the derived armor class', async () => {
    const { fake, user } = await openFantasy({
      preview: successOutcome({ derived: { armorClass: 11 } }),
    });
    await openBreakdown(user);

    await user.click(within(breakdown()).getByRole('button', { name: 'Constitution' }));
    await preview(user);

    await waitFor(() => expect(acValue()).toHaveTextContent('11'));
    expect(previewRequest(fake)).toEqual({ data: { config: { acUsesDex: 0, acUsesCon: 1 } } });
    expect(savedData(fake)).toBeUndefined();

    await openBreakdown(user);
    expect(within(breakdown()).getByTestId('sheet-ac-total')).toHaveTextContent('11');
  });
});

describe('the initiative and speed tiles', () => {
  it('Edge: a missing initiative shows the placeholder and offers no roll', async () => {
    const { user } = await openFantasy({
      data: fantasySheetData({ combat: { initiative: null } }),
    });

    expect(screen.getByTestId('sheet-initiative')).toHaveTextContent(message('sheet.play.noValue'));
    expect(screen.queryByRole('button', { name: rollName('Initiative') })).toBeNull();

    expect(screen.getByTestId('sheet-ac-shield')).toBeInTheDocument();
    await user.click(shield());
    expect(breakdown()).toBeInTheDocument();
  });

  it('Edge: a speed beyond the range clamps before it reaches the delta', async () => {
    const { fake, user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: 'Speed' }));
    const input = screen.getByRole('textbox', { name: 'Speed' });
    await user.clear(input);
    await user.type(input, '200{Enter}');

    expect(screen.getByRole('button', { name: 'Speed' })).toHaveTextContent('120');
    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { combat: { speed: 120 } } }));
  });

  it('Happy path: the tiles read the stored initiative and speed', async () => {
    await openFantasy();

    expect(screen.getByTestId('sheet-initiative')).toHaveTextContent('Initiative');
    expect(screen.getByRole('button', { name: rollName('Initiative') })).toHaveTextContent('+3');
    expect(screen.getByTestId('sheet-speed')).toHaveTextContent('Speed');
    expect(screen.getByRole('button', { name: 'Speed' })).toHaveTextContent('30');
  });

  it('Happy path: clicking initiative rolls it through the dice store', async () => {
    const { user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: rollName('Initiative') }));

    await waitFor(() => expect(document.querySelector('.lg-dice-panel')).not.toBeNull());
    expect(document.querySelector('.lg-dice-panel__total-label')).toHaveTextContent('Initiative');
    const total = Number(document.querySelector('.lg-dice-panel__total-value')?.textContent);
    expect(total).toBeGreaterThanOrEqual(4);
    expect(total).toBeLessThanOrEqual(23);
  });

  it('Happy path: a speed edit goes to the pending delta and Save sends only that leaf', async () => {
    const { fake, user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: 'Speed' }));
    const input = screen.getByRole('textbox', { name: 'Speed' });
    await user.clear(input);
    await user.type(input, '35{Enter}');

    expect(screen.getByRole('button', { name: 'Speed' })).toHaveTextContent('35');
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Speed 30 → 35');

    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { combat: { speed: 35 } } }));
  });
});

describe('the phone layout', () => {
  const css = readFileSync(stylesheet, 'utf8');
  const mobile = css.slice(css.indexOf('@media (max-width: 40rem)'));

  it('Edge: the phone strip is a three-column grid with the hit points card across it', () => {
    expect(mobile).toMatch(/\.lg-preset \.lg-combat-strip\s*\{[^}]*display:\s*grid/);
    expect(mobile).toMatch(/grid-template-columns:\s*repeat\(3, minmax\(0, 1fr\)\)/);
    expect(mobile).toMatch(/\.lg-combat-strip \.lg-hp-card\s*\{[^}]*grid-column:\s*1 \/ -1/);
  });

  it('Edge: the phone breakdown is a fixed bottom sheet, not clipped by scrolling containers', () => {
    expect(mobile).toMatch(/\.lg-ac-breakdown\s*\{[^}]*position:\s*fixed/);
    expect(mobile).toMatch(/\.lg-ac-breakdown\s*\{[^}]*inset-block-end:\s*0/);
    expect(mobile).toMatch(/\.lg-ac-breakdown\s*\{[^}]*overflow-y:\s*auto/);
  });
});
