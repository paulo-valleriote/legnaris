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
  readonly preview?: FakeOutcome;
  readonly save?: FakeOutcome;
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

function saveRow(ability: string) {
  return screen.getByTestId(`sheet-save-${ability}`);
}

function skillRow(skill: string) {
  return screen.getByTestId(`sheet-skill-${skill}`);
}

function dot(label: string) {
  return screen.getByRole('button', {
    name: message('sheet.play.proficiencyFor').replace('{label}', label),
  });
}

function previewRequest(fake: ReturnType<typeof createFakeTransport>) {
  return fake.requests.find((request) => request.url.endsWith('/preview'));
}

function previewBody(fake: ReturnType<typeof createFakeTransport>) {
  const request = previewRequest(fake);
  return request === undefined ? undefined : JSON.parse(request.body ?? '{}');
}

function savedData(fake: ReturnType<typeof createFakeTransport>) {
  const patch = fake.requests.find((request) => request.method === 'PATCH');
  return patch === undefined ? undefined : JSON.parse(patch.body ?? '{}');
}

async function save(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: message('sheet.save') }));
}

async function editBonus(user: ReturnType<typeof userEvent.setup>, label: string, text: string) {
  await user.click(screen.getByRole('button', { name: label }));
  const input = screen.getByRole('textbox', { name: label });
  await user.clear(input);
  if (text !== '') {
    await user.type(input, `${text}{Enter}`);
  } else {
    await user.keyboard('{Enter}');
  }
}

describe('saving throws', () => {
  it('Edge: the six abilities draw a dot, a name and a rollable total', async () => {
    await openFantasy();

    const section = screen.getByTestId('sheet-play-saves');
    expect(within(section).getAllByTestId(/^sheet-save-/)).toHaveLength(6);
    expect(within(section).getByText('Strength save')).toBeInTheDocument();
    expect(dot('Strength save')).toHaveAttribute('aria-pressed', 'false');
    expect(dot('Dexterity save')).toHaveAttribute('aria-pressed', 'true');
    expect(
      within(saveRow('strength')).getByRole('button', { name: rollName('Strength save') }),
    ).toHaveTextContent('+3');
    expect(
      within(saveRow('dexterity')).getByRole('button', { name: rollName('Dexterity save') }),
    ).toHaveTextContent('+4');
  });

  it('Edge: a missing total shows the placeholder and offers no roll', async () => {
    await openFantasy({ data: fantasySheetData({ saves: { strength: null } }) });

    expect(saveRow('strength')).toHaveTextContent(message('sheet.play.noValue'));
    expect(
      within(saveRow('strength')).queryByRole('button', { name: rollName('Strength save') }),
    ).toBeNull();
  });

  it('Edge: a save with its row dice turned off keeps the total and loses the roll target', async () => {
    await openFantasy({ dice: { section: {}, row: { 'saves.strength': null } } });

    expect(
      within(saveRow('strength')).queryByRole('button', { name: rollName('Strength save') }),
    ).toBeNull();
    expect(saveRow('strength')).toHaveTextContent('+3');
  });

  it('Edge: a template without the config leaves leaves the dot unpressed and writes nothing', async () => {
    const fields = fantasyFieldTree();
    delete fields.config;
    const { fake, user } = await openFantasy({ template: { fields } });

    await user.click(dot('Strength save'));

    expect(dot('Strength save')).toHaveAttribute('aria-pressed', 'false');
    expect(fake.requests.some((request) => request.url.endsWith('/preview'))).toBe(false);
    expect(fake.requests.some((request) => request.method === 'PATCH')).toBe(false);
  });

  it('Error: a failed preview keeps the dot pressed and shows the mapped error', async () => {
    const { user } = await openFantasy({
      preview: errorOutcome('INTERNAL_ERROR', 'boom', 500),
    });

    await user.click(dot('Strength save'));

    await waitFor(() =>
      expect(screen.getByTestId('sheet-pending-preview-error')).toHaveTextContent(
        message('api.error.serverError'),
      ),
    );
    expect(dot('Strength save')).toHaveAttribute('aria-pressed', 'true');
  });

  it('Happy path: an unpressed dot writes 1 to its config leaf and previews the total', async () => {
    const { fake, user } = await openFantasy({
      preview: successOutcome({ saves: { strength: 6 } }),
    });

    await user.click(dot('Strength save'));

    await waitFor(() =>
      expect(
        within(saveRow('strength')).getByRole('button', { name: rollName('Strength save') }),
      ).toHaveTextContent('+6'),
    );
    expect(dot('Strength save')).toHaveAttribute('aria-pressed', 'true');
    expect(previewBody(fake)).toEqual({ data: { config: { saveStrengthProf: 1 } } });
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent(
      'Strength save proficiency 0 → 1',
    );
  });

  it('Happy path: a pressed dot writes 0 and previews the new total', async () => {
    const { fake, user } = await openFantasy({
      preview: successOutcome({ saves: { dexterity: 2 } }),
    });

    await user.click(dot('Dexterity save'));

    await waitFor(() =>
      expect(
        within(saveRow('dexterity')).getByRole('button', { name: rollName('Dexterity save') }),
      ).toHaveTextContent('+2'),
    );
    expect(dot('Dexterity save')).toHaveAttribute('aria-pressed', 'false');
    expect(previewBody(fake)).toEqual({ data: { config: { saveDexterityProf: 0 } } });
  });

  it('Happy path: clicking a save total rolls it through the dice store', async () => {
    const { user } = await openFantasy();

    await user.click(
      within(saveRow('strength')).getByRole('button', { name: rollName('Strength save') }),
    );

    await waitFor(() => expect(document.querySelector('.lg-dice-panel')).not.toBeNull());
    expect(document.querySelector('.lg-dice-panel__total-label')).toHaveTextContent(
      'Strength save',
    );
    const total = Number(document.querySelector('.lg-dice-panel__total-value')?.textContent);
    expect(total).toBeGreaterThanOrEqual(4);
    expect(total).toBeLessThanOrEqual(23);
  });
});

describe('skills', () => {
  it('Edge: the twelve skills draw a dot, the name, its ability abbreviation and a total', async () => {
    await openFantasy();

    const section = screen.getByTestId('sheet-play-skills');
    expect(within(section).getAllByTestId(/^sheet-skill-/)).toHaveLength(12);

    expect(skillRow('athletics')).toHaveTextContent('Athletics');
    expect(within(skillRow('athletics')).getByText('STR')).toBeInTheDocument();
    expect(within(skillRow('stealth')).getByText('DEX')).toBeInTheDocument();
    expect(within(skillRow('perception')).getByText('WIS')).toBeInTheDocument();
    expect(within(skillRow('persuasion')).getByText('CHA')).toBeInTheDocument();
    expect(within(skillRow('knowledge')).getByText('INT')).toBeInTheDocument();
    expect(
      within(skillRow('athletics')).getByRole('button', { name: rollName('Athletics') }),
    ).toHaveTextContent('+5');
    expect(
      within(skillRow('stealth')).getByRole('button', { name: rollName('Stealth') }),
    ).toHaveTextContent('+3');
  });

  it('Edge: a skill total missing shows the placeholder and offers no roll', async () => {
    await openFantasy({
      data: fantasySheetData({ skills: { athletics: { bonus: 0, total: null } } }),
    });

    expect(skillRow('athletics')).toHaveTextContent(message('sheet.play.noValue'));
    expect(
      within(skillRow('athletics')).queryByRole('button', { name: rollName('Athletics') }),
    ).toBeNull();
  });

  it('Edge: a skill with its row dice turned off keeps the total and loses the roll target', async () => {
    await openFantasy({ dice: { section: {}, row: { 'skills.athletics': null } } });

    expect(
      within(skillRow('athletics')).queryByRole('button', { name: rollName('Athletics') }),
    ).toBeNull();
    expect(skillRow('athletics')).toHaveTextContent('+5');
  });

  it('Edge: a bonus text that never resolves leaves the bonus and the delta alone', async () => {
    const { fake, user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: 'Athletics bonus' }));
    const input = screen.getByRole('textbox', { name: 'Athletics bonus' });
    await user.clear(input);
    await user.type(input, 'abc{Enter}');

    expect(screen.getByRole('button', { name: 'Athletics bonus' })).toHaveTextContent('+0');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Edge: committing an empty bonus keeps the stored bonus', async () => {
    const { fake, user } = await openFantasy();

    await editBonus(user, 'Athletics bonus', '');

    expect(screen.getByRole('button', { name: 'Athletics bonus' })).toHaveTextContent('+0');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Edge: a bonus beyond the range clamps before it reaches the delta', async () => {
    const { fake, user } = await openFantasy();

    await editBonus(user, 'Athletics bonus', '30');

    expect(screen.getByRole('button', { name: 'Athletics bonus' })).toHaveTextContent('+20');
    await save(user);
    await waitFor(() =>
      expect(savedData(fake)).toEqual({ data: { skills: { athletics: { bonus: 20 } } } }),
    );
  });

  it('Edge: Escape cancels the bonus edit and leaves no pending change', async () => {
    const { fake, user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: 'Athletics bonus' }));
    const input = screen.getByRole('textbox', { name: 'Athletics bonus' });
    await user.clear(input);
    await user.type(input, '4{Escape}');

    expect(screen.getByRole('button', { name: 'Athletics bonus' })).toHaveTextContent('+0');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Happy path: a bonus edit goes to the pending delta and Save sends only that bonus', async () => {
    const { fake, user } = await openFantasy();

    await editBonus(user, 'Athletics bonus', '4');

    expect(screen.getByRole('button', { name: 'Athletics bonus' })).toHaveTextContent('+4');
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Athletics bonus 0 → 4');
    await save(user);
    await waitFor(() =>
      expect(savedData(fake)).toEqual({ data: { skills: { athletics: { bonus: 4 } } } }),
    );
  });

  it('Happy path: Preview updates the skill total and keeps the bonus unsaved', async () => {
    const { fake, user } = await openFantasy({
      preview: successOutcome({ skills: { athletics: { total: 9 } } }),
    });

    await editBonus(user, 'Athletics bonus', '4');
    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: message('sheet.pending.preview'),
      }),
    );

    await waitFor(() =>
      expect(
        within(skillRow('athletics')).getByRole('button', { name: rollName('Athletics') }),
      ).toHaveTextContent('+9'),
    );
    expect(screen.getByRole('button', { name: 'Athletics bonus' })).toHaveTextContent('+4');
    expect(savedData(fake)).toBeUndefined();
  });

  it('Happy path: clicking a skill total rolls the section dice with the skill total', async () => {
    const { user } = await openFantasy();

    await user.click(
      within(skillRow('athletics')).getByRole('button', { name: rollName('Athletics') }),
    );

    await waitFor(() => expect(document.querySelector('.lg-dice-panel')).not.toBeNull());
    expect(document.querySelector('.lg-dice-panel__total-label')).toHaveTextContent('Athletics');
    const total = Number(document.querySelector('.lg-dice-panel__total-value')?.textContent);
    expect(total).toBeGreaterThanOrEqual(6);
    expect(total).toBeLessThanOrEqual(25);
  });
});

describe('config in Play mode', () => {
  it('Edge: the Config section and its proficiency leaves are not rendered', async () => {
    await openFantasy();

    expect(screen.queryByRole('region', { name: 'Config' })).toBeNull();
    expect(screen.queryByText('Strength save proficiency')).toBeNull();
    expect(screen.queryByLabelText('Athletics proficiency')).toBeNull();
  });

  it('Happy path: saving throws sit between the abilities and skills in the Play view', async () => {
    await openFantasy();

    const play = screen.getByTestId('sheet-play');
    const sections = Array.from(play.children).map((element) =>
      element.getAttribute('data-testid'),
    );
    expect(sections.indexOf('sheet-play-saves')).toBeLessThan(
      sections.indexOf('sheet-play-skills'),
    );
    expect(sections).toContain('sheet-play-saves');
  });
});
