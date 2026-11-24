import { describe, expect, it, vi } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ResetAction } from '../api/models/template';
import { parseTemplateDetail } from '../api/models/template';
import type { Sheet } from '../api/models/sheet';
import { parseSheet } from '../api/models/sheet';
import { message } from '../l10n/strings';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import { configPathErrors } from '../sheet/preset/compose/config_paths';
import { fifthCharacterConfig } from '../sheet/preset/compose/configs/fifth_character';
import { PresetPlayView } from '../sheet/preset/play_view';
import { PRESET_CONFIGS } from '../sheet/preset/registry';
import { renderPreset } from './preset_fixtures';
import goldenCharacter from '../../../testdata/templates/fifth_character_v1.json';

const TEMPLATE_ID = 'fifth_character_v1';

function goldenTemplate() {
  return parseTemplateDetail(goldenCharacter.data);
}

function fifthSheet(): Sheet {
  return parseSheet({
    id: '01a14236-21c9-7546-945e-40c46d1fcd71',
    template_id: TEMPLATE_ID,
    kind: 'character',
    name: 'Brenna',
    data: {
      identity: {
        name: 'Brenna',
        class: 'Ranger',
        subclass: 'Hunter',
        species: 'Human',
        background: 'Outlander',
        level: 5,
        proficiencyBonus: 3,
        jackOfAllTrades: 0,
      },
      abilities: {
        strength: { score: 12, mod: 1 },
        dexterity: { score: 16, mod: 3 },
        constitution: { score: 14, mod: 2 },
        intelligence: { score: 10, mod: 0 },
        wisdom: { score: 13, mod: 1 },
        charisma: { score: 8, mod: -1 },
      },
      saves: {
        strength: { prof: 1, misc: 0, total: 4 },
        dexterity: { prof: 0, misc: 0, total: 3 },
        constitution: { prof: 1, misc: 0, total: 5 },
        intelligence: { prof: 0, misc: 0, total: 0 },
        wisdom: { prof: 0, misc: 0, total: 1 },
        charisma: { prof: 0, misc: 0, total: -1 },
      },
      skills: {
        arcana: { rank: 0, misc: 0, total: -4 },
        athletics: { rank: 1, misc: 0, total: 4 },
        stealth: { rank: 2, misc: 0, total: 9 },
      },
      exhaustion: { level: 2, penalty: -4 },
      passive: { perception: 11 },
      armor: { armorClass: 15 },
      combat: {
        initiative: { alert: 0, misc: 0, total: -1 },
        speed: { base: 30, total: 20 },
        hp: { base: 36, perLevel: 8, misc: 0, max: 40, current: 28, temp: 4 },
      },
      status: {
        deathSaves: { successes: 1, failures: 0 },
        conditions: { blinded: 0, poisoned: 1 },
      },
      hitDice: {
        primary: { size: 10, count: 5, spent: 2, remaining: 3 },
        d8: { extra: 1, spent: 1, remaining: 0 },
      },
      spellcasting: {
        slot1: { total: 4, spent: 1, remaining: 3 },
        pact: { count: 2, level: 1, used: 1, remaining: 1 },
      },
      attacks: {},
      inventory: {},
      features: {},
      spells: {},
      resources: {},
    },
    dice: { section: {}, row: {} },
    created_at: '2026-11-20T10:00:00Z',
    updated_at: '2026-11-20T10:00:00Z',
  });
}

function resetBinding(
  run: (action: ResetAction, requestId: string) => Promise<Record<string, unknown>>,
) {
  return {
    run,
    onApplied: vi.fn(),
    refresh: vi.fn(async () => undefined),
    requestId: () => 'request-1',
  };
}

function renderPlay() {
  const preset = presetFromConfig(fifthCharacterConfig);
  const run = vi.fn(async () => ({}));
  const binding = resetBinding(run);
  const rendered = renderPreset(<PresetPlayView preset={preset} />, {
    template: goldenTemplate(),
    sheet: fifthSheet(),
    reset: binding,
  });
  return { ...rendered, preset, run, binding };
}

describe('the 5E Character preset config', () => {
  it('Happy path: every bound path exists in the fifth character golden', () => {
    expect(fifthCharacterConfig.templateId).toBe(TEMPLATE_ID);
    expect(configPathErrors(fifthCharacterConfig, goldenTemplate())).toEqual([]);
  });

  it('Happy path: the registry resolves the fifth character template', () => {
    expect(PRESET_CONFIGS).toContain(fifthCharacterConfig);
  });

  it('Happy path: Play draws every placed piece', () => {
    renderPlay();

    const pieces = fifthCharacterConfig.pieces;
    const pipsCount =
      pieces.filter((piece) => piece.kind === 'pips').length +
      pieces.filter((piece) => piece.kind === 'conditions' && piece.exhaustion !== undefined)
        .length;

    expect(document.querySelectorAll('.lg-compose-pips')).toHaveLength(pipsCount);
    expect(document.querySelectorAll('[data-testid="compose-record-table"]')).toHaveLength(
      pieces.filter((piece) => piece.kind === 'recordTable').length,
    );
    expect(document.querySelectorAll('[data-testid="compose-stat-pills"]')).toHaveLength(
      pieces.filter((piece) => piece.kind === 'statPills').length,
    );
    expect(screen.getByTestId('compose-hit-points')).toBeInTheDocument();
    expect(screen.getByTestId('compose-ability-grid')).toBeInTheDocument();
    expect(screen.getByTestId('compose-save-list')).toBeInTheDocument();
    expect(screen.getByTestId('compose-skill-list')).toBeInTheDocument();
    expect(screen.getByTestId('compose-conditions')).toBeInTheDocument();
    expect(screen.queryAllByTestId('compose-record-row')).toHaveLength(0);
    expect(screen.getByRole('group', { name: message('sheet.reset.group') })).toBeInTheDocument();
    expect(screen.getByTestId('compose-roll-mode')).toBeInTheDocument();
  });

  it('Happy path: the wide and narrow layouts place every section exactly once', () => {
    const preset = presetFromConfig(fifthCharacterConfig);
    const sectionKeys = new Set(preset.sections.map((section) => section.key));
    const placed = (
      blocks: readonly { sections?: readonly string[]; columns?: readonly (readonly string[])[] }[],
    ) =>
      blocks.flatMap((block) =>
        block.columns === undefined ? [...(block.sections ?? [])] : block.columns.flat(),
      );

    expect(placed(preset.layout ?? []).sort()).toEqual([...sectionKeys].sort());
    expect(placed(preset.narrowLayout ?? []).sort()).toEqual([...sectionKeys].sort());
  });

  it('Happy path: a skill roll uses the Exhaustion penalised dice', async () => {
    const user = userEvent.setup();
    renderPlay();

    await user.click(screen.getByRole('button', { name: /Arcana total/ }));

    const panel = document.querySelector('.lg-dice-panel');
    expect(panel).not.toBeNull();
    expect(panel?.textContent).toContain('exhaustion.penalty');
  });

  it('Happy path: an ability, a save and the initiative pill roll from their bound Row', async () => {
    const user = userEvent.setup();
    renderPlay();

    await user.click(screen.getByRole('button', { name: /^Roll Strength modifier/ }));
    await user.click(
      within(screen.getByTestId('compose-save-saves.dexterity.total')).getByRole('button', {
        name: /^Roll Dexterity save total/,
      }),
    );
    await user.click(screen.getByRole('button', { name: /^Roll Initiative/ }));

    const panel = document.querySelector('.lg-dice-panel')?.textContent ?? '';
    expect(panel).toContain('abilities.strength.mod');
    expect(panel).toContain('saves.dexterity.total');
    expect(panel).toContain('combat.initiative.total');
  });

  it('Happy path: a long rest goes through the existing Reset path', async () => {
    const user = userEvent.setup();
    const { run } = renderPlay();

    await user.click(screen.getByRole('button', { name: 'Long rest' }));

    expect(run).toHaveBeenCalledTimes(1);
    expect(run).toHaveBeenCalledWith(expect.objectContaining({ id: 'long_rest' }), 'request-1');
  });
});
