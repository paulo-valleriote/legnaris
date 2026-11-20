import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import {
  FANTASY_SHEET_ID,
  fantasyFieldTree,
  fantasySheetData,
  fantasySheetPayload,
  fantasyTemplatePayload,
} from './fantasy_fixtures';
import type { FakeOutcome } from './fake_transport';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { rollName } from './dice_names';

const BASE_URL = 'http://api.test';

interface OpenFantasyOptions {
  readonly data?: Record<string, unknown>;
  readonly template?: Record<string, unknown>;
  readonly dice?: Record<string, unknown>;
  readonly save?: FakeOutcome;
  readonly preview?: FakeOutcome;
}

async function openFantasy(options: OpenFantasyOptions = {}) {
  const sheet = fantasySheetPayload(options.dice === undefined ? {} : { dice: options.dice });
  const fake = createFakeTransport(
    successOutcome(options.data === undefined ? sheet : { ...sheet, data: options.data }),
    successOutcome(fantasyTemplatePayload(options.template ?? {})),
    options.preview ?? options.save ?? successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

function savedData(fake: ReturnType<typeof createFakeTransport>) {
  const patch = fake.requests.find((request) => request.method === 'PATCH');
  return patch === undefined ? undefined : JSON.parse(patch.body ?? '{}');
}

function medallion(ability: string) {
  return screen.getByTestId(`sheet-ability-${ability}`);
}

function rollButton(label: string) {
  return screen.queryByRole('button', { name: rollName(label) });
}

function scoreButton(label: string) {
  return screen.getByRole('button', { name: label });
}

async function editScore(user: ReturnType<typeof userEvent.setup>, label: string, text: string) {
  await user.click(scoreButton(label));
  const input = screen.getByRole('textbox', { name: label });
  await user.clear(input);
  if (text !== '') {
    await user.type(input, `${text}{Enter}`);
  } else {
    await user.keyboard('{Enter}');
  }
}

async function save(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: message('sheet.save') }));
}

describe('ability medallions', () => {
  it('Edge: the six abilities each draw a roll target and a separate score target', async () => {
    await openFantasy();

    const abilities = screen.getAllByTestId(/^sheet-ability-/);
    expect(abilities).toHaveLength(6);

    const strength = medallion('strength');
    expect(within(strength).getByText('Strength')).toBeInTheDocument();
    expect(within(strength).getByRole('button', { name: 'Strength' })).toHaveTextContent('16');
    expect(
      within(strength).getByRole('button', { name: rollName('Strength modifier') }),
    ).toHaveTextContent('+3');
    expect(
      within(medallion('wisdom')).getByRole('button', { name: rollName('Wisdom modifier') }),
    ).toHaveTextContent('-1');
    expect(
      within(medallion('dexterity')).getByRole('button', { name: 'Dexterity' }),
    ).toHaveTextContent('14');
  });

  it('Edge: the modifier is not an editable control', async () => {
    const { user } = await openFantasy();

    expect(screen.queryByRole('button', { name: 'Strength modifier' })).not.toBeInTheDocument();
    await user.click(rollButton('Strength modifier') as HTMLElement);

    expect(screen.queryByRole('textbox', { name: 'Strength modifier' })).not.toBeInTheDocument();
    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
  });

  it('Edge: a missing modifier shows the placeholder and offers no roll', async () => {
    await openFantasy({
      data: fantasySheetData({ modifiers: { strengthMod: null } }),
    });

    const strength = medallion('strength');
    expect(strength).toHaveTextContent(message('sheet.play.noValue'));
    expect(
      within(strength).queryByRole('button', { name: rollName('Strength modifier') }),
    ).toBeNull();
  });

  it('Edge: a score with rows turned off keeps its modifier but loses the roll target', async () => {
    await openFantasy({
      dice: { section: {}, row: { 'stats.strength': null } },
    });

    const strength = medallion('strength');
    expect(rollButton('Strength modifier')).toBeNull();
    expect(strength).toHaveTextContent('+3');
    expect(within(strength).getByRole('button', { name: 'Strength' })).toBeInTheDocument();
  });

  it('Error: a template that marks the score non-editable keeps the stored score and writes nothing', async () => {
    const fields = fantasyFieldTree();
    const stats = fields.stats as {
      children: Record<string, { schema: Record<string, unknown> }>;
    };
    stats.children.strength = {
      schema: { ...stats.children.strength.schema, is_editable: false },
    };
    const { fake, user } = await openFantasy({ template: { fields } });

    await editScore(user, 'Strength', '18');

    expect(scoreButton('Strength')).toHaveTextContent('16');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Error: a failed preview keeps the score pending and shows the mapped error', async () => {
    const { fake, user } = await openFantasy({
      preview: errorOutcome('INTERNAL_ERROR', 'boom', 500),
    });

    await editScore(user, 'Strength', '12');
    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: message('sheet.pending.preview'),
      }),
    );

    await waitFor(() =>
      expect(
        within(screen.getByTestId('sheet-pending-card')).getByTestId('sheet-pending-preview-error'),
      ).toHaveTextContent(message('api.error.serverError')),
    );
    expect(scoreButton('Strength')).toHaveTextContent('12');
    expect(savedData(fake)).toBeUndefined();
  });

  it('Happy path: clicking the modifier rolls it through the dice store', async () => {
    const { user } = await openFantasy();

    await user.click(rollButton('Strength modifier') as HTMLElement);

    await waitFor(() => expect(document.querySelector('.lg-dice-panel')).not.toBeNull());
    expect(document.querySelector('.lg-dice-panel__total-label')).toHaveTextContent(
      'Strength modifier',
    );
    const total = Number(document.querySelector('.lg-dice-panel__total-value')?.textContent);
    expect(total).toBeGreaterThanOrEqual(4);
    expect(total).toBeLessThanOrEqual(23);
  });

  it('Happy path: a score edit goes to the pending delta and Save sends only that score', async () => {
    const { fake, user } = await openFantasy();

    await editScore(user, 'Strength', '18');

    expect(scoreButton('Strength')).toHaveTextContent('18');
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Strength 16 → 18');
    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { stats: { strength: 18 } } }));
  });

  it('Happy path: Preview updates the derived modifier and keeps the score unsaved', async () => {
    const { fake, user } = await openFantasy({
      preview: successOutcome({ modifiers: { strengthMod: 1 } }),
    });

    await editScore(user, 'Strength', '12');
    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: message('sheet.pending.preview'),
      }),
    );

    await waitFor(() =>
      expect(
        within(medallion('strength')).getByRole('button', {
          name: rollName('Strength modifier'),
        }),
      ).toHaveTextContent('+1'),
    );
    expect(scoreButton('Strength')).toHaveTextContent('12');
    expect(screen.getByTestId('sheet-pending-card')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Strength 16 → 12');
    expect(fake.requests.some((request) => request.url.endsWith('/preview'))).toBe(true);
    expect(savedData(fake)).toBeUndefined();
  });

  it('Happy path: passive awareness sits as a pill under the ability grid', async () => {
    await openFantasy();

    const section = screen.getByRole('region', { name: 'Stats' });
    const grid = within(section).getByRole('list');
    const passive = within(section).getByTestId('sheet-play-passive');

    expect(within(grid).getAllByRole('listitem')).toHaveLength(6);
    expect(passive).toHaveTextContent('14 Passive awareness');
    expect(grid.nextElementSibling).toBe(passive);
  });

  it('Edge: a missing passive awareness shows the placeholder', async () => {
    await openFantasy({
      data: fantasySheetData({ derived: { passiveAwareness: null } }),
    });

    expect(screen.getByTestId('sheet-play-passive')).toHaveTextContent(
      message('sheet.play.noValue'),
    );
  });
});

describe('the score badge', () => {
  it('Edge: a text that never resolves leaves the score and the delta alone', async () => {
    const { fake, user } = await openFantasy();

    await user.click(scoreButton('Strength'));
    const input = screen.getByRole('textbox', { name: 'Strength' });
    await user.clear(input);
    await user.type(input, 'abc{Enter}');

    expect(scoreButton('Strength')).toHaveTextContent('16');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Edge: committing an empty text keeps the stored score', async () => {
    const { fake, user } = await openFantasy();

    await editScore(user, 'Strength', '');

    expect(scoreButton('Strength')).toHaveTextContent('16');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Edge: a score beyond the range clamps before it reaches the delta', async () => {
    const { fake, user } = await openFantasy();

    await editScore(user, 'Strength', '25');

    expect(scoreButton('Strength')).toHaveTextContent('20');
    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { stats: { strength: 20 } } }));
  });

  it('Edge: Escape cancels the edit and leaves no pending change', async () => {
    const { fake, user } = await openFantasy();

    await user.click(scoreButton('Strength'));
    const input = screen.getByRole('textbox', { name: 'Strength' });
    await user.clear(input);
    await user.type(input, '18{Escape}');

    expect(scoreButton('Strength')).toHaveTextContent('16');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Happy path: a score correction back to the stored value clears the card', async () => {
    const { user } = await openFantasy();

    await editScore(user, 'Strength', '18');
    expect(screen.getByTestId('sheet-pending-card')).toBeInTheDocument();

    await editScore(user, 'Strength', '16');

    expect(screen.queryByTestId('sheet-pending-card')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
  });
});
