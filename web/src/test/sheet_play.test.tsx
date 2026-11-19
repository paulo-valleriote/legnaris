import { describe, expect, it } from 'vitest';
import { cleanup, screen, waitFor } from '@testing-library/react';
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
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';

interface OpenFantasyOptions {
  readonly data?: Record<string, unknown>;
  readonly template?: Record<string, unknown>;
}

async function openFantasy(options: OpenFantasyOptions = {}) {
  const fake = createFakeTransport(
    successOutcome(fantasySheetPayload(options.data === undefined ? {} : { data: options.data })),
    successOutcome(fantasyTemplatePayload(options.template ?? {})),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

async function openPlain() {
  const fake = createFakeTransport(
    successOutcome(sheetPayload()),
    successOutcome(templateDetailPayload()),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

function savedData(fake: ReturnType<typeof createFakeTransport>) {
  const patch = fake.requests.find((request) => request.method === 'PATCH');
  return patch === undefined ? undefined : JSON.parse(patch.body ?? '{}');
}

async function save(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: message('sheet.save') }));
}

const hero = () => screen.getByTestId('sheet-play-hero');

describe('Play and Edit modes', () => {
  it('Edge: a template without a preset keeps the generic form and offers no mode toggle', async () => {
    await openPlain();

    expect(screen.getByRole('textbox', { name: 'Level' })).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-play-hero')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('sheet.mode.play') }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('sheet.mode.edit') }),
    ).not.toBeInTheDocument();
  });

  it('Edge: Play is the default for the preset and draws no generic Section or field', async () => {
    await openFantasy();

    expect(hero()).toBeInTheDocument();
    expect(screen.getByTestId('sheet-play')).toBeInTheDocument();
    expect(document.querySelector('[data-section-key]')).toBeNull();
    expect(screen.queryByRole('textbox', { name: 'Character name' })).not.toBeInTheDocument();
    expect(screen.queryByTestId('section-strip')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('arrange.start') }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('sheet.pinnedOnly') }),
    ).not.toBeInTheDocument();
  });

  it('Edge: switching modes sends no request and leaves pending edits alone', async () => {
    const { fake, user } = await openFantasy();
    expect(fake.attempts).toBe(2);

    await user.click(screen.getByRole('button', { name: message('sheet.mode.edit') }));
    expect(screen.getByRole('textbox', { name: 'Character name' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: message('sheet.mode.play') }));

    expect(fake.attempts).toBe(2);
    expect(hero()).toBeInTheDocument();
  });

  it('Happy path: Edit shows the generic form with Arrange and the pinned filter, and Play returns the hero', async () => {
    const { user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: message('sheet.mode.edit') }));

    expect(screen.getByRole('textbox', { name: 'Character name' })).toHaveValue('Aria');
    expect(screen.getByRole('button', { name: message('arrange.start') })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.pinnedOnly') })).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-play-hero')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: message('sheet.mode.play') }));
    expect(hero()).toBeInTheDocument();
  });
});

describe('the hero header', () => {
  it('Edge: an empty character name falls back to the sheet name, never to the empty-field copy', async () => {
    await openFantasy({ data: fantasySheetData({ characterName: '' }) });

    const title = screen.getByRole('button', { name: 'Character name' });
    expect(title).toHaveTextContent('Aragorn');
    expect(title).not.toHaveTextContent(
      messageWith('sheet.inlineAdd', { label: 'Character name' }),
    );
  });

  it('Edge: empty ancestry, origin and player fields read Add followed by the field label', async () => {
    await openFantasy({
      data: fantasySheetData({ ancestry: '', origin: '', playerName: '' }),
    });

    expect(screen.getByRole('button', { name: 'Ancestry' })).toHaveTextContent(
      messageWith('sheet.inlineAdd', { label: 'Ancestry' }),
    );
    expect(screen.getByRole('button', { name: 'Origin' })).toHaveTextContent(
      messageWith('sheet.inlineAdd', { label: 'Origin' }),
    );
    expect(screen.getByRole('button', { name: 'Player name' })).toHaveTextContent(
      messageWith('sheet.inlineAdd', { label: 'Player name' }),
    );
  });

  it('Edge: the level stepper stops at both ends of its range', async () => {
    await openFantasy({ data: fantasySheetData({ level: 20 }) });
    expect(screen.getByTestId('sheet-play-level')).toHaveTextContent(
      messageWith('sheet.play.level', { count: 20 }),
    );
    expect(
      screen.getByRole('button', { name: messageWith('engine.increase', { label: 'Level' }) }),
    ).toBeDisabled();
    expect(
      screen.getByRole('button', { name: messageWith('engine.decrease', { label: 'Level' }) }),
    ).toBeEnabled();

    cleanup();
    await openFantasy({ data: fantasySheetData({ level: 1 }) });
    expect(
      screen.getByRole('button', { name: messageWith('engine.decrease', { label: 'Level' }) }),
    ).toBeDisabled();
  });

  it('Error: a title field the template marks non-editable keeps its stored value and writes nothing', async () => {
    const fields = fantasyFieldTree();
    const identity = fields.identity as {
      children: Record<string, { schema: Record<string, unknown> }>;
    };
    identity.children.characterName = {
      schema: { ...identity.children.characterName.schema, is_editable: false },
    };
    const { fake, user } = await openFantasy({ template: { fields } });

    await user.click(screen.getByRole('button', { name: 'Character name' }));
    const input = screen.getByRole('textbox', { name: 'Character name' });
    await user.clear(input);
    await user.type(input, 'Brenna{Enter}');

    expect(screen.getByRole('button', { name: 'Character name' })).toHaveTextContent('Aria');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Happy path: the hero shows the character title, ancestry, origin, level, player and proficiency', async () => {
    await openFantasy();

    expect(screen.getByRole('button', { name: 'Character name' })).toHaveTextContent('Aria');
    expect(screen.getByRole('button', { name: 'Ancestry' })).toHaveTextContent('Elf');
    expect(screen.getByRole('button', { name: 'Origin' })).toHaveTextContent('Wanderer');
    expect(screen.getByTestId('sheet-play-level')).toHaveTextContent(
      messageWith('sheet.play.level', { count: 3 }),
    );
    expect(hero()).toHaveTextContent('played by Sam');
    expect(hero()).toHaveTextContent(messageWith('sheet.play.proficiency', { modifier: '+2' }));
  });
});

describe('the inline edit control', () => {
  it('Edge: Escape cancels the edit and leaves no pending change', async () => {
    const { fake, user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: 'Character name' }));
    const input = screen.getByRole('textbox', { name: 'Character name' });
    await user.clear(input);
    await user.type(input, 'Brenna{Escape}');

    expect(screen.getByRole('button', { name: 'Character name' })).toHaveTextContent('Aria');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Edge: blur commits the typed value', async () => {
    const { user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: 'Ancestry' }));
    const input = screen.getByRole('textbox', { name: 'Ancestry' });
    await user.clear(input);
    await user.type(input, 'Half-elf');
    await user.tab();

    expect(screen.getByRole('button', { name: 'Ancestry' })).toHaveTextContent('Half-elf');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeEnabled();
  });

  it('Edge: committing an empty value writes the empty value and shows the Add copy', async () => {
    const { fake, user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: 'Origin' }));
    const input = screen.getByRole('textbox', { name: 'Origin' });
    await user.clear(input);
    await user.keyboard('{Enter}');

    expect(screen.getByRole('button', { name: 'Origin' })).toHaveTextContent(
      messageWith('sheet.inlineAdd', { label: 'Origin' }),
    );
    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { identity: { origin: '' } } }));
  });

  it('Happy path: Enter commits the title, which writes identity.characterName only', async () => {
    const { fake, user } = await openFantasy();

    await user.click(screen.getByRole('button', { name: 'Character name' }));
    const input = screen.getByRole('textbox', { name: 'Character name' });
    expect(input).toHaveValue('Aria');
    await user.clear(input);
    await user.type(input, 'Brenna{Enter}');

    expect(screen.getByRole('button', { name: 'Character name' })).toHaveTextContent('Brenna');
    await save(user);
    await waitFor(() =>
      expect(savedData(fake)).toEqual({ data: { identity: { characterName: 'Brenna' } } }),
    );
  });

  it('Happy path: the level stepper writes the next level', async () => {
    const { fake, user } = await openFantasy();

    await user.click(
      screen.getByRole('button', { name: messageWith('engine.increase', { label: 'Level' }) }),
    );

    expect(screen.getByTestId('sheet-play-level')).toHaveTextContent(
      messageWith('sheet.play.level', { count: 4 }),
    );
    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { identity: { level: 4 } } }));
  });
});
