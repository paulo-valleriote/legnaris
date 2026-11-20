import { describe, expect, it } from 'vitest';
import { valueAtPath } from '../api/models/sheet';
import { DEMO_SHEET, DEMO_TEMPLATE } from '../landing/demo_character';
import { createDemoEditStore, deriveDemoValues } from '../landing/demo_store';
import { pendingCount } from '../sheet/sheet_edit';

function openDemo() {
  return createDemoEditStore(DEMO_TEMPLATE, DEMO_SHEET);
}

function read(store: ReturnType<typeof openDemo>, path: string): unknown {
  return valueAtPath(store.getState().data, path);
}

describe('the demo edit store', () => {
  it('Happy path: it seeds the sample character with no pending work', () => {
    const store = openDemo();

    expect(read(store, 'identity.characterName')).toBe('Rowan Vale');
    expect(read(store, 'stats.strength')).toBe(16);
    expect(read(store, 'derived.armorClass')).toBe(12);
    expect(store.getState().editedPaths.size).toBe(0);
    expect(pendingCount(store.getState())).toBe(0);
    expect(store.getState().defaults).not.toBe(store.getState().data);
  });

  it('Happy path: editing an attribute recomputes every derived value locally', () => {
    const store = openDemo();

    store.setPath('stats.strength', 18);

    expect(read(store, 'stats.strength')).toBe(18);
    expect(read(store, 'modifiers.strengthMod')).toBe(4);
    expect(read(store, 'saves.strength')).toBe(4);
    expect(read(store, 'skills.athletics.total')).toBe(5);
    expect(read(store, 'derived.armorClass')).toBe(12);
    expect(read(store, 'derived.passiveAwareness')).toBe(11);
    expect(store.getState().editedPaths).toEqual(new Set(['stats.strength']));
  });

  it('Edge: the derived writes never join the edited paths', () => {
    const store = openDemo();

    store.setPath('stats.dexterity', 18);

    expect(read(store, 'derived.armorClass')).toBe(14);
    expect(store.getState().editedPaths.has('derived.armorClass')).toBe(false);
    expect(store.getState().editedPaths.has('modifiers.dexterityMod')).toBe(false);
  });

  it('Edge: a non-editable path is refused', () => {
    const store = openDemo();

    store.setPath('derived.armorClass', 30);

    expect(read(store, 'derived.armorClass')).toBe(12);
    expect(store.getState().editedPaths.size).toBe(0);
  });

  it('Happy path: the proficiency bonus feeds the saves, the skills and the focus values', () => {
    const store = openDemo();

    store.setPath('identity.proficiencyBonus', 4);

    expect(read(store, 'saves.dexterity')).toBe(6);
    expect(read(store, 'skills.stealth.total')).toBe(7);
    expect(read(store, 'derived.passiveAwareness')).toBe(13);
    expect(read(store, 'spells.attackBonus')).toBe(5);
    expect(read(store, 'spells.saveDC')).toBe(13);
  });

  it('Happy path: the rule leaves drive the modifier step and the passive base', () => {
    const store = openDemo();

    store.setPath('rules.modifierBase', 12);
    store.setPath('rules.passiveBase', 12);

    expect(read(store, 'modifiers.strengthMod')).toBe(2);
    expect(read(store, 'derived.passiveAwareness')).toBe(12);
  });

  it('Happy path: the armor ability choice recomputes the defense', () => {
    const store = openDemo();

    store.setExclusive('config.acUsesCon', ['config.acUsesDex', 'config.acUsesCon']);

    expect(read(store, 'config.acUsesDex')).toBe(0);
    expect(read(store, 'config.acUsesCon')).toBe(1);
    expect(read(store, 'derived.armorClass')).toBe(11);
  });

  it('Edge: a proficiency toggle recomputes its skill total', () => {
    const store = openDemo();

    store.setPath('config.skillStealthProf', 0);

    expect(read(store, 'skills.stealth.total')).toBe(3);
  });

  it('Edge: a zero modifier step still yields a finite modifier', () => {
    const store = openDemo();

    store.setPath('rules.modifierStep', 0);

    expect(read(store, 'modifiers.strengthMod')).toBe(3);
    expect(Number.isFinite(read(store, 'modifiers.dexterityMod'))).toBe(true);
  });

  it('Happy path: hit points change without touching the derived values', () => {
    const store = openDemo();

    store.setPath('combat.hitPoints', 19);

    expect(read(store, 'combat.hitPoints')).toBe(19);
    expect(store.getState().editedPaths).toEqual(new Set(['combat.hitPoints']));
  });

  it('Happy path: a dice override lands in the demo block and marks dice pending', () => {
    const store = openDemo();

    store.setDice('row', 'stats.strength', '1d20 + 2');

    expect(store.getState().dice.row['stats.strength']).toBe('1d20 + 2');
    expect(store.getPendingDice()).toEqual({
      section: {},
      row: { 'stats.strength': '1d20 + 2' },
    });
  });

  it('Happy path: save commits the local baseline and clears the pending work', async () => {
    const store = openDemo();
    store.setPath('stats.strength', 18);

    await store.save();

    expect(store.getState().editedPaths.size).toBe(0);
    expect(pendingCount(store.getState())).toBe(0);
    expect(valueAtPath(store.getState().defaults, 'stats.strength')).toBe(18);
    expect(read(store, 'modifiers.strengthMod')).toBe(4);
  });

  it('Edge: the demo never throttles repeated saves', async () => {
    const store = openDemo();

    for (let hitPoints = 20; hitPoints < 26; hitPoints += 1) {
      store.setPath('combat.hitPoints', hitPoints);
      await store.save();
    }

    expect(read(store, 'combat.hitPoints')).toBe(25);
    expect(pendingCount(store.getState())).toBe(0);
  });

  it('Happy path: preview settles locally without saving or touching the values', async () => {
    const store = openDemo();
    store.setPath('stats.strength', 18);
    const before = store.getState().data;

    await store.preview();

    expect(store.getState().data).toBe(before);
    expect(store.getState().previewedRevision).toBe(store.getState().revision);
    expect(store.getState().previewError).toBeUndefined();
    expect(pendingCount(store.getState())).toBe(1);
  });

  it('Happy path: discard returns to the seed and clears the pending work', () => {
    const store = openDemo();
    store.setPath('stats.strength', 18);
    store.setDice('row', 'stats.strength', '1d20');

    store.discard();

    expect(read(store, 'stats.strength')).toBe(16);
    expect(read(store, 'modifiers.strengthMod')).toBe(3);
    expect(store.getState().editedPaths.size).toBe(0);
    expect(store.getState().dice).toEqual({ section: {}, row: {} });
    expect(pendingCount(store.getState())).toBe(0);
  });

  it('Edge: restore reads no local record and dispose leaves the state alone', async () => {
    const store = openDemo();

    await store.restore();
    store.dispose();

    expect(read(store, 'stats.strength')).toBe(16);
    expect(pendingCount(store.getState())).toBe(0);
  });

  it('Edge: a fresh store never inherits the previous one', () => {
    const first = openDemo();
    first.setPath('stats.strength', 18);

    const second = openDemo();

    expect(read(second, 'stats.strength')).toBe(16);
    expect(read(second, 'modifiers.strengthMod')).toBe(3);
  });
});

describe('the demo cascade', () => {
  it('Happy path: it derives the six modifiers and their saves from the attributes', () => {
    const derived = deriveDemoValues({
      stats: { strength: 16, dexterity: 14, constitution: 12 },
      identity: { proficiencyBonus: 2 },
      config: { saveDexterityProf: 1 },
      rules: { modifierBase: 10, modifierStep: 2 },
    });

    expect(derived.get('modifiers.strengthMod')).toBe(3);
    expect(derived.get('modifiers.dexterityMod')).toBe(2);
    expect(derived.get('modifiers.constitutionMod')).toBe(1);
    expect(derived.get('saves.dexterity')).toBe(4);
    expect(derived.get('saves.strength')).toBe(3);
  });

  it('Edge: missing rules and values fall back to neutral defaults', () => {
    const derived = deriveDemoValues({});

    expect(derived.get('modifiers.strengthMod')).toBe(0);
    expect(derived.get('derived.armorClass')).toBe(10);
    expect(derived.get('derived.passiveAwareness')).toBe(10);
  });
});
