import { describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message } from '../l10n/strings';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import { AbilityGrid } from '../sheet/preset/compose/ability_grid';
import { configPathErrors } from '../sheet/preset/compose/config_paths';
import { SaveList } from '../sheet/preset/compose/save_list';
import {
  formatSigned,
  nextProficiency,
  nextRank,
  parseScore,
  rankValue,
} from '../sheet/preset/compose/scores_data';
import { SkillList } from '../sheet/preset/compose/skill_list';
import type {
  PresetAbilityGridPiece,
  PresetConfig,
  PresetSaveListPiece,
  PresetSkillListPiece,
} from '../sheet/preset/compose/types';
import { PresetPlayView } from '../sheet/preset/play_view';
import { presetConfig, presetSheet, presetTemplateDetail, renderPreset } from './preset_fixtures';

const MINUS = '\u2212';

function abilityGrid(
  abilities: readonly { readonly score: string; readonly modifier: string }[],
): PresetAbilityGridPiece {
  return { kind: 'abilityGrid', key: 'abilities', section: 'abilities', abilities };
}

function saveList(
  saves: readonly { readonly proficiency: string; readonly total: string }[],
): PresetSaveListPiece {
  return { kind: 'saveList', key: 'saves', section: 'saves', saves };
}

function skillList(
  skills: readonly {
    readonly rank: string;
    readonly total: string;
    readonly ability?: string;
  }[],
  max: string | number = 3,
): PresetSkillListPiece {
  return { kind: 'skillList', key: 'skills', section: 'skills', skills, max };
}

const might = { score: 'abilities.might', modifier: 'abilities.mightModifier' };
const agility = { score: 'abilities.agility', modifier: 'abilities.agilityModifier' };
const reflex = { proficiency: 'saves.reflexProf', total: 'saves.reflex' };
const will = { proficiency: 'saves.willProf', total: 'saves.will' };
const athletics = { rank: 'skills.athleticsRank', total: 'skills.athleticsTotal' };
const stealth = { rank: 'skills.stealthRank', total: 'skills.stealthTotal', ability: 'DEX' };

describe('the score helpers', () => {
  it('Edge: formatSigned marks a positive and a negative modifier', () => {
    expect(formatSigned(3)).toBe('+3');
    expect(formatSigned(0)).toBe('+0');
    expect(formatSigned(-2)).toBe(`${MINUS}2`);
    expect(formatSigned(-0)).toBe('+0');
  });

  it('Edge: formatSigned refuses a value that is not a finite number', () => {
    expect(formatSigned(Number.NaN)).toBe('');
    expect(formatSigned(Number.POSITIVE_INFINITY)).toBe('');
  });

  it('Edge: parseScore accepts a whole number and rejects everything else', () => {
    expect(parseScore('16')).toBe(16);
    expect(parseScore(' -2 ')).toBe(-2);
    expect(parseScore('')).toBeUndefined();
    expect(parseScore('abc')).toBeUndefined();
    expect(parseScore('12.5')).toBeUndefined();
    expect(parseScore('1e999')).toBeUndefined();
  });

  it('Edge: nextProficiency flips any non-zero flag to zero and back', () => {
    expect(nextProficiency(0)).toBe(1);
    expect(nextProficiency(undefined)).toBe(1);
    expect(nextProficiency(1)).toBe(0);
    expect(nextProficiency(2)).toBe(0);
  });

  it('Edge: rankValue clamps a stored rank into the 0..max range', () => {
    expect(rankValue(2, 3)).toBe(2);
    expect(rankValue(9, 3)).toBe(3);
    expect(rankValue(-1, 3)).toBe(0);
    expect(rankValue('2', 3)).toBe(0);
    expect(rankValue(undefined, 3)).toBe(0);
  });

  it('Edge: nextRank wraps at the maximum and stays at zero without one', () => {
    expect(nextRank(0, 3)).toBe(1);
    expect(nextRank(2, 3)).toBe(3);
    expect(nextRank(3, 3)).toBe(0);
    expect(nextRank(0, 0)).toBe(0);
    expect(nextRank(9, 3)).toBe(0);
  });
});

describe('the ability grid piece', () => {
  it('Happy path: each bound ability draws its score label, score and signed modifier', () => {
    renderPreset(<AbilityGrid piece={abilityGrid([might, agility])} />);

    const row = screen.getByTestId('compose-ability-abilities.might');
    expect(within(row).getByText('Might')).toBeInTheDocument();
    expect(within(row).getByRole('button', { name: 'Might' })).toHaveTextContent('16');
    expect(within(row).getByRole('button', { name: /Might Modifier/ })).toHaveTextContent('+3');
    expect(
      within(screen.getByTestId('compose-ability-abilities.agility')).getByRole('button', {
        name: /Agility Modifier/,
      }),
    ).toHaveTextContent('+1');
  });

  it('Edge: a negative modifier uses the typographic minus', () => {
    renderPreset(<AbilityGrid piece={abilityGrid([agility])} />, {
      sheet: presetSheet({
        data: { abilities: { agility: 8, agilityModifier: -1 } },
      }),
    });

    expect(screen.getByRole('button', { name: /Agility Modifier/ })).toHaveTextContent(`${MINUS}1`);
  });

  it('Edge: no bound ability draws nothing', () => {
    renderPreset(<AbilityGrid piece={abilityGrid([])} />);

    expect(screen.queryByTestId('compose-ability-grid')).not.toBeInTheDocument();
  });

  it('Edge: a missing modifier draws the shared copy and offers no roll', () => {
    renderPreset(<AbilityGrid piece={abilityGrid([might])} />, {
      sheet: presetSheet({ data: { abilities: { might: 16, mightModifier: null } } }),
    });

    const row = screen.getByTestId('compose-ability-abilities.might');
    expect(row).toHaveTextContent(message('sheet.play.noValue'));
    expect(within(row).queryByRole('button', { name: /Might Modifier/ })).not.toBeInTheDocument();
  });

  it('Happy path: the score commits an inline edit through the store', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<AbilityGrid piece={abilityGrid([might])} />);

    await user.click(screen.getByRole('button', { name: 'Might' }));
    const input = screen.getByRole('textbox', { name: 'Might' });
    await user.clear(input);
    await user.type(input, '18{Enter}');

    expect(store.getPendingDelta()).toEqual({ abilities: { might: 18 } });
  });

  it('Edge: an invalid score text commits nothing', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<AbilityGrid piece={abilityGrid([might])} />);

    await user.click(screen.getByRole('button', { name: 'Might' }));
    const input = screen.getByRole('textbox', { name: 'Might' });
    await user.clear(input);
    await user.type(input, 'eight{Enter}');

    expect(store.getPendingDelta()).toEqual({});
  });

  it('Happy path: the modifier rolls from a pointer click and the keyboard', async () => {
    const user = userEvent.setup();
    renderPreset(<AbilityGrid piece={abilityGrid([might])} />);

    await user.click(screen.getByRole('button', { name: /Might Modifier/ }));
    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();

    await user.click(screen.getByRole('button', { name: /close/i }));
    screen.getByRole('button', { name: /Might Modifier/ }).focus();
    await user.keyboard('{Enter}');

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
  });

  it('Happy path: the bound Section dice offer the Roll mode menu from the keyboard', async () => {
    const user = userEvent.setup();
    renderPreset(<AbilityGrid piece={abilityGrid([might])} />);

    const button = screen.getByRole('button', { name: /Might Modifier/ });
    expect(button).toHaveAttribute('aria-haspopup', 'dialog');
    button.focus();
    await user.keyboard('{Shift>}{F10}{/Shift}');

    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByRole('button', { name: message('dice.mode.normal') })).toBeVisible();
    expect(
      within(dialog).getByRole('button', { name: message('dice.mode.advantage') }),
    ).toBeVisible();
    expect(
      within(dialog).getByRole('button', { name: message('dice.mode.disadvantage') }),
    ).toBeVisible();
  });
});

describe('the save list piece', () => {
  it('Happy path: each bound save draws its label, total and proficiency flag', () => {
    renderPreset(<SaveList piece={saveList([reflex, will])} />);

    const row = screen.getByTestId('compose-save-saves.reflex');
    expect(within(row).getByText('Reflex')).toBeInTheDocument();
    expect(within(row).getByRole('button', { name: /^Roll Reflex/ })).toHaveTextContent('+5');
    expect(within(row).getByRole('button', { name: 'Proficiency for Reflex' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(
      within(screen.getByTestId('compose-save-saves.will')).getByRole('button', {
        name: 'Proficiency for Will',
      }),
    ).toHaveAttribute('aria-pressed', 'false');
  });

  it('Happy path: the proficiency flag commits a toggle through the store', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<SaveList piece={saveList([reflex])} />);

    await user.click(screen.getByRole('button', { name: 'Proficiency for Reflex' }));

    expect(store.getPendingDelta()).toEqual({ saves: { reflexProf: 0 } });
  });

  it('Edge: an unproficient flag commits proficiency', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<SaveList piece={saveList([will])} />);

    await user.click(screen.getByRole('button', { name: 'Proficiency for Will' }));

    expect(store.getPendingDelta()).toEqual({ saves: { willProf: 1 } });
  });

  it('Edge: a negative total uses the typographic minus', () => {
    renderPreset(<SaveList piece={saveList([will])} />);

    expect(screen.getByRole('button', { name: /^Roll Will/ })).toHaveTextContent(`${MINUS}2`);
  });

  it('Edge: a missing total draws the shared copy and offers no roll', () => {
    renderPreset(<SaveList piece={saveList([reflex])} />, {
      sheet: presetSheet({ data: { saves: { reflexProf: 1, reflex: null } } }),
    });

    const row = screen.getByTestId('compose-save-saves.reflex');
    expect(row).toHaveTextContent(message('sheet.play.noValue'));
    expect(within(row).queryByRole('button', { name: /^Roll/ })).not.toBeInTheDocument();
  });

  it('Edge: no bound save draws nothing', () => {
    renderPreset(<SaveList piece={saveList([])} />);

    expect(screen.queryByTestId('compose-save-list')).not.toBeInTheDocument();
  });

  it('Happy path: the total rolls from a click and the keyboard', async () => {
    const user = userEvent.setup();
    renderPreset(<SaveList piece={saveList([reflex])} />);

    await user.click(screen.getByRole('button', { name: /^Roll Reflex/ }));
    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();

    await user.click(screen.getByRole('button', { name: /close/i }));
    screen.getByRole('button', { name: /^Roll Reflex/ }).focus();
    await user.keyboard('{Enter}');

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
  });
});

describe('the skill list piece', () => {
  it('Happy path: each bound skill draws its rank pips, total and optional tag', () => {
    renderPreset(<SkillList piece={skillList([athletics, stealth])} />);

    const row = screen.getByTestId('compose-skill-skills.athleticsRank');
    expect(within(row).getByText('Athletics')).toBeInTheDocument();
    expect(within(row).getByRole('button', { name: 'Athletics, rank 2 of 3' })).toBeInTheDocument();
    expect(within(row).getByRole('button', { name: /Athletics Total/ })).toHaveTextContent('+6');

    const stealthRow = screen.getByTestId('compose-skill-skills.stealthRank');
    expect(within(stealthRow).getByText('DEX')).toBeInTheDocument();
    expect(within(row).queryByText('DEX')).not.toBeInTheDocument();
  });

  it('Happy path: a pointer click cycles the rank and wraps at the maximum', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<SkillList piece={skillList([athletics])} />);

    await user.click(screen.getByRole('button', { name: 'Athletics, rank 2 of 3' }));
    expect(store.getPendingDelta()).toEqual({ skills: { athleticsRank: 3 } });

    await user.click(screen.getByRole('button', { name: 'Athletics, rank 3 of 3' }));
    expect(store.getPendingDelta()).toEqual({ skills: { athleticsRank: 0 } });
  });

  it('Happy path: the rank cycles from the keyboard and wraps at the maximum', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<SkillList piece={skillList([athletics])} />);

    screen.getByRole('button', { name: 'Athletics, rank 2 of 3' }).focus();
    await user.keyboard('{Enter}');
    expect(store.getPendingDelta()).toEqual({ skills: { athleticsRank: 3 } });

    screen.getByRole('button', { name: 'Athletics, rank 3 of 3' }).focus();
    await user.keyboard(' ');
    expect(store.getPendingDelta()).toEqual({ skills: { athleticsRank: 0 } });
  });

  it('Edge: a stored rank above the maximum draws full pips and wraps to zero', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<SkillList piece={skillList([athletics])} />, {
      sheet: presetSheet({ data: { skills: { athleticsRank: 9, athleticsTotal: 6 } } }),
    });

    expect(screen.getByRole('button', { name: 'Athletics, rank 3 of 3' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Athletics, rank 3 of 3' }));

    expect(store.getPendingDelta()).toEqual({ skills: { athleticsRank: 0 } });
  });

  it('Edge: a maximum leaf sets the rank ceiling', () => {
    renderPreset(<SkillList piece={skillList([athletics], 'charges.limit')} />);

    expect(screen.getByRole('button', { name: 'Athletics, rank 2 of 4' })).toBeInTheDocument();
  });

  it('Edge: a missing maximum leaf draws the row without rank control', () => {
    renderPreset(<SkillList piece={skillList([athletics], 'ghost.max')} />);

    expect(screen.queryByRole('button', { name: /Athletics, rank/ })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Athletics Total/ })).toBeInTheDocument();
  });

  it('Edge: no bound skill draws nothing', () => {
    renderPreset(<SkillList piece={skillList([])} />);

    expect(screen.queryByTestId('compose-skill-list')).not.toBeInTheDocument();
  });

  it('Happy path: the total rolls from a click and the keyboard', async () => {
    const user = userEvent.setup();
    renderPreset(<SkillList piece={skillList([athletics])} />);

    await user.click(screen.getByRole('button', { name: /Athletics Total/ }));
    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();

    await user.click(screen.getByRole('button', { name: /close/i }));
    screen.getByRole('button', { name: /Athletics Total/ }).focus();
    await user.keyboard('{Enter}');

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
  });
});

describe('configPathErrors with the score pieces', () => {
  const template = presetTemplateDetail();

  function config(pieces: readonly PresetConfig['pieces'][number][]): PresetConfig {
    return presetConfig({ pieces });
  }

  it('Happy path: the score pieces accept their bound leaves', () => {
    const valid = config([
      abilityGrid([might, agility]),
      saveList([reflex, will]),
      skillList([athletics, stealth], 'charges.limit'),
    ]);

    expect(configPathErrors(valid, template)).toEqual([]);
  });

  it('Error: an unknown score, modifier, proficiency, total or rank is reported', () => {
    const unknown = config([
      abilityGrid([{ score: 'abilities.ghost', modifier: 'abilities.mightModifier' }]),
      saveList([{ proficiency: 'saves.ghostProf', total: 'saves.reflex' }]),
      skillList([{ rank: 'skills.ghostRank', total: 'skills.athleticsTotal' }], 3),
    ]);

    expect(configPathErrors(unknown, template)).toEqual([
      'abilities.ghost',
      'saves.ghostProf',
      'skills.ghostRank',
    ]);
  });

  it('Edge: a numeric rank maximum and an ability tag bind no path', () => {
    const tagOnly = config([skillList([{ ...athletics, ability: 'DEX' }], 3)]);

    expect(configPathErrors(tagOnly, template)).toEqual([]);
  });

  it('Error: an unknown maximum leaf is reported', () => {
    expect(configPathErrors(config([skillList([athletics], 'ghost.max')]), template)).toEqual([
      'ghost.max',
    ]);
  });
});

describe('the score pieces in the config adapter', () => {
  it('Happy path: a config renders the three pieces in order through the Play view', () => {
    const config = presetConfig({
      pieces: [abilityGrid([might]), saveList([reflex]), skillList([athletics, stealth], 3)],
    });
    const preset = presetFromConfig(config);

    renderPreset(
      <>
        <preset.Header />
        <PresetPlayView preset={preset} />
      </>,
    );

    expect(screen.getByTestId('compose-ability-grid')).toBeInTheDocument();
    expect(screen.getByTestId('compose-save-list')).toBeInTheDocument();
    expect(screen.getByTestId('compose-skill-list')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Athletics, rank 2 of 3' })).toBeInTheDocument();
  });
});
