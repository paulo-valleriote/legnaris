import { describe, expect, it } from 'vitest';
import type { PresetConfig, PresetPiece } from '../sheet/preset/compose/types';
import { configPathErrors } from '../sheet/preset/compose/config_paths';
import { presetConfig, presetTemplateDetail } from './preset_fixtures';

function statPills(paths: readonly string[]): PresetPiece {
  return { kind: 'statPills', key: 'stats', pills: paths.map((path) => ({ path })) };
}

function pips(count: string, max: string | number): PresetPiece {
  return { kind: 'pips', key: 'pips', count, max };
}

function config(overrides: Partial<PresetConfig>): PresetConfig {
  return presetConfig(overrides);
}

const template = presetTemplateDetail();

describe('configPathErrors', () => {
  it('Happy path: a config bound to leaves, a Record list and a Reset action has no errors', () => {
    const valid = config({
      pieces: [statPills(['identity.name', 'inventory']), pips('charges.used', 'charges.limit')],
      resetActions: ['rest'],
    });

    expect(configPathErrors(valid, template)).toEqual([]);
  });

  it('Happy path: an empty piece list binds only the header and has no errors', () => {
    expect(configPathErrors(config({ pieces: [] }), template)).toEqual([]);
  });

  it('Edge: the same unknown path listed twice is reported once', () => {
    const unknown = config({ pieces: [statPills(['identity.ghost', 'identity.ghost'])] });

    expect(configPathErrors(unknown, template)).toEqual(['identity.ghost']);
  });

  it('Error: an unknown header subtitle is reported', () => {
    const unknown = config({ header: { name: 'identity.name', subtitle: 'identity.ghost' } });

    expect(configPathErrors(unknown, template)).toEqual(['identity.ghost']);
  });

  it('Error: an unknown leaf is reported', () => {
    const unknown = config({ pieces: [statPills(['vitals.ghost'])] });

    expect(configPathErrors(unknown, template)).toEqual(['vitals.ghost']);
  });

  it('Error: an unknown Record list is reported', () => {
    const unknown = config({ pieces: [statPills(['ghostList'])] });

    expect(configPathErrors(unknown, template)).toEqual(['ghostList']);
  });

  it('Error: an unknown Reset action is reported', () => {
    const unknown = config({ resetActions: ['ghost_action'] });

    expect(configPathErrors(unknown, template)).toEqual(['ghost_action']);
  });

  it('Error: an unknown score Row is reported', () => {
    const unknown = config({
      pieces: [
        {
          kind: 'abilityGrid',
          key: 'abilities',
          section: 'abilities',
          abilities: [
            { score: 'abilities.might', modifier: 'abilities.mightModifier', row: 'ghost' },
          ],
        },
      ],
    });

    expect(configPathErrors(unknown, template)).toEqual(['ghost']);
  });

  it('Edge: a numeric pips maximum binds no path', () => {
    const numeric = config({ pieces: [pips('charges.used', 3)] });

    expect(configPathErrors(numeric, template)).toEqual([]);
  });
});
