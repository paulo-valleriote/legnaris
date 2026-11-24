import { describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message } from '../l10n/strings';
import { parseSheet } from '../api/models/sheet';
import { parseTemplateDetail } from '../api/models/template';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import { configPathErrors } from '../sheet/preset/compose/config_paths';
import { fifthStatBlockConfig } from '../sheet/preset/compose/configs/fifth_stat_block';
import { PresetPlayView } from '../sheet/preset/play_view';
import { presetFor } from '../sheet/preset/registry';
import { renderPreset } from './preset_fixtures';
import statBlockGolden from '../../../testdata/templates/fifth_stat_block_v1.json';

const TEMPLATE_ID = 'fifth_stat_block_v1';
const SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd43';
const ACTION_ID = 'r0123456789abcdef0123456789abcdef';

const template = parseTemplateDetail(statBlockGolden.data);

function statBlockSheet() {
  return parseSheet({
    id: SHEET_ID,
    template_id: TEMPLATE_ID,
    kind: 'npc',
    name: 'Gray Ooze',
    data: {
      identity: { name: 'Gray Ooze', challengeRating: 2, proficiencyBonus: 2 },
      abilities: {
        strength: { score: 12, mod: 1 },
        dexterity: { score: 8, mod: -1 },
        constitution: { score: 16, mod: 3 },
        intelligence: { score: 1, mod: -5 },
        wisdom: { score: 6, mod: -2 },
        charisma: { score: 2, mod: -4 },
      },
      saves: {
        strength: { prof: 0, misc: 0, total: 1 },
        dexterity: { prof: 0, misc: 0, total: -1 },
        constitution: { prof: 1, misc: 0, total: 5 },
        intelligence: { prof: 0, misc: 0, total: -5 },
        wisdom: { prof: 0, misc: 0, total: -2 },
        charisma: { prof: 0, misc: 0, total: -4 },
      },
      skills: {
        perception: { rank: 0, misc: 0, total: -2 },
        stealth: { rank: 1, misc: 0, total: 1 },
      },
      exhaustion: { level: 0, penalty: 0 },
      passive: { perception: 8 },
      armor: { armorClass: 8 },
      experience: { xp: 450 },
      hitPoints: { count: 2, size: 20, conBonus: 0, maximum: 9, current: 7, temp: 0 },
      initiative: { modifier: -1, score: 9 },
      speed: {
        walk: 10,
        walkTotal: 10,
        fly: 0,
        flyTotal: 0,
        swim: 0,
        swimTotal: 0,
        climb: 0,
        climbTotal: 0,
        burrow: 0,
        burrowTotal: 0,
      },
      details: {
        resistances: 'Acid, cold',
        immunities: '',
        senses: 'Blindsight 60 ft.',
        languages: 'None',
      },
      traits: {
        [ACTION_ID]: {
          name: 'Amorphous',
          text: 'It can move through a space as narrow as 1 inch.',
          position: 0,
        },
      },
      actions: {
        [ACTION_ID]: {
          name: 'Pseudopod',
          abilityStrength: 1,
          abilityDexterity: 0,
          abilityConstitution: 0,
          abilityIntelligence: 0,
          abilityWisdom: 0,
          abilityCharisma: 0,
          proficient: 1,
          misc: 0,
          toHit: 3,
          damage: '1d6 + 1',
          notes: '',
          position: 0,
        },
      },
      bonusActions: {},
      reactions: {},
      legendaryActions: {},
      resources: {},
    },
    created_at: '2026-11-20T10:00:00Z',
    updated_at: '2026-11-20T10:00:00Z',
  });
}

function play() {
  const preset = presetFromConfig(fifthStatBlockConfig);
  return renderPreset(
    <>
      <preset.Header />
      <PresetPlayView preset={preset} />
    </>,
    { template, sheet: statBlockSheet() },
  );
}

describe('the 5E Stat Block preset config', () => {
  it('Happy path: every bound path exists in the decoded golden', () => {
    expect(configPathErrors(fifthStatBlockConfig, template)).toEqual([]);
  });

  it('Happy path: the registry resolves the golden Template id to the config', () => {
    expect(presetFor(template)?.templateId).toBe(TEMPLATE_ID);
  });

  it('Happy path: Play mode draws the header and every placed piece', () => {
    play();

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Gray Ooze');
    expect(screen.getByText('Challenge rating')).toBeInTheDocument();
    expect(screen.getByText('Experience points')).toBeInTheDocument();
    expect(screen.getByText('Armor class')).toBeInTheDocument();
    expect(screen.getByText('Passive Perception')).toBeInTheDocument();
    expect(screen.getByTestId('compose-hit-points')).toBeInTheDocument();
    expect(screen.getByTestId('compose-ability-grid')).toBeInTheDocument();
    expect(screen.getByTestId('compose-save-list')).toBeInTheDocument();
    expect(screen.getByTestId('compose-skill-list')).toBeInTheDocument();
    expect(screen.getByTestId('compose-conditions')).toBeInTheDocument();
    expect(screen.getByTestId('compose-text-fields')).toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Traits' })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Actions' })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Bonus actions' })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Reactions' })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Legendary actions' })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Resources' })).toBeInTheDocument();
    expect(screen.getByTestId('compose-roll-mode')).toBeInTheDocument();
    expect(document.querySelector('.lg-preset__columns--compose')).not.toBeNull();
  });

  it('Edge: the hit point maximum roll offers no Roll mode', () => {
    play();

    const maximum = within(screen.getByTestId('compose-hit-points')).getByRole('button', {
      name: /^Roll Hit points/,
    });

    expect(maximum).not.toHaveAttribute('aria-haspopup', 'dialog');
    expect(screen.getByRole('button', { name: /^Roll Initiative score/ })).toHaveAttribute(
      'aria-haspopup',
      'dialog',
    );
  });

  it('Edge: a non-d20 roll keeps the chosen Roll mode', async () => {
    const user = userEvent.setup();
    play();

    const advantage = screen.getByRole('button', { name: message('dice.mode.advantage') });
    await user.click(advantage);
    expect(advantage).toHaveAttribute('aria-pressed', 'true');

    await user.click(
      within(screen.getByTestId('compose-hit-points')).getByRole('button', {
        name: /^Roll Hit points/,
      }),
    );

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
    expect(advantage).toHaveAttribute('aria-pressed', 'true');
  });

  it('Happy path: an ability modifier rolls from its bound Row', async () => {
    const user = userEvent.setup();
    play();

    await user.click(
      within(screen.getByTestId('compose-ability-abilities.dexterity.score')).getByRole('button', {
        name: /^Roll Dexterity modifier/,
      }),
    );

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
  });

  it('Happy path: a save total rolls from its bound Row', async () => {
    const user = userEvent.setup();
    play();

    await user.click(
      within(screen.getByTestId('compose-save-saves.dexterity.total')).getByRole('button', {
        name: /^Roll Dexterity save/,
      }),
    );

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
  });

  it('Happy path: a skill total rolls from its bound Row', async () => {
    const user = userEvent.setup();
    play();

    await user.click(
      within(screen.getByTestId('compose-skill-skills.perception.rank')).getByRole('button', {
        name: /^Roll Perception/,
      }),
    );

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
  });

  it('Happy path: an action Record rolls its to-hit', async () => {
    const user = userEvent.setup();
    play();

    const actions = screen.getByRole('group', { name: 'Actions' });
    const roll = within(actions).getByRole('button', { name: /^Roll/ });
    await user.click(roll);

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
    expect(roll).toHaveAttribute('aria-haspopup', 'dialog');
  });
});
