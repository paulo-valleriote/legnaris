import { describe, expect, it } from 'vitest';
import { valueAtPath } from '../api/models/sheet';
import { FIFTH_DEMO_SHEET, FIFTH_DEMO_TEMPLATE } from '../landing/fifth_demo_character';
import { createDemoEditStore } from '../landing/demo_store';
import { templateDerivation } from '../landing/template_derivation';
import { pendingCount } from '../sheet/sheet_edit';

function openDemo() {
  return createDemoEditStore(
    FIFTH_DEMO_TEMPLATE,
    FIFTH_DEMO_SHEET,
    templateDerivation(FIFTH_DEMO_TEMPLATE),
  );
}

function read(store: ReturnType<typeof openDemo>, path: string): unknown {
  return valueAtPath(store.getState().data, path);
}

describe('the demo edit store', () => {
  it('Happy path: it seeds the 5E character with no pending work', () => {
    const store = openDemo();

    expect(read(store, 'identity.name')).toBe('Ysolde Ashgrave');
    expect(read(store, 'abilities.wisdom.score')).toBe(16);
    expect(read(store, 'abilities.wisdom.mod')).toBe(3);
    expect(read(store, 'saves.wisdom.total')).toBe(5);
    expect(store.getState().editedPaths.size).toBe(0);
    expect(pendingCount(store.getState())).toBe(0);
    expect(store.getState().defaults).not.toBe(store.getState().data);
  });

  it('Happy path: editing an ability score recomputes every derived value locally', () => {
    const store = openDemo();

    store.setPath('abilities.wisdom.score', 18);

    expect(read(store, 'abilities.wisdom.score')).toBe(18);
    expect(read(store, 'abilities.wisdom.mod')).toBe(4);
    expect(read(store, 'saves.wisdom.total')).toBe(6);
    expect(read(store, 'spellcasting.saveDC')).toBe(14);
    expect(read(store, 'passive.perception')).toBe(14);
    expect(store.getState().editedPaths).toEqual(new Set(['abilities.wisdom.score']));
  });

  it('Edge: the derived writes never join the edited paths', () => {
    const store = openDemo();

    store.setPath('abilities.dexterity.score', 18);

    expect(read(store, 'abilities.dexterity.mod')).toBe(4);
    expect(read(store, 'combat.initiative.total')).toBe(4);
    expect(store.getState().editedPaths.has('abilities.dexterity.mod')).toBe(false);
    expect(store.getState().editedPaths.has('combat.initiative.total')).toBe(false);
  });

  it('Edge: a non-editable path is refused', () => {
    const store = openDemo();

    store.setPath('abilities.wisdom.mod', 9);

    expect(read(store, 'abilities.wisdom.mod')).toBe(3);
    expect(store.getState().editedPaths.size).toBe(0);
  });

  it('Happy path: a level change feeds the proficiency bonus and its dependents', () => {
    const store = openDemo();

    store.setPath('identity.level', 5);

    expect(read(store, 'identity.proficiencyBonus')).toBe(3);
    expect(read(store, 'saves.wisdom.total')).toBe(6);
    expect(read(store, 'skills.insight.total')).toBe(6);
    expect(read(store, 'spellcasting.attack')).toBe(6);
    expect(read(store, 'spellcasting.saveDC')).toBe(14);
  });

  it('Happy path: hit points change without touching the derived values', () => {
    const store = openDemo();

    store.setPath('combat.hp.current', 19);

    expect(read(store, 'combat.hp.current')).toBe(19);
    expect(read(store, 'abilities.wisdom.mod')).toBe(3);
    expect(store.getState().editedPaths).toEqual(new Set(['combat.hp.current']));
  });

  it('Happy path: a dice override lands in the demo block and marks dice pending', () => {
    const store = openDemo();

    store.setDice('row', 'abilities.wisdom.score', '1d20 + 2');

    expect(store.getState().dice.row['abilities.wisdom.score']).toBe('1d20 + 2');
    expect(store.getPendingDice()).toEqual({
      section: {},
      row: { 'abilities.wisdom.score': '1d20 + 2' },
    });
  });

  it('Happy path: save commits the local baseline and clears the pending work', async () => {
    const store = openDemo();
    store.setPath('abilities.wisdom.score', 18);

    await store.save();

    expect(store.getState().editedPaths.size).toBe(0);
    expect(pendingCount(store.getState())).toBe(0);
    expect(valueAtPath(store.getState().defaults, 'abilities.wisdom.score')).toBe(18);
    expect(read(store, 'abilities.wisdom.mod')).toBe(4);
  });

  it('Edge: the demo never throttles repeated saves', async () => {
    const store = openDemo();

    for (let hitPoints = 20; hitPoints < 26; hitPoints += 1) {
      store.setPath('combat.hp.current', hitPoints);
      await store.save();
    }

    expect(read(store, 'combat.hp.current')).toBe(25);
    expect(pendingCount(store.getState())).toBe(0);
  });

  it('Happy path: preview settles locally without saving or touching the values', async () => {
    const store = openDemo();
    store.setPath('abilities.wisdom.score', 18);
    const before = store.getState().data;

    await store.preview();

    expect(store.getState().data).toBe(before);
    expect(store.getState().previewedRevision).toBe(store.getState().revision);
    expect(store.getState().previewError).toBeUndefined();
    expect(pendingCount(store.getState())).toBe(1);
  });

  it('Happy path: discard returns to the seed and clears the pending work', () => {
    const store = openDemo();
    store.setPath('abilities.wisdom.score', 18);
    store.setDice('row', 'abilities.wisdom.score', '1d20');

    store.discard();

    expect(read(store, 'abilities.wisdom.score')).toBe(16);
    expect(read(store, 'abilities.wisdom.mod')).toBe(3);
    expect(store.getState().editedPaths.size).toBe(0);
    expect(store.getState().dice).toEqual({ section: {}, row: {} });
    expect(pendingCount(store.getState())).toBe(0);
  });

  it('Edge: restore reads no local record and dispose leaves the state alone', async () => {
    const store = openDemo();

    await store.restore();
    store.dispose();

    expect(read(store, 'abilities.wisdom.score')).toBe(16);
    expect(pendingCount(store.getState())).toBe(0);
  });

  it('Edge: a fresh store never inherits the previous one', () => {
    const first = openDemo();
    first.setPath('abilities.wisdom.score', 18);

    const second = openDemo();

    expect(read(second, 'abilities.wisdom.score')).toBe(16);
    expect(read(second, 'abilities.wisdom.mod')).toBe(3);
  });
});

describe('the demo derive seam', () => {
  it('Happy path: a supplied derive writes its values on the seed and every edit', () => {
    const derive = () => new Map([['abilities.wisdom.mod', 9]]);
    const store = createDemoEditStore(FIFTH_DEMO_TEMPLATE, FIFTH_DEMO_SHEET, derive);

    expect(valueAtPath(store.getState().data, 'abilities.wisdom.mod')).toBe(9);

    store.setPath('abilities.wisdom.score', 18);

    expect(valueAtPath(store.getState().data, 'abilities.wisdom.mod')).toBe(9);
    expect(store.getState().editedPaths).toEqual(new Set(['abilities.wisdom.score']));
  });

  it('Edge: an empty derivation keeps the stored saves.wisdom after an ability edit', () => {
    const store = createDemoEditStore(FIFTH_DEMO_TEMPLATE, FIFTH_DEMO_SHEET, () => new Map());

    store.setPath('abilities.wisdom.score', 18);

    expect(valueAtPath(store.getState().data, 'abilities.wisdom.score')).toBe(18);
    expect(valueAtPath(store.getState().data, 'abilities.wisdom.mod')).toBe(3);
    expect(valueAtPath(store.getState().data, 'saves.wisdom.total')).toBe(5);
    expect(store.getState().editedPaths).toEqual(new Set(['abilities.wisdom.score']));
  });

  it('Happy path: the template derivation recomputes the 5E modifiers and their dependents', () => {
    const store = openDemo();

    store.setPath('abilities.wisdom.score', 18);

    expect(valueAtPath(store.getState().data, 'abilities.wisdom.mod')).toBe(4);
    expect(valueAtPath(store.getState().data, 'saves.wisdom.total')).toBe(6);
    expect(valueAtPath(store.getState().data, 'spellcasting.saveDC')).toBe(14);
  });
});
