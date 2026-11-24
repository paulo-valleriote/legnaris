import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, screen, within } from '@testing-library/react';
import type { TemplateDetail } from '../api/models/template';
import { parseTemplateDetail } from '../api/models/template';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';
import characterGolden from '../../../testdata/templates/fifth_character_v1.json';
import statBlockGolden from '../../../testdata/templates/fifth_stat_block_v1.json';

const BASE_URL = 'http://api.test';
const CHARACTER_SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd41';
const STAT_BLOCK_SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd42';
const RECORD_ID = 'r0123456789abcdef0123456789abcdef';

interface Golden {
  readonly data: unknown;
}

function goldenTemplate(golden: Golden): TemplateDetail {
  return parseTemplateDetail(golden.data);
}

function openSheet(
  sheetId: string,
  templateId: string,
  golden: Golden,
  data: Record<string, unknown>,
  kind: 'character' | 'npc',
) {
  const fake = createFakeTransport(
    successOutcome({
      id: sheetId,
      template_id: templateId,
      kind,
      name: kind === 'character' ? 'Brenna' : 'Gray Ooze',
      data,
      dice: { section: {}, row: {} },
      created_at: '2026-11-20T10:00:00Z',
      updated_at: '2026-11-20T10:00:00Z',
    }),
    successOutcome(golden.data),
    successOutcome({}),
  );
  return renderApp(`/sheets/${sheetId}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
}

function row(key: string): HTMLElement {
  const found = document.querySelector<HTMLElement>(`[data-row-key="${key}"]`);
  if (found === null) {
    throw new Error(`row ${key} is not drawn`);
  }
  return found;
}

function recordRows(): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>('[data-row-kind="record"]'));
}

function characterData(): Record<string, unknown> {
  return {
    identity: { name: 'Brenna', level: 5 },
    abilities: { dexterity: { score: 14 } },
    exhaustion: { level: 2 },
    inventory: {
      items: {
        [RECORD_ID]: {
          name: 'Rope',
          quantity: 2,
          weight: 1,
          attuned: 1,
          notes: '',
          lineWeight: 2,
          position: 0,
        },
      },
    },
  };
}

function statBlockData(): Record<string, unknown> {
  return {
    identity: { name: 'Gray Ooze', challengeRating: 2 },
    hitPoints: { count: 2, size: 20, current: 30 },
    traits: {
      [RECORD_ID]: { name: 'Amorphous', text: 'It can move through a space as narrow as 1 inch.' },
    },
  };
}

afterEach(() => {
  cleanup();
});

describe('the real 5E Character DTO', () => {
  it('Happy path: the golden DTO decodes as a draft fifth-system character Template', () => {
    const template = goldenTemplate(characterGolden);

    expect(template.id).toBe('fifth_character_v1');
    expect(template.name).toBe('5E Character');
    expect(template.kinds).toEqual(['character']);
    expect(template.gameSystem?.id).toBe('fifth');
    expect(template.actions?.map((action) => action.label)).toEqual(['Long rest', 'Short rest']);
    expect(template.dice?.notD20?.section).toEqual(['hitDice']);
    expect(Object.keys(template.fields)).toContain('inventory');
  });

  it('Happy path: the character opens in Play and draws its pieces', async () => {
    openSheet(
      CHARACTER_SHEET_ID,
      'fifth_character_v1',
      characterGolden,
      characterData(),
      'character',
    );
    await screen.findByTestId('sheet-play');

    expect(screen.getByTestId('sheet-name')).toHaveTextContent('Brenna');
    expect(screen.getByTestId('compose-hit-points')).toBeInTheDocument();
    expect(screen.getByTestId('compose-ability-grid')).toBeInTheDocument();
    expect(screen.getByTestId('compose-save-list')).toBeInTheDocument();
    expect(screen.getByTestId('compose-skill-list')).toBeInTheDocument();
    expect(screen.getByTestId('compose-conditions')).toBeInTheDocument();

    expect(recordRows()).toHaveLength(0);
    const inventoryRow = screen.getByTestId('compose-record-row');
    expect(within(inventoryRow).getByRole('textbox', { name: 'Name' })).toHaveValue('Rope');
    expect(within(inventoryRow).getByRole('checkbox', { name: 'Attuned' })).toBeChecked();
  });

  it('Happy path: the Reset action bar draws Long rest and Short rest', async () => {
    openSheet(
      CHARACTER_SHEET_ID,
      'fifth_character_v1',
      characterGolden,
      characterData(),
      'character',
    );
    await screen.findByTestId('sheet-play');

    const bar = screen.getByRole('group', { name: message('sheet.reset.group') });
    expect(
      within(bar)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['Long rest', 'Short rest']);
  });

  it('Happy path: a d20 score offers a Roll mode and the Hit Die pips offer no roll', async () => {
    openSheet(
      CHARACTER_SHEET_ID,
      'fifth_character_v1',
      characterGolden,
      characterData(),
      'character',
    );
    await screen.findByTestId('sheet-play');

    expect(screen.getByRole('button', { name: /^Roll Dexterity modifier/ })).toHaveAttribute(
      'aria-haspopup',
      'dialog',
    );
    expect(screen.queryByRole('button', { name: /^Roll Primary Hit Dice/ })).toBeNull();
  });
});

describe('the real 5E Stat Block DTO through the generic form', () => {
  it('Happy path: the golden DTO decodes as a draft fifth-system stat block Template', () => {
    const template = goldenTemplate(statBlockGolden);

    expect(template.id).toBe('fifth_stat_block_v1');
    expect(template.name).toBe('5E Stat Block');
    expect(template.kinds).toEqual(['npc', 'enemy']);
    expect(template.gameSystem?.id).toBe('fifth');
    expect(template.actions).toBeUndefined();
    expect(template.dice?.notD20?.row).toEqual(['hitPoints.maximum']);
    expect(Object.keys(template.fields)).toContain('traits');
  });

  it('Happy path: every Section, toggle, rank and Record list draws', async () => {
    openSheet(STAT_BLOCK_SHEET_ID, 'fifth_stat_block_v1', statBlockGolden, statBlockData(), 'npc');
    await screen.findByTestId('sheet-name');

    for (const heading of [
      'Identity',
      'Abilities',
      'Saves',
      'Skills',
      'Exhaustion',
      'Passive',
      'Armor',
      'Experience',
      'Hit points',
      'Initiative',
      'Speed',
      'Details',
      'Traits',
      'Actions',
      'Bonus actions',
      'Reactions',
      'Legendary actions',
      'Resources',
    ]) {
      expect(screen.getByRole('heading', { name: heading })).toBeInTheDocument();
    }

    expect(screen.getByRole('checkbox', { name: 'XP 0' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'In lair' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Shield' })).toBeInTheDocument();

    expect(within(row('skills.arcana')).getByRole('textbox', { name: 'Arcana rank' })).toHaveValue(
      '0',
    );

    expect(recordRows()).toHaveLength(1);
    expect(
      within(recordRows()[0] as HTMLElement).getByRole('textbox', { name: 'Name' }),
    ).toHaveValue('Amorphous');
  });

  it('Edge: the stat block declares no Reset action, so no bar draws', async () => {
    openSheet(STAT_BLOCK_SHEET_ID, 'fifth_stat_block_v1', statBlockGolden, statBlockData(), 'npc');
    await screen.findByTestId('sheet-name');

    expect(screen.queryByRole('group', { name: message('sheet.reset.group') })).toBeNull();
  });

  it('Happy path: a d20 creature-Row offers a Roll mode while the Hit points Row offers none even for a d20 die', async () => {
    openSheet(STAT_BLOCK_SHEET_ID, 'fifth_stat_block_v1', statBlockGolden, statBlockData(), 'npc');
    await screen.findByTestId('sheet-name');

    const initiative = within(row('initiative.score')).getByRole('button', {
      name: /^Roll Initiative score/,
    });
    expect(initiative).toHaveAttribute('aria-haspopup', 'dialog');
    expect(initiative.textContent).toContain('d20');

    expect(
      within(row('hitPoints.maximum')).getByRole('button', { name: /^Roll Hit points/ }),
    ).not.toHaveAttribute('aria-haspopup', 'dialog');
  });
});
